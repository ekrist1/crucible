package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Form handling and input processing functions

// updateInput handles input in the input state
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

// processFormInput handles the form input flow for different actions
func (m Model) processFormInput() (tea.Model, tea.Cmd) {
	// This function handles the form input flow for different actions
	switch m.CurrentAction {
	case 100: // Create New Laravel Site
		return m.HandleLaravelSiteForm()
	case 101: // Update Laravel Site
		return m.HandleUpdateSiteForm()
	case 102: // Setup Laravel Queue Worker
		return m.HandleQueueWorkerForm()
	case 103: // Backup MySQL Database
		return m.HandleBackupForm()
	case 200: // Install MySQL
		return m.handleMySQLInstallForm()
	case 204: // Install Node.js with optional PM2
		return m.handleNodeInstallForm()
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

// StartInput initializes input state for form fields
func (m Model) StartInput(prompt, field string, action int) (Model, tea.Cmd) {
	m.State = StateInput
	m.InputPrompt = prompt
	m.InputField = field
	m.InputValue = ""
	m.CurrentAction = action
	m.Cursor = 0 // Reset cursor when starting input
	return m, tea.ClearScreen
}

// startInput is a convenience wrapper for StartInput
func (m Model) startInput(prompt, field string, action int) (tea.Model, tea.Cmd) {
	newModel, cmd := m.StartInput(prompt, field, action)
	return newModel, cmd
}

// startProcessingWithMessage initializes processing state with a message
func (m Model) startProcessingWithMessage(message string) (tea.Model, tea.Cmd) {
	m.State = StateProcessing
	m.ProcessingMsg = message
	return m, tea.Batch(tea.ClearScreen, m.Spinner.Tick)
}

// Input validation helpers

// validateYesNoInput validates y/n input responses
func validateYesNoInput(input string) bool {
	response := strings.ToLower(strings.TrimSpace(input))
	return response == "y" || response == "yes" || response == "n" || response == "no"
}

// isYesResponse checks if input is a positive response
func isYesResponse(input string) bool {
	response := strings.ToLower(strings.TrimSpace(input))
	return response == "y" || response == "yes"
}

// validateEmailInput performs basic email validation
func validateEmailInput(email string) bool {
	email = strings.TrimSpace(email)
	return email != "" && strings.Contains(email, "@")
}

// validatePasswordInput validates password strength
func validatePasswordInput(password string, minLength int) bool {
	return len(password) >= minLength
}
