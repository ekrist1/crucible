package tui

import (
	"fmt"
	"os/exec"
	"strings"

	"crucible/internal/services"
	tea "github.com/charmbracelet/bubbletea"
)

// Installation functions for various services

// checkServiceInstallations checks the installation status of all services
func (m *Model) checkServiceInstallations() {
	// Check PHP installation
	m.ServiceStatus["php"] = m.isServiceInstalled("php", "--version")

	// Check Composer installation
	m.ServiceStatus["composer"] = m.isServiceInstalled("composer", "--version")

	// Check Python installation
	m.ServiceStatus["python"] = m.isServiceInstalled("python3", "--version")

	// Check Node.js installation
	m.ServiceStatus["node"] = m.isServiceInstalled("node", "--version")

	// Check MySQL installation
	m.ServiceStatus["mysql"] = m.isServiceInstalled("mysql", "--version")

	// Check Caddy installation
	m.ServiceStatus["caddy"] = m.isServiceInstalled("caddy", "version")

	// Check Git installation
	m.ServiceStatus["git"] = m.isServiceInstalled("git", "--version")

	// Check Supervisor installation
	m.ServiceStatus["supervisor"] = m.isServiceInstalled("supervisorctl", "version")
}

// isServiceInstalled checks if a command/service is available
func (m Model) isServiceInstalled(command string, args ...string) bool {
	cmd := exec.Command(command, args...)
	err := cmd.Run()
	return err == nil
}

// refreshServiceStatus updates the status of a specific service
func (m *Model) refreshServiceStatus(serviceName string) {
	switch serviceName {
	case "php":
		m.ServiceStatus["php"] = m.isServiceInstalled("php", "--version")
	case "composer":
		m.ServiceStatus["composer"] = m.isServiceInstalled("composer", "--version")
	case "python":
		m.ServiceStatus["python"] = m.isServiceInstalled("python3", "--version")
	case "node":
		m.ServiceStatus["node"] = m.isServiceInstalled("node", "--version")
	case "mysql":
		m.ServiceStatus["mysql"] = m.isServiceInstalled("mysql", "--version")
	case "caddy":
		m.ServiceStatus["caddy"] = m.isServiceInstalled("caddy", "version")
	case "git":
		m.ServiceStatus["git"] = m.isServiceInstalled("git", "--version")
	}
}

// getServiceIcon returns the appropriate icon for service status
func (m Model) getServiceIcon(serviceName string) string {
	if m.ServiceStatus[serviceName] {
		return "✅"
	}
	return "⬜"
}

// Individual service installation functions

func (m Model) installPHP() (tea.Model, tea.Cmd) {
	commands, descriptions, err := services.InstallPHP()
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
		return m, tea.ClearScreen
	}
	return m.startCommandQueue(commands, descriptions, "php")
}

func (m Model) installComposer() (tea.Model, tea.Cmd) {
	commands, descriptions, err := services.InstallComposer()
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
		return m, tea.ClearScreen
	}
	return m.startCommandQueue(commands, descriptions, "composer")
}

func (m Model) installPython() (tea.Model, tea.Cmd) {
	commands, descriptions, err := services.InstallPython()
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
		return m, tea.ClearScreen
	}
	return m.startCommandQueue(commands, descriptions, "python")
}

func (m Model) installNode() (tea.Model, tea.Cmd) {
	commands, descriptions, err := services.InstallNode()
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
		return m, tea.ClearScreen
	}
	return m.startCommandQueue(commands, descriptions, "node")
}

func (m Model) installNodeWithPM2(installPM2 bool) (tea.Model, tea.Cmd) {
	commands, descriptions, err := services.InstallNodeWithPM2(installPM2)
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
		return m, tea.ClearScreen
	}
	return m.startCommandQueue(commands, descriptions, "node")
}

func (m Model) installMySQL() (tea.Model, tea.Cmd) {
	commands, descriptions, err := services.InstallMySQL("")
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
		return m, tea.ClearScreen
	}
	return m.startCommandQueue(commands, descriptions, "mysql")
}

func (m Model) installMySQLWithPassword() (tea.Model, tea.Cmd) {
	password := m.FormData["mysqlRootPassword"]
	commands, descriptions, err := services.InstallMySQL(password)
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
		return m, tea.ClearScreen
	}
	return m.startCommandQueue(commands, descriptions, "mysql")
}

func (m Model) installCaddy() (tea.Model, tea.Cmd) {
	commands, descriptions, err := services.InstallCaddy()
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
		return m, tea.ClearScreen
	}
	return m.startCommandQueue(commands, descriptions, "caddy")
}

func (m Model) installSupervisor() (tea.Model, tea.Cmd) {
	commands, descriptions, err := services.InstallSupervisor()
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
		return m, tea.ClearScreen
	}
	return m.startCommandQueue(commands, descriptions, "supervisor")
}

func (m Model) installGit() (tea.Model, tea.Cmd) {
	commands, descriptions, err := services.InstallGit()
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
		return m, tea.ClearScreen
	}
	return m.startCommandQueue(commands, descriptions, "git")
}

// Installation form handlers

func (m Model) handleMySQLInstallForm() (tea.Model, tea.Cmd) {
	// Validate password
	if len(m.InputValue) < 8 {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{WarnStyle.Render("❌ MySQL root password must be at least 8 characters long")}
		return m, tea.ClearScreen
	}

	// Store password and proceed with installation
	m.FormData["mysqlRootPassword"] = m.InputValue
	return m.installMySQLWithPassword()
}

func (m Model) handleNodeInstallForm() (tea.Model, tea.Cmd) {
	// Validate input (y/n)
	response := strings.ToLower(strings.TrimSpace(m.InputValue))
	installPM2 := response == "y" || response == "yes"

	// Store PM2 choice and proceed with installation
	m.FormData["installPM2"] = fmt.Sprintf("%t", installPM2)
	return m.installNodeWithPM2(installPM2)
}
