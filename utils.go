package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// MySQL backup is now handled by forms.go

func (m model) showSystemStatus() (tea.Model, tea.Cmd) {
	// Clear screen before showing system status
	clearScreen()
	m.state = stateProcessing
	m.processingMsg = "Checking system status..."
	m.report = []string{}

	m.report = append(m.report, infoStyle.Render("=== SYSTEM STATUS REPORT ==="))
	m.report = append(m.report, "")

	m.report = append(m.report, infoStyle.Render("📦 INSTALLED SERVICES:"))
	m.report = append(m.report, m.getServiceStatus("PHP", "php", "--version"))
	m.report = append(m.report, m.getServiceStatus("Composer", "composer", "--version"))
	m.report = append(m.report, m.getServiceStatus("Python", "python3", "--version"))
	m.report = append(m.report, m.getServiceStatus("pip", "pip3", "--version"))
	m.report = append(m.report, m.getServiceStatus("MySQL", "mysql", "--version"))
	m.report = append(m.report, m.getServiceStatus("Caddy", "caddy", "version"))
	m.report = append(m.report, m.getServiceStatus("Git", "git", "--version"))

	m.report = append(m.report, "")
	m.report = append(m.report, infoStyle.Render("🔧 SYSTEM SERVICES:"))
	m.report = append(m.report, m.getSystemServiceStatus("MySQL Server", "mysql"))
	m.report = append(m.report, m.getSystemServiceStatus("Caddy Web Server", "caddy"))
	m.report = append(m.report, m.getSystemServiceStatus("PHP-FPM 8.4", "php8.4-fpm"))

	m.report = append(m.report, "")
	m.report = append(m.report, infoStyle.Render("🌐 LARAVEL SITES:"))
	sites, err := m.listLaravelSites()
	if err != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Error listing sites: %v", err)))
	} else if len(sites) == 0 {
		m.report = append(m.report, infoStyle.Render("📋 No Laravel sites found in /var/www"))
	} else {
		for i, site := range sites {
			m.report = append(m.report, infoStyle.Render(fmt.Sprintf("✅ Site %d: %s", i+1, site)))
		}
	}

	m.report = append(m.report, "")
	m.report = append(m.report, infoStyle.Render("💾 SYSTEM RESOURCES:"))
	m.report = append(m.report, m.getDiskUsage())
	m.report = append(m.report, m.getMemoryUsage())

	m.report = append(m.report, "")
	m.report = append(m.report, infoStyle.Render("=== STATUS CHECK COMPLETED ==="))

	m.processingMsg = ""
	return m, nil
}

func (m model) getServiceStatus(name, command string, args ...string) string {
	cmd := exec.Command(command, args...)
	output, err := cmd.Output()

	if err != nil {
		return warnStyle.Render(fmt.Sprintf("❌ %s: Not installed", name))
	}

	version := strings.TrimSpace(string(output))
	// Clean up version output
	if lines := strings.Split(version, "\n"); len(lines) > 0 {
		version = lines[0]
	}

	// Extract just the version number for cleaner display
	if strings.Contains(version, "PHP") {
		fields := strings.Fields(version)
		if len(fields) >= 2 {
			version = fields[1]
		}
	} else if strings.Contains(version, "Composer") {
		fields := strings.Fields(version)
		if len(fields) >= 3 {
			version = fields[2]
		}
	} else if strings.Contains(version, "Python") {
		fields := strings.Fields(version)
		if len(fields) >= 2 {
			version = fields[1]
		}
	} else if strings.Contains(version, "pip") {
		fields := strings.Fields(version)
		if len(fields) >= 2 {
			version = fields[1]
		}
	} else if strings.Contains(version, "mysql") {
		fields := strings.Fields(version)
		for i, field := range fields {
			if strings.Contains(field, "Ver") && i+1 < len(fields) {
				version = fields[i+1]
				break
			}
		}
	}

	return infoStyle.Render(fmt.Sprintf("✅ %s: %s", name, version))
}

func (m model) getSystemServiceStatus(name, service string) string {
	cmd := exec.Command("systemctl", "is-active", service)
	output, err := cmd.Output()

	status := strings.TrimSpace(string(output))
	if err != nil || status != "active" {
		if status == "" {
			status = "inactive"
		}
		return warnStyle.Render(fmt.Sprintf("🔴 %s: %s", name, status))
	}
	return infoStyle.Render(fmt.Sprintf("🟢 %s: Running", name))
}

func (m model) upgradeToPHP85() (tea.Model, tea.Cmd) {
	// Clear screen before starting upgrade
	clearScreen()
	m.state = stateProcessing
	m.processingMsg = "Upgrading to PHP 8.5..."
	m.report = []string{infoStyle.Render("Upgrading to PHP 8.5")}

	osType := getOSType()
	var command string

	switch osType {
	case "ubuntu":
		command = `sudo apt update && \
sudo apt install -y php8.5 php8.5-fpm php8.5-mysql php8.5-xml php8.5-gd php8.5-curl php8.5-mbstring php8.5-zip php8.5-intl php8.5-bcmath && \
sudo a2dismod php8.4 && \
sudo a2enmod php8.5 && \
sudo systemctl restart apache2 && \
sudo update-alternatives --set php /usr/bin/php8.5`
	case "fedora":
		command = `sudo dnf module reset php -y && \
sudo dnf module enable php:remi-8.5 -y && \
sudo dnf update -y php php-fpm php-mysqlnd php-xml php-gd php-curl php-mbstring php-zip php-intl php-bcmath && \
sudo systemctl restart php-fpm`
	default:
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Unsupported operating system for PHP upgrade: %s", osType)))
		m.processingMsg = ""
		return m, nil
	}

	// Execute command with logging
	modelPtr := &m
	result := modelPtr.executeAndLogCommand(command)

	if result.Error != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to upgrade to PHP 8.5: %v", result.Error)))
		if strings.TrimSpace(result.Output) != "" {
			m.report = append(m.report, warnStyle.Render(fmt.Sprintf("Output: %s", result.Output)))
		}
	} else {
		m.report = append(m.report, infoStyle.Render("✅ Successfully upgraded to PHP 8.5"))

		// Update Caddy configuration for new PHP version
		m.updateCaddyPHPVersion("8.5")
		m.refreshServiceStatus("php")
	}

	m.processingMsg = ""
	return m, nil
}

func (m model) updateCaddyPHPVersion(version string) {
	m.report = append(m.report, infoStyle.Render(fmt.Sprintf("Updating Caddy PHP-FPM configuration to version %s", version)))

	// Update Laravel snippet
	laravelSnippet := fmt.Sprintf(`php_fastcgi unix//run/php/php%s-fpm.sock {
	root /var/www/{args[1]}
	split .php
	index index.php
	try_files {path} {path}/ /index.php?{query}
}`, version)

	err := os.WriteFile("/etc/caddy/snippets/laravel.caddy", []byte(laravelSnippet), 0644)
	if err != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to update Laravel snippet: %v", err)))
		return
	}

	// Reload Caddy
	cmd := exec.Command("sudo", "systemctl", "reload", "caddy")
	if output, err := cmd.CombinedOutput(); err != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to reload Caddy: %v\nOutput: %s", err, string(output))))
	} else {
		m.report = append(m.report, infoStyle.Render("✅ Caddy configuration updated successfully"))
	}
}

// getDiskUsage returns disk usage information as a styled string
func (m model) getDiskUsage() string {
	cmd := exec.Command("df", "-h", "/var/www")
	if output, err := cmd.Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		if len(lines) > 1 {
			fields := strings.Fields(lines[1])
			if len(fields) >= 5 {
				return infoStyle.Render(fmt.Sprintf("💾 /var/www: %s used of %s (%s)", fields[2], fields[1], fields[4]))
			}
		}
	}
	return warnStyle.Render("❌ Unable to check disk usage for /var/www")
}

// getMemoryUsage returns memory usage information as a styled string
func (m model) getMemoryUsage() string {
	cmd := exec.Command("free", "-h")
	if output, err := cmd.Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		if len(lines) > 1 {
			fields := strings.Fields(lines[1])
			if len(fields) >= 3 {
				return infoStyle.Render(fmt.Sprintf("🧠 Memory: %s used of %s", fields[2], fields[1]))
			}
		}
	}
	return warnStyle.Render("❌ Unable to check memory usage")
}

// Laravel site functions moved to forms.go
