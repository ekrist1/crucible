package models

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"crucible/internal/actions"
	"crucible/internal/logging"
)

// ProcessingModel handles command execution and result display
type ProcessingModel struct {
	BaseModel
	message      string
	report       []string
	scrollPos    int
	isComplete   bool
	canNavigate  bool
}

// NewProcessingModel creates a new processing model
func NewProcessingModel(shared *SharedData) *ProcessingModel {
	return &ProcessingModel{
		BaseModel:   NewBaseModel(shared),
		message:     "",
		report:      []string{},
		scrollPos:   0,
		isComplete:  false,
		canNavigate: true,
	}
}

// Init initializes the processing model
func (m *ProcessingModel) Init() tea.Cmd {
	// Start processing if there are commands in the queue
	if m.shared.CommandQueue != nil && m.shared.CommandQueue.HasNext() {
		return m.startProcessing()
	}
	return nil
}

// Update handles processing updates
func (m *ProcessingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.isComplete {
				return m, tea.Quit
			}
			// Don't quit while processing
			return m, nil

		case "esc", "enter", " ":
			if m.isComplete && m.canNavigate {
				return m, m.GoBack()
			}
			return m, nil

		// Scrolling controls
		case "up", "k":
			if m.scrollPos > 0 {
				m.scrollPos--
			}
		case "down", "j":
			viewableLines := m.shared.GetViewableLines()
			maxScroll := len(m.report) - viewableLines
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.scrollPos < maxScroll {
				m.scrollPos++
			}
		case "pageup":
			m.scrollPos -= 10
			if m.scrollPos < 0 {
				m.scrollPos = 0
			}
		case "pagedown":
			viewableLines := m.shared.GetViewableLines()
			maxScroll := len(m.report) - viewableLines
			if maxScroll < 0 {
				maxScroll = 0
			}
			m.scrollPos += 10
			if m.scrollPos > maxScroll {
				m.scrollPos = maxScroll
			}
		case "home":
			m.scrollPos = 0
		case "end":
			viewableLines := m.shared.GetViewableLines()
			maxScroll := len(m.report) - viewableLines
			if maxScroll < 0 {
				maxScroll = 0
			}
			m.scrollPos = maxScroll
		}

	case CmdCompletedMsg:
		// Handle command completion
		m.handleCommandCompleted(msg)
		return m, nil
	}

	return m, nil
}

// View renders the processing interface
func (m *ProcessingModel) View() string {
	var s strings.Builder

	// Title
	s.WriteString(titleStyle.Render("🔄 Processing"))
	s.WriteString("\n\n")

	// Current processing message
	if m.message != "" && !m.isComplete {
		s.WriteString(infoStyle.Render(fmt.Sprintf("⏳ %s", m.message)))
		s.WriteString("\n\n")
	}

	// Report content (with scrolling)
	if len(m.report) > 0 {
		startLine := m.scrollPos
		viewableLines := m.shared.GetViewableLines()
		endLine := startLine + viewableLines
		if endLine > len(m.report) {
			endLine = len(m.report)
		}

		for i := startLine; i < endLine; i++ {
			s.WriteString(m.report[i])
			s.WriteString("\n")
		}

		// Scroll indicator
		if len(m.report) > viewableLines {
			totalLines := len(m.report)
			s.WriteString("\n")
			scrollInfo := fmt.Sprintf("Showing lines %d-%d of %d", 
				startLine+1, endLine, totalLines)
			s.WriteString(helpStyle.Render(scrollInfo))
		}
	}

	// Help text
	s.WriteString("\n")
	if m.isComplete {
		if m.canNavigate {
			s.WriteString(helpStyle.Render("Press Enter, Space, or Esc to continue"))
		} else {
			s.WriteString(helpStyle.Render("Process completed"))
		}
	} else {
		s.WriteString(helpStyle.Render("Processing... Press Ctrl+C to quit"))
	}

	// Scrolling help
	if len(m.report) > m.shared.GetViewableLines() {
		s.WriteString("\n")
		s.WriteString(helpStyle.Render("↑/↓=Scroll, PageUp/PageDown=Fast scroll, Home/End=Jump"))
	}

	return s.String()
}

// handleCommandCompleted processes command completion
func (m *ProcessingModel) handleCommandCompleted(msg CmdCompletedMsg) {
	queue := m.shared.CommandQueue

	if msg.Result.Error != nil {
		// Command failed
		queue.Reset()
		m.report = append(m.report, errorStyle.Render(fmt.Sprintf("❌ Failed: %v", msg.Result.Error)))
		if strings.TrimSpace(msg.Result.Output) != "" {
			m.report = append(m.report, fmt.Sprintf("Output: %s", msg.Result.Output))
		}
		m.message = ""
		m.isComplete = true
		return
	}

	// Command succeeded
	m.report = append(m.report, infoStyle.Render("✅ Command completed successfully"))
	if strings.TrimSpace(msg.Result.Output) != "" {
		m.report = append(m.report, msg.Result.Output)
	}

	// Check if there are more commands in queue
	if queue.HasNext() {
		// Execute next command
		command, description, ok := queue.Next()
		if ok {
			m.message = description
			// Execute the command asynchronously
			go m.executeCommand(command, msg.ServiceName)
		}
	} else {
		// All commands completed
		queue.Reset()
		m.message = ""
		m.isComplete = true
		m.report = append(m.report, infoStyle.Render("✅ All operations completed successfully"))
	}
}

// SetMessage sets the current processing message
func (m *ProcessingModel) SetMessage(message string) {
	m.message = message
	m.isComplete = false
}

// AddReportLine adds a line to the report
func (m *ProcessingModel) AddReportLine(line string) {
	m.report = append(m.report, line)
}

// SetReport sets the entire report
func (m *ProcessingModel) SetReport(report []string) {
	m.report = report
}

// SetComplete marks processing as complete
func (m *ProcessingModel) SetComplete(canNavigate bool) {
	m.isComplete = true
	m.canNavigate = canNavigate
	m.message = ""
}

// Initialize implements the ModelInitializer interface
func (m *ProcessingModel) Initialize(data interface{}) {
	if initData, ok := data.(map[string]interface{}); ok {
		// Handle specific actions
		if action, exists := initData["action"].(string); exists {
			m.handleAction(action, initData)
			return
		}
		
		// Handle legacy initialization data
		if message, exists := initData["message"].(string); exists {
			m.SetMessage(message)
		}
		if report, exists := initData["report"].([]string); exists {
			m.SetReport(report)
		}
		if complete, exists := initData["complete"].(bool); exists {
			m.SetComplete(complete)
		}
	}
}

// handleAction handles different processing actions
func (m *ProcessingModel) handleAction(action string, data map[string]interface{}) {
	switch action {
	case "system-status":
		m.handleSystemStatus()
	case "laravel-list":
		m.handleLaravelList()
	case "laravel-queue":
		m.handleLaravelQueue()
	case "github-auth":
		m.handleGitHubAuth()
	case "service-status":
		if serviceName, ok := data["service"].(string); ok {
			m.handleServiceStatus(serviceName)
		}
	case "service-control":
		if serviceName, ok := data["service"].(string); ok {
			if serviceAction, ok := data["serviceAction"].(string); ok {
				m.handleServiceControl(serviceName, serviceAction)
			}
		}
	default:
		m.SetMessage(fmt.Sprintf("Unknown action: %s", action))
		m.SetComplete(true)
	}
}

// executeCommand executes a shell command asynchronously
func (m *ProcessingModel) executeCommand(command, serviceName string) {
	startTime := time.Now()
	
	// Parse the command to separate command and arguments
	parts := strings.Fields(command)
	if len(parts) == 0 {
		// Empty command, send completion message with error
		result := logging.LoggedExecResult{
			Command:   command,
			Output:    "",
			Error:     fmt.Errorf("empty command"),
			ExitCode:  1,
			StartTime: startTime,
		}
		
		// Handle the error result
		completedMsg := CmdCompletedMsg{
			Result:      result,
			ServiceName: serviceName,
		}
		m.handleCommandCompleted(completedMsg)
		return
	}
	
	var cmd *exec.Cmd
	if len(parts) == 1 {
		cmd = exec.Command(parts[0])
	} else {
		cmd = exec.Command(parts[0], parts[1:]...)
	}
	
	// Execute the command
	output, err := cmd.CombinedOutput()
	
	// Get exit code safely
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = 1 // Default error code
		}
	} else if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	
	// Create the result
	result := logging.LoggedExecResult{
		Command:   command,
		Output:    string(output),
		Error:     err,
		ExitCode:  exitCode,
		StartTime: startTime,
	}
	
	// Log the command execution
	if m.shared.Logger != nil {
		m.shared.Logger.LogCommand(result)
	}
	
	// Send completion message - this is the tricky part
	// We need to send this back to the Bubble Tea program
	// In a real implementation, we'd use a channel or other mechanism
	// For now, we'll simulate this by directly calling the handler
	// This is a limitation of the current architecture
	completedMsg := CmdCompletedMsg{
		Result:      result,
		ServiceName: serviceName,
	}
	
	// In a proper implementation, we'd send this through a channel
	// that the main Update loop monitors, but for now we'll handle it directly
	// This is a limitation of the current architecture
	m.handleCommandCompleted(completedMsg)
}

// startProcessing begins processing the command queue
func (m *ProcessingModel) startProcessing() tea.Cmd {
	queue := m.shared.CommandQueue
	if queue == nil || !queue.HasNext() {
		return nil
	}
	
	// Mark as processing
	queue.IsProcessing = true
	m.isComplete = false
	
	// Get the first command
	command, description, ok := queue.Next()
	if !ok {
		return nil
	}
	
	// Set the current message
	m.message = description
	
	// Start executing the command asynchronously
	go m.executeCommand(command, queue.ServiceName)
	
	return nil
}

// handleSystemStatus displays system status information
func (m *ProcessingModel) handleSystemStatus() {
	m.message = "Generating system status report..."
	m.isComplete = false
	
	// Create a comprehensive system status report
	report := []string{
		titleStyle.Render("=== SYSTEM STATUS REPORT ==="),
		"",
		infoStyle.Render("📊 System Information:"),
	}
	
	// Get system information
	if output, err := m.executeCommandSync("uname", "-a"); err == nil {
		report = append(report, fmt.Sprintf("System: %s", strings.TrimSpace(output)))
	}
	
	if output, err := m.executeCommandSync("uptime"); err == nil {
		report = append(report, fmt.Sprintf("Uptime: %s", strings.TrimSpace(output)))
	}
	
	if output, err := m.executeCommandSync("df", "-h", "/"); err == nil {
		lines := strings.Split(output, "\n")
		if len(lines) > 1 {
			report = append(report, fmt.Sprintf("Disk Usage: %s", strings.TrimSpace(lines[1])))
		}
	}
	
	if output, err := m.executeCommandSync("free", "-h"); err == nil {
		lines := strings.Split(output, "\n")
		if len(lines) > 1 {
			report = append(report, fmt.Sprintf("Memory: %s", strings.TrimSpace(lines[1])))
		}
	}
	
	report = append(report, "", infoStyle.Render("🔧 Service Status:"))
	
	// Check common services
	services := []string{"mysql", "mysqld", "caddy", "nginx", "php-fpm", "supervisor"}
	for _, service := range services {
		if output, err := m.executeCommandSync("systemctl", "is-active", service); err == nil {
			status := strings.TrimSpace(output)
			icon := "🔴"
			if status == "active" {
				icon = "🟢"
			}
			report = append(report, fmt.Sprintf("%s %s: %s", icon, service, status))
		}
	}
	
	report = append(report, "", infoStyle.Render("💻 Software Versions:"))
	
	// Check software versions
	softwareChecks := map[string][]string{
		"PHP":     {"php", "--version"},
		"Node.js": {"node", "--version"},
		"Git":     {"git", "--version"},
		"Composer": {"composer", "--version"},
		"Python":  {"python3", "--version"},
	}
	
	for name, cmd := range softwareChecks {
		if output, err := m.executeCommandSync(cmd[0], cmd[1:]...); err == nil {
			lines := strings.Split(output, "\n")
			if len(lines) > 0 {
				version := strings.TrimSpace(lines[0])
				if len(version) > 60 {
					version = version[:60] + "..."
				}
				report = append(report, fmt.Sprintf("✅ %s: %s", name, version))
			}
		} else {
			report = append(report, fmt.Sprintf("❌ %s: Not installed", name))
		}
	}
	
	m.SetReport(report)
	m.SetComplete(true)
}

// handleLaravelList shows Laravel sites for update selection
func (m *ProcessingModel) handleLaravelList() {
	m.message = "Loading Laravel sites..."
	m.isComplete = false
	
	// Import the actions package functionality
	sites, err := actions.ListLaravelSites()
	if err != nil {
		m.SetReport([]string{
			errorStyle.Render("❌ Error loading Laravel sites:"),
			err.Error(),
		})
		m.SetComplete(true)
		return
	}
	
	if len(sites) == 0 {
		m.SetReport([]string{
			warnStyle.Render("⚠️ No Laravel sites found"),
			"",
			helpStyle.Render("Create a Laravel site first from the Laravel Management menu."),
		})
		m.SetComplete(true)
		return
	}
	
	report := []string{
		titleStyle.Render("🚀 Laravel Sites Available for Update"),
		"",
		infoStyle.Render("Available Laravel installations:"),
		"",
	}
	
	for i, site := range sites {
		statusIcon := "🔴"
		if isRunning, err := actions.GetLaravelSiteStatus(site); err == nil && isRunning {
			statusIcon = "🟢"
		}
		report = append(report, fmt.Sprintf("%s [%d] %s (/var/www/%s)", statusIcon, i+1, site, site))
	}
	
	report = append(report, 
		"",
		helpStyle.Render("TODO: Site selection and update functionality will be implemented."),
		helpStyle.Render("This would normally allow you to select a site to update."),
	)
	
	m.SetReport(report)
	m.SetComplete(true)
}

// handleLaravelQueue shows Laravel queue worker setup
func (m *ProcessingModel) handleLaravelQueue() {
	m.SetReport([]string{
		titleStyle.Render("🔄 Laravel Queue Worker Setup"),
		"",
		helpStyle.Render("Laravel Queue Worker setup will be implemented here."),
		helpStyle.Render("This would configure Supervisor to manage Laravel queue workers."),
	})
	m.SetComplete(true)
}

// handleGitHubAuth shows GitHub authentication setup
func (m *ProcessingModel) handleGitHubAuth() {
	m.SetReport([]string{
		titleStyle.Render("🔐 GitHub Authentication Setup"),
		"",
		helpStyle.Render("GitHub authentication setup will be implemented here."),
		helpStyle.Render("This would configure SSH keys and GitHub CLI authentication."),
	})
	m.SetComplete(true)
}

// executeCommandSync executes a command synchronously and returns output
func (m *ProcessingModel) executeCommandSync(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	output, err := cmd.Output()
	return string(output), err
}

// handleServiceStatus displays detailed status for a service
func (m *ProcessingModel) handleServiceStatus(serviceName string) {
	m.message = fmt.Sprintf("Getting detailed status for %s...", serviceName)
	m.isComplete = false
	
	commands, _ := actions.GetServiceStatus(serviceName)
	if len(commands) == 0 {
		m.SetReport([]string{
			errorStyle.Render("❌ No status command available"),
		})
		m.SetComplete(true)
		return
	}
	
	report := []string{
		titleStyle.Render(fmt.Sprintf("🔧 Service Status: %s", serviceName)),
		"",
	}
	
	// Execute the status command
	if output, err := m.executeCommandSync("systemctl", "status", serviceName, "--no-pager", "--lines=15"); err == nil {
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				report = append(report, line)
			}
		}
	} else {
		report = append(report, errorStyle.Render(fmt.Sprintf("❌ Error getting status: %v", err)))
	}
	
	m.SetReport(report)
	m.SetComplete(true)
}

// handleServiceControl executes a control action on a service
func (m *ProcessingModel) handleServiceControl(serviceName, serviceAction string) {
	m.message = fmt.Sprintf("Performing %s on %s...", serviceAction, serviceName)
	m.isComplete = false
	
	config := actions.ServiceActionConfig{
		ServiceName: serviceName,
		Action:      serviceAction,
	}
	
	commands, descriptions, err := actions.ControlService(config)
	if err != nil {
		m.SetReport([]string{
			errorStyle.Render(fmt.Sprintf("❌ Invalid action: %v", err)),
		})
		m.SetComplete(true)
		return
	}
	
	report := []string{
		titleStyle.Render(fmt.Sprintf("🔧 Service Control: %s %s", serviceAction, serviceName)),
		"",
	}
	
	// Execute each command
	for i, command := range commands {
		if i < len(descriptions) {
			report = append(report, infoStyle.Render(descriptions[i]))
		}
		
		// Parse command into parts
		parts := strings.Fields(command)
		if len(parts) == 0 {
			continue
		}
		
		var cmd *exec.Cmd
		if len(parts) == 1 {
			cmd = exec.Command(parts[0])
		} else {
			cmd = exec.Command(parts[0], parts[1:]...)
		}
		
		output, err := cmd.CombinedOutput()
		if err != nil {
			report = append(report, errorStyle.Render(fmt.Sprintf("❌ Command failed: %v", err)))
			if len(output) > 0 {
				report = append(report, fmt.Sprintf("Output: %s", string(output)))
			}
		} else {
			report = append(report, infoStyle.Render("✅ Command completed successfully"))
			if len(output) > 0 && strings.TrimSpace(string(output)) != "" {
				report = append(report, fmt.Sprintf("Output: %s", string(output)))
			}
		}
		report = append(report, "")
	}
	
	m.SetReport(report)
	m.SetComplete(true)
}