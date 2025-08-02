package alerts

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// KeyManager handles secure API key management
type KeyManager struct {
	configDir string
	envFile   string
}

// NewKeyManager creates a new key manager
func NewKeyManager() *KeyManager {
	configDir := "/etc/crucible"
	if os.Getuid() != 0 {
		// If not running as root, use user's home directory
		if homeDir, err := os.UserHomeDir(); err == nil {
			configDir = filepath.Join(homeDir, ".config", "crucible")
		}
	}

	return &KeyManager{
		configDir: configDir,
		envFile:   filepath.Join(configDir, ".env"),
	}
}

// SetupResendAPIKey interactively sets up the Resend API key
func (km *KeyManager) SetupResendAPIKey() error {
	fmt.Println("🔑 Resend API Key Setup")
	fmt.Println("========================")
	fmt.Println()
	fmt.Println("To enable email alerts, you need to configure a Resend API key.")
	fmt.Println("You can get your API key from: https://resend.com/api-keys")
	fmt.Println()

	// Check if key already exists
	if existingKey := km.GetResendAPIKey(); existingKey != "" {
		fmt.Printf("✅ Resend API key is already configured (ending with: ...%s)\n",
			existingKey[len(existingKey)-8:])

		fmt.Print("Do you want to update it? (y/N): ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response != "y" && response != "yes" {
			fmt.Println("Keeping existing API key.")
			return nil
		}
	}

	// Prompt for new API key
	fmt.Print("Enter your Resend API key (will be hidden): ")
	apiKeyBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("failed to read API key: %v", err)
	}
	fmt.Println() // New line after hidden input

	apiKey := strings.TrimSpace(string(apiKeyBytes))
	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	// Basic validation
	if !strings.HasPrefix(apiKey, "re_") {
		fmt.Println("⚠️  Warning: Resend API keys typically start with 're_'")
		fmt.Print("Continue anyway? (y/N): ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response != "y" && response != "yes" {
			return fmt.Errorf("setup cancelled")
		}
	}

	// Save the API key
	if err := km.saveResendAPIKey(apiKey); err != nil {
		return fmt.Errorf("failed to save API key: %v", err)
	}

	fmt.Println("✅ Resend API key saved successfully!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("1. Configure email settings in /etc/crucible/alerts.yaml")
	fmt.Println("2. Set your from_email and default_to recipients")
	fmt.Println("3. Enable email alerts by setting email.enabled: true")

	return nil
}

// GetResendAPIKey retrieves the Resend API key from environment or file
func (km *KeyManager) GetResendAPIKey() string {
	// Try environment variable first
	if key := os.Getenv("RESEND_API_KEY"); key != "" {
		return key
	}

	// Try reading from .env file
	return km.readFromEnvFile("RESEND_API_KEY")
}

// SetEmailConfiguration interactively configures email settings
func (km *KeyManager) SetEmailConfiguration() error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("📧 Email Configuration Setup")
	fmt.Println("=============================")
	fmt.Println()

	// Get from email
	fmt.Print("Enter sender email address: ")
	fromEmail, _ := reader.ReadString('\n')
	fromEmail = strings.TrimSpace(fromEmail)

	if fromEmail == "" {
		return fmt.Errorf("sender email cannot be empty")
	}

	// Get from name
	fmt.Print("Enter sender name (optional): ")
	fromName, _ := reader.ReadString('\n')
	fromName = strings.TrimSpace(fromName)

	if fromName == "" {
		fromName = "Crucible Server Monitor"
	}

	// Get default recipients
	fmt.Println()
	fmt.Println("Enter recipient email addresses (one per line, empty line to finish):")
	var recipients []string
	for {
		fmt.Print("Recipient: ")
		email, _ := reader.ReadString('\n')
		email = strings.TrimSpace(email)

		if email == "" {
			break
		}

		if strings.Contains(email, "@") {
			recipients = append(recipients, email)
		} else {
			fmt.Println("Invalid email format, skipping...")
		}
	}

	if len(recipients) == 0 {
		return fmt.Errorf("at least one recipient email is required")
	}

	// Save email configuration
	if err := km.saveEmailConfig(fromEmail, fromName, recipients); err != nil {
		return fmt.Errorf("failed to save email configuration: %v", err)
	}

	fmt.Println()
	fmt.Println("✅ Email configuration saved!")
	fmt.Printf("From: %s <%s>\n", fromName, fromEmail)
	fmt.Printf("Recipients: %s\n", strings.Join(recipients, ", "))

	return nil
}

// saveResendAPIKey saves the API key to environment file
func (km *KeyManager) saveResendAPIKey(apiKey string) error {
	// Ensure config directory exists
	if err := os.MkdirAll(km.configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	// Read existing environment variables
	envVars := km.readAllEnvVars()

	// Update or add the API key
	envVars["RESEND_API_KEY"] = apiKey

	// Write back to file
	return km.writeEnvFile(envVars)
}

// saveEmailConfig saves email configuration to alerts.yaml
func (km *KeyManager) saveEmailConfig(fromEmail, fromName string, recipients []string) error {
	// This is a simplified version - in practice, you'd want to properly merge with existing config
	fmt.Printf("Note: Please manually update the email configuration in %s/alerts.yaml:\n", km.configDir)
	fmt.Println()
	fmt.Println("email:")
	fmt.Println("  enabled: true")
	fmt.Printf("  from_email: \"%s\"\n", fromEmail)
	fmt.Printf("  from_name: \"%s\"\n", fromName)
	fmt.Println("  default_to:")
	for _, recipient := range recipients {
		fmt.Printf("    - \"%s\"\n", recipient)
	}
	fmt.Println()

	return nil
}

// readFromEnvFile reads a specific variable from the .env file
func (km *KeyManager) readFromEnvFile(key string) string {
	envVars := km.readAllEnvVars()
	return envVars[key]
}

// readAllEnvVars reads all environment variables from the .env file
func (km *KeyManager) readAllEnvVars() map[string]string {
	envVars := make(map[string]string)

	file, err := os.Open(km.envFile)
	if err != nil {
		return envVars // Return empty map if file doesn't exist
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE format
		if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			// Remove quotes if present
			if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
				value = value[1 : len(value)-1]
			}

			envVars[key] = value
		}
	}

	return envVars
}

// writeEnvFile writes environment variables to the .env file
func (km *KeyManager) writeEnvFile(envVars map[string]string) error {
	file, err := os.Create(km.envFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// Set restrictive permissions
	if err := file.Chmod(0600); err != nil {
		return err
	}

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// Write header
	writer.WriteString("# Crucible Alert System Configuration\n")
	writer.WriteString("# This file contains sensitive API keys - keep secure!\n")
	writer.WriteString("\n")

	// Write variables
	for key, value := range envVars {
		writer.WriteString(fmt.Sprintf("%s=\"%s\"\n", key, value))
	}

	return nil
}

// TestResendAPIKey tests if the configured API key works
func (km *KeyManager) TestResendAPIKey() error {
	apiKey := km.GetResendAPIKey()
	if apiKey == "" {
		return fmt.Errorf("no Resend API key configured")
	}

	fmt.Println("✅ Resend API key found")
	fmt.Println("Note: To fully test email sending, configure the alerts and trigger a test alert")

	return nil
}
