package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Menu navigation and handling functions

// updateMenu handles input in main menu state
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
	case 3: // Monitoring Dashboard
		newModel, cmd := m.showMonitoringDashboard()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	case 4: // Settings
		m.State = StateSubmenu
		m.CurrentMenu = MenuSettings
		m.Choices = []string{
			"Email Alert Configuration",
			"API Keys Management",
			"Test Email Notifications",
			"View Current Settings",
			"Reset to Defaults",
			"Back to Main Menu",
		}
		m.Cursor = 0
		return m, tea.ClearScreen
	case 5: // Exit
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
		"Core Services (PHP, Node, Caddy, etc.)",
		"Laravel Management",
		"Server Management",
		"Monitoring Dashboard",
		"Settings",
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
	case MenuSettings:
		return m.handleSettingsSelection()
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
		return m.startInput("Install PM2 for Next.js applications? (y/n, optional):", "nodePM2", 204)
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
