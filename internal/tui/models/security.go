package models

import (
	"fmt"
	"strconv"
	"strings"

	"crucible/internal/actions"
	tea "github.com/charmbracelet/bubbletea"
)

// SecurityModel handles security management interface
type SecurityModel struct {
	BaseModel
	currentView SecurityView
	menuItems   []MenuItem
	cursor      int
}

// SecurityView represents different security views
type SecurityView int

const (
	SecurityViewMain SecurityView = iota
	SecurityViewAssessment
	SecurityViewHardening
	SecurityViewStatus
)

// NewSecurityModel creates a new security management model
func NewSecurityModel(shared *SharedData) *SecurityModel {
	model := &SecurityModel{
		BaseModel:   NewBaseModel(shared),
		currentView: SecurityViewMain,
		cursor:      0,
	}
	model.setupMainMenu()
	return model
}

// Init initializes the security model
func (m *SecurityModel) Init() tea.Cmd {
	return nil
}

// Update handles security model updates
func (m *SecurityModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.currentView == SecurityViewMain {
				return m, m.GoBack()
			} else {
				m.currentView = SecurityViewMain
				m.setupMainMenu()
				return m, tea.ClearScreen
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.menuItems)-1 {
				m.cursor++
			}
		case "enter", " ":
			return m.handleSelection()
		}
	}
	return m, nil
}

// View renders the security interface
func (m *SecurityModel) View() string {
	switch m.currentView {
	case SecurityViewMain:
		return m.renderMainMenu()
	case SecurityViewAssessment:
		return m.renderAssessmentView()
	case SecurityViewStatus:
		return m.renderStatusView()
	default:
		return m.renderMainMenu()
	}
}

// setupMainMenu configures the main security menu
func (m *SecurityModel) setupMainMenu() {
	m.menuItems = []MenuItem{
		{Label: "🔍 Security Assessment", Action: ActionNavigate},
		{Label: "🛡️ Quick Security Hardening", Action: ActionNavigate},
		{Label: "🔧 Custom Security Configuration", Action: ActionNavigate},
		{Label: "📊 Security Status Dashboard", Action: ActionNavigate},
		{Label: "📋 Generate Security Report", Action: ActionNavigate},
		{Label: "🔙 Back to Server Management", Action: ActionBack},
	}
	m.cursor = 0
}

// renderMainMenu renders the main security menu
func (m *SecurityModel) renderMainMenu() string {
	var s strings.Builder

	// Title
	s.WriteString(titleStyle.Render("🔒 Linux Security - Phase 1: Core Essentials"))
	s.WriteString("\n\n")

	// Description
	description := `Secure your VPS with transparent, educational security hardening.
Phase 1 includes: SSH hardening, firewall setup, intrusion detection, and auto-updates.

Every action shows you exactly what commands will be executed and why.`

	s.WriteString(description)
	s.WriteString("\n\n")

	// Menu items
	for i, item := range m.menuItems {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}

		displayText := item.Label
		if m.cursor == i {
			displayText = selectedStyle.Render(displayText)
		} else {
			displayText = choiceStyle.Render(displayText)
		}

		s.WriteString(fmt.Sprintf("%s %s\n", cursor, displayText))
	}

	s.WriteString("\n")
	s.WriteString("Press Enter to select, Esc to go back, ↑↓ to navigate.\n")

	return s.String()
}

// renderAssessmentView renders the security assessment view
func (m *SecurityModel) renderAssessmentView() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("🔍 Security Assessment"))
	s.WriteString("\n\n")

	s.WriteString("This will analyze your current security configuration and provide recommendations.\n\n")

	s.WriteString(infoStyle.Render("Commands that will be executed:"))
	s.WriteString("\n")

	commands, descriptions := actions.SecurityAssessment()
	for i, desc := range descriptions {
		s.WriteString(fmt.Sprintf("  %d. %s\n", i+1, desc))
		if i < len(commands) {
			s.WriteString(fmt.Sprintf("     → %s\n", choiceStyle.Render(commands[i])))
		}
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString("Press Enter to run assessment, Esc to go back.\n")

	return s.String()
}

// renderStatusView renders the security status dashboard
func (m *SecurityModel) renderStatusView() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("📊 Security Status Dashboard"))
	s.WriteString("\n\n")

	s.WriteString("Current security configuration overview:\n\n")

	s.WriteString(infoStyle.Render("Status checks that will be performed:"))
	s.WriteString("\n")

	commands, descriptions := actions.GetSecurityStatus()
	for i, desc := range descriptions {
		s.WriteString(fmt.Sprintf("  • %s\n", desc))
		if i < len(commands) && !strings.HasPrefix(commands[i], "echo") {
			s.WriteString(fmt.Sprintf("    → %s\n", choiceStyle.Render(commands[i])))
		}
	}

	s.WriteString("\n")
	s.WriteString("Press Enter to view status, Esc to go back.\n")

	return s.String()
}

// handleSelection handles menu item selection
func (m *SecurityModel) handleSelection() (tea.Model, tea.Cmd) {
	if m.cursor >= len(m.menuItems) {
		return m, nil
	}

	selectedItem := m.menuItems[m.cursor]

	switch selectedItem.Label {
	case "🔍 Security Assessment":
		return m, m.NavigateTo(StateProcessing, map[string]interface{}{
			"action": "security-assessment",
			"title":  "Running Security Assessment",
		})

	case "🛡️ Quick Security Hardening":
		return m, m.NavigateTo(StateSecurityHardening, nil)

	case "🔧 Custom Security Configuration":
		return m, m.NavigateTo(StateSecurityCustom, nil)

	case "📊 Security Status Dashboard":
		return m, m.NavigateTo(StateProcessing, map[string]interface{}{
			"action": "security-status",
			"title":  "Security Status Report",
		})

	case "📋 Generate Security Report":
		return m, m.NavigateTo(StateProcessing, map[string]interface{}{
			"action": "security-report",
			"title":  "Generating Security Report",
		})

	case "🔙 Back to Server Management":
		return m, m.GoBack()
	}

	return m, nil
}

// SecurityHardeningModel handles the security hardening form
type SecurityHardeningModel struct {
	BaseModel
	form *HybridFormModel
}

// NewSecurityHardeningModel creates a new security hardening form
func NewSecurityHardeningModel(shared *SharedData) *SecurityHardeningModel {
	model := &SecurityHardeningModel{
		BaseModel: NewBaseModel(shared),
	}
	model.setupForm()
	return model
}

// setupForm configures the security hardening form
func (m *SecurityHardeningModel) setupForm() {
	m.form = NewHybridFormModel(
		m.shared,
		"🛡️ Quick Security Hardening",
		"Configure core security settings. Each option shows exactly what will be changed.",
	)

	// SSH Port Configuration
	m.form.AddField(HybridFormField{
		Label:       "SSH Port",
		FieldType:   HybridFieldTypeText,
		Placeholder: "2222 (leave empty to keep default 22)",
		Required:    false,
		MaxLength:   5,
		Validator: func(value string) error {
			if value == "" {
				return nil
			}
			port, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("port must be a number")
			}
			if port < 1024 || port > 65535 {
				return fmt.Errorf("port must be between 1024-65535")
			}
			return nil
		},
	})

	// SSH Root Login
	m.form.AddField(HybridFormField{
		Label:     "Disable SSH Root Login",
		FieldType: HybridFieldTypeSelection,
		Required:  true,
		Options: []SelectionOption{
			{Value: "yes", Description: "Yes - Disable root login (RECOMMENDED)"},
			{Value: "no", Description: "No - Keep root login enabled"},
		},
		SelectedIndex: 0,
	})

	// SSH Password Authentication
	m.form.AddField(HybridFormField{
		Label:     "Disable SSH Password Authentication",
		FieldType: HybridFieldTypeSelection,
		Required:  true,
		Options: []SelectionOption{
			{Value: "yes", Description: "Yes - Use SSH keys only (RECOMMENDED)"},
			{Value: "no", Description: "No - Allow password authentication"},
		},
		SelectedIndex: 0,
	})

	// Firewall Configuration
	m.form.AddField(HybridFormField{
		Label:     "Configure Firewall (UFW)",
		FieldType: HybridFieldTypeSelection,
		Required:  true,
		Options: []SelectionOption{
			{Value: "yes", Description: "Yes - Set up firewall with web server rules"},
			{Value: "no", Description: "No - Skip firewall configuration"},
		},
		SelectedIndex: 0,
	})

	// Fail2ban Installation
	m.form.AddField(HybridFormField{
		Label:     "Install Fail2ban (Intrusion Detection)",
		FieldType: HybridFieldTypeSelection,
		Required:  true,
		Options: []SelectionOption{
			{Value: "yes", Description: "Yes - Install Fail2ban protection"},
			{Value: "no", Description: "No - Skip Fail2ban installation"},
		},
		SelectedIndex: 0,
	})

	// Automatic Updates
	m.form.AddField(HybridFormField{
		Label:     "Enable Automatic Security Updates",
		FieldType: HybridFieldTypeSelection,
		Required:  true,
		Options: []SelectionOption{
			{Value: "yes", Description: "Yes - Automatically install security updates"},
			{Value: "no", Description: "No - Manual updates only"},
		},
		SelectedIndex: 0,
	})

	m.form.SetSubmitLabel("🚀 Start Security Hardening")
	m.form.SetSubmitHandler(m.handleSubmit)
	m.form.SetCancelHandler(m.handleCancel)
}

// Init initializes the form
func (m *SecurityHardeningModel) Init() tea.Cmd {
	return m.form.Init()
}

// Update handles form updates
func (m *SecurityHardeningModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	newModel, cmd := m.form.Update(msg)
	if hybridModel, ok := newModel.(*HybridFormModel); ok {
		m.form = hybridModel
	}
	return m, cmd
}

// View renders the form
func (m *SecurityHardeningModel) View() string {
	return m.form.View()
}

// handleSubmit processes the security hardening form
func (m *SecurityHardeningModel) handleSubmit(values []string) tea.Cmd {
	return func() tea.Msg {
		// Parse form values
		sshPortStr := values[0]
		disableRootLogin := values[1] == "yes"
		disablePasswordAuth := values[2] == "yes"
		configureFirewall := values[3] == "yes"
		installFail2ban := values[4] == "yes"
		enableAutoUpdates := values[5] == "yes"

		// Parse SSH port
		sshPort := 0
		if sshPortStr != "" {
			port, err := strconv.Atoi(sshPortStr)
			if err != nil {
				return securityErrorMsg{err: fmt.Errorf("invalid SSH port: %s", sshPortStr)}
			}
			sshPort = port
		}

		// Create security configuration
		config := actions.SecurityConfig{
			SSHPort:             sshPort,
			DisableRootLogin:    disableRootLogin,
			DisablePasswordAuth: disablePasswordAuth,
		}

		// Generate commands based on selected options
		var commands []string
		var descriptions []string

		if disableRootLogin || disablePasswordAuth || sshPort != 0 {
			sshCommands, sshDescriptions := actions.SSHHardening(config)
			commands = append(commands, sshCommands...)
			descriptions = append(descriptions, sshDescriptions...)
		}

		if configureFirewall {
			fwCommands, fwDescriptions := actions.ConfigureFirewall(config)
			commands = append(commands, fwCommands...)
			descriptions = append(descriptions, fwDescriptions...)
		}

		if installFail2ban {
			f2bCommands, f2bDescriptions := actions.InstallFail2ban(config)
			commands = append(commands, f2bCommands...)
			descriptions = append(descriptions, f2bDescriptions...)
		}

		if enableAutoUpdates {
			updateCommands, updateDescriptions := actions.EnableAutoUpdates()
			commands = append(commands, updateCommands...)
			descriptions = append(descriptions, updateDescriptions...)
		}

		// Add final security status check
		statusCommands, statusDescriptions := actions.GetSecurityStatus()
		commands = append(commands, statusCommands...)
		descriptions = append(descriptions, statusDescriptions...)

		return securityHardeningMsg{
			config:       config,
			commands:     commands,
			descriptions: descriptions,
		}
	}
}

// handleCancel handles form cancellation
func (m *SecurityHardeningModel) handleCancel() tea.Cmd {
	return m.GoBack()
}

// Message types for security operations
type securityHardeningMsg struct {
	config       actions.SecurityConfig
	commands     []string
	descriptions []string
}

type securityErrorMsg struct {
	err error
}
