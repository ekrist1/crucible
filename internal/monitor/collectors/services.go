package collectors

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"crucible/internal/monitor"
)

// ServicesCollector collects systemd service status information
type ServicesCollector struct {
	services []string
}

// NewServicesCollector creates a new services collector
func NewServicesCollector(services []string) *ServicesCollector {
	return &ServicesCollector{
		services: services,
	}
}

// Collect gathers current service status information
func (s *ServicesCollector) Collect() ([]monitor.ServiceStatus, error) {
	var serviceStatuses []monitor.ServiceStatus

	// If no specific services configured, get all services
	services := s.services
	if len(services) == 0 {
		allServices, err := s.getAllServices()
		if err != nil {
			return nil, fmt.Errorf("failed to get all services: %w", err)
		}
		services = allServices
	}

	for _, serviceName := range services {
		status, err := s.getServiceStatus(serviceName)
		if err != nil {
			// Continue with other services if one fails
			continue
		}
		serviceStatuses = append(serviceStatuses, status)
	}

	return serviceStatuses, nil
}

// getAllServices gets a list of all systemd services
func (s *ServicesCollector) getAllServices() ([]string, error) {
	cmd := exec.Command("systemctl", "list-units", "--type=service", "--no-legend", "--no-pager")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	var services []string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) > 0 {
			serviceName := fields[0]
			// Remove .service suffix if present
			serviceName = strings.TrimSuffix(serviceName, ".service")
			services = append(services, serviceName)
		}
	}

	return services, nil
}

// getServiceStatus gets detailed status for a specific service
func (s *ServicesCollector) getServiceStatus(serviceName string) (monitor.ServiceStatus, error) {
	// Ensure service name has .service suffix for systemctl
	fullServiceName := serviceName
	if !strings.HasSuffix(serviceName, ".service") {
		fullServiceName = serviceName + ".service"
	}

	// Get service status using systemctl show
	cmd := exec.Command("systemctl", "show", fullServiceName,
		"--property=LoadState,ActiveState,SubState,ActiveEnterTimestamp,NRestarts,ExecMainStartTimestamp")
	output, err := cmd.Output()
	if err != nil {
		return monitor.ServiceStatus{}, fmt.Errorf("failed to get service status for %s: %w", serviceName, err)
	}

	status := monitor.ServiceStatus{
		Name: serviceName,
	}

	// Parse systemctl show output
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		switch key {
		case "LoadState":
			status.Status = value
		case "ActiveState":
			status.Active = value
		case "SubState":
			status.Sub = value
		case "ActiveEnterTimestamp":
			if value != "" && value != "n/a" {
				// Parse systemd timestamp format
				if timestamp, err := s.parseSystemdTimestamp(value); err == nil {
					status.Since = timestamp
				}
			}
		case "NRestarts":
			if restarts, err := strconv.Atoi(value); err == nil {
				status.RestartCount = restarts
			}
		case "ExecMainStartTimestamp":
			if value != "" && value != "n/a" {
				if timestamp, err := s.parseSystemdTimestamp(value); err == nil {
					status.LastRestart = timestamp
				}
			}
		}
	}

	// Add metadata for important services commonly used with Laravel/web servers
	status.Metadata = s.getServiceMetadata(serviceName)

	return status, nil
}

// parseSystemdTimestamp parses systemd timestamp format
func (s *ServicesCollector) parseSystemdTimestamp(timestamp string) (time.Time, error) {
	// Systemd timestamps are usually in format: "Mon 2023-08-01 15:30:45 UTC"
	// We'll try multiple common formats
	formats := []string{
		"Mon 2006-01-02 15:04:05 MST",
		"Mon 2006-01-02 15:04:05 UTC",
		time.RFC3339,
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timestamp); err == nil {
			return t, nil
		}
	}

	// If all parsing fails, try Unix timestamp
	if unixTime, err := strconv.ParseInt(timestamp, 10, 64); err == nil {
		return time.Unix(unixTime, 0), nil
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", timestamp)
}

// getServiceMetadata adds metadata for important services
func (s *ServicesCollector) getServiceMetadata(serviceName string) map[string]string {
	metadata := make(map[string]string)

	// Add service category and description for common services
	switch serviceName {
	case "mysql", "mariadb":
		metadata["category"] = "database"
		metadata["description"] = "MySQL/MariaDB database server"
	case "postgresql", "postgres":
		metadata["category"] = "database"
		metadata["description"] = "PostgreSQL database server"
	case "redis", "redis-server":
		metadata["category"] = "cache"
		metadata["description"] = "Redis in-memory data store"
	case "nginx":
		metadata["category"] = "webserver"
		metadata["description"] = "Nginx web server"
	case "apache2", "httpd":
		metadata["category"] = "webserver"
		metadata["description"] = "Apache web server"
	case "caddy":
		metadata["category"] = "webserver"
		metadata["description"] = "Caddy web server"
	case "php8.4-fpm", "php8.3-fpm", "php8.2-fpm", "php-fpm":
		metadata["category"] = "runtime"
		metadata["description"] = "PHP FastCGI Process Manager"
	case "supervisor":
		metadata["category"] = "process-manager"
		metadata["description"] = "Process control system"
	case "docker":
		metadata["category"] = "container"
		metadata["description"] = "Docker container runtime"
	case "fail2ban":
		metadata["category"] = "security"
		metadata["description"] = "Intrusion prevention system"
	case "ufw":
		metadata["category"] = "security"
		metadata["description"] = "Uncomplicated Firewall"
	case "ssh", "sshd":
		metadata["category"] = "remote-access"
		metadata["description"] = "SSH daemon"
	default:
		metadata["category"] = "system"
		metadata["description"] = "System service"
	}

	return metadata
}

// GetImportantServices returns a list of services commonly used in web development
func GetImportantServices() []string {
	return []string{
		"mysql",
		"mariadb",
		"postgresql",
		"redis",
		"nginx",
		"apache2",
		"caddy",
		"php8.4-fpm",
		"php8.3-fpm",
		"php8.2-fpm",
		"php-fpm",
		"supervisor",
		"docker",
		"fail2ban",
		"ssh",
	}
}
