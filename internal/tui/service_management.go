package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"crucible/internal/actions"
)

// Service management functions

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
		"Monitoring Dashboard",
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
