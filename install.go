package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) installPHP() (tea.Model, tea.Cmd) {
	m.state = stateProcessing
	m.processingMsg = "Installing PHP 8.4..."
	m.report = []string{infoStyle.Render("Starting PHP 8.4 installation")}

	var command string
	osType := getOSType()

	switch osType {
	case "ubuntu":
		command = `sudo apt update && \
sudo apt install -y software-properties-common && \
sudo add-apt-repository ppa:ondrej/php -y && \
sudo apt update && \
sudo apt install -y php8.4 php8.4-fpm php8.4-mysql php8.4-xml php8.4-gd php8.4-curl php8.4-mbstring php8.4-zip php8.4-intl php8.4-bcmath`
	case "fedora":
		command = `sudo dnf install -y https://rpms.remirepo.net/fedora/remi-release-$(rpm -E %fedora).rpm && \
sudo dnf module reset php -y && \
sudo dnf module enable php:remi-8.4 -y && \
sudo dnf install -y php php-fpm php-mysqlnd php-xml php-gd php-curl php-mbstring php-zip php-intl php-bcmath`
	default:
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Unsupported operating system: %s", osType)))
		m.processingMsg = ""
		return m, nil
	}

	// Execute command with logging
	modelPtr := &m
	result := modelPtr.executeAndLogCommand(command)

	if result.Error != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to install PHP: %v", result.Error)))
		if strings.TrimSpace(result.Output) != "" {
			m.report = append(m.report, warnStyle.Render(fmt.Sprintf("Output: %s", result.Output)))
		}
	} else {
		m.report = append(m.report, infoStyle.Render("✅ PHP 8.4 installed successfully"))
		m.refreshServiceStatus("php")
	}

	m.processingMsg = ""
	return m, nil
}

func (m model) installComposer() (tea.Model, tea.Cmd) {
	m.state = stateProcessing
	m.processingMsg = "Installing PHP Composer..."
	m.report = []string{infoStyle.Render("Installing PHP Composer")}

	command := `curl -sS https://getcomposer.org/installer | php && \
sudo mv composer.phar /usr/local/bin/composer && \
sudo chmod +x /usr/local/bin/composer`

	// Execute command with logging
	modelPtr := &m
	result := modelPtr.executeAndLogCommand(command)

	if result.Error != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to install Composer: %v", result.Error)))
		if strings.TrimSpace(result.Output) != "" {
			m.report = append(m.report, warnStyle.Render(fmt.Sprintf("Output: %s", result.Output)))
		}
	} else {
		m.report = append(m.report, infoStyle.Render("✅ Composer installed successfully"))
		m.refreshServiceStatus("composer")
	}

	m.processingMsg = ""
	return m, nil
}

// installPython installs Python 3.13 with pip and virtual environment support
// It includes comprehensive setup for modern Python development:
// - Python 3.13 interpreter
// - pip package manager with latest version
// - venv module for virtual environments
// - Essential development packages (dev, distutils)
// - Verification tests for functionality
func (m model) installPython() (tea.Model, tea.Cmd) {
	m.state = stateProcessing
	m.processingMsg = "Installing Python 3.13, pip, and virtual environment tools..."
	m.report = []string{infoStyle.Render("Starting Python 3.13 installation with pip and venv")}

	osType := getOSType()
	var command string

	switch osType {
	case "ubuntu":
		command = `sudo apt update && \
sudo apt install -y software-properties-common && \
sudo add-apt-repository ppa:deadsnakes/ppa -y && \
sudo apt update && \
sudo apt install -y python3.13 python3.13-venv python3.13-pip python3.13-dev python3.13-distutils && \
sudo update-alternatives --install /usr/bin/python3 python3 /usr/bin/python3.13 1 && \
python3.13 -m ensurepip --default-pip && \
python3.13 -m pip install --upgrade pip setuptools wheel virtualenv`
	case "fedora":
		command = `sudo dnf install -y python3.13 python3.13-pip python3.13-devel python3.13-setuptools && \
sudo alternatives --install /usr/bin/python3 python3 /usr/bin/python3.13 1 && \
python3.13 -m ensurepip --default-pip && \
python3.13 -m pip install --upgrade pip setuptools wheel virtualenv`
	default:
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Unsupported operating system: %s", osType)))
		m.processingMsg = ""
		return m, nil
	}

	// Execute command with logging
	modelPtr := &m
	result := modelPtr.executeAndLogCommand(command)

	if result.Error != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to install Python: %v", result.Error)))
		if strings.TrimSpace(result.Output) != "" {
			m.report = append(m.report, warnStyle.Render(fmt.Sprintf("Output: %s", result.Output)))
		}
	} else {
		m.report = append(m.report, infoStyle.Render("✅ Python 3.13 installed successfully"))

		// Verify installation and show version
		verifyCmd := "python3 --version && pip3 --version"
		verifyResult := modelPtr.executeAndLogCommand(verifyCmd)
		if verifyResult.Error == nil {
			m.report = append(m.report, infoStyle.Render("✅ Installation verified:"))
			lines := strings.Split(strings.TrimSpace(verifyResult.Output), "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					m.report = append(m.report, infoStyle.Render("  "+line))
				}
			}
		}

		// Create a sample virtual environment to verify venv works
		m.report = append(m.report, infoStyle.Render("✅ Testing virtual environment creation..."))
		testVenvCmd := `cd /tmp && python3 -m venv test_env && source test_env/bin/activate && python --version && deactivate && rm -rf test_env`
		testResult := modelPtr.executeAndLogCommand(testVenvCmd)
		if testResult.Error == nil {
			m.report = append(m.report, infoStyle.Render("✅ Virtual environment functionality verified"))
		} else {
			m.report = append(m.report, warnStyle.Render("⚠️  Virtual environment test had issues, but Python should still work"))
		}

		m.refreshServiceStatus("python")
	}

	m.processingMsg = ""
	return m, nil
}

func (m model) installMySQL() (tea.Model, tea.Cmd) {
	m.state = stateProcessing
	m.processingMsg = "Installing MySQL..."
	m.report = []string{infoStyle.Render("Installing MySQL with best practices")}

	osType := getOSType()
	isSystemd := isSystemdAvailable()

	m.report = append(m.report, infoStyle.Render(fmt.Sprintf("Detected OS: %s, Init system: systemd=%t", osType, isSystemd)))

	var command string

	switch osType {
	case "ubuntu":
		startCmd := getServiceStartCommand("mysql", isSystemd)
		enableCmd := getServiceEnableCommand("mysql", isSystemd)
		cmdParts := []string{
			"sudo apt update",
			"sudo apt install -y mysql-server",
		}

		// Add service management commands
		if startCmd != "" {
			cmdParts = append(cmdParts, startCmd)
		}
		if enableCmd != "" {
			cmdParts = append(cmdParts, enableCmd)
		}

		// Note: mysql_secure_installation requires interactive input, so we'll skip it
		m.report = append(m.report, warnStyle.Render("Note: mysql_secure_installation skipped (requires manual interaction)"))

		command = strings.Join(cmdParts, " && ")
	case "fedora":
		startCmd := getServiceStartCommand("mysqld", isSystemd)
		enableCmd := getServiceEnableCommand("mysqld", isSystemd)
		cmdParts := []string{
			"sudo dnf install -y mysql-server",
		}

		// Add service management commands (mysqld on Fedora, not mysql)
		if startCmd != "" {
			cmdParts = append(cmdParts, startCmd)
		}
		if enableCmd != "" {
			cmdParts = append(cmdParts, enableCmd)
		}

		m.report = append(m.report, warnStyle.Render("Note: mysql_secure_installation skipped (requires manual interaction)"))

		command = strings.Join(cmdParts, " && ")
	default:
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Unsupported operating system: %s", osType)))
		m.processingMsg = ""
		return m, nil
	}

	m.report = append(m.report, infoStyle.Render(fmt.Sprintf("Executing: %s", command)))

	// Execute command with logging
	modelPtr := &m
	result := modelPtr.executeAndLogCommand(command)

	if result.Error != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to install MySQL: %v", result.Error)))
		if strings.TrimSpace(result.Output) != "" {
			m.report = append(m.report, warnStyle.Render(fmt.Sprintf("Output: %s", result.Output)))
		}
	} else {
		m.report = append(m.report, infoStyle.Render("✅ MySQL package installed successfully"))

		// Verify MySQL service status
		statusCmd := "sudo systemctl status mysql 2>/dev/null || sudo systemctl status mysqld 2>/dev/null || echo 'Service status check failed'"
		statusResult := modelPtr.executeAndLogCommand(statusCmd)
		if statusResult.Error == nil && strings.Contains(statusResult.Output, "active") {
			m.report = append(m.report, infoStyle.Render("✅ MySQL service is running"))
		} else {
			m.report = append(m.report, warnStyle.Render("⚠️  MySQL service status unclear - check manually"))
		}

		m.report = append(m.report, infoStyle.Render("💡 Next steps:"))
		m.report = append(m.report, infoStyle.Render("  1. Run 'sudo mysql_secure_installation' to secure MySQL"))
		m.report = append(m.report, infoStyle.Render("  2. Create database users as needed"))

		m.refreshServiceStatus("mysql")
	}

	m.processingMsg = ""
	return m, nil
}

func (m model) installCaddy() (tea.Model, tea.Cmd) {
	m.state = stateProcessing
	m.processingMsg = "Installing Caddy server..."
	m.report = []string{infoStyle.Render("Installing Caddy server")}

	osType := getOSType()
	var command string

	switch osType {
	case "ubuntu":
		command = `sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https && \
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg && \
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list && \
sudo apt update && \
sudo apt install -y caddy`
	case "fedora":
		command = `sudo dnf install -y 'dnf-command(copr)' && \
sudo dnf copr enable @caddy/caddy -y && \
sudo dnf install -y caddy`
	default:
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Unsupported operating system: %s", osType)))
		m.processingMsg = ""
		return m, nil
	}

	// Execute command with logging
	modelPtr := &m
	result := modelPtr.executeAndLogCommand(command)

	if result.Error != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to install Caddy: %v", result.Error)))
		if strings.TrimSpace(result.Output) != "" {
			m.report = append(m.report, warnStyle.Render(fmt.Sprintf("Output: %s", result.Output)))
		}
		m.processingMsg = ""
		return m, nil
	}

	// Create Laravel Caddy configuration
	m.setupCaddyLaravelConfig()

	m.report = append(m.report, infoStyle.Render("✅ Caddy installed successfully"))
	m.refreshServiceStatus("caddy")
	m.processingMsg = ""
	return m, nil
}

func (m model) installGit() (tea.Model, tea.Cmd) {
	m.state = stateProcessing
	m.processingMsg = "Installing Git CLI..."
	m.report = []string{infoStyle.Render("Installing Git CLI")}

	osType := getOSType()
	var command string

	switch osType {
	case "ubuntu":
		command = "sudo apt update && sudo apt install -y git"
	case "fedora":
		command = "sudo dnf install -y git"
	default:
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Unsupported operating system: %s", osType)))
		m.processingMsg = ""
		return m, nil
	}

	// Execute command with logging
	modelPtr := &m
	result := modelPtr.executeAndLogCommand(command)

	if result.Error != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to install Git: %v", result.Error)))
		if strings.TrimSpace(result.Output) != "" {
			m.report = append(m.report, warnStyle.Render(fmt.Sprintf("Output: %s", result.Output)))
		}
	} else {
		m.report = append(m.report, infoStyle.Render("✅ Git installed successfully"))
		m.refreshServiceStatus("git")
	}

	m.processingMsg = ""
	return m, nil
}

func (m model) setupCaddyLaravelConfig() {
	m.report = append(m.report, infoStyle.Render("Setting up Caddy Laravel configuration"))

	// Create snippets directory
	os.MkdirAll("/etc/caddy/snippets", 0755)

	// Create Laravel snippet
	laravelSnippet := `php_fastcgi unix//run/php/php8.4-fpm.sock {
    root /var/www/{args[1]}
    split .php
    index index.php
    try_files {path} {path}/ /index.php?{query}
}`

	err := os.WriteFile("/etc/caddy/snippets/laravel.caddy", []byte(laravelSnippet), 0644)
	if err != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to create Laravel snippet: %v", err)))
		return
	}

	// Create main Caddyfile template
	caddyfile := `# Import all snippets
import snippets/*

# Laravel app template
# Usage: import laravel-app example.com /var/www/html/laravel-app
(laravel-app) {
    root * {args[1]}/public

    # Handle PHP files
    import laravel

    # Static file serving
    file_server

    # Security headers
    header {
        X-Content-Type-Options nosniff
        X-Frame-Options DENY
        X-XSS-Protection "1; mode=block"
        Referrer-Policy strict-origin-when-cross-origin
    }

    # Gzip compression
    encode gzip

    # Laravel specific
    try_files {path} {path}/ /index.php?{query}
}

# Example configuration (uncomment and modify as needed)
# example.com {
#     import laravel-app /var/www/html/laravel-app
# }`

	err = os.WriteFile("/etc/caddy/Caddyfile", []byte(caddyfile), 0644)
	if err != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to create Caddyfile: %v", err)))
		return
	}

	m.report = append(m.report, infoStyle.Render("✅ Caddy Laravel configuration created successfully"))
}

func getOSType() string {
	if runtime.GOOS != "linux" {
		return "unknown"
	}

	// Check for Ubuntu
	if _, err := os.Stat("/etc/lsb-release"); err == nil {
		content, err := os.ReadFile("/etc/lsb-release")
		if err == nil && strings.Contains(string(content), "Ubuntu") {
			return "ubuntu"
		}
	}

	// Check for Fedora
	if _, err := os.Stat("/etc/fedora-release"); err == nil {
		return "fedora"
	}

	// Check os-release for more distributions
	if content, err := os.ReadFile("/etc/os-release"); err == nil {
		contentStr := string(content)
		if strings.Contains(contentStr, "Ubuntu") {
			return "ubuntu"
		}
		if strings.Contains(contentStr, "Fedora") {
			return "fedora"
		}
	}

	return "unknown"
}

// isSystemdAvailable checks if systemd is the init system with multiple detection methods
func isSystemdAvailable() bool {
	// Method 1: Check if systemctl command exists and is functional
	if _, err := exec.LookPath("systemctl"); err == nil {
		cmd := exec.Command("systemctl", "--version")
		if err := cmd.Run(); err == nil {
			return true
		}
	}

	// Method 2: Check if /run/systemd/system directory exists (systemd is active)
	if _, err := os.Stat("/run/systemd/system"); err == nil {
		return true
	}

	// Method 3: Check PID 1 process name (fallback)
	cmd := exec.Command("ps", "-p", "1", "-o", "comm=")
	output, err := cmd.Output()
	if err != nil {
		// If we can't detect, assume systemd on modern systems
		return true
	}
	return strings.TrimSpace(string(output)) == "systemd"
}

// getServiceStartCommand returns the appropriate start command based on init system
func getServiceStartCommand(service string, isSystemd bool) string {
	if isSystemd {
		// Try systemctl first, with fallback verification
		return fmt.Sprintf("sudo systemctl start %s", service)
	}
	// Fallback to service command if available
	if _, err := exec.LookPath("service"); err == nil {
		return fmt.Sprintf("sudo service %s start", service)
	}
	// If neither is available, return systemctl anyway (most likely scenario)
	return fmt.Sprintf("sudo systemctl start %s", service)
}

// getServiceEnableCommand returns the appropriate enable command based on init system
func getServiceEnableCommand(service string, isSystemd bool) string {
	if isSystemd {
		return fmt.Sprintf("sudo systemctl enable %s", service)
	}
	// SysVinit doesn't have a direct equivalent for enable, but try chkconfig if available
	if _, err := exec.LookPath("chkconfig"); err == nil {
		return fmt.Sprintf("sudo chkconfig %s on", service)
	}
	return "" // No enable equivalent available
}
