package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// GitHub authentication and SSH key management functions

// handleGitHubAuth handles GitHub authentication workflow
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

// showExistingSSHKey displays the existing SSH public key
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

// handleGitHubEmailInput processes GitHub email input
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

// handleGitHubPassphraseInput processes SSH key passphrase input
func (m Model) handleGitHubPassphraseInput() (tea.Model, tea.Cmd) {
	// Store passphrase (can be empty)
	m.FormData["githubPassphrase"] = m.InputValue
	return m.generateSSHKey()
}

// generateSSHKey creates a new SSH key for GitHub
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

// handleGitHubActionInput processes action selection for existing SSH keys
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

// testGitHubConnection tests the SSH connection to GitHub
func (m Model) testGitHubConnection() (tea.Model, tea.Cmd) {
	// Add timeout and better error handling for SSH test
	commands := []string{"timeout 10 ssh -o ConnectTimeout=5 -o BatchMode=yes -T git@github.com"}
	descriptions := []string{"Testing GitHub SSH connection..."}

	return m.startCommandQueue(commands, descriptions, "github-test")
}

// showGeneratedSSHKey displays the newly generated SSH key with instructions
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

// showGitHubTestResults displays GitHub SSH connection test results
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
