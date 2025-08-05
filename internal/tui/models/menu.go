package models

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// MenuItem represents a single menu item
type MenuItem struct {
	Label       string
	Action      MenuAction
	ServiceKey  string // For service status tracking
	Enabled     bool
}

// MenuAction represents the action to take when a menu item is selected
type MenuAction int

const (
	ActionNavigate MenuAction = iota
	ActionExecute
	ActionQuit
	ActionBack
)

// MenuModel handles menu navigation and display
type MenuModel struct {
	BaseModel
	title         string
	items         []MenuItem
	cursor        int
	level         MenuLevel
	showIcons     bool
	menuStack     []MenuLevel // Internal navigation stack for menu levels
}

// Styles are now centralized in styles.go

// NewMenuModel creates a new menu model
func NewMenuModel(shared *SharedData) *MenuModel {
	menu := &MenuModel{
		BaseModel: NewBaseModel(shared),
		level:     MenuMain,
		showIcons: true,
	}
	menu.setupMainMenu()
	// Initialize service status on creation
	menu.refreshServiceStatus()
	return menu
}

// Init initializes the menu model
func (m *MenuModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the menu model
func (m *MenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.level == MenuMain {
				return m, tea.Quit
			}
			// Handle quit differently in submenus
			return m, m.GoBack()

		case "esc":
			if m.level != MenuMain {
				// Use internal menu navigation for submenus
				m.popMenuLevel()
				return m, tea.ClearScreen
			}

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}

		case "enter", " ":
			return m.handleSelection()

		case "r", "R":
			// Refresh service status
			m.refreshServiceStatus()
			return m, nil
		}
	}

	return m, nil
}

// View renders the menu
func (m *MenuModel) View() string {
	s := titleStyle.Render(m.title) + "\n\n"

	for i, item := range m.items {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}

		// Determine the display text
		displayText := item.Label
		if m.cursor == i {
			displayText = selectedStyle.Render(displayText)
		} else {
			displayText = choiceStyle.Render(displayText)
		}

		// Add service status icon if applicable
		serviceIcon := ""
		if m.showIcons && item.ServiceKey != "" {
			if installed, exists := m.shared.ServiceStatus[item.ServiceKey]; exists {
				if installed {
					serviceIcon = "✅ "
				} else {
					serviceIcon = "⬜ "
				}
			}
		}

		s += fmt.Sprintf("%s %s%s\n", cursor, serviceIcon, displayText)
	}

	// Add help text
	if m.level == MenuMain {
		s += "\nPress q to quit, Enter to select, r to refresh.\n"
	} else {
		s += "\nPress Enter to select, Esc to go back, r to refresh.\n"
	}

	return s
}

// handleSelection handles menu item selection
func (m *MenuModel) handleSelection() (tea.Model, tea.Cmd) {
	if m.cursor >= len(m.items) {
		return m, nil
	}

	selectedItem := m.items[m.cursor]

	switch selectedItem.Action {
	case ActionQuit:
		return m, tea.Quit

	case ActionBack:
		// Use internal menu navigation for "Back to Main Menu"
		m.popMenuLevel()
		return m, tea.ClearScreen

	case ActionNavigate:
		return m.handleNavigation(selectedItem)

	case ActionExecute:
		return m.handleExecution(selectedItem)
	}

	return m, nil
}

// handleNavigation handles navigation to different states/menus
func (m *MenuModel) handleNavigation(item MenuItem) (tea.Model, tea.Cmd) {
	switch m.level {
	case MenuMain:
		return m.handleMainMenuNavigation(item)
	case MenuCoreServices:
		return m.handleCoreServicesNavigation(item)
	case MenuLaravelManagement:
		return m.handleLaravelNavigation(item)
	case MenuServerManagement:
		return m.handleServerNavigation(item)
	case MenuSettings:
		return m.handleSettingsNavigation(item)
	}
	return m, nil
}

// handleMainMenuNavigation handles main menu navigation
func (m *MenuModel) handleMainMenuNavigation(item MenuItem) (tea.Model, tea.Cmd) {
	switch item.Label {
	case "Core Services (PHP, Node, Caddy, etc.)":
		// Store current menu level in navigation stack
		m.pushMenuLevel(MenuMain)
		m.setupCoreServicesMenu()
		return m, tea.ClearScreen

	case "Laravel Management":
		// Store current menu level in navigation stack  
		m.pushMenuLevel(MenuMain)
		m.setupLaravelManagementMenu()
		return m, tea.ClearScreen

	case "Next.js Management":
		return m, m.NavigateTo(StateNextJSMenu, nil)

	case "Server Management":
		// Store current menu level in navigation stack
		m.pushMenuLevel(MenuMain)
		m.setupServerManagementMenu()
		return m, tea.ClearScreen

	case "Monitoring Dashboard":
		return m, m.NavigateTo(StateMonitoring, nil)

	case "Settings":
		// Store current menu level in navigation stack
		m.pushMenuLevel(MenuMain)
		m.setupSettingsMenu()
		return m, tea.ClearScreen
	}
	return m, nil
}

// handleCoreServicesNavigation handles core services menu navigation
func (m *MenuModel) handleCoreServicesNavigation(item MenuItem) (tea.Model, tea.Cmd) {
	// This would trigger specific installation processes
	// For now, we'll navigate to processing state with the service name
	return m, m.NavigateTo(StateProcessing, map[string]interface{}{
		"action":  "install",
		"service": item.ServiceKey,
		"label":   item.Label,
	})
}

// handleLaravelNavigation handles Laravel management navigation
func (m *MenuModel) handleLaravelNavigation(item MenuItem) (tea.Model, tea.Cmd) {
	switch item.Label {
	case "Create a new Laravel Site":
		return m, m.NavigateTo(StateLaravelCreate, nil)
	case "Update Laravel Site":
		return m, m.NavigateTo(StateProcessing, map[string]interface{}{
			"action": "laravel-list",
		})
	case "Setup Laravel Queue Worker":
		return m, m.NavigateTo(StateProcessing, map[string]interface{}{
			"action": "laravel-queue",
		})
	case "GitHub Authentication":
		return m, m.NavigateTo(StateProcessing, map[string]interface{}{
			"action": "github-auth",
		})
	}
	return m, nil
}

// handleServerNavigation handles server management navigation
func (m *MenuModel) handleServerNavigation(item MenuItem) (tea.Model, tea.Cmd) {
	switch item.Label {
	case "Backup MySQL Database":
		return m, m.NavigateTo(StateInput, map[string]interface{}{
			"prompt": "Enter database name:",
			"field":  "dbName",
		})
	case "System Status":
		return m, m.NavigateTo(StateProcessing, map[string]interface{}{
			"action": "system-status",
		})
	case "View Installation Logs":
		return m, m.NavigateTo(StateLogViewer, nil)
	case "Service Management":
		return m, m.NavigateTo(StateServiceList, nil)
	case "Monitoring Dashboard":
		return m, m.NavigateTo(StateMonitoring, nil)
	}
	return m, nil
}

// handleSettingsNavigation handles settings navigation
func (m *MenuModel) handleSettingsNavigation(item MenuItem) (tea.Model, tea.Cmd) {
	// Handle settings-specific navigation
	return m, m.NavigateTo(StateProcessing, map[string]interface{}{
		"action": "settings",
		"item":   item.Label,
	})
}

// handleExecution handles direct execution of actions
func (m *MenuModel) handleExecution(item MenuItem) (tea.Model, tea.Cmd) {
	// This would be used for actions that don't require navigation
	// but execute directly (like refresh, etc.)
	return m, nil
}

// setupMainMenu configures the main menu
func (m *MenuModel) setupMainMenu() {
	m.title = "🔧 Crucible - Server Setup made easy for Laravel and Python"
	m.level = MenuMain
	m.cursor = 0
	m.items = []MenuItem{
		{Label: "Core Services (PHP, Node, Caddy, etc.)", Action: ActionNavigate},
		{Label: "Laravel Management", Action: ActionNavigate},
		{Label: "Next.js Management", Action: ActionNavigate},
		{Label: "Server Management", Action: ActionNavigate},
		{Label: "Monitoring Dashboard", Action: ActionNavigate},
		{Label: "Settings", Action: ActionNavigate},
		{Label: "Exit", Action: ActionQuit},
	}
}

// setupCoreServicesMenu configures the core services menu
func (m *MenuModel) setupCoreServicesMenu() {
	m.title = "🔧 Core Services"
	m.level = MenuCoreServices
	m.cursor = 0
	m.items = []MenuItem{
		{Label: "Install PHP 8.4", Action: ActionNavigate, ServiceKey: "php"},
		{Label: "Install PHP Composer", Action: ActionNavigate, ServiceKey: "composer"},
		{Label: "Install Python, pip, and virtualenv", Action: ActionNavigate, ServiceKey: "python"},
		{Label: "Install Node.js and npm", Action: ActionNavigate, ServiceKey: "nodejs"},
		{Label: "Install MySQL", Action: ActionNavigate, ServiceKey: "mysql"},
		{Label: "Install Caddy Server (recommended)", Action: ActionNavigate, ServiceKey: "caddy"},
		{Label: "Install Supervisor (recommended)", Action: ActionNavigate, ServiceKey: "supervisor"},
		{Label: "Install Git CLI (recommended)", Action: ActionNavigate, ServiceKey: "git"},
		{Label: "Back to Main Menu", Action: ActionBack},
	}
}

// setupLaravelManagementMenu configures the Laravel management menu
func (m *MenuModel) setupLaravelManagementMenu() {
	m.title = "🚀 Laravel Management"
	m.level = MenuLaravelManagement
	m.cursor = 0
	m.items = []MenuItem{
		{Label: "Create a new Laravel Site", Action: ActionNavigate},
		{Label: "Update Laravel Site", Action: ActionNavigate},
		{Label: "Setup Laravel Queue Worker", Action: ActionNavigate},
		{Label: "GitHub Authentication", Action: ActionNavigate},
		{Label: "Back to Main Menu", Action: ActionBack},
	}
}

// setupServerManagementMenu configures the server management menu
func (m *MenuModel) setupServerManagementMenu() {
	m.title = "⚙️ Server Management"
	m.level = MenuServerManagement
	m.cursor = 0
	m.items = []MenuItem{
		{Label: "Backup MySQL Database", Action: ActionNavigate},
		{Label: "System Status", Action: ActionNavigate},
		{Label: "View Installation Logs", Action: ActionNavigate},
		{Label: "Service Management", Action: ActionNavigate},
		{Label: "Monitoring Dashboard", Action: ActionNavigate},
		{Label: "Back to Main Menu", Action: ActionBack},
	}
}

// setupSettingsMenu configures the settings menu
func (m *MenuModel) setupSettingsMenu() {
	m.title = "⚙️ Settings"
	m.level = MenuSettings
	m.cursor = 0
	m.items = []MenuItem{
		{Label: "Email Alert Configuration", Action: ActionNavigate},
		{Label: "API Keys Management", Action: ActionNavigate},
		{Label: "Test Email Notifications", Action: ActionNavigate},
		{Label: "View Current Settings", Action: ActionNavigate},
		{Label: "Reset to Defaults", Action: ActionNavigate},
		{Label: "Back to Main Menu", Action: ActionBack},
	}
}

// refreshServiceStatus refreshes the service installation status
func (m *MenuModel) refreshServiceStatus() {
	if m.shared.ServiceStatus == nil {
		m.shared.ServiceStatus = make(map[string]bool)
	}
	
	// Check actual service/software installation status
	serviceChecks := map[string]func() bool{
		"php":        func() bool { return m.checkCommand("php", "--version") || m.checkSystemdService("php-fpm") },
		"composer":   func() bool { return m.checkCommand("composer", "--version") },
		"python":     func() bool { return m.checkCommand("python3", "--version") || m.checkCommand("python", "--version") },
		"nodejs":     func() bool { return m.checkCommand("node", "--version") || m.checkCommand("nodejs", "--version") },
		"mysql":      func() bool { return m.checkSystemdService("mysqld") || m.checkSystemdService("mysql") || m.checkCommand("mysql", "--version") },
		"caddy":      func() bool { return m.checkSystemdService("caddy") || m.checkCommand("caddy", "version") },
		"supervisor": func() bool { return m.checkSystemdService("supervisor") || m.checkSystemdService("supervisord") || m.checkCommand("supervisorctl", "version") },
		"git":        func() bool { return m.checkCommand("git", "--version") },
	}
	
	for service, checkFunc := range serviceChecks {
		m.shared.ServiceStatus[service] = checkFunc()
	}
}

// checkCommand checks if a command is available and working
func (m *MenuModel) checkCommand(command string, args ...string) bool {
	cmd := exec.Command(command, args...)
	err := cmd.Run()
	return err == nil
}

// checkSystemdService checks if a systemd service exists (installed, not necessarily running)
func (m *MenuModel) checkSystemdService(serviceName string) bool {
	// Check if service unit file exists
	cmd := exec.Command("systemctl", "list-unit-files", serviceName+".service", "--no-pager")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	
	// If the service is listed, it exists
	return strings.Contains(string(output), serviceName+".service")
}

// SetLevel allows external setting of menu level (for navigation)
func (m *MenuModel) SetLevel(level MenuLevel) {
	switch level {
	case MenuMain:
		m.setupMainMenu()
	case MenuCoreServices:
		m.setupCoreServicesMenu()
	case MenuLaravelManagement:
		m.setupLaravelManagementMenu()
	case MenuServerManagement:
		m.setupServerManagementMenu()
	case MenuSettings:
		m.setupSettingsMenu()
	}
}

// Initialize implements the ModelInitializer interface
func (m *MenuModel) Initialize(data interface{}) {
	if levelData, ok := data.(MenuLevel); ok {
		m.SetLevel(levelData)
	}
}

// pushMenuLevel adds a menu level to the internal navigation stack
func (m *MenuModel) pushMenuLevel(level MenuLevel) {
	m.menuStack = append(m.menuStack, level)
}

// popMenuLevel returns to the previous menu level
func (m *MenuModel) popMenuLevel() {
	if len(m.menuStack) == 0 {
		return
	}
	
	// Pop the last level from the stack
	prevLevel := m.menuStack[len(m.menuStack)-1]
	m.menuStack = m.menuStack[:len(m.menuStack)-1]
	
	// Set the previous menu level
	m.SetLevel(prevLevel)
}