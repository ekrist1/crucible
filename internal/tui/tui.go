package tui

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	
	"crucible/internal/services"
)

type AppState int

const (
	StateMenu AppState = iota
	StateSubmenu
	StateInput
	StateProcessing
	StateLogViewer
)

type MenuLevel int

const (
	MenuMain MenuLevel = iota
	MenuCoreServices
	MenuLaravelManagement
	MenuServerManagement
)

// Message types for async command execution
type CmdExecutionMsg struct {
	Command     string
	Description string
	ServiceName string
}

type CmdCompletedMsg struct {
	Result      LoggedExecResult
	ServiceName string
}

type CmdQueueMsg struct {
	Commands     []string
	Descriptions []string
	ServiceName  string
	CurrentIndex int
}

type Model struct {
	Choices       []string
	Cursor        int
	Selected      map[int]struct{}
	Logger        *log.Logger
	State         AppState
	CurrentMenu   MenuLevel // Track which menu we're in
	InputPrompt   string
	InputValue    string
	InputField    string
	FormData      map[string]string
	CurrentAction int
	ServiceStatus map[string]bool // Track installation status of services
	Spinner       spinner.Model   // Spinner for long-running tasks
	ProcessingMsg string          // Message to display during processing
	Report        []string        // System status report lines
	// Log viewer state
	LogLines  []string // All log lines
	LogScroll int      // Current scroll position
	// Command queue state
	CommandQueue     []string // Commands to execute in sequence
	DescriptionQueue []string // Descriptions for each command
	QueueIndex       int      // Current command index
	QueueServiceName string   // Service name for queue
}

var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")). // White
			Background(lipgloss.Color("5")).  // Magenta
			Padding(0, 1)

	SelectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")). // Bright Green
			Bold(true)

	ChoiceStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")) // Gray

	InputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14")). // Cyan
			Background(lipgloss.Color("0")).  // Black
			Padding(0, 1)

	PromptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")). // Yellow
			Bold(true)

	InfoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // Bright Green for info/success
	WarnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // Yellow for warnings
)

func NewModel() Model {
	logger := log.NewWithOptions(os.Stdout, log.Options{
		ReportCaller:    false,
		ReportTimestamp: true,
		Prefix:          "Crucible 🔧",
	})

	// Initialize spinner
	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // Bright Green

	m := Model{
		Choices: []string{
			"Core Services",
			"Laravel Management",
			"Server Management",
			"Exit",
		},
		Selected:      make(map[int]struct{}),
		Logger:        logger,
		State:         StateMenu,
		CurrentMenu:   MenuMain,
		FormData:      make(map[string]string),
		ServiceStatus: make(map[string]bool),
		Spinner:       s,
		ProcessingMsg: "",
		Report:        []string{},
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

func (m Model) Init() tea.Cmd {
	return m.Spinner.Tick
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.State {
	case StateMenu:
		return m.updateMenu(msg)
	case StateSubmenu:
		return m.updateSubmenu(msg)
	case StateInput:
		return m.updateInput(msg)
	case StateProcessing:
		return m.updateProcessing(msg)
	case StateLogViewer:
		return m.updateLogViewer(msg)
	}
	return m, nil
}

func (m Model) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}

		case "down", "j":
			if m.Cursor < len(m.Choices)-1 {
				m.Cursor++
			}

		case "enter", " ":
			if m.Cursor == len(m.Choices)-1 {
				return m, tea.Quit
			}

			// Handle main menu selection - enter submenu
			return m.enterSubmenu()

		case "r", "R":
			// Refresh service installation status
			modelPtr := &m
			modelPtr.checkServiceInstallations()
			return *modelPtr, nil
		}
	}

	return m, nil
}

// enterSubmenu handles entering a submenu from the main menu
func (m Model) enterSubmenu() (tea.Model, tea.Cmd) {
	switch m.Cursor {
	case 0: // Core Services
		m.State = StateSubmenu
		m.CurrentMenu = MenuCoreServices
		m.Choices = []string{
			"Install PHP 8.4",
			"Install PHP Composer",
			"Install Python, pip, and virtualenv",
			"Install Node.js and npm",
			"Install MySQL",
			"Install Caddy Server (recommended)",
			"Install Supervisor (recommended)",
			"Install Git CLI (recommended)",
			"Back to Main Menu",
		}
		m.Cursor = 0
		return m, tea.ClearScreen
	case 1: // Laravel Management
		m.State = StateSubmenu
		m.CurrentMenu = MenuLaravelManagement
		m.Choices = []string{
			"Create a new Laravel Site",
			"Update Laravel Site",
			"Setup Laravel Queue Worker",
			"Back to Main Menu",
		}
		m.Cursor = 0
		return m, tea.ClearScreen
	case 2: // Server Management
		m.State = StateSubmenu
		m.CurrentMenu = MenuServerManagement
		m.Choices = []string{
			"Backup MySQL Database",
			"System Status",
			"View Installation Logs",
			"Back to Main Menu",
		}
		m.Cursor = 0
		return m, tea.ClearScreen
	case 3: // Exit
		return m, tea.Quit
	}
	return m, nil
}

// updateSubmenu handles input in submenu state
func (m Model) updateSubmenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			// Go back to main menu
			return m.returnToMainMenu()
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(m.Choices)-1 {
				m.Cursor++
			}
		case "enter", " ":
			// Check if this is "Back to Main Menu" option
			if m.Cursor == len(m.Choices)-1 {
				return m.returnToMainMenu()
			}
			// Handle submenu selection
			return m.handleSubmenuSelection()
		case "r", "R":
			// Refresh service installation status
			modelPtr := &m
			modelPtr.checkServiceInstallations()
			return *modelPtr, nil
		}
	}
	return m, nil
}

// returnToMainMenu returns to the main menu
func (m Model) returnToMainMenu() (tea.Model, tea.Cmd) {
	m.State = StateMenu
	m.CurrentMenu = MenuMain
	m.Choices = []string{
		"Core Services",
		"Laravel Management",
		"Server Management",
		"Exit",
	}
	m.Cursor = 0
	return m, tea.ClearScreen
}

// handleSubmenuSelection handles selections within submenus
func (m Model) handleSubmenuSelection() (tea.Model, tea.Cmd) {
	switch m.CurrentMenu {
	case MenuCoreServices:
		return m.handleCoreServicesSelection()
	case MenuLaravelManagement:
		return m.handleLaravelManagementSelection()
	case MenuServerManagement:
		return m.handleServerManagementSelection()
	}
	return m, nil
}

// handleCoreServicesSelection handles Core Services submenu selections
func (m Model) handleCoreServicesSelection() (tea.Model, tea.Cmd) {
	switch m.Cursor {
	case 0: // Install PHP 8.4
		newModel, cmd := m.installPHP()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	case 1: // Install PHP Composer
		newModel, cmd := m.installComposer()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	case 2: // Install Python, pip, and virtualenv
		newModel, cmd := m.installPython()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	case 3: // Install Node.js and npm
		newModel, cmd := m.installNode()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	case 4: // Install MySQL
		newModel, cmd := m.installMySQL()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	case 5: // Install Caddy Server
		newModel, cmd := m.installCaddy()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	case 6: // Install Supervisor
		newModel, cmd := m.installSupervisor()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	case 7: // Install Git CLI
		newModel, cmd := m.installGit()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	}
	return m, nil
}

// handleLaravelManagementSelection handles Laravel Management submenu selections
func (m Model) handleLaravelManagementSelection() (tea.Model, tea.Cmd) {
	switch m.Cursor {
	case 0: // Create a new Laravel Site
		return m.startInput("Enter site name (e.g., myapp):", "siteName", 100)
	case 1: // Update Laravel Site
		return m.startInput("Select site number:", "siteIndex", 101)
	case 2: // Setup Laravel Queue Worker
		return m.startInput("Enter Laravel site name:", "queueSiteName", 102)
	}
	return m, nil
}

// handleServerManagementSelection handles Server Management submenu selections
func (m Model) handleServerManagementSelection() (tea.Model, tea.Cmd) {
	switch m.Cursor {
	case 0: // Backup MySQL Database
		return m.startInput("Enter database name:", "dbName", 103)
	case 1: // System Status
		newModel, cmd := m.showSystemStatus()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	case 2: // View Installation Logs
		newModel, cmd := m.showInstallationLogs()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	}
	return m, nil
}

func (m Model) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			// Clear screen and cancel input, return to menu
			m.State = StateMenu
			m.InputValue = ""
			m.InputPrompt = ""
			m.Cursor = 0 // Reset cursor when canceling input
			return m, tea.ClearScreen
		case "enter":
			// Save input and proceed
			m.FormData[m.InputField] = m.InputValue
			return m.processFormInput()
		case "backspace":
			if len(m.InputValue) > 0 {
				m.InputValue = m.InputValue[:len(m.InputValue)-1]
			}
		default:
			// Add character to input
			m.InputValue += msg.String()
		}
	}

	return m, nil
}

func (m Model) updateProcessing(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "enter", " ":
			// Only allow exit if processing is complete (no processingMsg)
			if m.ProcessingMsg == "" {
				// Return to main menu after processing and refresh service status
				m.State = StateMenu
				m.CurrentMenu = MenuMain
				m.Choices = []string{
					"Core Services",
					"Laravel Management",
					"Server Management",
					"Exit",
				}
				m.FormData = make(map[string]string)
				m.Report = []string{} // Clear report
				m.Cursor = 0          // Reset cursor to top of menu
				modelPtr := &m
				modelPtr.checkServiceInstallations() // Refresh all service statuses
				return *modelPtr, tea.ClearScreen
			}
		}
	case CmdCompletedMsg:
		// Command execution completed
		modelPtr := &m

		// Log the command execution
		if logErr := modelPtr.logCommand(msg.Result); logErr != nil {
			m.Logger.Error("Failed to log command", "error", logErr)
		}

		if msg.Result.Error != nil {
			// Command failed - stop queue and show error
			m.ProcessingMsg = ""
			m.CommandQueue = []string{}
			m.DescriptionQueue = []string{}
			m.QueueIndex = 0
			m.Report = append(m.Report, WarnStyle.Render(fmt.Sprintf("❌ Failed: %v", msg.Result.Error)))
			if strings.TrimSpace(msg.Result.Output) != "" {
				m.Report = append(m.Report, WarnStyle.Render(fmt.Sprintf("Output: %s", msg.Result.Output)))
			}
			return *modelPtr, nil
		}

		// Command succeeded - check if there are more commands in queue
		if len(m.CommandQueue) > 0 && m.QueueIndex < len(m.CommandQueue)-1 {
			// Execute next command in queue
			m.QueueIndex++
			m.ProcessingMsg = m.DescriptionQueue[m.QueueIndex]
			m.Report = append(m.Report, InfoStyle.Render(fmt.Sprintf("✅ %s", m.DescriptionQueue[m.QueueIndex-1])))
			return *modelPtr, tea.Batch(
				m.Spinner.Tick,
				ExecuteCommandAsync(m.CommandQueue[m.QueueIndex], m.DescriptionQueue[m.QueueIndex], m.QueueServiceName),
			)
		}

		// All commands completed successfully
		m.ProcessingMsg = ""
		m.Report = append(m.Report, InfoStyle.Render("✅ All operations completed successfully"))
		if msg.ServiceName != "" || m.QueueServiceName != "" {
			serviceName := msg.ServiceName
			if serviceName == "" {
				serviceName = m.QueueServiceName
			}

			// Handle service-specific post-installation setup
			if serviceName == "caddy" {
				modelPtr.setupCaddyLaravelConfig()
			}

			modelPtr.refreshServiceStatus(serviceName)
		}

		// Clear queue
		m.CommandQueue = []string{}
		m.DescriptionQueue = []string{}
		m.QueueIndex = 0
		m.QueueServiceName = ""

		return *modelPtr, nil
	default:
		// Update spinner
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) processFormInput() (tea.Model, tea.Cmd) {
	// This function handles the form input flow for different actions
	switch m.CurrentAction {
	case 100: // Create New Laravel Site
		return m.handleLaravelSiteForm()
	case 101: // Update Laravel Site
		return m.handleUpdateSiteForm()
	case 102: // Setup Laravel Queue Worker
		return m.handleQueueWorkerForm()
	case 103: // Backup MySQL Database
		return m.handleBackupForm()
	}

	// Default: return to menu
	m.State = StateMenu
	m.Cursor = 0 // Reset cursor when returning to menu
	return m, nil
}

func (m Model) StartInput(prompt, field string, action int) (Model, tea.Cmd) {
	m.State = StateInput
	m.InputPrompt = prompt
	m.InputField = field
	m.InputValue = ""
	m.CurrentAction = action
	m.Cursor = 0 // Reset cursor when starting input
	return m, tea.ClearScreen
}

func (m Model) startInput(prompt, field string, action int) (tea.Model, tea.Cmd) {
	newModel, cmd := m.StartInput(prompt, field, action)
	return newModel, cmd
}

func (m Model) startProcessingWithMessage(message string) (tea.Model, tea.Cmd) {
	m.State = StateProcessing
	m.ProcessingMsg = message
	return m, tea.Batch(tea.ClearScreen, m.Spinner.Tick)
}

func (m *Model) checkServiceInstallations() {
	// Check PHP installation
	m.ServiceStatus["php"] = m.isServiceInstalled("php", "--version")

	// Check Composer installation
	m.ServiceStatus["composer"] = m.isServiceInstalled("composer", "--version")

	// Check Python installation
	m.ServiceStatus["python"] = m.isServiceInstalled("python3", "--version")

	// Check Node.js installation
	m.ServiceStatus["node"] = m.isServiceInstalled("node", "--version")

	// Check MySQL installation
	m.ServiceStatus["mysql"] = m.isServiceInstalled("mysql", "--version")

	// Check Caddy installation
	m.ServiceStatus["caddy"] = m.isServiceInstalled("caddy", "version")

	// Check Git installation
	m.ServiceStatus["git"] = m.isServiceInstalled("git", "--version")

	// Check Supervisor installation
	m.ServiceStatus["supervisor"] = m.isServiceInstalled("supervisorctl", "version")
}

func (m Model) isServiceInstalled(command string, args ...string) bool {
	cmd := exec.Command(command, args...)
	err := cmd.Run()
	return err == nil
}

func (m *Model) refreshServiceStatus(serviceName string) {
	switch serviceName {
	case "php":
		m.ServiceStatus["php"] = m.isServiceInstalled("php", "--version")
	case "composer":
		m.ServiceStatus["composer"] = m.isServiceInstalled("composer", "--version")
	case "python":
		m.ServiceStatus["python"] = m.isServiceInstalled("python3", "--version")
	case "node":
		m.ServiceStatus["node"] = m.isServiceInstalled("node", "--version")
	case "mysql":
		m.ServiceStatus["mysql"] = m.isServiceInstalled("mysql", "--version")
	case "caddy":
		m.ServiceStatus["caddy"] = m.isServiceInstalled("caddy", "version")
	case "git":
		m.ServiceStatus["git"] = m.isServiceInstalled("git", "--version")
	}
}

func (m Model) getServiceIcon(serviceName string) string {
	if m.ServiceStatus[serviceName] {
		return "✅"
	}
	return "⬜"
}

func (m Model) View() string {
	switch m.State {
	case StateMenu:
		return m.viewMenu()
	case StateSubmenu:
		return m.viewSubmenu()
	case StateInput:
		return m.viewInput()
	case StateProcessing:
		return m.viewProcessing()
	case StateLogViewer:
		return m.viewLogViewer()
	}
	return ""
}

func (m Model) viewMenu() string {
	s := TitleStyle.Render("🔧 Crucible - Server Setup made easy for Laravel and Python") + "\n\n"

	for i, choice := range m.Choices {
		cursor := " "
		if m.Cursor == i {
			cursor = ">"
			choice = SelectedStyle.Render(choice)
		} else {
			choice = ChoiceStyle.Render(choice)
		}

		s += fmt.Sprintf("%s %s\n", cursor, choice)
	}

	s += "\nPress q to quit, Enter to select.\n"
	return s
}

func (m Model) viewSubmenu() string {
	var title string
	switch m.CurrentMenu {
	case MenuCoreServices:
		title = "🔧 Core Services"
	case MenuLaravelManagement:
		title = "🚀 Laravel Management"
	case MenuServerManagement:
		title = "⚙️ Server Management"
	}

	s := TitleStyle.Render(title) + "\n\n"

	for i, choice := range m.Choices {
		cursor := " "
		serviceIcon := ""

		// Add service status icons for installation options in Core Services
		if m.CurrentMenu == MenuCoreServices && i < len(m.Choices)-1 {
			switch i {
			case 0: // Install PHP 8.4
				serviceIcon = m.getServiceIcon("php") + " "
			case 1: // Install PHP Composer
				serviceIcon = m.getServiceIcon("composer") + " "
			case 2: // Install Python, pip, and virtualenv
				serviceIcon = m.getServiceIcon("python") + " "
			case 3: // Install Node.js and npm
				serviceIcon = m.getServiceIcon("node") + " "
			case 4: // Install MySQL
				serviceIcon = m.getServiceIcon("mysql") + " "
			case 5: // Install Caddy Server
				serviceIcon = m.getServiceIcon("caddy") + " "
			case 6: // Install Supervisor
				serviceIcon = m.getServiceIcon("supervisor") + " "
			case 7: // Install Git CLI
				serviceIcon = m.getServiceIcon("git") + " "
			}
		}

		if m.Cursor == i {
			cursor = ">"
			choice = SelectedStyle.Render(serviceIcon + choice)
		} else {
			choice = ChoiceStyle.Render(serviceIcon + choice)
		}

		s += fmt.Sprintf("%s %s\n", cursor, choice)
	}

	s += "\nPress Esc or select 'Back to Main Menu' to return, q to quit, r to refresh.\n"
	if m.CurrentMenu == MenuCoreServices {
		s += "\n✅ = Installed  ⬜ = Not installed\n"
	}
	return s
}

func (m Model) viewInput() string {
	s := TitleStyle.Render("🔧 Crucible - Laravel Server Setup") + "\n\n"
	s += PromptStyle.Render(m.InputPrompt) + "\n\n"
	s += InputStyle.Render(m.InputValue+"│") + "\n\n"
	s += "Press Enter to continue, Esc to cancel\n"
	return s
}

func (m Model) viewProcessing() string {
	s := TitleStyle.Render("🔧 Crucible - Laravel Server Setup") + "\n\n"
	if m.ProcessingMsg != "" {
		s += fmt.Sprintf("%s %s\n\n", m.Spinner.View(), m.ProcessingMsg)
		s += "Please wait...\n"
	} else {
		if len(m.Report) > 0 {
			s += strings.Join(m.Report, "\n") + "\n\n"
		}
		s += "Processing completed!\n"
		s += "Press any key to return to main menu.\n"
	}
	return s
}

func (m Model) updateLogViewer(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			// Return to main menu
			m.State = StateMenu
			m.Cursor = 0
			m.LogLines = []string{}
			m.LogScroll = 0
			return m, tea.ClearScreen
		case "up", "k":
			// Scroll up
			if m.LogScroll > 0 {
				m.LogScroll--
			}
		case "down", "j":
			// Scroll down
			logViewHeight := 18
			maxScroll := len(m.LogLines) - logViewHeight
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.LogScroll < maxScroll {
				m.LogScroll++
			}
		case "home", "g":
			// Go to top
			m.LogScroll = 0
		case "end", "G":
			// Go to bottom
			logViewHeight := 18
			maxScroll := len(m.LogLines) - logViewHeight
			if maxScroll < 0 {
				maxScroll = 0
			}
			m.LogScroll = maxScroll
		case "pageup":
			// Page up
			logViewHeight := 18
			m.LogScroll -= logViewHeight
			if m.LogScroll < 0 {
				m.LogScroll = 0
			}
		case "pagedown":
			// Page down
			logViewHeight := 18
			maxScroll := len(m.LogLines) - logViewHeight
			if maxScroll < 0 {
				maxScroll = 0
			}
			m.LogScroll += logViewHeight
			if m.LogScroll > maxScroll {
				m.LogScroll = maxScroll
			}
		}
	}
	return m, nil
}

func (m Model) viewLogViewer() string {
	s := TitleStyle.Render("🔧 Crucible - Installation Logs") + "\n\n"

	// Calculate view height (assuming terminal height of about 24 lines, minus header and footer)
	logViewHeight := 18

	if len(m.LogLines) == 0 {
		s += "No log lines to display.\n\n"
	} else {
		// Calculate visible range
		startIdx := m.LogScroll
		endIdx := startIdx + logViewHeight

		if endIdx > len(m.LogLines) {
			endIdx = len(m.LogLines)
		}

		// Show log lines with line numbers
		for i := startIdx; i < endIdx; i++ {
			line := m.LogLines[i]
			lineNum := fmt.Sprintf("%4d: ", i+1)

			// Style different types of log lines
			if strings.Contains(line, "COMMAND:") {
				s += InfoStyle.Render(lineNum) + InfoStyle.Render(line) + "\n"
			} else if strings.Contains(line, "ERROR:") || strings.Contains(line, "EXIT CODE:") {
				s += WarnStyle.Render(lineNum) + WarnStyle.Render(line) + "\n"
			} else if strings.Contains(line, "STATUS: SUCCESS") {
				s += InfoStyle.Render(lineNum) + InfoStyle.Render(line) + "\n"
			} else {
				s += ChoiceStyle.Render(lineNum) + line + "\n"
			}
		}

		s += "\n"

		// Show scroll position info
		totalLines := len(m.LogLines)
		visibleStart := m.LogScroll + 1
		visibleEnd := m.LogScroll + (endIdx - startIdx)

		s += ChoiceStyle.Render(fmt.Sprintf("Lines %d-%d of %d", visibleStart, visibleEnd, totalLines)) + "\n"
	}

	s += "\nNavigation: ↑/↓ scroll, Home/End jump, PgUp/PgDn page, q/Esc to exit\n"
	return s
}

// startCommandQueue starts executing a queue of commands sequentially
func (m Model) startCommandQueue(commands, descriptions []string, serviceName string) (Model, tea.Cmd) {
	if len(commands) == 0 || len(descriptions) == 0 {
		return m, nil
	}

	m.State = StateProcessing
	m.CommandQueue = commands
	m.DescriptionQueue = descriptions
	m.QueueIndex = 0
	m.QueueServiceName = serviceName
	m.ProcessingMsg = descriptions[0]
	m.Report = []string{InfoStyle.Render("Starting multi-step operation...")}

	return m, tea.Batch(
		m.Spinner.Tick,
		ExecuteCommandAsync(commands[0], descriptions[0], serviceName),
	)
}

// ExecuteCommandAsync creates a command that executes a shell command asynchronously
func ExecuteCommandAsync(command, description, serviceName string) tea.Cmd {
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

		return CmdCompletedMsg{
			Result:      result,
			ServiceName: serviceName,
		}
	}
}

// Placeholder types and functions that need to be implemented or moved from other files
type LoggedExecResult struct {
	Command   string
	Output    string
	Error     error
	ExitCode  int
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
}

// These functions will need to be implemented or moved from other files
func (m *Model) initializeLogging() error {
	// TODO: Move from utils.go or implement
	return nil
}

func (m Model) logCommand(result LoggedExecResult) error {
	// TODO: Move from utils.go or implement
	return nil
}

func (m *Model) setupCaddyLaravelConfig() {
	// This will be properly connected when we finish the refactor
}

// getServiceStatus checks if a service/command is installed and returns a formatted status string
func (m Model) getServiceStatus(name, command string, args ...string) string {
	cmd := exec.Command(command, args...)
	output, err := cmd.Output()
	
	if err != nil {
		return WarnStyle.Render(fmt.Sprintf("❌ %s: Not installed", name))
	}
	
	// Extract version from output (first line usually contains version info)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	version := lines[0]
	
	// Clean up version string
	if len(version) > 60 {
		version = version[:60] + "..."
	}
	
	return InfoStyle.Render(fmt.Sprintf("✅ %s: %s", name, version))
}

// getSystemServiceStatus checks if a systemd service is running
func (m Model) getSystemServiceStatus(name, service string) string {
	cmd := exec.Command("systemctl", "is-active", service)
	output, err := cmd.Output()
	
	if err != nil {
		return WarnStyle.Render(fmt.Sprintf("❌ %s: Service not found or not running", name))
	}
	
	status := strings.TrimSpace(string(output))
	if status == "active" {
		return InfoStyle.Render(fmt.Sprintf("✅ %s: Running", name))
	}
	
	return WarnStyle.Render(fmt.Sprintf("⚠️ %s: %s", name, status))
}

// getDiskUsage returns formatted disk usage information
func (m Model) getDiskUsage() string {
	cmd := exec.Command("df", "-h", "/")
	output, err := cmd.Output()
	
	if err != nil {
		return WarnStyle.Render("❌ Disk Usage: Unable to retrieve")
	}
	
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return WarnStyle.Render("❌ Disk Usage: Unable to parse")
	}
	
	// Parse the df output (skip header)
	fields := strings.Fields(lines[1])
	if len(fields) >= 5 {
		used := fields[4] // Usage percentage
		available := fields[3] // Available space
		return InfoStyle.Render(fmt.Sprintf("💾 Disk Usage: %s used, %s available", used, available))
	}
	
	return WarnStyle.Render("❌ Disk Usage: Unable to parse")
}

// getMemoryUsage returns formatted memory usage information
func (m Model) getMemoryUsage() string {
	cmd := exec.Command("free", "-h")
	output, err := cmd.Output()
	
	if err != nil {
		return WarnStyle.Render("❌ Memory Usage: Unable to retrieve")
	}
	
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return WarnStyle.Render("❌ Memory Usage: Unable to parse")
	}
	
	// Parse the free output (get memory line)
	for _, line := range lines {
		if strings.HasPrefix(line, "Mem:") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				total := fields[1] // Total memory
				used := fields[2]  // Used memory
				return InfoStyle.Render(fmt.Sprintf("🧠 Memory Usage: %s used / %s total", used, total))
			}
		}
	}
	
	return WarnStyle.Render("❌ Memory Usage: Unable to parse")
}

// Installation functions
func (m Model) installPHP() (tea.Model, tea.Cmd) {
	commands, descriptions, err := services.InstallPHP()
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
		return m, tea.ClearScreen
	}
	return m.startCommandQueue(commands, descriptions, "php")
}

func (m Model) installComposer() (tea.Model, tea.Cmd) {
	commands, descriptions, err := services.InstallComposer()
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
		return m, tea.ClearScreen
	}
	return m.startCommandQueue(commands, descriptions, "composer")
}

func (m Model) installPython() (tea.Model, tea.Cmd) {
	commands, descriptions, err := services.InstallPython()
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
		return m, tea.ClearScreen
	}
	return m.startCommandQueue(commands, descriptions, "python")
}

func (m Model) installNode() (tea.Model, tea.Cmd) {
	commands, descriptions, err := services.InstallNode()
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
		return m, tea.ClearScreen
	}
	return m.startCommandQueue(commands, descriptions, "node")
}

func (m Model) installMySQL() (tea.Model, tea.Cmd) {
	commands, descriptions, err := services.InstallMySQL()
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
		return m, tea.ClearScreen
	}
	return m.startCommandQueue(commands, descriptions, "mysql")
}

func (m Model) installCaddy() (tea.Model, tea.Cmd) {
	commands, descriptions, err := services.InstallCaddy()
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
		return m, tea.ClearScreen
	}
	return m.startCommandQueue(commands, descriptions, "caddy")
}

func (m Model) installSupervisor() (tea.Model, tea.Cmd) {
	commands, descriptions, err := services.InstallSupervisor()
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
		return m, tea.ClearScreen
	}
	return m.startCommandQueue(commands, descriptions, "supervisor")
}

func (m Model) installGit() (tea.Model, tea.Cmd) {
	commands, descriptions, err := services.InstallGit()
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
		return m, tea.ClearScreen
	}
	return m.startCommandQueue(commands, descriptions, "git")
}

func (m Model) showSystemStatus() (tea.Model, tea.Cmd) {
	m.State = StateProcessing
	m.Report = []string{}
	m.ProcessingMsg = ""
	
	// Build system status report
	m.Report = append(m.Report, TitleStyle.Render("=== SYSTEM STATUS ==="))
	m.Report = append(m.Report, "")
	
	// Check service statuses
	m.Report = append(m.Report, InfoStyle.Render("📦 Service Status:"))
	m.Report = append(m.Report, m.getServiceStatus("PHP", "php", "--version"))
	m.Report = append(m.Report, m.getServiceStatus("Composer", "composer", "--version"))
	m.Report = append(m.Report, m.getServiceStatus("Node.js", "node", "--version"))
	m.Report = append(m.Report, m.getServiceStatus("Python", "python3", "--version"))
	m.Report = append(m.Report, m.getServiceStatus("MySQL", "mysql", "--version"))
	m.Report = append(m.Report, m.getServiceStatus("Git", "git", "--version"))
	m.Report = append(m.Report, m.getServiceStatus("Caddy", "caddy", "version"))
	m.Report = append(m.Report, "")
	
	// Check system services
	m.Report = append(m.Report, InfoStyle.Render("⚙️ System Services:"))
	m.Report = append(m.Report, m.getSystemServiceStatus("MySQL", "mysql"))
	m.Report = append(m.Report, m.getSystemServiceStatus("Caddy", "caddy"))
	m.Report = append(m.Report, m.getSystemServiceStatus("Supervisor", "supervisor"))
	m.Report = append(m.Report, "")
	
	// System resources
	m.Report = append(m.Report, InfoStyle.Render("💾 System Resources:"))
	m.Report = append(m.Report, m.getDiskUsage())
	m.Report = append(m.Report, m.getMemoryUsage())
	
	return m, tea.ClearScreen
}

func (m Model) showInstallationLogs() (tea.Model, tea.Cmd) {
	// Try to read log file using the logging package
	logPath := "/home/" + os.Getenv("USER") + "/.crucible/logs/crucible.log"
	
	// Try alternative log paths if the default doesn't exist
	alternativePaths := []string{
		logPath,
		"/tmp/crucible.log",
		"./crucible.log",
	}
	
	var logLines []string
	var foundLog bool
	
	for _, path := range alternativePaths {
		if file, err := os.Open(path); err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			logLines = []string{}
			for scanner.Scan() {
				logLines = append(logLines, scanner.Text())
			}
			foundLog = true
			break
		}
	}
	
	if !foundLog {
		// No log file found, show empty state
		m.State = StateLogViewer
		m.LogLines = []string{
			"No installation logs found.",
			"",
			"Log files are created when you perform installation operations.",
			"Try installing a service first, then check back here.",
		}
		m.LogScroll = 0
		return m, tea.ClearScreen
	}
	
	// Set up log viewer state
	m.State = StateLogViewer
	m.LogLines = logLines
	m.LogScroll = 0
	
	// If there are many lines, start at the bottom
	if len(logLines) > 18 {
		m.LogScroll = len(logLines) - 18
	}
	
	return m, tea.ClearScreen
}

// Form handling functions - now implemented in forms.go
func (m Model) handleLaravelSiteForm() (tea.Model, tea.Cmd) {
	newModel, cmd := m.HandleLaravelSiteForm()
	return newModel, cmd
}

func (m Model) handleUpdateSiteForm() (tea.Model, tea.Cmd) {
	newModel, cmd := m.HandleUpdateSiteForm()
	return newModel, cmd
}

func (m Model) handleQueueWorkerForm() (tea.Model, tea.Cmd) {
	newModel, cmd := m.HandleQueueWorkerForm()
	return newModel, cmd
}

func (m Model) handleBackupForm() (tea.Model, tea.Cmd) {
	newModel, cmd := m.HandleBackupForm()
	return newModel, cmd
}