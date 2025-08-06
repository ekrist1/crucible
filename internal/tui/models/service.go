package models

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"crucible/internal/actions"
)

// Service message types
type servicesLoadedMsg struct {
	services []actions.ServiceInfo
	err      error
}

// ServiceModel handles service management
type ServiceModel struct {
	BaseModel
	serviceList list.Model
	services    []actions.ServiceInfo
	message     string
	loading     bool
}

// ServiceItem wraps ServiceInfo for the list component
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

// NewServiceModel creates a new service management model
func NewServiceModel(shared *SharedData) *ServiceModel {
	// Create list with default items
	items := []list.Item{}
	// Use terminal dimensions from shared data
	width := shared.GetContentWidth()
	height := shared.GetViewableLines()
	if height < 10 {
		height = 10 // Minimum height for service list
	}

	serviceList := list.New(items, list.NewDefaultDelegate(), width, height)
	serviceList.Title = "System Services"
	serviceList.SetShowStatusBar(false)
	serviceList.SetFilteringEnabled(false)
	serviceList.SetShowHelp(false)

	return &ServiceModel{
		BaseModel:   NewBaseModel(shared),
		serviceList: serviceList,
		services:    []actions.ServiceInfo{},
		message:     "",
		loading:     false,
	}
}

// Init initializes the service model
func (m *ServiceModel) Init() tea.Cmd {
	return m.loadServices()
}

// Update handles service management updates
func (m *ServiceModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle global keys first
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			return m, m.GoBack()
		case "r":
			// Refresh services
			m.loading = true
			m.message = "Loading services..."
			return m, m.loadServices()
		}

		// Handle service-specific actions
		switch msg.String() {
		case "s":
			// Show service status
			if selectedItem, ok := m.serviceList.SelectedItem().(ServiceItem); ok {
				return m.showServiceStatus(selectedItem.ServiceInfo)
			}
		case "t":
			// Stop service
			if selectedItem, ok := m.serviceList.SelectedItem().(ServiceItem); ok {
				return m.performServiceAction(selectedItem.ServiceInfo, "stop")
			}
		case "a":
			// Start service
			if selectedItem, ok := m.serviceList.SelectedItem().(ServiceItem); ok {
				return m.performServiceAction(selectedItem.ServiceInfo, "start")
			}
		case "e":
			// Restart service
			if selectedItem, ok := m.serviceList.SelectedItem().(ServiceItem); ok {
				return m.performServiceAction(selectedItem.ServiceInfo, "restart")
			}
		case "enter", " ":
			// Show service details
			if selectedItem, ok := m.serviceList.SelectedItem().(ServiceItem); ok {
				return m.showServiceDetails(selectedItem.ServiceInfo)
			}
		}

	case servicesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.message = fmt.Sprintf("Error loading services: %v", msg.err)
		} else {
			m.services = msg.services
			m.message = fmt.Sprintf("Loaded %d services", len(msg.services))
			m.updateServiceList()
		}
		return m, nil

	case tea.WindowSizeMsg:
		// Update shared terminal size
		m.shared.SetTerminalSize(msg.Width, msg.Height)
		// Update list size with proper calculations
		contentWidth := m.shared.GetContentWidth()
		viewableHeight := m.shared.GetViewableLines()
		if viewableHeight < 5 {
			viewableHeight = 5 // Minimum height
		}
		m.serviceList.SetWidth(contentWidth)
		m.serviceList.SetHeight(viewableHeight)
	}

	// Update the list
	m.serviceList, cmd = m.serviceList.Update(msg)
	return m, cmd
}

// View renders the service management interface
func (m *ServiceModel) View() string {
	var s strings.Builder

	// Title
	s.WriteString(titleStyle.Render("⚙️ Service Management"))
	s.WriteString("\n\n")

	// Status message
	if m.message != "" {
		if strings.Contains(m.message, "Error") {
			s.WriteString(errorStyle.Render(m.message))
		} else {
			s.WriteString(infoStyle.Render(m.message))
		}
		s.WriteString("\n\n")
	}

	// Loading indicator
	if m.loading {
		s.WriteString("Loading services...\n\n")
	}

	// Service list
	s.WriteString(m.serviceList.View())
	s.WriteString("\n")

	// Help text
	help := []string{
		"Actions: s=Status, a=Start, t=Stop, e=Restart",
		"Navigation: ↑/↓=Select, Enter=Details, r=Refresh",
		"Esc=Back to menu, q=Quit",
	}
	s.WriteString(helpStyle.Render(strings.Join(help, " | ")))

	return s.String()
}

// loadServices loads the list of system services
func (m *ServiceModel) loadServices() tea.Cmd {
	return func() tea.Msg {
		services, err := actions.GetSystemServices()
		return servicesLoadedMsg{services: services, err: err}
	}
}

// updateServiceList updates the list component with current services
func (m *ServiceModel) updateServiceList() {
	items := make([]list.Item, len(m.services))
	for i, service := range m.services {
		items[i] = ServiceItem{ServiceInfo: service}
	}
	m.serviceList.SetItems(items)
}

// showServiceStatus shows detailed status for a service
func (m *ServiceModel) showServiceStatus(service actions.ServiceInfo) (tea.Model, tea.Cmd) {
	// Navigate to processing state to show detailed service status
	return m, m.NavigateTo(StateProcessing, map[string]interface{}{
		"action":  "service-status",
		"service": service.Name,
	})
}

// performServiceAction performs an action on a service
func (m *ServiceModel) performServiceAction(service actions.ServiceInfo, action string) (tea.Model, tea.Cmd) {
	// Navigate to processing state to execute the service action
	return m, m.NavigateTo(StateProcessing, map[string]interface{}{
		"action":        "service-control",
		"service":       service.Name,
		"serviceAction": action,
	})
}

// showServiceDetails shows detailed information about a service
func (m *ServiceModel) showServiceDetails(service actions.ServiceInfo) (tea.Model, tea.Cmd) {
	// Navigate to processing state to show service details
	return m, m.NavigateTo(StateProcessing, map[string]interface{}{
		"action":  "service-details",
		"service": service,
	})
}
