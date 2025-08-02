package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"

	"crucible/internal/actions"
	"crucible/internal/logging"
	"crucible/internal/monitor"
	"crucible/internal/services"
)

type AppState int

const (
	StateMenu AppState = iota
	StateSubmenu
	StateInput
	StateProcessing
	StateLogViewer
	StateServiceList
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
	Result      logging.LoggedExecResult
	ServiceName string
}

type CmdQueueMsg struct {
	Commands     []string
	Descriptions []string
	ServiceName  string
	CurrentIndex int
}

// ServiceItem represents a service in the list
type ServiceItem struct {
	ServiceInfo actions.ServiceInfo
}

// Implement list.Item interface
func (i ServiceItem) FilterValue() string { return i.ServiceInfo.Name }
func (i ServiceItem) Title() string       { return i.ServiceInfo.Name }
func (i ServiceItem) Description() string {
	status := "●"
	if i.ServiceInfo.Active == "active" {
		status = "🟢"
	} else {
		status = "🔴"
	}
	return fmt.Sprintf("%s %s - %s", status, i.ServiceInfo.Status, i.ServiceInfo.Sub)
}

type Model struct {
	Choices       []string
	Cursor        int
	Selected      map[int]struct{}
	Logger        *logging.Logger
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
	// Service list state
	ServiceList         list.Model            // Service management list
	Services            []actions.ServiceInfo // Parsed services
	ReturnToServiceList bool                  // Flag to return to service list instead of main menu
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
	logger, err := logging.NewLogger(logging.DefaultLogPath())
	if err != nil {
		// Fallback to basic stdout logger if file logger fails
		baseLogger := log.NewWithOptions(os.Stdout, log.Options{
			ReportCaller:    false,
			ReportTimestamp: true,
			Prefix:          "Crucible 🔧",
		})
		logger = &logging.Logger{Logger: baseLogger}
	}

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
	case StateServiceList:
		return m.updateServiceList(msg)
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
			"GitHub Authentication",
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
			"Service Management",
			"Monitoring Dashboard",
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
		return m.startInput("Enter MySQL root password (min 8 chars, this will be used for automated setup):", "mysqlRootPassword", 200)
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
		return m.showLaravelSiteList()
	case 2: // Setup Laravel Queue Worker
		return m.showLaravelSiteListForQueue()
	case 3: // GitHub Authentication
		return m.handleGitHubAuth()
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
	case 3: // Service Management
		newModel, cmd := m.showServiceManagement()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	case 4: // Monitoring Dashboard
		newModel, cmd := m.showMonitoringDashboard()
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
				// Special case: if we're showing service list, start interactive control
				if m.QueueServiceName == "list-services" {
					m.QueueServiceName = "" // Clear the service name
					return m.controlServiceInteractive()
				}

				// Special case: if we need to return to service list
				if m.ReturnToServiceList {
					m.ReturnToServiceList = false // Clear the flag
					m.State = StateServiceList
					m.Report = []string{} // Clear report
					return m, tea.ClearScreen
				}

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
			} else if serviceName == "github-ssh" {
				// Show the generated SSH key and instructions
				modelPtr.showGeneratedSSHKey()
			} else if serviceName == "github-test" {
				// Show GitHub connection test results
				modelPtr.showGitHubTestResults()
			} else if serviceName == "list-services" {
				// Parse services and create list
				services := actions.ParseServiceList(msg.Result.Output)
				modelPtr.createServiceList(services)
				modelPtr.State = StateServiceList
				return *modelPtr, tea.ClearScreen
			} else if strings.HasPrefix(serviceName, "service-status-") {
				// Service status command completed - show results and return to service list
				serviceNameOnly := strings.TrimPrefix(serviceName, "service-status-")
				modelPtr.showServiceStatusResults(serviceNameOnly, msg.Result.Output)
				return *modelPtr, tea.ClearScreen
			} else if strings.HasPrefix(serviceName, "service-") && (strings.Contains(serviceName, "-restart") || strings.Contains(serviceName, "-stop") || strings.Contains(serviceName, "-start")) {
				// Service action command completed - show results and return to service list
				parts := strings.Split(serviceName, "-")
				if len(parts) >= 3 {
					serviceNameOnly := strings.Join(parts[1:len(parts)-1], "-") // Remove "service-" prefix and "-action" suffix
					action := parts[len(parts)-1]
					modelPtr.showServiceActionResults(serviceNameOnly, action, msg.Result.Output)
				}
				return *modelPtr, tea.ClearScreen
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
	case 200: // Install MySQL
		return m.handleMySQLInstallForm()
	case 300: // GitHub Authentication - Email input
		return m.handleGitHubEmailInput()
	case 301: // GitHub Authentication - Passphrase input
		return m.handleGitHubPassphraseInput()
	case 302: // GitHub Authentication - Action selection
		return m.handleGitHubActionInput()
	case 400: // Service Control
		return m.handleServiceControlInput()
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
	case StateServiceList:
		return m.viewServiceList()
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

	// Hide password input
	displayValue := m.InputValue
	if m.InputField == "mysqlRootPassword" || m.InputField == "githubPassphrase" {
		displayValue = strings.Repeat("*", len(m.InputValue))
	}

	s += InputStyle.Render(displayValue+"│") + "\n\n"
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

		result := logging.LoggedExecResult{
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

// Logging functions
func (m Model) logCommand(result logging.LoggedExecResult) error {
	if m.Logger != nil {
		return m.Logger.LogCommand(result)
	}
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
		used := fields[4]      // Usage percentage
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
	commands, descriptions, err := services.InstallMySQL("")
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
		return m, tea.ClearScreen
	}
	return m.startCommandQueue(commands, descriptions, "mysql")
}

func (m Model) installMySQLWithPassword() (tea.Model, tea.Cmd) {
	password := m.FormData["mysqlRootPassword"]
	commands, descriptions, err := services.InstallMySQL(password)
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

// showMonitoringDashboard displays the monitoring dashboard with real-time metrics
func (m Model) showMonitoringDashboard() (tea.Model, tea.Cmd) {
	m.State = StateProcessing
	m.Report = []string{}
	m.ProcessingMsg = "Loading monitoring data..."

	// Build monitoring dashboard report
	m.Report = append(m.Report, TitleStyle.Render("=== MONITORING DASHBOARD ==="))
	m.Report = append(m.Report, "")

	// Check if monitoring agent is running
	agentStatus := m.checkMonitoringAgent()
	m.Report = append(m.Report, InfoStyle.Render("🔧 Monitoring Agent:"))
	m.Report = append(m.Report, agentStatus)
	m.Report = append(m.Report, "")

	// If agent is running, fetch metrics
	if strings.Contains(agentStatus, "✅") {
		// Fetch system metrics
		systemMetrics := m.fetchSystemMetrics()
		m.Report = append(m.Report, InfoStyle.Render("📊 System Metrics:"))
		m.Report = append(m.Report, systemMetrics...)
		m.Report = append(m.Report, "")

		// Fetch service metrics
		serviceMetrics := m.fetchServiceMetrics()
		m.Report = append(m.Report, InfoStyle.Render("⚙️ Service Status:"))
		m.Report = append(m.Report, serviceMetrics...)
		m.Report = append(m.Report, "")

		// Fetch HTTP check results
		httpMetrics := m.fetchHTTPMetrics()
		m.Report = append(m.Report, InfoStyle.Render("🌐 HTTP Health Checks:"))
		m.Report = append(m.Report, httpMetrics...)
	} else {
		m.Report = append(m.Report, WarnStyle.Render("⚠️ Start monitoring agent with: ./crucible-monitor"))
		m.Report = append(m.Report, WarnStyle.Render("⚠️ Or use: make run-monitor"))
	}

	m.ProcessingMsg = ""
	return m, tea.ClearScreen
}

func (m Model) showInstallationLogs() (tea.Model, tea.Cmd) {
	// Use the logger to read log lines if available
	var logLines []string
	var err error

	if m.Logger != nil {
		logLines, err = m.Logger.ReadLogLines()
	}

	if err != nil || len(logLines) == 0 {
		// No log file found or error reading, show empty state
		m.State = StateLogViewer
		m.LogLines = []string{
			"No installation logs found.",
			"",
			"Log files are created when you perform installation operations.",
			"Try installing a service first, then check back here.",
			"",
			fmt.Sprintf("Log file location: %s", func() string {
				if m.Logger != nil {
					return m.Logger.GetLogFilePath()
				}
				return logging.DefaultLogPath()
			}()),
		}
		if err != nil {
			m.LogLines = append(m.LogLines, "", fmt.Sprintf("Error reading logs: %v", err))
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

func (m Model) handleMySQLInstallForm() (tea.Model, tea.Cmd) {
	// Validate password
	if len(m.InputValue) < 8 {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render("❌ MySQL root password must be at least 8 characters long")}
		return m, tea.ClearScreen
	}

	// Store password and proceed with installation
	m.FormData["mysqlRootPassword"] = m.InputValue
	return m.installMySQLWithPassword()
}

func (m Model) handleGitHubAuth() (tea.Model, tea.Cmd) {
	// Check if SSH key already exists
	homeDir, err := os.UserHomeDir()
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error getting home directory: %v", err))}
		return m, tea.ClearScreen
	}

	pubKeyPath := fmt.Sprintf("%s/.ssh/id_ed25519.pub", homeDir)
	if _, err := os.Stat(pubKeyPath); err == nil {
		// SSH key exists, ask user what they want to do
		return m.startInput("SSH key exists. Options: [s]how key, [t]est connection, [r]egenerate:", "githubAction", 302)
	}

	// SSH key doesn't exist, ask for email to generate one
	return m.startInput("Enter your GitHub email address:", "githubEmail", 300)
}

func (m Model) showExistingSSHKey() (tea.Model, tea.Cmd) {
	homeDir, _ := os.UserHomeDir()
	pubKeyPath := fmt.Sprintf("%s/.ssh/id_ed25519.pub", homeDir)

	content, err := os.ReadFile(pubKeyPath)
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error reading SSH key: %v", err))}
		return m, tea.ClearScreen
	}

	m.State = StateProcessing
	m.ProcessingMsg = ""
	m.Report = []string{
		TitleStyle.Render("🔑 GitHub SSH Key Found"),
		"",
		InfoStyle.Render("Your existing SSH public key:"),
		"",
		ChoiceStyle.Render(string(content)),
		"",
		InfoStyle.Render("📋 Instructions to add this key to GitHub:"),
		"1. Copy the key above (select and Ctrl+C)",
		"2. Go to GitHub.com → Settings → SSH and GPG keys",
		"3. Click 'New SSH key'",
		"4. Paste your key and give it a title",
		"5. Click 'Add SSH key'",
		"",
		InfoStyle.Render("🧪 Test your connection with:"),
		ChoiceStyle.Render("ssh -T git@github.com"),
		"",
		WarnStyle.Render("Note: You may see a warning about authenticity - type 'yes' to continue"),
		"",
		InfoStyle.Render("💡 Tip: Run the GitHub Authentication menu again to test the connection after adding the key"),
	}

	return m, tea.ClearScreen
}

func (m Model) handleGitHubEmailInput() (tea.Model, tea.Cmd) {
	// Validate email format (basic validation)
	email := strings.TrimSpace(m.InputValue)
	if email == "" || !strings.Contains(email, "@") {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render("❌ Please enter a valid email address")}
		return m, tea.ClearScreen
	}

	// Store email and ask for passphrase
	m.FormData["githubEmail"] = email
	return m.startInput("Enter SSH key passphrase (optional, press Enter to skip):", "githubPassphrase", 301)
}

func (m Model) handleGitHubPassphraseInput() (tea.Model, tea.Cmd) {
	// Store passphrase (can be empty)
	m.FormData["githubPassphrase"] = m.InputValue
	return m.generateSSHKey()
}

func (m Model) generateSSHKey() (tea.Model, tea.Cmd) {
	email := m.FormData["githubEmail"]
	passphrase := m.FormData["githubPassphrase"]

	homeDir, _ := os.UserHomeDir()
	sshDir := fmt.Sprintf("%s/.ssh", homeDir)

	var commands []string
	var descriptions []string

	// Create .ssh directory if it doesn't exist
	commands = append(commands, fmt.Sprintf("mkdir -p %s", sshDir))
	descriptions = append(descriptions, "Creating SSH directory...")

	// Remove existing key files first to avoid prompts
	commands = append(commands, fmt.Sprintf("rm -f %s/id_ed25519 %s/id_ed25519.pub", sshDir, sshDir))
	descriptions = append(descriptions, "Removing existing SSH keys...")

	// Generate SSH key
	keygenCmd := fmt.Sprintf("ssh-keygen -t ed25519 -C \"%s\" -f %s/id_ed25519", email, sshDir)
	if passphrase != "" {
		keygenCmd += fmt.Sprintf(" -N \"%s\"", passphrase)
	} else {
		keygenCmd += " -N \"\""
	}
	commands = append(commands, keygenCmd)
	descriptions = append(descriptions, "Generating SSH key...")

	// Set proper permissions
	commands = append(commands, fmt.Sprintf("chmod 600 %s/id_ed25519", sshDir))
	descriptions = append(descriptions, "Setting private key permissions...")
	commands = append(commands, fmt.Sprintf("chmod 644 %s/id_ed25519.pub", sshDir))
	descriptions = append(descriptions, "Setting public key permissions...")

	// Note: We skip SSH agent setup here as it's complex in automated scripts
	// The user will be instructed how to add the key manually if needed

	return m.startCommandQueue(commands, descriptions, "github-ssh")
}

func (m Model) handleGitHubActionInput() (tea.Model, tea.Cmd) {
	action := strings.ToLower(strings.TrimSpace(m.InputValue))

	switch action {
	case "s", "show":
		return m.showExistingSSHKey()
	case "t", "test":
		return m.testGitHubConnection()
	case "r", "regenerate":
		m.FormData["githubAction"] = "regenerate"
		return m.startInput("⚠️  This will overwrite your existing SSH key. Enter your GitHub email address:", "githubEmail", 300)
	default:
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render("❌ Invalid option. Please enter 's' (show), 't' (test), or 'r' (regenerate)")}
		return m, tea.ClearScreen
	}
}

func (m Model) testGitHubConnection() (tea.Model, tea.Cmd) {
	// Add timeout and better error handling for SSH test
	commands := []string{"timeout 10 ssh -o ConnectTimeout=5 -o BatchMode=yes -T git@github.com"}
	descriptions := []string{"Testing GitHub SSH connection..."}

	return m.startCommandQueue(commands, descriptions, "github-test")
}

func (m *Model) showGeneratedSSHKey() {
	homeDir, _ := os.UserHomeDir()
	pubKeyPath := fmt.Sprintf("%s/.ssh/id_ed25519.pub", homeDir)

	content, err := os.ReadFile(pubKeyPath)
	if err != nil {
		m.Report = append(m.Report, "", WarnStyle.Render(fmt.Sprintf("❌ Error reading generated SSH key: %v", err)))
		return
	}

	// Check if a passphrase was used
	passphrase := m.FormData["githubPassphrase"]

	// Clear previous report and show the key with instructions
	// Check if this was a regeneration or new generation
	isRegeneration := m.FormData["githubAction"] == "r" || m.FormData["githubAction"] == "regenerate"
	title := "🎉 SSH Key Generated Successfully!"
	if isRegeneration {
		title = "🔄 SSH Key Regenerated Successfully!"
	}

	m.Report = []string{
		TitleStyle.Render(title),
		"",
		InfoStyle.Render("Your new SSH public key:"),
		"",
		ChoiceStyle.Render(string(content)),
		"",
	}

	// Add SSH agent instructions if passphrase was used
	if passphrase != "" {
		m.Report = append(m.Report,
			InfoStyle.Render("🔐 SSH Agent Setup (since you used a passphrase):"),
			"1. Start SSH agent: eval \"$(ssh-agent -s)\"",
			fmt.Sprintf("2. Add your key: ssh-add %s/.ssh/id_ed25519", homeDir),
			"3. Enter your passphrase when prompted",
			"",
		)
	}

	steps := "📋 Next steps to add this key to GitHub:"
	if isRegeneration {
		steps = "📋 Next steps to update this key on GitHub:"
		m.Report = append(m.Report,
			WarnStyle.Render("⚠️  Important: You need to replace your old key on GitHub with this new one!"),
			"",
		)
	}

	m.Report = append(m.Report,
		InfoStyle.Render(steps),
		"1. Copy the key above (select and Ctrl+C)",
		"2. Go to GitHub.com → Settings → SSH and GPG keys",
	)

	if isRegeneration {
		m.Report = append(m.Report,
			"3. Find your old key and click 'Delete'",
			"4. Click 'New SSH key'",
			"5. Paste your new key and give it a title (e.g., 'My Server')",
			"6. Click 'Add SSH key'",
		)
	} else {
		m.Report = append(m.Report,
			"3. Click 'New SSH key'",
			"4. Paste your key and give it a title (e.g., 'My Server')",
			"5. Click 'Add SSH key'",
		)
	}

	m.Report = append(m.Report,
		"",
		InfoStyle.Render("🧪 After adding to GitHub, test your connection with:"),
		ChoiceStyle.Render("ssh -T git@github.com"),
		"",
		InfoStyle.Render("Expected response:"),
		ChoiceStyle.Render("Hi [username]! You've successfully authenticated, but GitHub does not provide shell access."),
		"",
		WarnStyle.Render("Note: You may see a warning about authenticity - type 'yes' to continue"),
		"",
		InfoStyle.Render("💡 Tip: You can also test the connection from the GitHub Authentication menu"),
	)
}

func (m *Model) showGitHubTestResults() {
	// The test results should already be in the report from the command execution
	// We just need to interpret them and add helpful information

	// Check if the test was successful by looking for the success message
	if len(m.Report) > 0 {
		for _, line := range m.Report {
			if strings.Contains(line, "Hi ") && strings.Contains(line, "You've successfully authenticated") {
				// Connection successful
				m.Report = []string{
					TitleStyle.Render("🎉 GitHub SSH Connection Successful!"),
					"",
					InfoStyle.Render("✅ Your SSH key is properly configured"),
					InfoStyle.Render("✅ GitHub authentication is working"),
					"",
					InfoStyle.Render("Connection test output:"),
					ChoiceStyle.Render(line),
					"",
					InfoStyle.Render("🚀 You're ready to:"),
					"• Clone private repositories with SSH URLs",
					"• Push to repositories you have access to",
					"• Use git commands without password prompts",
					"",
					InfoStyle.Render("Example usage:"),
					ChoiceStyle.Render("git clone git@github.com:username/repository.git"),
				}
				return
			}
		}
	}

	// Connection failed or other issue
	homeDir, _ := os.UserHomeDir()
	m.Report = append(m.Report, "",
		WarnStyle.Render("❌ GitHub SSH connection test failed"),
		"",
		InfoStyle.Render("Common solutions:"),
		"1. Make sure you've added your SSH key to GitHub",
		"2. If you used a passphrase, add key to SSH agent:",
		"   • eval \"$(ssh-agent -s)\"",
		fmt.Sprintf("   • ssh-add %s/.ssh/id_ed25519", homeDir),
		"3. Try accepting GitHub's fingerprint manually: ssh -T git@github.com",
		"4. Verify your SSH key exists: cat ~/.ssh/id_ed25519.pub",
		"",
		InfoStyle.Render("Common error meanings:"),
		"• 'Permission denied' → SSH key not added to GitHub",
		"• 'Host key verification failed' → Type 'yes' when prompted",
		"• 'Could not open connection' → SSH agent not running",
		"• 'Connection timeout' → Network or firewall issues",
	)
}

func (m Model) showLaravelSiteList() (tea.Model, tea.Cmd) {
	// Use the actions package to list Laravel sites
	sites, err := actions.ListLaravelSites()
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{
			WarnStyle.Render(fmt.Sprintf("❌ Error scanning for Laravel sites: %v", err)),
			"",
			InfoStyle.Render("Make sure /var/www exists and is accessible"),
		}
		return m, tea.ClearScreen
	}

	if len(sites) == 0 {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{
			WarnStyle.Render("❌ No Laravel sites found in /var/www"),
			"",
			InfoStyle.Render("Create a Laravel site first using 'Create a new Laravel Site'"),
		}
		return m, tea.ClearScreen
	}

	// Build the report showing available sites
	m.State = StateProcessing
	m.ProcessingMsg = ""
	m.Report = []string{
		TitleStyle.Render("📂 Available Laravel Sites"),
		"",
		InfoStyle.Render("Found the following Laravel sites in /var/www:"),
		"",
	}

	for i, site := range sites {
		sitePath := fmt.Sprintf("/var/www/%s", site)
		// Check if it's a git repository
		gitStatus := "📁 Regular site"
		if _, err := os.Stat(fmt.Sprintf("%s/.git", sitePath)); err == nil {
			gitStatus = "📦 Git repository"
		}

		m.Report = append(m.Report,
			InfoStyle.Render(fmt.Sprintf("%d. %s", i+1, site)),
			ChoiceStyle.Render(fmt.Sprintf("   Path: %s", sitePath)),
			ChoiceStyle.Render(fmt.Sprintf("   Type: %s", gitStatus)),
			"",
		)
	}

	// Store sites for later use and ask for selection
	m.FormData["availableSites"] = fmt.Sprintf("%v", sites) // Convert to string for storage

	m.Report = append(m.Report,
		InfoStyle.Render("Select a site to update:"),
	)

	newModel, cmd := m.startInput("Enter site number (1-"+fmt.Sprintf("%d", len(sites))+"):", "siteIndex", 101)
	return newModel, cmd
}

func (m Model) showLaravelSiteListForQueue() (tea.Model, tea.Cmd) {
	// Use the actions package to list Laravel sites
	sites, err := actions.ListLaravelSites()
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{
			WarnStyle.Render(fmt.Sprintf("❌ Error scanning for Laravel sites: %v", err)),
			"",
			InfoStyle.Render("Make sure /var/www exists and is accessible"),
		}
		return m, tea.ClearScreen
	}

	if len(sites) == 0 {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{
			WarnStyle.Render("❌ No Laravel sites found in /var/www"),
			"",
			InfoStyle.Render("Create a Laravel site first using 'Create a new Laravel Site'"),
		}
		return m, tea.ClearScreen
	}

	// Build the report showing available sites
	m.State = StateProcessing
	m.ProcessingMsg = ""
	m.Report = []string{
		TitleStyle.Render("🚀 Setup Queue Worker"),
		"",
		InfoStyle.Render("Select a Laravel site to setup queue worker for:"),
		"",
	}

	for i, site := range sites {
		sitePath := fmt.Sprintf("/var/www/%s", site)
		m.Report = append(m.Report,
			InfoStyle.Render(fmt.Sprintf("%d. %s", i+1, site)),
			ChoiceStyle.Render(fmt.Sprintf("   Path: %s", sitePath)),
			"",
		)
	}

	m.Report = append(m.Report,
		InfoStyle.Render("Select a site for queue worker setup:"),
	)

	newModel, cmd := m.startInput("Enter site number (1-"+fmt.Sprintf("%d", len(sites))+"):", "queueSiteIndex", 102)
	return newModel, cmd
}

// showServiceManagement displays the service management interface
func (m Model) showServiceManagement() (tea.Model, tea.Cmd) {
	m.State = StateProcessing
	m.ProcessingMsg = ""
	m.Report = []string{}

	// Get list of active services
	commands, descriptions := actions.ListActiveServices()

	// Execute the command to get active services
	return m.startCommandQueue(commands, descriptions, "list-services")
}

// showServiceList displays the parsed service list and management options
func (m *Model) showServiceList(output string) {
	// Parse the service list output
	services := actions.ParseServiceList(output)

	m.Report = []string{
		TitleStyle.Render("⚙️ Service Management"),
		"",
		InfoStyle.Render("🟢 Active Services:"),
		"",
	}

	if len(services) == 0 {
		m.Report = append(m.Report, WarnStyle.Render("No active services found"))
	} else {
		// Show first 15 services to avoid cluttering
		maxServices := len(services)
		if maxServices > 15 {
			maxServices = 15
		}

		for i := 0; i < maxServices; i++ {
			service := services[i]
			status := "●"
			if service.Active == "active" {
				status = InfoStyle.Render("● ")
			} else {
				status = WarnStyle.Render("● ")
			}

			m.Report = append(m.Report,
				fmt.Sprintf("%s%s (%s - %s)", status, service.Name, service.Status, service.Sub),
			)
		}

		if len(services) > 15 {
			m.Report = append(m.Report, "",
				ChoiceStyle.Render(fmt.Sprintf("... and %d more services", len(services)-15)),
			)
		}
	}

	m.Report = append(m.Report, "",
		InfoStyle.Render("Service Management Options:"),
		"",
		InfoStyle.Render("1. Control a specific service (start/stop/restart/reload)"),
		InfoStyle.Render("2. View detailed service status"),
		InfoStyle.Render("3. Enable/disable service at boot"),
		"",
		ChoiceStyle.Render("Command format:"),
		ChoiceStyle.Render("  c <service-name> <action>  - Control service"),
		ChoiceStyle.Render("  s <service-name>           - Show service status"),
		"",
		ChoiceStyle.Render("Examples: 'c caddy restart', 'c mysql stop', 's php8.4-fpm'"),
		ChoiceStyle.Render("Actions: start, stop, restart, reload, enable, disable, status"),
		"",
		InfoStyle.Render("Press Enter to start interactive service management..."),
	)
}

// controlServiceInteractive starts an interactive service control session
func (m Model) controlServiceInteractive() (tea.Model, tea.Cmd) {
	return m.startInput("Enter service control command (format: 'c service-name action' or 's service-name'):", "serviceControl", 400)
}

// handleServiceControlInput processes service control commands
func (m Model) handleServiceControlInput() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.InputValue)
	if input == "" {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render("❌ Command cannot be empty")}
		return m, tea.ClearScreen
	}

	parts := strings.Fields(input)
	if len(parts) < 2 {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{
			WarnStyle.Render("❌ Invalid command format"),
			"",
			InfoStyle.Render("Usage:"),
			ChoiceStyle.Render("  c <service-name> <action>  - Control service"),
			ChoiceStyle.Render("  s <service-name>           - Show service status"),
			"",
			InfoStyle.Render("Examples:"),
			ChoiceStyle.Render("  c caddy restart"),
			ChoiceStyle.Render("  c mysql stop"),
			ChoiceStyle.Render("  s php8.4-fpm"),
			"",
			InfoStyle.Render("Actions: start, stop, restart, reload, enable, disable, status"),
		}
		return m, tea.ClearScreen
	}

	command := parts[0]
	serviceName := parts[1]

	switch command {
	case "c": // Control service
		if len(parts) < 3 {
			m.State = StateProcessing
			m.ProcessingMsg = ""
			m.Report = []string{
				WarnStyle.Render("❌ Missing action for control command"),
				"",
				InfoStyle.Render("Usage: c <service-name> <action>"),
				InfoStyle.Render("Actions: start, stop, restart, reload, enable, disable, status"),
			}
			return m, tea.ClearScreen
		}

		action := parts[2]
		config := actions.ServiceActionConfig{
			ServiceName: serviceName,
			Action:      action,
		}

		commands, descriptions, err := actions.ControlService(config)
		if err != nil {
			m.State = StateProcessing
			m.ProcessingMsg = ""
			m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
			return m, tea.ClearScreen
		}

		return m.startCommandQueue(commands, descriptions, fmt.Sprintf("service-%s-%s", serviceName, action))

	case "s": // Show service status
		commands, descriptions := actions.GetServiceStatus(serviceName)
		return m.startCommandQueue(commands, descriptions, fmt.Sprintf("service-status-%s", serviceName))

	default:
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{
			WarnStyle.Render(fmt.Sprintf("❌ Unknown command: %s", command)),
			"",
			InfoStyle.Render("Available commands:"),
			ChoiceStyle.Render("  c - Control service (start, stop, restart, etc.)"),
			ChoiceStyle.Render("  s - Show service status"),
		}
		return m, tea.ClearScreen
	}
}

// updateServiceList handles input in service list state
func (m Model) updateServiceList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Update list dimensions to fit the terminal
		m.ServiceList.SetWidth(msg.Width)
		m.ServiceList.SetHeight(msg.Height - 4) // Reserve space for title and help
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			// Return to Server Management menu
			return m.returnToServerManagement()
		case "enter":
			// Get selected service and show action menu
			if selectedItem, ok := m.ServiceList.SelectedItem().(ServiceItem); ok {
				return m.showServiceActions(selectedItem.ServiceInfo)
			}
		case "s":
			// Show service status
			if selectedItem, ok := m.ServiceList.SelectedItem().(ServiceItem); ok {
				commands, descriptions := actions.GetServiceStatus(selectedItem.ServiceInfo.Name)
				return m.startCommandQueue(commands, descriptions, fmt.Sprintf("service-status-%s", selectedItem.ServiceInfo.Name))
			}
		case "r":
			// Restart service
			if selectedItem, ok := m.ServiceList.SelectedItem().(ServiceItem); ok {
				config := actions.ServiceActionConfig{
					ServiceName: selectedItem.ServiceInfo.Name,
					Action:      "restart",
				}
				commands, descriptions, err := actions.ControlService(config)
				if err != nil {
					m.State = StateProcessing
					m.ProcessingMsg = ""
					m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
					return m, tea.ClearScreen
				}
				return m.startCommandQueue(commands, descriptions, fmt.Sprintf("service-%s-restart", selectedItem.ServiceInfo.Name))
			}
		case "t":
			// Stop service
			if selectedItem, ok := m.ServiceList.SelectedItem().(ServiceItem); ok {
				config := actions.ServiceActionConfig{
					ServiceName: selectedItem.ServiceInfo.Name,
					Action:      "stop",
				}
				commands, descriptions, err := actions.ControlService(config)
				if err != nil {
					m.State = StateProcessing
					m.ProcessingMsg = ""
					m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
					return m, tea.ClearScreen
				}
				return m.startCommandQueue(commands, descriptions, fmt.Sprintf("service-%s-stop", selectedItem.ServiceInfo.Name))
			}
		case "a":
			// Start service
			if selectedItem, ok := m.ServiceList.SelectedItem().(ServiceItem); ok {
				config := actions.ServiceActionConfig{
					ServiceName: selectedItem.ServiceInfo.Name,
					Action:      "start",
				}
				commands, descriptions, err := actions.ControlService(config)
				if err != nil {
					m.State = StateProcessing
					m.ProcessingMsg = ""
					m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
					return m, tea.ClearScreen
				}
				return m.startCommandQueue(commands, descriptions, fmt.Sprintf("service-%s-start", selectedItem.ServiceInfo.Name))
			}
		}
	}

	// Update the list
	var cmd tea.Cmd
	m.ServiceList, cmd = m.ServiceList.Update(msg)
	return m, cmd
}

// viewServiceList renders the service list
func (m Model) viewServiceList() string {
	return TitleStyle.Render("⚙️ Service Management") + "\n" + m.ServiceList.View()
}

// returnToServerManagement returns to the Server Management menu
func (m Model) returnToServerManagement() (tea.Model, tea.Cmd) {
	m.State = StateSubmenu
	m.CurrentMenu = MenuServerManagement
	m.Choices = []string{
		"Backup MySQL Database",
		"System Status",
		"View Installation Logs",
		"Service Management",
		"Back to Main Menu",
	}
	m.Cursor = 3 // Position cursor on Service Management
	return m, tea.ClearScreen
}

// showServiceActions shows available actions for a service
func (m Model) showServiceActions(service actions.ServiceInfo) (tea.Model, tea.Cmd) {
	m.State = StateProcessing
	m.ProcessingMsg = ""
	m.Report = []string{
		TitleStyle.Render(fmt.Sprintf("🔧 Service: %s", service.Name)),
		"",
		InfoStyle.Render(fmt.Sprintf("Status: %s", service.Status)),
		InfoStyle.Render(fmt.Sprintf("Active: %s", service.Active)),
		InfoStyle.Render(fmt.Sprintf("Sub-state: %s", service.Sub)),
		"",
		InfoStyle.Render("Available Actions:"),
		ChoiceStyle.Render("  s - Show detailed status"),
		ChoiceStyle.Render("  r - Restart service"),
		ChoiceStyle.Render("  t - Stop service"),
		ChoiceStyle.Render("  a - Start service"),
		"",
		InfoStyle.Render("Press the corresponding key or Esc to go back"),
	}
	return m, tea.ClearScreen
}

// createServiceList creates and initializes the service list
func (m *Model) createServiceList(services []actions.ServiceInfo) {
	// Convert services to list items
	items := make([]list.Item, len(services))
	for i, service := range services {
		items[i] = ServiceItem{ServiceInfo: service}
	}

	// Create list with custom styling
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.SetHeight(2)

	// Set reasonable default dimensions - will be updated in Update() with actual window size
	listWidth := 80
	listHeight := 20
	m.ServiceList = list.New(items, delegate, listWidth, listHeight)
	m.ServiceList.Title = "Active Services"
	m.ServiceList.SetShowStatusBar(true)
	m.ServiceList.SetShowPagination(true)
	m.ServiceList.SetShowHelp(true)

	// Add custom key bindings help
	m.ServiceList.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "show status")),
			key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "restart")),
			key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "stop")),
			key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "start")),
		}
	}

	m.Services = services
}

// showServiceStatusResults shows the status results and returns to service list
func (m *Model) showServiceStatusResults(serviceName, output string) {
	m.State = StateProcessing
	m.ProcessingMsg = ""
	m.ReturnToServiceList = true // Flag to return to service list
	m.Report = []string{
		TitleStyle.Render(fmt.Sprintf("📊 Service Status: %s", serviceName)),
		"",
		InfoStyle.Render("Service Status Details:"),
		"",
	}

	// Add the status output
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			m.Report = append(m.Report, ChoiceStyle.Render(line))
		}
	}

	m.Report = append(m.Report, "",
		InfoStyle.Render("Press any key to return to service list..."),
	)
}

// showServiceActionResults shows the action results and returns to service list
func (m *Model) showServiceActionResults(serviceName, action, output string) {
	m.State = StateProcessing
	m.ProcessingMsg = ""
	m.ReturnToServiceList = true // Flag to return to service list

	// Determine icon based on action
	actionIcon := "⚙️"
	actionDesc := action
	switch action {
	case "start":
		actionIcon = "▶️"
		actionDesc = "Started"
	case "stop":
		actionIcon = "⏹️"
		actionDesc = "Stopped"
	case "restart":
		actionIcon = "🔄"
		actionDesc = "Restarted"
	}

	m.Report = []string{
		TitleStyle.Render(fmt.Sprintf("%s Service %s: %s", actionIcon, actionDesc, serviceName)),
		"",
		InfoStyle.Render("Command executed successfully!"),
		"",
	}

	// Add any output if available
	if strings.TrimSpace(output) != "" {
		m.Report = append(m.Report, InfoStyle.Render("Output:"))
		lines := strings.Split(strings.TrimSpace(output), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				m.Report = append(m.Report, ChoiceStyle.Render(line))
			}
		}
		m.Report = append(m.Report, "")
	}

	m.Report = append(m.Report,
		InfoStyle.Render("Press any key to return to service list..."),
	)
}

// Monitoring helper functions

// checkMonitoringAgent checks if the monitoring agent is running
func (m Model) checkMonitoringAgent() string {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9090/api/v1/health")
	if err != nil {
		return WarnStyle.Render("❌ Agent not running")
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return InfoStyle.Render("✅ Agent running on port 9090")
	}
	return WarnStyle.Render("❌ Agent unhealthy")
}

// fetchSystemMetrics fetches system metrics from the monitoring agent
func (m Model) fetchSystemMetrics() []string {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9090/api/v1/metrics/system")
	if err != nil {
		return []string{WarnStyle.Render("❌ Failed to fetch system metrics")}
	}
	defer resp.Body.Close()

	var metrics monitor.SystemMetrics
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		return []string{WarnStyle.Render("❌ Failed to parse system metrics")}
	}

	var result []string

	// CPU metrics
	result = append(result, fmt.Sprintf("  CPU Usage: %.1f%% (User: %.1f%%, System: %.1f%%, I/O Wait: %.1f%%)",
		metrics.CPU.UsagePercent, metrics.CPU.UserPercent, metrics.CPU.SystemPercent, metrics.CPU.IOWaitPercent))

	// Memory metrics
	memUsedGB := float64(metrics.Memory.UsedBytes) / (1024 * 1024 * 1024)
	memTotalGB := float64(metrics.Memory.TotalBytes) / (1024 * 1024 * 1024)
	result = append(result, fmt.Sprintf("  Memory: %.1fGB/%.1fGB (%.1f%%) | Swap: %.1f%%",
		memUsedGB, memTotalGB, metrics.Memory.UsagePercent, metrics.Memory.SwapUsagePercent))

	// Load average
	result = append(result, fmt.Sprintf("  Load Average: %.2f, %.2f, %.2f",
		metrics.Load.Load1, metrics.Load.Load5, metrics.Load.Load15))

	// Disk usage for main partitions
	for _, disk := range metrics.Disk {
		if disk.MountPoint == "/" || disk.MountPoint == "/home" {
			usedGB := float64(disk.UsedBytes) / (1024 * 1024 * 1024)
			totalGB := float64(disk.TotalBytes) / (1024 * 1024 * 1024)
			result = append(result, fmt.Sprintf("  Disk %s: %.1fGB/%.1fGB (%.1f%%)",
				disk.MountPoint, usedGB, totalGB, disk.UsagePercent))
		}
	}

	// Network stats (top interfaces)
	for i, net := range metrics.Network {
		if i >= 2 { // Limit to top 2 interfaces
			break
		}
		recvMB := float64(net.BytesRecv) / (1024 * 1024)
		sentMB := float64(net.BytesSent) / (1024 * 1024)
		result = append(result, fmt.Sprintf("  Network %s: ↓%.1fMB ↑%.1fMB (Errors: %d)",
			net.Interface, recvMB, sentMB, net.ErrorsRecv+net.ErrorsSent))
	}

	return result
}

// fetchServiceMetrics fetches service status from the monitoring agent
func (m Model) fetchServiceMetrics() []string {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9090/api/v1/metrics/services")
	if err != nil {
		return []string{WarnStyle.Render("❌ Failed to fetch service metrics")}
	}
	defer resp.Body.Close()

	var services []monitor.ServiceStatus
	if err := json.NewDecoder(resp.Body).Decode(&services); err != nil {
		return []string{WarnStyle.Render("❌ Failed to parse service metrics")}
	}

	var result []string

	// Group services by category
	categories := map[string][]monitor.ServiceStatus{
		"database":  {},
		"webserver": {},
		"runtime":   {},
		"security":  {},
		"system":    {},
	}

	for _, service := range services {
		category := "system"
		if cat, exists := service.Metadata["category"]; exists {
			category = cat
		}
		categories[category] = append(categories[category], service)
	}

	// Display important categories first
	for _, category := range []string{"database", "webserver", "runtime", "security"} {
		if len(categories[category]) > 0 {
			for _, service := range categories[category] {
				status := "❌"
				if service.Active == "active" && service.Sub == "running" {
					status = "✅"
				} else if service.Active == "active" {
					status = "⚠️"
				}

				uptime := time.Since(service.Since)
				result = append(result, fmt.Sprintf("  %s %s (%s) - Up: %s",
					status, service.Name, service.Sub, formatDuration(uptime)))
			}
		}
	}

	// Show count of other services
	otherCount := len(categories["system"])
	if otherCount > 0 {
		result = append(result, fmt.Sprintf("  + %d other system services", otherCount))
	}

	return result
}

// fetchHTTPMetrics fetches HTTP check results from the monitoring agent
func (m Model) fetchHTTPMetrics() []string {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9090/api/v1/metrics/http")
	if err != nil {
		return []string{WarnStyle.Render("❌ Failed to fetch HTTP metrics")}
	}
	defer resp.Body.Close()

	var checks []monitor.HTTPCheckResult
	if err := json.NewDecoder(resp.Body).Decode(&checks); err != nil {
		return []string{WarnStyle.Render("❌ Failed to parse HTTP metrics")}
	}

	if len(checks) == 0 {
		return []string{
			ChoiceStyle.Render("  No HTTP checks configured"),
			ChoiceStyle.Render("  Enable in configs/monitor.yaml to monitor web endpoints"),
		}
	}

	var result []string
	for _, check := range checks {
		status := "❌"
		if check.Success {
			status = "✅"
		}

		result = append(result, fmt.Sprintf("  %s %s - %dms (Status: %d)",
			status, check.Name, check.ResponseTime.Milliseconds(), check.StatusCode))

		if check.Error != "" {
			// Simplify connection refused errors
			errorMsg := check.Error
			if strings.Contains(errorMsg, "connection refused") {
				errorMsg = "Connection refused - service not running"
			} else if strings.Contains(errorMsg, "no such host") {
				errorMsg = "Host not found"
			} else if strings.Contains(errorMsg, "timeout") {
				errorMsg = "Request timeout"
			}
			result = append(result, fmt.Sprintf("    %s", errorMsg))
		}
	}

	return result
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	} else {
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
