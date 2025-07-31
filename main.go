package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

type appState int

const (
	stateMenu appState = iota
	stateInput
	stateProcessing
	stateLogViewer
)

// Message types for async command execution
type cmdExecutionMsg struct {
	command     string
	description string
	serviceName string
}

type cmdCompletedMsg struct {
	result      LoggedExecResult
	serviceName string
}

type cmdQueueMsg struct {
	commands     []string
	descriptions []string
	serviceName  string
	currentIndex int
}

type model struct {
	choices       []string
	cursor        int
	selected      map[int]struct{}
	logger        *log.Logger
	state         appState
	inputPrompt   string
	inputValue    string
	inputField    string
	formData      map[string]string
	currentAction int
	serviceStatus map[string]bool // Track installation status of services
	spinner       spinner.Model   // Spinner for long-running tasks
	processingMsg string          // Message to display during processing
	report        []string        // System status report lines
	// Log viewer state
	logLines  []string // All log lines
	logScroll int      // Current scroll position
	// Command queue state
	commandQueue     []string // Commands to execute in sequence
	descriptionQueue []string // Descriptions for each command
	queueIndex       int      // Current command index
	queueServiceName string   // Service name for queue
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")). // White
			Background(lipgloss.Color("5")).  // Magenta
			Padding(0, 1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")). // Bright Green
			Bold(true)

	choiceStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")) // Gray

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14")). // Cyan
			Background(lipgloss.Color("0")).  // Black
			Padding(0, 1)

	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")). // Yellow
			Bold(true)

	infoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // Bright Green for info/success
	warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // Yellow for warnings
)

func initialModel() model {
	logger := log.NewWithOptions(os.Stdout, log.Options{
		ReportCaller:    false,
		ReportTimestamp: true,
		Prefix:          "Crucible 🔧",
	})

	// Initialize spinner
	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // Bright Green

	m := model{
		choices: []string{
			"Install PHP 8.4",
			"Upgrade to PHP 8.5 (Alpha - unavailable)",
			"Install PHP Composer",
			"Install Python & pip",
			"Install MySQL",
			"Install Caddy Server",
			"Install Git CLI",
			"Install Supervisor",
			"Create New Laravel Site",
			"Update Laravel Site",
			"Setup Laravel Queue Worker",
			"Backup MySQL Database",
			"System Status",
			"View Installation Logs",
			"Exit",
		},
		selected:      make(map[int]struct{}),
		logger:        logger,
		state:         stateMenu,
		formData:      make(map[string]string),
		serviceStatus: make(map[string]bool),
		spinner:       s,
		processingMsg: "",
		report:        []string{},
	}

	// Check initial service installation status
	m.checkServiceInstallations()

	// Initialize logging
	modelPtr := &m
	if err := modelPtr.initializeLogging(); err != nil {
		logger.Error("Failed to initialize logging", "error", err)
	}

	return m
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateMenu:
		return m.updateMenu(msg)
	case stateInput:
		return m.updateInput(msg)
	case stateProcessing:
		return m.updateProcessing(msg)
	case stateLogViewer:
		return m.updateLogViewer(msg)
	}
	return m, nil
}

func (m model) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}

		case "enter", " ":
			if m.cursor == len(m.choices)-1 {
				return m, tea.Quit
			}

			// Handle the selected option
			return m.handleSelection()

		case "r", "R":
			// Refresh service installation status
			modelPtr := &m
			modelPtr.checkServiceInstallations()
			return *modelPtr, nil
		}
	}

	return m, nil
}

func (m model) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			// Clear screen and cancel input, return to menu
			m.state = stateMenu
			m.inputValue = ""
			m.inputPrompt = ""
			m.cursor = 0 // Reset cursor when canceling input
			return m, tea.ClearScreen
		case "enter":
			// Save input and proceed
			m.formData[m.inputField] = m.inputValue
			return m.processFormInput()
		case "backspace":
			if len(m.inputValue) > 0 {
				m.inputValue = m.inputValue[:len(m.inputValue)-1]
			}
		default:
			// Add character to input
			m.inputValue += msg.String()
		}
	}

	return m, nil
}

func (m model) updateProcessing(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "enter", " ":
			// Only allow exit if processing is complete (no processingMsg)
			if m.processingMsg == "" {
				// Return to main menu after processing and refresh service status
				m.state = stateMenu
				m.formData = make(map[string]string)
				m.report = []string{} // Clear report
				m.cursor = 0          // Reset cursor to top of menu
				modelPtr := &m
				modelPtr.checkServiceInstallations() // Refresh all service statuses
				return *modelPtr, tea.ClearScreen
			}
		}
	case cmdCompletedMsg:
		// Command execution completed
		modelPtr := &m

		// Log the command execution
		if logErr := modelPtr.logCommand(msg.result); logErr != nil {
			m.logger.Error("Failed to log command", "error", logErr)
		}

		if msg.result.Error != nil {
			// Command failed - stop queue and show error
			m.processingMsg = ""
			m.commandQueue = []string{}
			m.descriptionQueue = []string{}
			m.queueIndex = 0
			m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed: %v", msg.result.Error)))
			if strings.TrimSpace(msg.result.Output) != "" {
				m.report = append(m.report, warnStyle.Render(fmt.Sprintf("Output: %s", msg.result.Output)))
			}
			return *modelPtr, nil
		}

		// Command succeeded - check if there are more commands in queue
		if len(m.commandQueue) > 0 && m.queueIndex < len(m.commandQueue)-1 {
			// Execute next command in queue
			m.queueIndex++
			m.processingMsg = m.descriptionQueue[m.queueIndex]
			m.report = append(m.report, infoStyle.Render(fmt.Sprintf("✅ %s", m.descriptionQueue[m.queueIndex-1])))
			return *modelPtr, tea.Batch(
				m.spinner.Tick,
				executeCommandAsync(m.commandQueue[m.queueIndex], m.descriptionQueue[m.queueIndex], m.queueServiceName),
			)
		}

		// All commands completed successfully
		m.processingMsg = ""
		m.report = append(m.report, infoStyle.Render("✅ All operations completed successfully"))
		if msg.serviceName != "" || m.queueServiceName != "" {
			serviceName := msg.serviceName
			if serviceName == "" {
				serviceName = m.queueServiceName
			}

			// Handle service-specific post-installation setup
			if serviceName == "caddy" {
				modelPtr.setupCaddyLaravelConfig()
			}

			modelPtr.refreshServiceStatus(serviceName)
		}

		// Clear queue
		m.commandQueue = []string{}
		m.descriptionQueue = []string{}
		m.queueIndex = 0
		m.queueServiceName = ""

		return *modelPtr, nil
	default:
		// Update spinner
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) handleSelection() (tea.Model, tea.Cmd) {
	choice := m.choices[m.cursor]
	m.logger.Info("Selected option", "choice", choice)

	switch m.cursor {
	case 0: // Install PHP 8.4
		newModel, cmd := m.installPHP()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	case 1: // Upgrade to PHP 8.5
		//newModel, cmd := m.upgradeToPHP85()
		//return newModel, tea.Batch(tea.ClearScreen, cmd)
		return m, nil
	case 2: // Install PHP Composer
		newModel, cmd := m.installComposer()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	case 3: // Install Python & pip
		newModel, cmd := m.installPython()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	case 4: // Install MySQL
		newModel, cmd := m.installMySQL()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	case 5: // Install Caddy Server
		newModel, cmd := m.installCaddy()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	case 6: // Install Git CLI
		newModel, cmd := m.installGit()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	case 7: // Install Supervisor
		newModel, cmd := m.installSupervisor()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	case 8: // Create New Laravel Site
		return m.startInput("Enter site name (e.g., myapp):", "siteName", 8)
	case 9: // Update Laravel Site
		return m.startInput("Select site number:", "siteIndex", 9)
	case 10: // Setup Laravel Queue Worker
		return m.startInput("Enter Laravel site name:", "queueSiteName", 10)
	case 11: // Backup MySQL Database
		return m.startInput("Enter database name:", "dbName", 11)
	case 12: // System Status
		newModel, cmd := m.showSystemStatus()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	case 13: // View Installation Logs
		newModel, cmd := m.showInstallationLogs()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	}

	return m, nil
}

func (m model) processFormInput() (tea.Model, tea.Cmd) {
	// This function handles the form input flow for different actions
	switch m.currentAction {
	case 8: // Create New Laravel Site
		return m.handleLaravelSiteForm()
	case 9: // Update Laravel Site
		return m.handleUpdateSiteForm()
	case 10: // Setup Laravel Queue Worker
		return m.handleQueueWorkerForm()
	case 11: // Backup MySQL Database
		return m.handleBackupForm()
	}

	// Default: return to menu
	m.state = stateMenu
	m.cursor = 0 // Reset cursor when returning to menu
	return m, nil
}

func (m model) startInput(prompt, field string, action int) (tea.Model, tea.Cmd) {
	m.state = stateInput
	m.inputPrompt = prompt
	m.inputField = field
	m.inputValue = ""
	m.currentAction = action
	m.cursor = 0 // Reset cursor when starting input
	return m, tea.ClearScreen
}

func (m model) startProcessingWithMessage(message string) (tea.Model, tea.Cmd) {
	m.state = stateProcessing
	m.processingMsg = message
	return m, tea.Batch(tea.ClearScreen, m.spinner.Tick)
}

func (m *model) checkServiceInstallations() {
	// Check PHP installation
	m.serviceStatus["php"] = m.isServiceInstalled("php", "--version")

	// Check Composer installation
	m.serviceStatus["composer"] = m.isServiceInstalled("composer", "--version")

	// Check Python installation
	m.serviceStatus["python"] = m.isServiceInstalled("python3", "--version")

	// Check MySQL installation
	m.serviceStatus["mysql"] = m.isServiceInstalled("mysql", "--version")

	// Check Caddy installation
	m.serviceStatus["caddy"] = m.isServiceInstalled("caddy", "version")

	// Check Git installation
	m.serviceStatus["git"] = m.isServiceInstalled("git", "--version")

	// Check Supervisor installation
	m.serviceStatus["supervisor"] = m.isServiceInstalled("supervisorctl", "version")
}

func (m model) isServiceInstalled(command string, args ...string) bool {
	cmd := exec.Command(command, args...)
	err := cmd.Run()
	return err == nil
}

func (m *model) refreshServiceStatus(serviceName string) {
	switch serviceName {
	case "php":
		m.serviceStatus["php"] = m.isServiceInstalled("php", "--version")
	case "composer":
		m.serviceStatus["composer"] = m.isServiceInstalled("composer", "--version")
	case "python":
		m.serviceStatus["python"] = m.isServiceInstalled("python3", "--version")
	case "mysql":
		m.serviceStatus["mysql"] = m.isServiceInstalled("mysql", "--version")
	case "caddy":
		m.serviceStatus["caddy"] = m.isServiceInstalled("caddy", "version")
	case "git":
		m.serviceStatus["git"] = m.isServiceInstalled("git", "--version")
	}
}

func (m model) getServiceIcon(serviceName string) string {
	if m.serviceStatus[serviceName] {
		return "✅"
	}
	return "⬜"
}

func (m model) View() string {
	switch m.state {
	case stateMenu:
		return m.viewMenu()
	case stateInput:
		return m.viewInput()
	case stateProcessing:
		return m.viewProcessing()
	case stateLogViewer:
		return m.viewLogViewer()
	}
	return ""
}

func (m model) viewMenu() string {
	s := titleStyle.Render("🔧 Crucible - Server Setup made easy for Laravel and Python") + "\n\n"

	for i, choice := range m.choices {
		cursor := " "
		serviceIcon := ""

		// Add service status icons for installation options
		switch i {
		case 0: // Install PHP 8.4
			serviceIcon = m.getServiceIcon("php") + " "
		case 1: // Upgrade to PHP 8.5
			serviceIcon = m.getServiceIcon("php") + " "
		case 2: // Install PHP Composer
			serviceIcon = m.getServiceIcon("composer") + " "
		case 3: // Install Python & pip
			serviceIcon = m.getServiceIcon("python") + " "
		case 4: // Install MySQL
			serviceIcon = m.getServiceIcon("mysql") + " "
		case 5: // Install Caddy Server
			serviceIcon = m.getServiceIcon("caddy") + " "
		case 6: // Install Git CLI
			serviceIcon = m.getServiceIcon("git") + " "
		case 7: // Install Supervisor
			serviceIcon = m.getServiceIcon("supervisor") + " "
		}

		if m.cursor == i {
			cursor = ">"
			if i == 1 { // Style for disabled selected option
				choice = choiceStyle.Render(serviceIcon + choice)
			} else {
				choice = selectedStyle.Render(serviceIcon + choice)
			}
		} else {
			if i == 1 { // Style for disabled non-selected option
				choice = choiceStyle.Render(serviceIcon + choice)
			} else {
				choice = choiceStyle.Render(serviceIcon + choice)
			}
		}

		s += fmt.Sprintf("%s %s\n", cursor, choice)
	}

	s += "\nPress q to quit, r to refresh service status.\n"
	s += "\n✅ = Installed  ⬜ = Not installed\n"
	return s
}

func (m model) viewInput() string {
	s := titleStyle.Render("🔧 Crucible - Laravel Server Setup") + "\n\n"
	s += promptStyle.Render(m.inputPrompt) + "\n\n"
	s += inputStyle.Render(m.inputValue+"│") + "\n\n"
	s += "Press Enter to continue, Esc to cancel\n"
	return s
}

func (m model) viewProcessing() string {
	s := titleStyle.Render("🔧 Crucible - Laravel Server Setup") + "\n\n"
	if m.processingMsg != "" {
		s += fmt.Sprintf("%s %s\n\n", m.spinner.View(), m.processingMsg)
		s += "Please wait...\n"
	} else {
		if len(m.report) > 0 {
			s += strings.Join(m.report, "\n") + "\n\n"
		}
		s += "Processing completed!\n"
		s += "Press any key to return to main menu.\n"
	}
	return s
}

func (m model) updateLogViewer(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			// Return to main menu
			m.state = stateMenu
			m.cursor = 0
			m.logLines = []string{}
			m.logScroll = 0
			return m, tea.ClearScreen
		case "up", "k":
			// Scroll up
			if m.logScroll > 0 {
				m.logScroll--
			}
		case "down", "j":
			// Scroll down
			logViewHeight := 18
			maxScroll := len(m.logLines) - logViewHeight
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.logScroll < maxScroll {
				m.logScroll++
			}
		case "home", "g":
			// Go to top
			m.logScroll = 0
		case "end", "G":
			// Go to bottom
			logViewHeight := 18
			maxScroll := len(m.logLines) - logViewHeight
			if maxScroll < 0 {
				maxScroll = 0
			}
			m.logScroll = maxScroll
		case "pageup":
			// Page up
			logViewHeight := 18
			m.logScroll -= logViewHeight
			if m.logScroll < 0 {
				m.logScroll = 0
			}
		case "pagedown":
			// Page down
			logViewHeight := 18
			maxScroll := len(m.logLines) - logViewHeight
			if maxScroll < 0 {
				maxScroll = 0
			}
			m.logScroll += logViewHeight
			if m.logScroll > maxScroll {
				m.logScroll = maxScroll
			}
		}
	}
	return m, nil
}

func (m model) viewLogViewer() string {
	s := titleStyle.Render("🔧 Crucible - Installation Logs") + "\n\n"

	// Calculate view height (assuming terminal height of about 24 lines, minus header and footer)
	logViewHeight := 18

	if len(m.logLines) == 0 {
		s += "No log lines to display.\n\n"
	} else {
		// Calculate visible range
		startIdx := m.logScroll
		endIdx := startIdx + logViewHeight

		if endIdx > len(m.logLines) {
			endIdx = len(m.logLines)
		}

		// Show log lines with line numbers
		for i := startIdx; i < endIdx; i++ {
			line := m.logLines[i]
			lineNum := fmt.Sprintf("%4d: ", i+1)

			// Style different types of log lines
			if strings.Contains(line, "COMMAND:") {
				s += infoStyle.Render(lineNum) + infoStyle.Render(line) + "\n"
			} else if strings.Contains(line, "ERROR:") || strings.Contains(line, "EXIT CODE:") {
				s += warnStyle.Render(lineNum) + warnStyle.Render(line) + "\n"
			} else if strings.Contains(line, "STATUS: SUCCESS") {
				s += infoStyle.Render(lineNum) + infoStyle.Render(line) + "\n"
			} else {
				s += choiceStyle.Render(lineNum) + line + "\n"
			}
		}

		s += "\n"

		// Show scroll position info
		totalLines := len(m.logLines)
		visibleStart := m.logScroll + 1
		visibleEnd := m.logScroll + (endIdx - startIdx)

		s += choiceStyle.Render(fmt.Sprintf("Lines %d-%d of %d", visibleStart, visibleEnd, totalLines)) + "\n"
	}

	s += "\nNavigation: ↑/↓ scroll, Home/End jump, PgUp/PgDn page, q/Esc to exit\n"
	return s
}

// startCommandQueue starts executing a queue of commands sequentially
func (m model) startCommandQueue(commands, descriptions []string, serviceName string) (tea.Model, tea.Cmd) {
	if len(commands) == 0 || len(descriptions) == 0 {
		return m, nil
	}

	m.state = stateProcessing
	m.commandQueue = commands
	m.descriptionQueue = descriptions
	m.queueIndex = 0
	m.queueServiceName = serviceName
	m.processingMsg = descriptions[0]
	m.report = []string{infoStyle.Render("Starting multi-step operation...")}

	return m, tea.Batch(
		m.spinner.Tick,
		executeCommandAsync(commands[0], descriptions[0], serviceName),
	)
}

// executeCommandAsync creates a command that executes a shell command asynchronously
func executeCommandAsync(command, description, serviceName string) tea.Cmd {
	return func() tea.Msg {
		// Execute the command using the existing logging infrastructure
		startTime := time.Now()
		cmd := exec.Command("bash", "-c", command)
		output, err := cmd.CombinedOutput()
		endTime := time.Now()
		duration := endTime.Sub(startTime)

		exitCode := 0
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				exitCode = exitError.ExitCode()
			}
		}

		result := LoggedExecResult{
			Command:   command,
			Output:    string(output),
			Error:     err,
			ExitCode:  exitCode,
			StartTime: startTime,
			EndTime:   endTime,
			Duration:  duration,
		}

		// Log the command execution (need to create a model instance for logging)
		// This is a simplified version - in a real app you might want to pass a logger
		// For now, we'll let the result handling in updateProcessing deal with it

		return cmdCompletedMsg{
			result:      result,
			serviceName: serviceName,
		}
	}
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}
