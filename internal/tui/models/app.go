package models

import (
	"fmt"

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
	a.models[StateLaravelCreate] = NewLaravelFormModel(a.shared)
	a.models[StateMonitoring] = NewMonitoringModel(a.shared)
	a.models[StateLogViewer] = NewLogViewerModel(a.shared)
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

// ModelInitializer interface for models that need initialization data
type ModelInitializer interface {
	Initialize(data interface{})
}

// ExecuteCommandAsync creates a command for async execution
func ExecuteCommandAsync(command, description, serviceName string) tea.Cmd {
	return func() tea.Msg {
		// This would contain the actual command execution logic
		// For now, we'll return a placeholder
		return CmdCompletedMsg{
			Result: logging.LoggedExecResult{
				Command: command,
				Output:  "Command executed successfully",
				Error:   nil,
			},
			ServiceName: serviceName,
		}
	}
}