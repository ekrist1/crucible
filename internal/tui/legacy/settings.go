package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"crucible/internal/monitor/alerts"
)

// Settings management functions

// handleSettingsSelection handles Settings submenu selections
func (m Model) handleSettingsSelection() (tea.Model, tea.Cmd) {
	switch m.Cursor {
	case 0: // Email Alert Configuration
		return m.startEmailConfiguration()
	case 1: // API Keys Management
		return m.startAPIKeyManagement()
	case 2: // Test Email Notifications
		return m.startEmailTest()
	case 3: // View Current Settings
		return m.showCurrentSettings()
	case 4: // Reset to Defaults
		return m.startSettingsReset()
	case 5: // Back to Main Menu
		return m.returnToMainMenu()
	}
	return m, nil
}

// startEmailConfiguration starts the email configuration workflow
func (m Model) startEmailConfiguration() (tea.Model, tea.Cmd) {
	return m.startInput("Enter sender email address:", "fromEmail", 500)
}

// startAPIKeyManagement starts the API key management workflow
func (m Model) startAPIKeyManagement() (tea.Model, tea.Cmd) {
	// Check if API key already exists
	keyManager := alerts.NewKeyManager()
	if existingKey := keyManager.GetResendAPIKey(); existingKey != "" {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{
			TitleStyle.Render("🔑 API Keys Management"),
			"",
			InfoStyle.Render("Current Status:"),
			"",
			InfoStyle.Render(fmt.Sprintf("✅ Resend API Key: Configured (ending with: ...%s)", existingKey[len(existingKey)-8:])),
			"",
			InfoStyle.Render("Options:"),
			ChoiceStyle.Render("1. Update API key"),
			ChoiceStyle.Render("2. Test current API key"),
			ChoiceStyle.Render("3. Remove API key"),
			"",
			InfoStyle.Render("Enter your choice (1-3):"),
		}
		return m.startInput("Enter choice (1-3):", "apiKeyAction", 501)
	}

	return m.startInput("Enter your Resend API key:", "resendAPIKey", 502)
}

// startEmailTest starts the email testing workflow
func (m Model) startEmailTest() (tea.Model, tea.Cmd) {
	keyManager := alerts.NewKeyManager()
	if keyManager.GetResendAPIKey() == "" {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{
			WarnStyle.Render("❌ Email Testing Not Available"),
			"",
			InfoStyle.Render("Resend API key is not configured."),
			InfoStyle.Render("Please configure your API key first:"),
			"",
			ChoiceStyle.Render("1. Go to Settings → API Keys Management"),
			ChoiceStyle.Render("2. Enter your Resend API key"),
			ChoiceStyle.Render("3. Return here to test"),
			"",
			InfoStyle.Render("You can get an API key from: https://resend.com/api-keys"),
		}
		return m, tea.ClearScreen
	}

	return m.startInput("Enter test email address:", "testEmail", 503)
}

// showCurrentSettings displays the current configuration
func (m Model) showCurrentSettings() (tea.Model, tea.Cmd) {
	m.State = StateProcessing
	m.ProcessingMsg = ""
	m.Report = []string{}

	// Build settings report
	m.Report = append(m.Report, TitleStyle.Render("⚙️ Current Settings"))
	m.Report = append(m.Report, "")

	// API Keys section
	m.Report = append(m.Report, InfoStyle.Render("🔑 API Keys:"))
	keyManager := alerts.NewKeyManager()

	if apiKey := keyManager.GetResendAPIKey(); apiKey != "" {
		m.Report = append(m.Report, InfoStyle.Render(fmt.Sprintf("  ✅ Resend API Key: Configured (ending with: ...%s)", apiKey[len(apiKey)-8:])))
	} else {
		m.Report = append(m.Report, WarnStyle.Render("  ❌ Resend API Key: Not configured"))
	}
	m.Report = append(m.Report, "")

	// Email Configuration section
	m.Report = append(m.Report, InfoStyle.Render("📧 Email Configuration:"))

	// Try to load alert configuration
	config, err := alerts.LoadConfig("configs/alerts.yaml")
	if err != nil {
		m.Report = append(m.Report, WarnStyle.Render("  ❌ Alert configuration: Could not load"))
	} else {
		if config.Email.Enabled {
			m.Report = append(m.Report, InfoStyle.Render("  ✅ Email Alerts: Enabled"))
			m.Report = append(m.Report, InfoStyle.Render(fmt.Sprintf("  📤 From: %s <%s>", config.Email.FromName, config.Email.FromEmail)))
			if len(config.Email.DefaultTo) > 0 {
				m.Report = append(m.Report, InfoStyle.Render(fmt.Sprintf("  📥 To: %s", strings.Join(config.Email.DefaultTo, ", "))))
			} else {
				m.Report = append(m.Report, WarnStyle.Render("  ⚠️ Recipients: None configured"))
			}
		} else {
			m.Report = append(m.Report, WarnStyle.Render("  ❌ Email Alerts: Disabled"))
		}
	}
	m.Report = append(m.Report, "")

	// Configuration File Locations
	m.Report = append(m.Report, InfoStyle.Render("📁 Configuration Files:"))
	m.Report = append(m.Report, ChoiceStyle.Render("  • API Keys: /etc/crucible/.env (or ~/.config/crucible/.env)"))
	m.Report = append(m.Report, ChoiceStyle.Render("  • Alert Rules: configs/alerts.yaml"))
	m.Report = append(m.Report, "")

	// System Status
	m.Report = append(m.Report, InfoStyle.Render("🏥 System Status:"))
	if config != nil && config.Email.Enabled && keyManager.GetResendAPIKey() != "" {
		m.Report = append(m.Report, InfoStyle.Render("  ✅ Email Notifications: Ready"))
	} else {
		m.Report = append(m.Report, WarnStyle.Render("  ⚠️ Email Notifications: Incomplete setup"))
		m.Report = append(m.Report, "")
		m.Report = append(m.Report, InfoStyle.Render("Setup Steps:"))
		if keyManager.GetResendAPIKey() == "" {
			m.Report = append(m.Report, ChoiceStyle.Render("  1. Configure Resend API key"))
		}
		if config == nil || !config.Email.Enabled {
			m.Report = append(m.Report, ChoiceStyle.Render("  2. Configure email settings"))
		}
		m.Report = append(m.Report, ChoiceStyle.Render("  3. Test email notifications"))
	}

	return m, tea.ClearScreen
}

// startSettingsReset starts the settings reset workflow
func (m Model) startSettingsReset() (tea.Model, tea.Cmd) {
	m.State = StateProcessing
	m.ProcessingMsg = ""
	m.Report = []string{
		WarnStyle.Render("⚠️ Reset Settings to Defaults"),
		"",
		InfoStyle.Render("This will:"),
		ChoiceStyle.Render("• Remove all API keys"),
		ChoiceStyle.Render("• Reset email configuration"),
		ChoiceStyle.Render("• Restore default alert rules"),
		"",
		WarnStyle.Render("This action cannot be undone!"),
		"",
		InfoStyle.Render("Type 'RESET' to confirm, or anything else to cancel:"),
	}
	return m.startInput("Type RESET to confirm:", "resetConfirm", 504)
}

// handleSettingsInput processes settings-related input
func (m Model) handleSettingsInput() (tea.Model, tea.Cmd) {
	switch m.CurrentAction {
	case 500: // Email configuration - from email
		return m.handleEmailFromInput()
	case 501: // API key management - action choice
		return m.handleAPIKeyActionInput()
	case 502: // API key management - new key
		return m.handleAPIKeyInput()
	case 503: // Email test - test email
		return m.handleTestEmailInput()
	case 504: // Settings reset confirmation
		return m.handleResetConfirmInput()
	case 505: // Email configuration - from name
		return m.handleEmailFromNameInput()
	case 506: // Email configuration - recipient
		return m.handleEmailRecipientInput()
	}

	// Default: return to settings menu
	return m.returnToSettingsMenu()
}

// handleEmailFromInput processes the sender email input
func (m Model) handleEmailFromInput() (tea.Model, tea.Cmd) {
	email := strings.TrimSpace(m.InputValue)
	if email == "" || !strings.Contains(email, "@") {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render("❌ Please enter a valid email address")}
		return m, tea.ClearScreen
	}

	m.FormData["fromEmail"] = email
	return m.startInput("Enter sender name (e.g., 'Crucible Server Monitor'):", "fromName", 505)
}

// handleEmailFromNameInput processes the sender name input
func (m Model) handleEmailFromNameInput() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.InputValue)
	if name == "" {
		name = "Crucible Server Monitor"
	}

	m.FormData["fromName"] = name
	m.FormData["recipients"] = "" // Initialize recipients list
	return m.startInput("Enter recipient email (press Enter when done to add more):", "recipient", 506)
}

// handleEmailRecipientInput processes recipient email input
func (m Model) handleEmailRecipientInput() (tea.Model, tea.Cmd) {
	email := strings.TrimSpace(m.InputValue)

	if email == "" {
		// Done adding recipients, save configuration
		return m.saveEmailConfiguration()
	}

	if !strings.Contains(email, "@") {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render("❌ Invalid email format. Please try again.")}
		return m, tea.ClearScreen
	}

	// Add to recipients list
	recipients := m.FormData["recipients"]
	if recipients == "" {
		recipients = email
	} else {
		recipients += "," + email
	}
	m.FormData["recipients"] = recipients

	// Ask for next recipient
	return m.startInput("Enter another recipient email (or press Enter to finish):", "recipient", 506)
}

// handleAPIKeyActionInput processes API key action selection
func (m Model) handleAPIKeyActionInput() (tea.Model, tea.Cmd) {
	choice := strings.TrimSpace(m.InputValue)

	switch choice {
	case "1":
		return m.startInput("Enter new Resend API key:", "resendAPIKey", 502)
	case "2":
		return m.testCurrentAPIKey()
	case "3":
		return m.removeAPIKey()
	default:
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render("❌ Invalid choice. Please enter 1, 2, or 3.")}
		return m, tea.ClearScreen
	}
}

// handleAPIKeyInput processes API key input
func (m Model) handleAPIKeyInput() (tea.Model, tea.Cmd) {
	apiKey := strings.TrimSpace(m.InputValue)

	if apiKey == "" {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render("❌ API key cannot be empty")}
		return m, tea.ClearScreen
	}

	// Basic validation
	if !strings.HasPrefix(apiKey, "re_") {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{
			WarnStyle.Render("⚠️ Warning: Resend API keys typically start with 're_'"),
			"",
			InfoStyle.Render("Continue anyway? (y/N):"),
		}
		m.FormData["pendingAPIKey"] = apiKey
		return m.startInput("Continue? (y/N):", "confirmAPIKey", 507)
	}

	return m.saveAPIKey(apiKey)
}

// handleTestEmailInput processes test email input
func (m Model) handleTestEmailInput() (tea.Model, tea.Cmd) {
	email := strings.TrimSpace(m.InputValue)
	if email == "" || !strings.Contains(email, "@") {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render("❌ Please enter a valid email address")}
		return m, tea.ClearScreen
	}

	return m.sendTestEmail(email)
}

// handleResetConfirmInput processes reset confirmation
func (m Model) handleResetConfirmInput() (tea.Model, tea.Cmd) {
	confirm := strings.TrimSpace(strings.ToUpper(m.InputValue))

	if confirm == "RESET" {
		return m.performSettingsReset()
	}

	m.State = StateProcessing
	m.ProcessingMsg = ""
	m.Report = []string{
		InfoStyle.Render("✅ Settings reset cancelled"),
		"",
		ChoiceStyle.Render("No changes were made to your configuration."),
	}
	return m, tea.ClearScreen
}

// saveEmailConfiguration saves the email configuration
func (m Model) saveEmailConfiguration() (tea.Model, tea.Cmd) {
	fromEmail := m.FormData["fromEmail"]
	fromName := m.FormData["fromName"]
	recipientsList := m.FormData["recipients"]

	recipients := strings.Split(recipientsList, ",")
	if len(recipients) == 0 || recipients[0] == "" {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render("❌ At least one recipient email is required")}
		return m, tea.ClearScreen
	}

	// Show configuration instructions
	m.State = StateProcessing
	m.ProcessingMsg = ""
	m.Report = []string{
		TitleStyle.Render("✅ Email Configuration Ready"),
		"",
		InfoStyle.Render("Please add these settings to configs/alerts.yaml:"),
		"",
		ChoiceStyle.Render("email:"),
		ChoiceStyle.Render("  enabled: true"),
		ChoiceStyle.Render(fmt.Sprintf("  from_email: \"%s\"", fromEmail)),
		ChoiceStyle.Render(fmt.Sprintf("  from_name: \"%s\"", fromName)),
		ChoiceStyle.Render("  default_to:"),
	}

	for _, recipient := range recipients {
		recipient = strings.TrimSpace(recipient)
		if recipient != "" {
			m.Report = append(m.Report, ChoiceStyle.Render(fmt.Sprintf("    - \"%s\"", recipient)))
		}
	}

	m.Report = append(m.Report, "")
	m.Report = append(m.Report, InfoStyle.Render("Email configuration will be applied after updating the config file."))

	return m, tea.ClearScreen
}

// saveAPIKey saves the API key
func (m Model) saveAPIKey(apiKey string) (tea.Model, tea.Cmd) {
	// Create the .env file content
	envContent := fmt.Sprintf("RESEND_API_KEY=\"%s\"\n", apiKey)

	m.State = StateProcessing
	m.ProcessingMsg = ""

	// For now, show manual instructions
	m.Report = []string{
		TitleStyle.Render("✅ API Key Configuration"),
		"",
		InfoStyle.Render("Please save your API key using one of these methods:"),
		"",
		InfoStyle.Render("Method 1: Environment Variable"),
		ChoiceStyle.Render("export RESEND_API_KEY=\"" + apiKey + "\""),
		"",
		InfoStyle.Render("Method 2: Configuration File"),
		ChoiceStyle.Render("Create: /etc/crucible/.env"),
		ChoiceStyle.Render("Content: " + envContent),
		ChoiceStyle.Render("Permissions: chmod 600 /etc/crucible/.env"),
		"",
		InfoStyle.Render("Method 3: User Configuration (non-root)"),
		ChoiceStyle.Render("Create: ~/.config/crucible/.env"),
		ChoiceStyle.Render("Content: " + envContent),
		"",
		WarnStyle.Render("🔒 Important: Keep your API key secure!"),
	}

	return m, tea.ClearScreen
}

// testCurrentAPIKey tests the current API key
func (m Model) testCurrentAPIKey() (tea.Model, tea.Cmd) {
	keyManager := alerts.NewKeyManager()

	m.State = StateProcessing
	m.ProcessingMsg = ""

	if err := keyManager.TestResendAPIKey(); err != nil {
		m.Report = []string{
			WarnStyle.Render("❌ API Key Test Failed"),
			"",
			InfoStyle.Render(fmt.Sprintf("Error: %v", err)),
		}
	} else {
		m.Report = []string{
			InfoStyle.Render("✅ API Key Test Successful"),
			"",
			ChoiceStyle.Render("Your Resend API key is properly configured."),
			ChoiceStyle.Render("You can now send test emails and receive alerts."),
		}
	}

	return m, tea.ClearScreen
}

// removeAPIKey removes the current API key
func (m Model) removeAPIKey() (tea.Model, tea.Cmd) {
	m.State = StateProcessing
	m.ProcessingMsg = ""
	m.Report = []string{
		InfoStyle.Render("🗑️ API Key Removal"),
		"",
		ChoiceStyle.Render("To remove your API key:"),
		ChoiceStyle.Render("1. Delete the RESEND_API_KEY environment variable"),
		ChoiceStyle.Render("2. Remove /etc/crucible/.env file"),
		ChoiceStyle.Render("3. Or remove ~/.config/crucible/.env file"),
		"",
		InfoStyle.Render("Email notifications will be disabled until a new key is configured."),
	}

	return m, tea.ClearScreen
}

// sendTestEmail sends a test email
func (m Model) sendTestEmail(email string) (tea.Model, tea.Cmd) {
	m.State = StateProcessing
	m.ProcessingMsg = ""
	m.Report = []string{
		TitleStyle.Render("📧 Test Email"),
		"",
		InfoStyle.Render("Test email functionality:"),
		ChoiceStyle.Render("1. Ensure your API key is configured"),
		ChoiceStyle.Render("2. Configure email settings in alerts.yaml"),
		ChoiceStyle.Render("3. Trigger a test alert from the monitoring dashboard"),
		"",
		InfoStyle.Render(fmt.Sprintf("Test would be sent to: %s", email)),
		"",
		WarnStyle.Render("Full email testing requires complete alert system integration."),
	}

	return m, tea.ClearScreen
}

// performSettingsReset resets all settings to defaults
func (m Model) performSettingsReset() (tea.Model, tea.Cmd) {
	m.State = StateProcessing
	m.ProcessingMsg = ""
	m.Report = []string{
		WarnStyle.Render("🔄 Settings Reset Instructions"),
		"",
		InfoStyle.Render("To reset settings to defaults:"),
		"",
		ChoiceStyle.Render("1. Remove API keys:"),
		ChoiceStyle.Render("   rm -f /etc/crucible/.env"),
		ChoiceStyle.Render("   rm -f ~/.config/crucible/.env"),
		ChoiceStyle.Render("   unset RESEND_API_KEY"),
		"",
		ChoiceStyle.Render("2. Reset alert configuration:"),
		ChoiceStyle.Render("   git checkout configs/alerts.yaml"),
		"",
		ChoiceStyle.Render("3. Clear any cached settings:"),
		ChoiceStyle.Render("   systemctl restart crucible-monitor"),
		"",
		InfoStyle.Render("All settings will be restored to defaults."),
	}

	return m, tea.ClearScreen
}

// returnToSettingsMenu returns to the Settings menu
func (m Model) returnToSettingsMenu() (tea.Model, tea.Cmd) {
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
}
