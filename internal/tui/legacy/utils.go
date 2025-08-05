package tui

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"crucible/internal/logging"
)

// System utility and helper functions

// getServiceStatus checks if a command/service is available and returns version info
func (m Model) getServiceStatus(name, command string, args ...string) string {
	cmd := exec.Command(command, args...)
	output, err := cmd.Output()

	if err != nil {
		return WarnStyle.Render(fmt.Sprintf("❌ %s: Not installed", name))
	}

	// Extract version from output (first line usually contains version info)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	version := lines[0]

	// Clean up version string
	if len(version) > 60 {
		version = version[:60] + "..."
	}

	return InfoStyle.Render(fmt.Sprintf("✅ %s: %s", name, version))
}

// getSystemServiceStatus checks if a systemd service is running
func (m Model) getSystemServiceStatus(name, service string) string {
	cmd := exec.Command("systemctl", "is-active", service)
	output, err := cmd.Output()

	if err != nil {
		return WarnStyle.Render(fmt.Sprintf("❌ %s: Service not found or not running", name))
	}

	status := strings.TrimSpace(string(output))
	if status == "active" {
		return InfoStyle.Render(fmt.Sprintf("✅ %s: Running", name))
	}

	return WarnStyle.Render(fmt.Sprintf("⚠️ %s: %s", name, status))
}

// getDiskUsage returns formatted disk usage information
func (m Model) getDiskUsage() string {
	cmd := exec.Command("df", "-h", "/")
	output, err := cmd.Output()

	if err != nil {
		return WarnStyle.Render("❌ Disk Usage: Unable to retrieve")
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return WarnStyle.Render("❌ Disk Usage: Unable to parse")
	}

	// Parse the df output (skip header)
	fields := strings.Fields(lines[1])
	if len(fields) >= 5 {
		used := fields[4]      // Usage percentage
		available := fields[3] // Available space
		return InfoStyle.Render(fmt.Sprintf("💾 Disk Usage: %s used, %s available", used, available))
	}

	return WarnStyle.Render("❌ Disk Usage: Unable to parse")
}

// getMemoryUsage returns formatted memory usage information
func (m Model) getMemoryUsage() string {
	cmd := exec.Command("free", "-h")
	output, err := cmd.Output()

	if err != nil {
		return WarnStyle.Render("❌ Memory Usage: Unable to retrieve")
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return WarnStyle.Render("❌ Memory Usage: Unable to parse")
	}

	// Parse the free output (get memory line)
	for _, line := range lines {
		if strings.HasPrefix(line, "Mem:") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				total := fields[1] // Total memory
				used := fields[2]  // Used memory
				return InfoStyle.Render(fmt.Sprintf("🧠 Memory Usage: %s used / %s total", used, total))
			}
		}
	}

	return WarnStyle.Render("❌ Memory Usage: Unable to parse")
}

// showSystemStatus displays comprehensive system status
func (m Model) showSystemStatus() (tea.Model, tea.Cmd) {
	m.State = StateProcessing
	m.Report = []string{}
	m.ProcessingMsg = ""

	// Build system status report
	m.Report = append(m.Report, TitleStyle.Render("=== SYSTEM STATUS ==="))
	m.Report = append(m.Report, "")

	// Check service statuses
	m.Report = append(m.Report, InfoStyle.Render("📦 Service Status:"))
	m.Report = append(m.Report, m.getServiceStatus("PHP", "php", "--version"))
	m.Report = append(m.Report, m.getServiceStatus("Composer", "composer", "--version"))
	m.Report = append(m.Report, m.getServiceStatus("Node.js", "node", "--version"))
	m.Report = append(m.Report, m.getServiceStatus("Python", "python3", "--version"))
	m.Report = append(m.Report, m.getServiceStatus("MySQL", "mysql", "--version"))
	m.Report = append(m.Report, m.getServiceStatus("Git", "git", "--version"))
	m.Report = append(m.Report, m.getServiceStatus("Caddy", "caddy", "version"))
	m.Report = append(m.Report, "")

	// Check system services
	m.Report = append(m.Report, InfoStyle.Render("⚙️ System Services:"))
	m.Report = append(m.Report, m.getSystemServiceStatus("MySQL", "mysql"))
	m.Report = append(m.Report, m.getSystemServiceStatus("Caddy", "caddy"))
	m.Report = append(m.Report, m.getSystemServiceStatus("Supervisor", "supervisor"))
	m.Report = append(m.Report, "")

	// System resources
	m.Report = append(m.Report, InfoStyle.Render("💾 System Resources:"))
	m.Report = append(m.Report, m.getDiskUsage())
	m.Report = append(m.Report, m.getMemoryUsage())

	return m, tea.ClearScreen
}

// showInstallationLogs displays installation logs
func (m Model) showInstallationLogs() (tea.Model, tea.Cmd) {
	// Use the logger to read log lines if available
	var logLines []string
	var err error

	if m.Logger != nil {
		logLines, err = m.Logger.ReadLogLines()
	}

	if err != nil || len(logLines) == 0 {
		// No log file found or error reading, show empty state
		m.State = StateLogViewer
		m.LogLines = []string{
			"No installation logs found.",
			"",
			"Log files are created when you perform installation operations.",
			"Try installing a service first, then check back here.",
			"",
			fmt.Sprintf("Log file location: %s", func() string {
				if m.Logger != nil {
					return m.Logger.GetLogFilePath()
				}
				return logging.DefaultLogPath()
			}()),
		}
		if err != nil {
			m.LogLines = append(m.LogLines, "", fmt.Sprintf("Error reading logs: %v", err))
		}
		m.LogScroll = 0
		return m, tea.ClearScreen
	}

	// Set up log viewer state
	m.State = StateLogViewer
	m.LogLines = logLines
	m.LogScroll = 0

	// If there are many lines, start at the bottom
	if len(logLines) > 18 {
		m.LogScroll = len(logLines) - 18
	}

	return m, tea.ClearScreen
}
