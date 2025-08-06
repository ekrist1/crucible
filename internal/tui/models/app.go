package models

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"crucible/internal/logging"
)

// AppModel is the main application model that coordinates between sub-models
type AppModel struct {
	currentState AppState
	currentView  tea.Model
	models       map[AppState]tea.Model
	shared       *SharedData
	spinner      spinner.Model
	navigator    *Navigator
}

// Navigator handles navigation between different views
type Navigator struct {
	stack   []AppState
	current AppState
}

// NewAppModel creates a new application model
func NewAppModel(logger *logging.Logger) *AppModel {
	// Initialize spinner
	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // Bright Green

	// Create shared data
	shared := NewSharedData(logger)

	app := &AppModel{
		currentState: StateMenu,
		models:       make(map[AppState]tea.Model),
		shared:       shared,
		spinner:      s,
		navigator: &Navigator{
			stack:   []AppState{},
			current: StateMenu,
		},
	}

	// Initialize all models
	app.initializeModels()

	// Set the initial view
	app.currentView = app.models[StateMenu]

	return app
}

// initializeModels creates all the sub-models
func (a *AppModel) initializeModels() {
	a.models[StateMenu] = NewMenuModel(a.shared)
	a.models[StateProcessing] = NewProcessingModel(a.shared)
	a.models[StateServiceList] = NewServiceModel(a.shared)
	a.models[StateNextJSMenu] = NewNextJSModel(a.shared)
	a.models[StateNextJSCreate] = NewNextJSFormModel(a.shared)
	a.models[StateNextJSCreateTextInput] = NewNextJSTextInputFormModel(a.shared)
	a.models[StateNextJSCreateHybrid] = NewNextJSHybridFormModel(a.shared)
	a.models[StateLaravel] = NewLaravelModel(a.shared)
	a.models[StateLaravelCreate] = NewLaravelFormModel(a.shared)
	a.models[StateLaravelCreateTextInput] = NewLaravelTextInputFormModel(a.shared)
	a.models[StateLaravelCreateHybrid] = NewLaravelHybridFormModel(a.shared)
	a.models[StateNodeJSInstall] = NewNodeJSFormModel(a.shared)
	a.models[StateMonitoring] = NewMonitoringModel(a.shared)
	a.models[StateLogViewer] = NewLogViewerModel(a.shared)
	a.models[StateSettings] = NewSettingsModel(a.shared)
	a.models[StateMySQLBackup] = NewMySQLBackupModel(a.shared)
	a.models[StateSecurity] = NewSecurityModel(a.shared)
	a.models[StateSecurityHardening] = NewSecurityHardeningModel(a.shared)
}

// Init initializes the application model
func (a *AppModel) Init() tea.Cmd {
	return tea.Batch(
		a.spinner.Tick,
		tea.EnableBracketedPaste,
		a.currentView.Init(),
	)
}

// Update handles messages and delegates to the current view
func (a *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		}

	case NavigationMsg:
		return a.navigateTo(msg.State, msg.Data)

	case BackNavigationMsg:
		return a.navigateBack()

	case CmdCompletedMsg:
		// Handle command completion at the app level
		return a.handleCommandCompleted(msg)

	case backupCreatedMsg:
		// Handle MySQL backup request
		return a.handleBackupCreated(msg)

	case backupErrorMsg:
		// Handle MySQL backup error
		return a.handleBackupError(msg)

	case securityHardeningMsg:
		// Handle security hardening request
		return a.handleSecurityHardening(msg)

	case securityErrorMsg:
		// Handle security error
		return a.handleSecurityError(msg)

	case tea.WindowSizeMsg:
		// Update shared terminal size for all models
		a.shared.SetTerminalSize(msg.Width, msg.Height)

	default:
		// Update spinner
		a.spinner, cmd = a.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Delegate to current view
	a.currentView, cmd = a.currentView.Update(msg)
	cmds = append(cmds, cmd)

	return a, tea.Batch(cmds...)
}

// View renders the current view
func (a *AppModel) View() string {
	return a.currentView.View()
}

// navigateTo changes the current view to the specified state
func (a *AppModel) navigateTo(state AppState, data interface{}) (tea.Model, tea.Cmd) {
	// Add current state to navigation stack
	a.navigator.stack = append(a.navigator.stack, a.currentState)

	// Update state
	a.currentState = state
	a.navigator.current = state

	// Get the model for the new state
	if model, exists := a.models[state]; exists {
		a.currentView = model

		// If the model needs initialization data, handle it
		if initializer, ok := model.(ModelInitializer); ok {
			initializer.Initialize(data)
		}

		return a, tea.Batch(
			tea.ClearScreen,
			a.currentView.Init(),
		)
	}

	return a, nil
}

// navigateBack returns to the previous view
func (a *AppModel) navigateBack() (tea.Model, tea.Cmd) {
	if len(a.navigator.stack) == 0 {
		return a, nil
	}

	// Pop the last state from the stack
	prevState := a.navigator.stack[len(a.navigator.stack)-1]
	a.navigator.stack = a.navigator.stack[:len(a.navigator.stack)-1]

	// Update state
	a.currentState = prevState
	a.navigator.current = prevState

	// Set the view
	if model, exists := a.models[prevState]; exists {
		a.currentView = model
		return a, tea.Batch(
			tea.ClearScreen,
			a.currentView.Init(),
		)
	}

	return a, nil
}

// handleCommandCompleted processes command completion messages
func (a *AppModel) handleCommandCompleted(msg CmdCompletedMsg) (tea.Model, tea.Cmd) {
	queue := a.shared.CommandQueue

	// Log the command execution
	if a.shared.Logger != nil {
		a.shared.Logger.LogCommand(msg.Result)
	}

	if msg.Result.Error != nil {
		// Command failed - stop queue and show error
		queue.Reset()
		queue.AddResult(fmt.Sprintf("❌ Failed: %v", msg.Result.Error))
		a.shared.ProcessingMsg = ""

		// Navigate to processing view to show error
		return a.navigateTo(StateProcessing, nil)
	}

	// Command succeeded - add result
	queue.AddResult("✅ Command completed successfully")

	// Check if there are more commands
	if queue.HasNext() {
		// Execute next command
		command, description, _ := queue.Next()
		a.shared.ProcessingMsg = description

		return a, ExecuteCommandAsync(command, description, queue.ServiceName)
	}

	// All commands completed
	queue.IsProcessing = false
	a.shared.ProcessingMsg = ""

	// Navigate to processing view to show results
	return a.navigateTo(StateProcessing, nil)
}

// handleBackupCreated handles successful backup configuration
func (a *AppModel) handleBackupCreated(msg backupCreatedMsg) (tea.Model, tea.Cmd) {
	// Create command queue from backup commands
	a.shared.CommandQueue = &CommandQueue{
		Commands:     msg.commands,
		Descriptions: msg.descriptions,
		Index:        0,
		ServiceName:  fmt.Sprintf("MySQL Backup (%s)", msg.config.DBName),
		Results:      []string{},
		IsProcessing: true,
	}

	// Navigate to processing view to execute backup commands
	return a.navigateTo(StateProcessing, map[string]interface{}{
		"action": "mysql-backup",
		"config": msg.config,
	})
}

// handleBackupError handles backup configuration errors
func (a *AppModel) handleBackupError(msg backupErrorMsg) (tea.Model, tea.Cmd) {
	// Navigate to processing view to show error
	return a.navigateTo(StateProcessing, map[string]interface{}{
		"action": "backup-error",
		"error":  msg.err.Error(),
	})
}

// handleSecurityHardening handles security hardening requests
func (a *AppModel) handleSecurityHardening(msg securityHardeningMsg) (tea.Model, tea.Cmd) {
	// Create command queue from security commands
	a.shared.CommandQueue = &CommandQueue{
		Commands:     msg.commands,
		Descriptions: msg.descriptions,
		Index:        0,
		ServiceName:  "Security Hardening",
		Results:      []string{},
		IsProcessing: true,
	}

	// Navigate to processing view to execute security commands
	return a.navigateTo(StateProcessing, map[string]interface{}{
		"action": "security-hardening",
		"config": msg.config,
	})
}

// handleSecurityError handles security configuration errors
func (a *AppModel) handleSecurityError(msg securityErrorMsg) (tea.Model, tea.Cmd) {
	// Navigate to processing view to show error
	return a.navigateTo(StateProcessing, map[string]interface{}{
		"action": "security-error",
		"error":  msg.err.Error(),
	})
}

// ModelInitializer interface for models that need initialization data
type ModelInitializer interface {
	Initialize(data interface{})
}

// ExecuteCommandAsync creates a command for async execution with proper error handling and logging
func ExecuteCommandAsync(command, description, serviceName string) tea.Cmd {
	return func() tea.Msg {
		result := executeCommand(command, description)

		// Log the command execution result
		logCommandResult(result)

		return CmdCompletedMsg{
			Result:      result,
			ServiceName: serviceName,
		}
	}
}

// executeCommand executes a single command with proper error handling and output capture
func executeCommand(command, description string) logging.LoggedExecResult {
	startTime := time.Now()

	result := logging.LoggedExecResult{
		Command:   command,
		StartTime: startTime,
	}

	// Execute the command using bash -c for shell compatibility
	cmd := exec.Command("bash", "-c", command)

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()
	endTime := time.Now()

	result.EndTime = endTime
	result.Duration = endTime.Sub(startTime)
	result.Output = string(output)
	result.Error = err

	// Get exit code
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
		} else {
			result.ExitCode = -1 // Command couldn't be started
		}
	} else {
		result.ExitCode = 0
	}

	return result
}

// ExecuteCommandBatchAsync executes multiple commands in sequence asynchronously
func ExecuteCommandBatchAsync(commands []string, descriptions []string, serviceName string) tea.Cmd {
	return func() tea.Msg {
		if len(commands) != len(descriptions) {
			return CmdCompletedMsg{
				Result: logging.LoggedExecResult{
					Command:  "batch execution",
					Error:    fmt.Errorf("commands and descriptions length mismatch"),
					ExitCode: -1,
				},
				ServiceName: serviceName,
			}
		}

		var allOutput []string
		var firstError error
		var totalDuration time.Duration
		startTime := time.Now()

		// Execute commands in sequence
		for i, command := range commands {
			result := executeCommand(command, descriptions[i])
			allOutput = append(allOutput, fmt.Sprintf("Step %d (%s):\n%s", i+1, descriptions[i], result.Output))
			totalDuration += result.Duration

			// Stop on first error
			if result.Error != nil {
				firstError = fmt.Errorf("failed at step %d (%s): %w", i+1, descriptions[i], result.Error)
				break
			}
		}

		// Return combined result
		return CmdCompletedMsg{
			Result: logging.LoggedExecResult{
				Command:   fmt.Sprintf("batch execution (%d commands)", len(commands)),
				Output:    fmt.Sprintf("=== BATCH EXECUTION RESULTS ===\n%s\n=== END RESULTS ===", strings.Join(allOutput, "\n---\n")),
				Error:     firstError,
				StartTime: startTime,
				EndTime:   time.Now(),
				Duration:  totalDuration,
				ExitCode:  getExitCodeFromError(firstError),
			},
			ServiceName: serviceName,
		}
	}
}

// getExitCodeFromError extracts exit code from error
func getExitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		return exitError.ExitCode()
	}
	return -1
}

// logCommandResult logs the command execution result using the global logger
func logCommandResult(result logging.LoggedExecResult) {
	// Try to get a logger instance - in a real implementation this would be passed in
	// For now, create a basic logger for this execution
	logger, err := logging.NewLogger(logging.DefaultLogPath())
	if err != nil {
		// If we can't create a logger, at least print to stdout
		if result.Error != nil {
			fmt.Printf("❌ Command failed: %s (exit code: %d)\n", result.Command, result.ExitCode)
			fmt.Printf("   Error: %v\n", result.Error)
		} else {
			fmt.Printf("✅ Command succeeded: %s (duration: %s)\n", result.Command, result.Duration)
		}
		return
	}

	// Log using the proper logger
	if err := logger.LogCommand(result); err != nil {
		fmt.Printf("Warning: Failed to log command execution: %v\n", err)
	}
}
