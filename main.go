package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

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
)

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
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	m := model{
		choices: []string{
			"Install PHP 8.4",
			"Upgrade to PHP 8.5",
			"Install PHP Composer",
			"Install MySQL",
			"Install Caddy Server",
			"Install Git CLI",
			"Create New Laravel Site",
			"Update Laravel Site",
			"Backup MySQL Database",
			"System Status",
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
			// Cancel input and return to menu
			m.state = stateMenu
			m.inputValue = ""
			m.inputPrompt = ""
			m.cursor = 0 // Reset cursor when canceling input
			return m, nil
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
			// Return to main menu after processing and refresh service status
			m.state = stateMenu
			m.formData = make(map[string]string)
			m.processingMsg = ""
			m.report = []string{} // Clear report
			m.cursor = 0          // Reset cursor to top of menu
			modelPtr := &m
			modelPtr.checkServiceInstallations() // Refresh all service statuses
			return *modelPtr, nil
		}
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
		return m.installPHP()
	case 1: // Upgrade to PHP 8.5
		return m.upgradeToPHP85()
	case 2: // Install PHP Composer
		return m.installComposer()
	case 3: // Install MySQL
		return m.installMySQL()
	case 4: // Install Caddy Server
		return m.installCaddy()
	case 5: // Install Git CLI
		return m.installGit()
	case 6: // Create New Laravel Site
		return m.startInput("Enter site name (e.g., myapp):", "siteName", 6)
	case 7: // Update Laravel Site
		return m.startInput("Select site number:", "siteIndex", 7)
	case 8: // Backup MySQL Database
		return m.startInput("Enter database name:", "dbName", 8)
	case 9: // System Status
		return m.showSystemStatus()
	}

	return m, nil
}

func (m model) processFormInput() (tea.Model, tea.Cmd) {
	// This function handles the form input flow for different actions
	switch m.currentAction {
	case 6: // Create New Laravel Site
		return m.handleLaravelSiteForm()
	case 7: // Update Laravel Site
		return m.handleUpdateSiteForm()
	case 8: // Backup MySQL Database
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
	return m, nil
}

func (m model) startProcessingWithMessage(message string) (tea.Model, tea.Cmd) {
	m.state = stateProcessing
	m.processingMsg = message
	return m, m.spinner.Tick
}

func (m *model) checkServiceInstallations() {
	// Check PHP installation
	m.serviceStatus["php"] = m.isServiceInstalled("php", "--version")

	// Check Composer installation
	m.serviceStatus["composer"] = m.isServiceInstalled("composer", "--version")

	// Check MySQL installation
	m.serviceStatus["mysql"] = m.isServiceInstalled("mysql", "--version")

	// Check Caddy installation
	m.serviceStatus["caddy"] = m.isServiceInstalled("caddy", "version")

	// Check Git installation
	m.serviceStatus["git"] = m.isServiceInstalled("git", "--version")
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
	}
	return ""
}

func (m model) viewMenu() string {
	s := titleStyle.Render("🔧 Crucible - Laravel Server Setup") + "\n\n"

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
		case 3: // Install MySQL
			serviceIcon = m.getServiceIcon("mysql") + " "
		case 4: // Install Caddy Server
			serviceIcon = m.getServiceIcon("caddy") + " "
		case 5: // Install Git CLI
			serviceIcon = m.getServiceIcon("git") + " "
		}

		if m.cursor == i {
			cursor = ">"
			choice = selectedStyle.Render(serviceIcon + choice)
		} else {
			choice = choiceStyle.Render(serviceIcon + choice)
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

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}
