package models

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"crucible/internal/logging"
	"crucible/internal/services"
)

// InstallationModel handles clean service installations with spinner
type InstallationModel struct {
	BaseModel
	spinner       spinner.Model
	serviceName   string
	currentStep   string
	totalSteps    int
	currentIndex  int
	isComplete    bool
	hasError      bool
	errorMessage  string
	results       []string
}

// NewInstallationModel creates a new installation model with spinner
func NewInstallationModel(shared *SharedData) *InstallationModel {
	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = selectedStyle

	return &InstallationModel{
		BaseModel:   NewBaseModel(shared),
		spinner:     s,
		isComplete:  false,
		hasError:    false,
		results:     []string{},
		currentIndex: 0,
	}
}

// SetService configures the installation for a specific service
func (m *InstallationModel) SetService(serviceName string, totalSteps int) {
	m.serviceName = serviceName
	m.totalSteps = totalSteps
	m.currentIndex = 0
	m.currentStep = "Preparing installation..."
}

// UpdateProgress updates the current installation step
func (m *InstallationModel) UpdateProgress(stepIndex int, stepDescription string) {
	m.currentIndex = stepIndex + 1
	m.currentStep = stepDescription
}

// SetError sets an error state with message
func (m *InstallationModel) SetError(errorMsg string) {
	m.hasError = true
	m.errorMessage = errorMsg
	m.isComplete = true
}

// SetComplete marks installation as complete
func (m *InstallationModel) SetComplete(results []string) {
	m.isComplete = true
	m.results = results
}

// Init initializes the installation model
func (m *InstallationModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update handles installation model updates
func (m *InstallationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc", "enter", " ":
			if m.isComplete {
				return m, m.GoBack()
			}
		}

	case CmdCompletedMsg:
		return m.handleCommandCompleted(msg)

	default:
		// Update spinner
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View renders the installation interface
func (m *InstallationModel) View() string {
	var s strings.Builder

	// Title
	if m.hasError {
		s.WriteString(titleStyle.Render(fmt.Sprintf("❌ %s Installation Failed", m.serviceName)))
	} else if m.isComplete {
		s.WriteString(titleStyle.Render(fmt.Sprintf("✅ %s Installation Complete", m.serviceName)))
	} else {
		s.WriteString(titleStyle.Render(fmt.Sprintf("🔧 Installing %s", m.serviceName)))
	}
	s.WriteString("\n\n")

	if m.hasError {
		// Show error state
		s.WriteString(errorStyle.Render("Installation failed with error:"))
		s.WriteString("\n\n")
		s.WriteString(m.errorMessage)
		s.WriteString("\n\n")
		s.WriteString(helpStyle.Render("Press Enter or Esc to go back"))

	} else if m.isComplete {
		// Show completion state
		s.WriteString(infoStyle.Render("🎉 Installation completed successfully!"))
		s.WriteString("\n\n")

		// Show summary if available
		if len(m.results) > 0 {
			s.WriteString(helpStyle.Render("Installation Summary:"))
			s.WriteString("\n")
			for _, result := range m.results {
				if strings.TrimSpace(result) != "" {
					s.WriteString(fmt.Sprintf("• %s\n", result))
				}
			}
			s.WriteString("\n")
		}

		s.WriteString(helpStyle.Render("Press Enter or Esc to continue"))

	} else {
		// Show installation progress
		progressBar := m.renderProgressBar()
		s.WriteString(progressBar)
		s.WriteString("\n\n")

		// Spinner and current step
		s.WriteString(fmt.Sprintf("%s %s", m.spinner.View(), m.currentStep))
		s.WriteString("\n\n")

		s.WriteString(helpStyle.Render("Installing... Press Ctrl+C to cancel"))
	}

	return s.String()
}

// renderProgressBar creates a visual progress bar
func (m *InstallationModel) renderProgressBar() string {
	if m.totalSteps == 0 {
		return ""
	}

	width := 40
	completed := float64(m.currentIndex) / float64(m.totalSteps)
	filledWidth := int(completed * float64(width))

	bar := strings.Builder{}
	bar.WriteString("Progress: [")

	for i := 0; i < width; i++ {
		if i < filledWidth {
			bar.WriteString("█")
		} else {
			bar.WriteString("░")
		}
	}

	bar.WriteString(fmt.Sprintf("] %d/%d", m.currentIndex, m.totalSteps))

	return infoStyle.Render(bar.String())
}

// handleCommandCompleted processes command completion with clean output
func (m *InstallationModel) handleCommandCompleted(msg CmdCompletedMsg) (tea.Model, tea.Cmd) {
	queue := m.shared.CommandQueue

	if msg.Result.Error != nil {
		// Command failed - show error
		errorMsg := fmt.Sprintf("Step %d failed: %v", m.currentIndex, msg.Result.Error)
		if strings.TrimSpace(msg.Result.Output) != "" {
			errorMsg += "\n\nOutput:\n" + msg.Result.Output
		}
		m.SetError(errorMsg)
		queue.Reset()
		return m, nil
	}

	// Command succeeded - check if there are more commands
	if queue.HasNext() {
		// Execute next command
		command, description, ok := queue.Next()
		if ok {
			m.UpdateProgress(queue.Index-1, description)
			// Execute the command asynchronously
			go m.executeCommand(command, msg.ServiceName)
		}
	} else {
		// All commands completed successfully
		queue.Reset()
		
		// Create success summary
		results := []string{
			fmt.Sprintf("%s installed successfully", m.serviceName),
			fmt.Sprintf("Completed %d installation steps", m.totalSteps),
		}

		// Add service-specific success messages
		switch strings.ToLower(m.serviceName) {
		case "php":
			results = append(results, "PHP-FPM service configured and started")
			results = append(results, "Ready for Laravel and web development")
		case "mysql":
			results = append(results, "MySQL service started and enabled")
			results = append(results, "Database server ready for applications")
		case "caddy":
			results = append(results, "Caddy web server installed and running")
			results = append(results, "Ready to serve web applications")
		case "nodejs", "node.js":
			results = append(results, "Node.js and npm installed successfully")
			results = append(results, "Ready for Next.js development")
		}

		m.SetComplete(results)
	}

	return m, nil
}

// executeCommand executes a command with clean output handling
func (m *InstallationModel) executeCommand(command, serviceName string) {
	startTime := time.Now()

	// Use the same execution pattern as processing model
	result := executeCommandForInstallation(command, startTime)

	// Handle completion
	completedMsg := CmdCompletedMsg{
		Result:      result,
		ServiceName: serviceName,
	}

	// Note: In a real implementation, this would be sent through a channel
	// For now, we handle it directly (same limitation as processing model)
	m.handleCommandCompleted(completedMsg)
}

// executeCommandForInstallation executes a command and returns clean results
func executeCommandForInstallation(command string, startTime time.Time) logging.LoggedExecResult {
	result := logging.LoggedExecResult{
		Command:   command,
		StartTime: startTime,
	}

	// Execute using shell for compatibility
	cmd := exec.Command("bash", "-c", command)
	output, err := cmd.CombinedOutput()
	endTime := time.Now()

	result.EndTime = endTime
	result.Duration = endTime.Sub(startTime)
	result.Error = err

	// For clean installation UX, we only capture error output
	if err != nil {
		result.Output = string(output) // Show full output on error
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
		} else {
			result.ExitCode = -1
		}
	} else {
		// Success - don't capture verbose output for clean UX
		result.Output = "" // Suppress success output for clean interface
		result.ExitCode = 0
	}

	return result
}

// Initialize implements the ModelInitializer interface
func (m *InstallationModel) Initialize(data interface{}) {
	if initData, ok := data.(map[string]interface{}); ok {
		// Handle service installation
		if action, exists := initData["action"].(string); exists && action == "install" {
			if serviceKey, ok := initData["service"].(string); ok {
				m.startServiceInstallation(serviceKey, initData)
			} else {
				m.SetError("No service specified for installation")
			}
		}
	}
}

// startServiceInstallation begins the installation process for a service
func (m *InstallationModel) startServiceInstallation(serviceKey string, data map[string]interface{}) {
	// Get service label for display
	labelInterface := data["label"]
	label := strings.Title(serviceKey)
	if labelInterface != nil {
		if labelStr, ok := labelInterface.(string); ok {
			label = labelStr
		}
	}
	
	// Set service name
	m.serviceName = label
	m.currentStep = "Preparing installation..."
	
	// Get installation commands based on service key
	var commands []string
	var descriptions []string
	var err error
	
	switch serviceKey {
	case "php":
		commands, descriptions, err = services.InstallPHP()
	case "composer":
		commands, descriptions, err = services.InstallComposer()
	case "python":
		commands, descriptions, err = services.InstallPython()
	case "mysql":
		// For now, install MySQL without password (interactive mode)
		commands, descriptions, err = services.InstallMySQL("")
	case "caddy":
		commands, descriptions, err = services.InstallCaddy()
	case "supervisor":
		commands, descriptions, err = services.InstallSupervisor()
	case "git":
		commands, descriptions, err = services.InstallGit()
	default:
		m.SetError(fmt.Sprintf("Unknown service: %s", serviceKey))
		return
	}
	
	if err != nil {
		m.SetError(fmt.Sprintf("Failed to prepare installation: %v", err))
		return
	}
	
	if len(commands) == 0 {
		m.SetError("No installation commands available for this service")
		return
	}
	
	// Set up the command queue
	queue := m.shared.CommandQueue
	queue.Reset()
	queue.ServiceName = serviceKey
	
	// Add commands to queue
	for i, cmd := range commands {
		desc := "Installing..."
		if i < len(descriptions) {
			desc = descriptions[i]
		}
		queue.AddCommand(cmd, desc)
	}
	
	// Set total steps and start first command
	m.SetService(label, len(commands))
	
	if queue.HasNext() {
		_, description, ok := queue.Next()
		if ok {
			m.UpdateProgress(0, description)
			// Installation will be handled by the processing model
		}
	}
}