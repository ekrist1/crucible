package actions

import (
	"fmt"
	"strings"
)

// ServiceInfo represents information about a systemd service
type ServiceInfo struct {
	Name   string
	Status string
	Active string
	Sub    string
}

// ServiceActionConfig contains configuration for service actions
type ServiceActionConfig struct {
	ServiceName string
	Action      string // start, stop, restart, reload, enable, disable
}

// ListActiveServices returns commands to list all active services
func ListActiveServices() ([]string, []string) {
	commands := []string{
		"systemctl list-units --type=service --state=active --no-pager --plain",
	}
	descriptions := []string{
		"Listing active services...",
	}
	return commands, descriptions
}

// GetServiceStatus returns commands to get detailed status of a specific service
func GetServiceStatus(serviceName string) ([]string, []string) {
	commands := []string{
		fmt.Sprintf("systemctl status %s --no-pager --lines=10", serviceName),
	}
	descriptions := []string{
		fmt.Sprintf("Getting status of %s service...", serviceName),
	}
	return commands, descriptions
}

// ControlService returns commands to control a service (start, stop, restart, reload, enable, disable)
func ControlService(config ServiceActionConfig) ([]string, []string, error) {
	var commands []string
	var descriptions []string

	switch strings.ToLower(config.Action) {
	case "start":
		commands = append(commands, fmt.Sprintf("sudo systemctl start %s", config.ServiceName))
		descriptions = append(descriptions, fmt.Sprintf("Starting %s service...", config.ServiceName))
	case "stop":
		commands = append(commands, fmt.Sprintf("sudo systemctl stop %s", config.ServiceName))
		descriptions = append(descriptions, fmt.Sprintf("Stopping %s service...", config.ServiceName))
	case "restart":
		commands = append(commands, fmt.Sprintf("sudo systemctl restart %s", config.ServiceName))
		descriptions = append(descriptions, fmt.Sprintf("Restarting %s service...", config.ServiceName))
	case "reload":
		commands = append(commands, fmt.Sprintf("sudo systemctl reload %s", config.ServiceName))
		descriptions = append(descriptions, fmt.Sprintf("Reloading %s service...", config.ServiceName))
	case "enable":
		commands = append(commands, fmt.Sprintf("sudo systemctl enable %s", config.ServiceName))
		descriptions = append(descriptions, fmt.Sprintf("Enabling %s service...", config.ServiceName))
	case "disable":
		commands = append(commands, fmt.Sprintf("sudo systemctl disable %s", config.ServiceName))
		descriptions = append(descriptions, fmt.Sprintf("Disabling %s service...", config.ServiceName))
	case "status":
		commands = append(commands, fmt.Sprintf("systemctl status %s --no-pager --lines=15", config.ServiceName))
		descriptions = append(descriptions, fmt.Sprintf("Checking %s service status...", config.ServiceName))
	default:
		return nil, nil, fmt.Errorf("unsupported service action: %s. Supported actions: start, stop, restart, reload, enable, disable, status", config.Action)
	}

	// Add a status check after the action (except for status action itself)
	if config.Action != "status" {
		commands = append(commands, fmt.Sprintf("systemctl is-active %s", config.ServiceName))
		descriptions = append(descriptions, fmt.Sprintf("Verifying %s service state...", config.ServiceName))
	}

	return commands, descriptions, nil
}

// GetCommonServices returns a list of commonly managed services
func GetCommonServices() []string {
	return []string{
		"apache2",
		"nginx",
		"caddy",
		"mysql",
		"mariadb",
		"postgresql",
		"php8.4-fpm",
		"php8.3-fpm",
		"php-fpm",
		"redis",
		"supervisor",
		"ssh",
		"ufw",
		"fail2ban",
	}
}

// ParseServiceList parses the output of systemctl list-units command
func ParseServiceList(output string) []ServiceInfo {
	var services []ServiceInfo
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "UNIT") || strings.HasPrefix(line, "●") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 4 && strings.HasSuffix(fields[0], ".service") {
			services = append(services, ServiceInfo{
				Name:   strings.TrimSuffix(fields[0], ".service"),
				Status: fields[1],
				Active: fields[2],
				Sub:    fields[3],
			})
		}
	}

	return services
}
