package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	tea "github.com/charmbracelet/bubbletea"
	"crucible/internal/actions"
)

// TUI form handling functions - these handle the input flow and validation

func (m Model) HandleLaravelSiteForm() (Model, tea.Cmd) {
	switch m.InputField {
	case "siteName":
		if m.InputValue == "" {
			m.Report = []string{WarnStyle.Render("❌ Site name cannot be empty")}
			m.State = StateMenu
			return m, nil
		}
		m.FormData["siteName"] = m.InputValue
		return m.StartInput("Enter domain (e.g., myapp.local):", "domain", 100)

	case "domain":
		if m.InputValue == "" {
			m.Report = []string{WarnStyle.Render("❌ Domain cannot be empty")}
			m.State = StateMenu
			return m, nil
		}
		m.FormData["domain"] = m.InputValue
		return m.StartInput("Enter Git repository URL (https://github.com/user/repo.git or git@github.com:user/repo.git, optional):", "gitRepo", 100)

	case "gitRepo":
		// Validate GitHub URL if provided
		if m.InputValue != "" && !isValidGitURL(m.InputValue) {
			m.Report = []string{WarnStyle.Render("❌ Invalid Git repository URL. Please use format: https://github.com/user/repo.git or git@github.com:user/repo.git")}
			m.State = StateMenu
			return m, nil
		}
		m.FormData["gitRepo"] = m.InputValue
		// TODO: Connect to actions package
		newModel, cmd := m.createLaravelSiteWithData()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	}

	m.State = StateMenu
	return m, nil
}

func (m Model) HandleUpdateSiteForm() (Model, tea.Cmd) {
	switch m.InputField {
	case "siteIndex":
		if m.InputValue == "" {
			m.Report = []string{WarnStyle.Render("❌ Site index cannot be empty")}
			m.State = StateMenu
			return m, nil
		}
		m.FormData["siteIndex"] = m.InputValue
		// TODO: Connect to actions package
		newModel, cmd := m.updateLaravelSiteWithData()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	}

	m.State = StateMenu
	return m, nil
}

func (m Model) HandleBackupForm() (Model, tea.Cmd) {
	switch m.InputField {
	case "dbName":
		if m.InputValue == "" {
			m.Report = []string{WarnStyle.Render("❌ Database name cannot be empty")}
			m.State = StateMenu
			return m, nil
		}
		m.FormData["dbName"] = m.InputValue
		return m.StartInput("Enter MySQL username (default: root):", "dbUser", 103)

	case "dbUser":
		if m.InputValue == "" {
			m.InputValue = "root"
		}
		m.FormData["dbUser"] = m.InputValue
		return m.StartInput("Enter MySQL password:", "dbPassword", 103)

	case "dbPassword":
		if m.InputValue == "" {
			m.Report = []string{WarnStyle.Render("❌ Password cannot be empty")}
			m.State = StateMenu
			return m, nil
		}
		m.FormData["dbPassword"] = m.InputValue
		return m.StartInput("Enter remote host (e.g., user@server.com):", "remoteHost", 103)

	case "remoteHost":
		if m.InputValue == "" {
			m.Report = []string{WarnStyle.Render("❌ Remote host cannot be empty")}
			m.State = StateMenu
			return m, nil
		}
		m.FormData["remoteHost"] = m.InputValue
		return m.StartInput("Enter remote backup path (default: ~/backups/):", "remotePath", 103)

	case "remotePath":
		if m.InputValue == "" {
			m.InputValue = "~/backups/"
		}
		m.FormData["remotePath"] = m.InputValue
		// TODO: Connect to actions package
		newModel, cmd := m.backupMySQLWithData()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	}

	m.State = StateMenu
	return m, nil
}

func (m Model) HandleQueueWorkerForm() (Model, tea.Cmd) {
	switch m.InputField {
	case "queueSiteName":
		if m.InputValue == "" {
			m.Report = []string{WarnStyle.Render("❌ Site name cannot be empty")}
			m.State = StateMenu
			return m, nil
		}

		// Check if site exists
		sitePath := filepath.Join("/var/www", m.InputValue)
		if _, err := os.Stat(sitePath); os.IsNotExist(err) {
			m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Laravel site '%s' does not exist at %s", m.InputValue, sitePath))}
			m.State = StateMenu
			return m, nil
		}

		m.FormData["queueSiteName"] = m.InputValue
		return m.StartInput("Enter queue connection (default: database):", "queueConnection", 102)

	case "queueConnection":
		if m.InputValue == "" {
			m.InputValue = "database"
		}
		m.FormData["queueConnection"] = m.InputValue
		return m.StartInput("Enter number of worker processes (default: 1):", "queueProcesses", 102)

	case "queueProcesses":
		if m.InputValue == "" {
			m.InputValue = "1"
		}
		// Validate numeric input
		if !regexp.MustCompile(`^\d+$`).MatchString(m.InputValue) {
			m.Report = []string{WarnStyle.Render("❌ Number of processes must be a valid number")}
			m.State = StateMenu
			return m, nil
		}
		m.FormData["queueProcesses"] = m.InputValue
		return m.StartInput("Enter queue name (default: default):", "queueName", 102)

	case "queueName":
		if m.InputValue == "" {
			m.InputValue = "default"
		}
		m.FormData["queueName"] = m.InputValue
		// TODO: Connect to actions package
		newModel, cmd := m.setupQueueWorkerWithData()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	}

	m.State = StateMenu
	return m, nil
}

// Helper functions for form validation and utilities

// isValidGitURL validates common Git repository URL formats
func isValidGitURL(url string) bool {
	// HTTPS format: https://github.com/user/repo.git
	httpsPattern := `^https://github\.com/[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+\.git$`

	// SSH format: git@github.com:user/repo.git
	sshPattern := `^git@github\.com:[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+\.git$`

	// HTTPS without .git suffix: https://github.com/user/repo
	httpsNoGitPattern := `^https://github\.com/[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+$`

	httpsRegex := regexp.MustCompile(httpsPattern)
	sshRegex := regexp.MustCompile(sshPattern)
	httpsNoGitRegex := regexp.MustCompile(httpsNoGitPattern)

	return httpsRegex.MatchString(url) || sshRegex.MatchString(url) || httpsNoGitRegex.MatchString(url)
}

// parseInt converts string to int for simple cases
func parseInt(s string) int {
	switch s {
	case "1":
		return 1
	case "2":
		return 2
	case "3":
		return 3
	case "4":
		return 4
	case "5":
		return 5
	case "6":
		return 6
	case "7":
		return 7
	case "8":
		return 8
	case "9":
		return 9
	default:
		return 0
	}
}

// Action functions - these now use the actions packages to get command sequences
func (m Model) createLaravelSiteWithData() (Model, tea.Cmd) {
	// Get command sequences from actions package instead of generating here
	config := actions.LaravelSiteConfig{
		SiteName: m.FormData["siteName"],
		Domain:   m.FormData["domain"],
		GitRepo:  m.FormData["gitRepo"],
	}
	
	commands, descriptions := actions.CreateLaravelSite(config)
	return m.startCommandQueue(commands, descriptions, "")
}

func (m Model) updateLaravelSiteWithData() (Model, tea.Cmd) {
	// List available sites first
	sites, err := actions.ListLaravelSites()
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Failed to list sites: %v", err))}
		return m, nil
	}

	if len(sites) == 0 {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{InfoStyle.Render("📋 No Laravel sites found in /var/www")}
		return m, nil
	}

	config := actions.UpdateSiteConfig{
		SiteIndex: m.FormData["siteIndex"],
		Sites:     sites,
	}

	commands, descriptions, err := actions.UpdateLaravelSite(config)
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ %v", err))}
		return m, nil
	}

	return m.startCommandQueue(commands, descriptions, "")
}

func (m Model) backupMySQLWithData() (Model, tea.Cmd) {
	config := actions.MySQLBackupConfig{
		DBName:     m.FormData["dbName"],
		DBUser:     m.FormData["dbUser"],
		DBPassword: m.FormData["dbPassword"],
		RemoteHost: m.FormData["remoteHost"],
		RemotePath: m.FormData["remotePath"],
	}
	
	commands, descriptions := actions.BackupMySQL(config)
	return m.startCommandQueue(commands, descriptions, "mysql-backup")
}

func (m Model) setupQueueWorkerWithData() (Model, tea.Cmd) {
	config := actions.QueueWorkerConfig{
		SiteName:   m.FormData["queueSiteName"],
		Connection: m.FormData["queueConnection"],
		Processes:  m.FormData["queueProcesses"],
		QueueName:  m.FormData["queueName"],
	}
	
	commands, descriptions := actions.SetupQueueWorker(config)
	return m.startCommandQueue(commands, descriptions, "queue-worker")
}