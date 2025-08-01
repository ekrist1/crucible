package system

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// GetOSType detects the operating system type
func GetOSType() string {
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

// IsSystemdAvailable checks if systemd is the init system with multiple detection methods
func IsSystemdAvailable() bool {
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

// GetServiceStartCommand returns the appropriate start command based on init system
func GetServiceStartCommand(service string, isSystemd bool) string {
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

// GetServiceEnableCommand returns the appropriate enable command based on init system
func GetServiceEnableCommand(service string, isSystemd bool) string {
	if isSystemd {
		return fmt.Sprintf("sudo systemctl enable %s", service)
	}
	// SysVinit doesn't have a direct equivalent for enable, but try chkconfig if available
	if _, err := exec.LookPath("chkconfig"); err == nil {
		return fmt.Sprintf("sudo chkconfig %s on", service)
	}
	return "" // No enable equivalent available
}

// GetWebServerUser returns the appropriate web server user based on the operating system
func GetWebServerUser() string {
	osType := GetOSType()
	switch osType {
	case "ubuntu":
		return "www-data"
	case "fedora":
		// Check if nginx is installed, otherwise use apache
		if _, err := exec.LookPath("nginx"); err == nil {
			return "nginx"
		}
		return "apache"
	default:
		// Default fallback - try www-data first, then nginx, then apache
		for _, user := range []string{"www-data", "nginx", "apache"} {
			if UserExists(user) {
				return user
			}
		}
		return "www-data" // Final fallback
	}
}

// UserExists checks if a user exists on the system
func UserExists(username string) bool {
	cmd := exec.Command("id", username)
	err := cmd.Run()
	return err == nil
}
