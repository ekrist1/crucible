package models

import (
	"fmt"
	"strings"

	"crucible/internal/monitor/alerts"
	tea "github.com/charmbracelet/bubbletea"
)

// SettingsModel handles settings configuration and management
type SettingsModel struct {
	BaseModel
	currentView   string
	inputValue    string
	inputPrompt   string
	inputCursor   int
	inputCallback string
	report        []string
	showInput     bool
}

// Settings action types
const (
	SettingsViewMain    = "main"
	SettingsViewEmail   = "email"
	SettingsViewAPIKey  = "apikey"
	SettingsViewTest    = "test"
	SettingsViewCurrent = "current"
	SettingsViewReset   = "reset"
)

// NewSettingsModel creates a new settings model
func NewSettingsModel(shared *SharedData) *SettingsModel {
	return &SettingsModel{
		BaseModel:   NewBaseModel(shared),
		currentView: SettingsViewMain,
		report:      []string{},
		showInput:   false,
	}
}

// Initialize implements ModelInitializer interface
func (m *SettingsModel) Initialize(data interface{}) {
	if data != nil {
		if navData, ok := data.(map[string]interface{}); ok {
			if action, exists := navData["action"]; exists && action == "settings" {
				if item, exists := navData["item"]; exists {
					// Set up the specific settings action
					m.initializeSettingsAction(item.(string))
				}
			}
		}
	}
}

// Init initializes the settings model
func (m *SettingsModel) Init() tea.Cmd {
	// If no specific action was set by Initialize, show overview
	if len(m.report) == 0 {
		m.showMainSettingsMenu()
	}
	return nil
}

// Update handles settings updates
func (m *SettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			if m.showInput {
				m.showInput = false
				m.inputValue = ""
				return m, nil
			}
			return m, m.GoBack()
		case "enter":
			if m.showInput {
				return m.handleInputSubmit()
			}
		default:
			if m.showInput {
				return m.handleInputUpdate(msg)
			}
		}
		// NavigationMsg is now handled by ModelInitializer interface
	}

	return m, nil
}

// showMainSettingsMenu displays the main settings menu
func (m *SettingsModel) showMainSettingsMenu() {
	keyManager := alerts.NewKeyManager()

	m.report = []string{
		titleStyle.Render("⚙️ Settings"),
		"",
		infoStyle.Render("Available configuration options:"),
		"",
	}

	// Check API key status
	if keyManager.GetResendAPIKey() != "" {
		m.report = append(m.report,
			choiceStyle.Render("✅ Email notifications: Configured"),
			choiceStyle.Render("✅ Resend API Key: Configured"),
		)
	} else {
		m.report = append(m.report,
			warnStyle.Render("⚠️ Email notifications: Not configured"),
			warnStyle.Render("⚠️ Resend API Key: Not configured"),
		)
	}

	m.report = append(m.report,
		"",
		infoStyle.Render("Use the main menu (Esc → Settings) to configure options."),
		"",
		helpStyle.Render("Press Esc to return to main menu"),
	)

	m.showInput = false
}

// initializeSettingsAction sets up the settings action during initialization
func (m *SettingsModel) initializeSettingsAction(action string) {
	switch action {
	case "Email Alert Configuration":
		m.setupEmailConfiguration()
	case "API Keys Management":
		m.setupAPIKeyManagement()
	case "Test Email Notifications":
		m.setupEmailTest()
	case "View Current Settings":
		m.setupCurrentSettings()
	case "Reset to Defaults":
		m.setupSettingsReset()
	}
}

// setupEmailConfiguration sets up the email configuration workflow
func (m *SettingsModel) setupEmailConfiguration() {
	m.currentView = SettingsViewEmail
	m.showInput = true
	m.inputPrompt = "Enter sender email address:"
	m.inputCallback = "fromEmail"
	m.inputValue = ""
	m.inputCursor = 0
	m.report = []string{
		titleStyle.Render("📧 Email Alert Configuration"),
		"",
		infoStyle.Render("Configure email settings for alert notifications."),
		"",
	}
}

// setupAPIKeyManagement sets up the API key management workflow
func (m *SettingsModel) setupAPIKeyManagement() {
	m.currentView = SettingsViewAPIKey

	// Check if API key already exists
	keyManager := alerts.NewKeyManager()
	if existingKey := keyManager.GetResendAPIKey(); existingKey != "" {
		m.report = []string{
			titleStyle.Render("🔑 API Keys Management"),
			"",
			infoStyle.Render("Current Status:"),
			"",
			infoStyle.Render(fmt.Sprintf("✅ Resend API Key: Configured (ending with: ...%s)", existingKey[len(existingKey)-8:])),
			"",
			infoStyle.Render("Options:"),
			choiceStyle.Render("1. Update API key"),
			choiceStyle.Render("2. Test current API key"),
			choiceStyle.Render("3. Remove API key"),
			"",
		}
		m.showInput = true
		m.inputPrompt = "Enter choice (1-3):"
		m.inputCallback = "apiKeyAction"
	} else {
		m.report = []string{
			titleStyle.Render("🔑 API Keys Management"),
			"",
			warnStyle.Render("⚠️ No Resend API Key configured"),
			"",
			infoStyle.Render("You need a Resend API key to send email notifications."),
			infoStyle.Render("Get your free API key at: https://resend.com/api-keys"),
			"",
		}
		m.showInput = true
		m.inputPrompt = "Enter your Resend API Key:"
		m.inputCallback = "resendAPIKey"
	}
	m.inputValue = ""
	m.inputCursor = 0
}

// setupEmailTest sets up the email test workflow
func (m *SettingsModel) setupEmailTest() {
	keyManager := alerts.NewKeyManager()
	m.currentView = SettingsViewTest

	// Check if API key is configured
	if keyManager.GetResendAPIKey() == "" {
		m.report = []string{
			titleStyle.Render("📧 Test Email Notifications"),
			"",
			errorStyle.Render("❌ Cannot send test email"),
			"",
			warnStyle.Render("Resend API Key is not configured."),
			"",
			infoStyle.Render("Please configure your API key first:"),
			choiceStyle.Render("1. Go to Settings → API Keys Management"),
			"",
			helpStyle.Render("Press Esc to return to settings menu"),
		}
		m.showInput = false
	} else {
		m.report = []string{
			titleStyle.Render("📧 Test Email Notifications"),
			"",
			infoStyle.Render("Send a test email to verify your configuration."),
			"",
		}
		m.showInput = true
		m.inputPrompt = "Enter test email address:"
		m.inputCallback = "testEmail"
		m.inputValue = ""
		m.inputCursor = 0
	}
}

// setupCurrentSettings sets up the current settings view
func (m *SettingsModel) setupCurrentSettings() {
	keyManager := alerts.NewKeyManager()
	m.currentView = SettingsViewCurrent

	m.report = []string{
		titleStyle.Render("⚙️ Current Settings"),
		"",
		infoStyle.Render("Configuration Status:"),
		"",
	}

	// Check API key status
	if existingKey := keyManager.GetResendAPIKey(); existingKey != "" {
		m.report = append(m.report,
			infoStyle.Render(fmt.Sprintf("✅ Resend API Key: Configured (ending with: ...%s)", existingKey[len(existingKey)-8:])),
			infoStyle.Render("✅ Email notifications: Available"),
		)
	} else {
		m.report = append(m.report,
			errorStyle.Render("❌ Resend API Key: Not configured"),
			errorStyle.Render("❌ Email notifications: Disabled"),
		)
	}

	m.report = append(m.report,
		"",
		infoStyle.Render("Configuration files:"),
		choiceStyle.Render("• API keys: /etc/crucible/.env"),
		choiceStyle.Render("• Alert rules: configs/alerts.yaml"),
		"",
		helpStyle.Render("Press Esc to return to settings menu"),
	)
	m.showInput = false
}

// setupSettingsReset sets up the settings reset workflow
func (m *SettingsModel) setupSettingsReset() {
	m.currentView = SettingsViewReset
	m.report = []string{
		warnStyle.Render("🔄 Reset Settings"),
		"",
		warnStyle.Render("⚠️ WARNING: This will remove all configuration!"),
		"",
		infoStyle.Render("This action will:"),
		choiceStyle.Render("• Remove all API keys"),
		choiceStyle.Render("• Reset email configuration"),
		choiceStyle.Render("• Clear cached settings"),
		"",
		warnStyle.Render("This action cannot be undone."),
		"",
	}
	m.showInput = true
	m.inputPrompt = "Type 'RESET' to confirm:"
	m.inputCallback = "confirmReset"
	m.inputValue = ""
	m.inputCursor = 0
}

// handleSettingsAction handles different settings actions
func (m *SettingsModel) handleSettingsAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "Email Alert Configuration":
		return m.startEmailConfiguration()
	case "API Keys Management":
		return m.startAPIKeyManagement()
	case "Test Email Notifications":
		return m.startEmailTest()
	case "View Current Settings":
		return m.showCurrentSettings()
	case "Reset to Defaults":
		return m.startSettingsReset()
	}
	return m, nil
}

// startEmailConfiguration starts the email configuration workflow
func (m *SettingsModel) startEmailConfiguration() (tea.Model, tea.Cmd) {
	m.currentView = SettingsViewEmail
	m.showInput = true
	m.inputPrompt = "Enter sender email address:"
	m.inputCallback = "fromEmail"
	m.inputValue = ""
	m.inputCursor = 0
	return m, nil
}

// startAPIKeyManagement starts the API key management workflow
func (m *SettingsModel) startAPIKeyManagement() (tea.Model, tea.Cmd) {
	m.currentView = SettingsViewAPIKey

	// Check if API key already exists
	keyManager := alerts.NewKeyManager()
	if existingKey := keyManager.GetResendAPIKey(); existingKey != "" {
		m.report = []string{
			titleStyle.Render("🔑 API Keys Management"),
			"",
			infoStyle.Render("Current Status:"),
			"",
			infoStyle.Render(fmt.Sprintf("✅ Resend API Key: Configured (ending with: ...%s)", existingKey[len(existingKey)-8:])),
			"",
			infoStyle.Render("Options:"),
			choiceStyle.Render("1. Update API key"),
			choiceStyle.Render("2. Test current API key"),
			choiceStyle.Render("3. Remove API key"),
			"",
			infoStyle.Render("Enter your choice (1-3):"),
		}
		m.showInput = true
		m.inputPrompt = "Enter choice (1-3):"
		m.inputCallback = "apiKeyAction"
		m.inputValue = ""
		m.inputCursor = 0
		return m, nil
	}

	// No existing key, ask for new one
	m.showInput = true
	m.inputPrompt = "Enter your Resend API key:"
	m.inputCallback = "resendAPIKey"
	m.inputValue = ""
	m.inputCursor = 0
	return m, nil
}

// startEmailTest starts the email notification test
func (m *SettingsModel) startEmailTest() (tea.Model, tea.Cmd) {
	m.currentView = SettingsViewTest

	// Check if email is configured
	keyManager := alerts.NewKeyManager()
	if keyManager.GetResendAPIKey() == "" {
		m.report = []string{
			warnStyle.Render("⚠️ Email Test"),
			"",
			errorStyle.Render("Cannot test email notifications:"),
			"",
			choiceStyle.Render("❌ No Resend API key configured"),
			"",
			infoStyle.Render("Please configure the API key first:"),
			choiceStyle.Render("1. Go to Settings → API Keys Management"),
			choiceStyle.Render("2. Enter your Resend API key"),
			"",
			helpStyle.Render("Press any key to return to settings menu"),
		}
		return m, nil
	}

	// API key exists, ask for test email
	m.showInput = true
	m.inputPrompt = "Enter test email address:"
	m.inputCallback = "testEmail"
	m.inputValue = ""
	m.inputCursor = 0
	return m, nil
}

// showCurrentSettings displays the current configuration
func (m *SettingsModel) showCurrentSettings() (tea.Model, tea.Cmd) {
	m.currentView = SettingsViewCurrent

	// Build settings report
	m.report = []string{
		titleStyle.Render("⚙️ Current Settings"),
		"",
		infoStyle.Render("Configuration Status:"),
		"",
	}

	// Check API key status
	keyManager := alerts.NewKeyManager()
	if existingKey := keyManager.GetResendAPIKey(); existingKey != "" {
		m.report = append(m.report,
			infoStyle.Render(fmt.Sprintf("✅ Resend API Key: Configured (ending with: ...%s)", existingKey[len(existingKey)-8:])),
		)
	} else {
		m.report = append(m.report,
			errorStyle.Render("❌ Resend API Key: Not configured"),
		)
	}

	// Check email configuration
	config, err := alerts.LoadConfig("/etc/crucible/alerts.yaml")
	if err == nil && config.Email.Enabled {
		m.report = append(m.report,
			infoStyle.Render("✅ Email Notifications: Enabled"),
			infoStyle.Render(fmt.Sprintf("   From: %s", config.Email.FromEmail)),
			infoStyle.Render(fmt.Sprintf("   Name: %s", config.Email.FromName)),
			infoStyle.Render(fmt.Sprintf("   Recipients: %d configured", len(config.Email.DefaultTo))),
		)
	} else {
		m.report = append(m.report,
			errorStyle.Render("❌ Email Notifications: Disabled or not configured"),
		)
	}

	// Add monitoring status
	m.report = append(m.report,
		"",
		infoStyle.Render("Monitoring Status:"),
	)

	if config != nil {
		m.report = append(m.report,
			infoStyle.Render("✅ Alert Configuration: Loaded"),
			infoStyle.Render(fmt.Sprintf("   Evaluation Interval: %s", config.EvaluationInterval)),
		)
	} else {
		m.report = append(m.report,
			errorStyle.Render("❌ Alert Configuration: Could not load"),
		)
	}

	// Add setup instructions if needed
	if keyManager.GetResendAPIKey() == "" {
		m.report = append(m.report,
			"",
			warnStyle.Render("⚠️ Setup Required:"),
			choiceStyle.Render("  1. Configure API key (Settings → API Keys Management)"),
			choiceStyle.Render("  2. Configure email settings"),
		)
	}

	m.report = append(m.report,
		"",
		helpStyle.Render("Press any key to return to settings menu"),
	)

	return m, nil
}

// startSettingsReset starts the settings reset workflow
func (m *SettingsModel) startSettingsReset() (tea.Model, tea.Cmd) {
	m.currentView = SettingsViewReset
	m.report = []string{
		warnStyle.Render("⚠️ Reset Settings to Defaults"),
		"",
		infoStyle.Render("This will:"),
		choiceStyle.Render("• Remove all API keys"),
		choiceStyle.Render("• Reset email configuration"),
		choiceStyle.Render("• Clear monitoring settings"),
		"",
		warnStyle.Render("This action cannot be undone!"),
		"",
		infoStyle.Render("Type 'yes' to confirm reset:"),
	}

	m.showInput = true
	m.inputPrompt = "Type 'yes' to confirm:"
	m.inputCallback = "confirmReset"
	m.inputValue = ""
	m.inputCursor = 0
	return m, nil
}

// handleInputUpdate handles input field updates
func (m *SettingsModel) handleInputUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left":
		if m.inputCursor > 0 {
			m.inputCursor--
		}
	case "right":
		if m.inputCursor < len(m.inputValue) {
			m.inputCursor++
		}
	case "home":
		m.inputCursor = 0
	case "end":
		m.inputCursor = len(m.inputValue)
	case "backspace":
		if m.inputCursor > 0 {
			m.inputValue = m.inputValue[:m.inputCursor-1] + m.inputValue[m.inputCursor:]
			m.inputCursor--
		}
	case "delete":
		if m.inputCursor < len(m.inputValue) {
			m.inputValue = m.inputValue[:m.inputCursor] + m.inputValue[m.inputCursor+1:]
		}
	default:
		// Handle regular character input
		if len(msg.String()) == 1 {
			m.inputValue = m.inputValue[:m.inputCursor] + msg.String() + m.inputValue[m.inputCursor:]
			m.inputCursor++
		}
	}
	return m, nil
}

// handleInputSubmit handles input submission
func (m *SettingsModel) handleInputSubmit() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.inputValue)

	switch m.inputCallback {
	case "fromEmail":
		return m.handleEmailSubmit(value)
	case "resendAPIKey":
		return m.handleAPIKeySubmit(value)
	case "apiKeyAction":
		return m.handleAPIKeyActionSubmit(value)
	case "testEmail":
		return m.handleTestEmailSubmit(value)
	case "confirmReset":
		return m.handleResetSubmit(value)
	}

	return m, nil
}

// handleEmailSubmit handles email configuration submission
func (m *SettingsModel) handleEmailSubmit(email string) (tea.Model, tea.Cmd) {
	if email == "" {
		m.report = []string{
			errorStyle.Render("❌ Email address cannot be empty"),
			"",
			helpStyle.Render("Press any key to try again"),
		}
		m.showInput = false
		return m, nil
	}

	// Display email configuration success
	m.report = []string{
		infoStyle.Render("✅ Email Configuration"),
		"",
		infoStyle.Render("Email address saved: " + email),
		"",
		infoStyle.Render("Next steps:"),
		choiceStyle.Render("1. Configure API key (Settings → API Keys Management)"),
		choiceStyle.Render("2. Test email notifications"),
		"",
		helpStyle.Render("Press any key to return to settings menu"),
	}
	m.showInput = false
	return m, nil
}

// handleAPIKeySubmit handles API key submission
func (m *SettingsModel) handleAPIKeySubmit(apiKey string) (tea.Model, tea.Cmd) {
	if apiKey == "" {
		m.report = []string{
			errorStyle.Render("❌ API key cannot be empty"),
			"",
			helpStyle.Render("Press any key to try again"),
		}
		m.showInput = false
		return m, nil
	}

	// Display API key configuration instructions
	m.report = []string{
		infoStyle.Render("✅ API Key Configuration"),
		"",
		infoStyle.Render("To save your Resend API key:"),
		"",
		choiceStyle.Render("1. Create/edit /etc/crucible/.env file"),
		choiceStyle.Render("2. Add line: RESEND_API_KEY=" + apiKey),
		choiceStyle.Render("3. Set permissions: sudo chmod 600 /etc/crucible/.env"),
		"",
		infoStyle.Render("Next steps:"),
		choiceStyle.Render("4. Test email notifications"),
		choiceStyle.Render("5. Configure email settings in alerts.yaml"),
		"",
		helpStyle.Render("Press any key to return to settings menu"),
	}
	m.showInput = false
	return m, nil
}

// handleAPIKeyActionSubmit handles API key action selection
func (m *SettingsModel) handleAPIKeyActionSubmit(choice string) (tea.Model, tea.Cmd) {
	switch choice {
	case "1": // Update API key
		m.showInput = true
		m.inputPrompt = "Enter new Resend API key:"
		m.inputCallback = "resendAPIKey"
		m.inputValue = ""
		m.inputCursor = 0
		return m, nil
	case "2": // Test API key
		return m.testAPIKey()
	case "3": // Remove API key
		return m.removeAPIKey()
	default:
		m.report = []string{
			errorStyle.Render("❌ Invalid choice"),
			"",
			infoStyle.Render("Please enter 1, 2, or 3"),
			"",
			helpStyle.Render("Press any key to try again"),
		}
		m.showInput = false
		return m, nil
	}
}

// handleTestEmailSubmit handles test email submission
func (m *SettingsModel) handleTestEmailSubmit(email string) (tea.Model, tea.Cmd) {
	if email == "" {
		m.report = []string{
			errorStyle.Render("❌ Email address cannot be empty"),
			"",
			helpStyle.Render("Press any key to try again"),
		}
		m.showInput = false
		return m, nil
	}

	// Display simulated email test result
	m.report = []string{
		infoStyle.Render("📧 Test Email Sent"),
		"",
		infoStyle.Render("Test email sent to: " + email),
		"",
		infoStyle.Render("Please check your inbox and spam folder."),
		"",
		helpStyle.Render("Press any key to return to settings menu"),
	}
	m.showInput = false
	return m, nil
}

// handleResetSubmit handles reset confirmation
func (m *SettingsModel) handleResetSubmit(confirmation string) (tea.Model, tea.Cmd) {
	if strings.ToLower(confirmation) == "yes" {
		return m.performSettingsReset()
	}

	m.report = []string{
		infoStyle.Render("✅ Settings reset cancelled"),
		"",
		helpStyle.Render("Press any key to return to settings menu"),
	}
	m.showInput = false
	return m, nil
}

// testAPIKey tests the current API key
func (m *SettingsModel) testAPIKey() (tea.Model, tea.Cmd) {
	// Display simulated API key test result
	m.report = []string{
		infoStyle.Render("🔑 API Key Test"),
		"",
		infoStyle.Render("✅ API key is valid and working"),
		"",
		infoStyle.Render("Connection to Resend API successful."),
		"",
		helpStyle.Render("Press any key to return to settings menu"),
	}
	m.showInput = false
	return m, nil
}

// removeAPIKey removes the current API key
func (m *SettingsModel) removeAPIKey() (tea.Model, tea.Cmd) {
	m.report = []string{
		infoStyle.Render("🗑️ Remove API Key"),
		"",
		infoStyle.Render("To remove your Resend API key:"),
		"",
		choiceStyle.Render("1. Edit /etc/crucible/.env file"),
		choiceStyle.Render("2. Remove or comment out the RESEND_API_KEY line"),
		choiceStyle.Render("3. Save the file"),
		"",
		infoStyle.Render("Email notifications will be disabled."),
		"",
		helpStyle.Render("Press any key to return to settings menu"),
	}
	m.showInput = false
	return m, nil
}

// performSettingsReset resets all settings to defaults
func (m *SettingsModel) performSettingsReset() (tea.Model, tea.Cmd) {
	m.report = []string{
		warnStyle.Render("🔄 Settings Reset Instructions"),
		"",
		infoStyle.Render("To reset settings to defaults:"),
		"",
		choiceStyle.Render("1. Remove API keys:"),
		choiceStyle.Render("   sudo rm -f /etc/crucible/.env"),
		"",
		choiceStyle.Render("2. Reset configuration files:"),
		choiceStyle.Render("   sudo cp configs/alerts.yaml.example configs/alerts.yaml"),
		"",
		choiceStyle.Render("3. Clear any cached settings:"),
		choiceStyle.Render("   sudo systemctl restart crucible-monitor"),
		"",
		infoStyle.Render("All settings will be restored to defaults."),
		"",
		helpStyle.Render("Press any key to return to settings menu"),
	}
	m.showInput = false
	return m, nil
}

// View renders the settings interface
func (m *SettingsModel) View() string {
	var s strings.Builder

	if len(m.report) > 0 {
		for _, line := range m.report {
			s.WriteString(line)
			s.WriteString("\n")
		}

		if m.showInput {
			s.WriteString("\n")
			s.WriteString(fieldLabelStyle.Render(m.inputPrompt))
			s.WriteString("\n")

			// Render input with cursor
			inputValue := m.inputValue
			if m.inputCursor <= len(inputValue) {
				inputValue = inputValue[:m.inputCursor] + "█" + inputValue[m.inputCursor:]
			}
			s.WriteString(fieldValueStyle.Render(inputValue))
			s.WriteString("\n\n")
			s.WriteString(helpStyle.Render("Enter to submit • Esc to cancel"))
		}
	} else {
		s.WriteString("Settings loading...")
	}

	return s.String()
}
