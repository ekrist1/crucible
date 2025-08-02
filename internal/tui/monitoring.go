package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"crucible/internal/monitor"
)

// System monitoring and dashboard functions

// showMonitoringDashboard displays the monitoring dashboard with system metrics
func (m Model) showMonitoringDashboard() (tea.Model, tea.Cmd) {
	m.State = StateProcessing
	m.Report = []string{}
	m.ProcessingMsg = "Loading monitoring data..."

	// Build monitoring dashboard report
	m.Report = append(m.Report, TitleStyle.Render("=== MONITORING DASHBOARD ==="))
	m.Report = append(m.Report, "")

	// Check if monitoring agent is running
	agentStatus := m.checkMonitoringAgent()
	m.Report = append(m.Report, InfoStyle.Render("🔧 Monitoring Agent:"))
	m.Report = append(m.Report, agentStatus)
	m.Report = append(m.Report, "")

	// If agent is running, fetch metrics
	if strings.Contains(agentStatus, "✅") {
		// Fetch system metrics
		systemMetrics := m.fetchSystemMetrics()
		m.Report = append(m.Report, InfoStyle.Render("📊 System Metrics:"))
		m.Report = append(m.Report, systemMetrics...)
		m.Report = append(m.Report, "")

		// Fetch service metrics
		serviceMetrics := m.fetchServiceMetrics()
		m.Report = append(m.Report, InfoStyle.Render("⚙️ Service Status:"))
		m.Report = append(m.Report, serviceMetrics...)
		m.Report = append(m.Report, "")

		// Fetch HTTP check results
		httpMetrics := m.fetchHTTPMetrics()
		m.Report = append(m.Report, InfoStyle.Render("🌐 HTTP Health Checks:"))
		m.Report = append(m.Report, httpMetrics...)
	} else {
		m.Report = append(m.Report, WarnStyle.Render("⚠️ Start monitoring agent with: ./crucible-monitor"))
		m.Report = append(m.Report, WarnStyle.Render("⚠️ Or use: make run-monitor"))
	}

	m.ProcessingMsg = ""
	return m, tea.ClearScreen
}

// checkMonitoringAgent checks if the monitoring agent is running
func (m Model) checkMonitoringAgent() string {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9090/api/v1/health")
	if err != nil {
		return WarnStyle.Render("❌ Agent not running")
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return InfoStyle.Render("✅ Agent running on port 9090")
	}
	return WarnStyle.Render("❌ Agent unhealthy")
}

// fetchSystemMetrics fetches system metrics from the monitoring agent
func (m Model) fetchSystemMetrics() []string {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9090/api/v1/metrics/system")
	if err != nil {
		return []string{WarnStyle.Render("❌ Failed to fetch system metrics")}
	}
	defer resp.Body.Close()

	var metrics monitor.SystemMetrics
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		return []string{WarnStyle.Render("❌ Failed to parse system metrics")}
	}

	var result []string

	// CPU metrics
	result = append(result, fmt.Sprintf("  CPU Usage: %.1f%% (User: %.1f%%, System: %.1f%%, I/O Wait: %.1f%%)",
		metrics.CPU.UsagePercent, metrics.CPU.UserPercent, metrics.CPU.SystemPercent, metrics.CPU.IOWaitPercent))

	// Memory metrics
	memUsedGB := float64(metrics.Memory.UsedBytes) / (1024 * 1024 * 1024)
	memTotalGB := float64(metrics.Memory.TotalBytes) / (1024 * 1024 * 1024)
	result = append(result, fmt.Sprintf("  Memory: %.1fGB/%.1fGB (%.1f%%) | Swap: %.1f%%",
		memUsedGB, memTotalGB, metrics.Memory.UsagePercent, metrics.Memory.SwapUsagePercent))

	// Load average
	result = append(result, fmt.Sprintf("  Load Average: %.2f, %.2f, %.2f",
		metrics.Load.Load1, metrics.Load.Load5, metrics.Load.Load15))

	// Disk usage for main partitions
	for _, disk := range metrics.Disk {
		if disk.MountPoint == "/" || disk.MountPoint == "/home" {
			usedGB := float64(disk.UsedBytes) / (1024 * 1024 * 1024)
			totalGB := float64(disk.TotalBytes) / (1024 * 1024 * 1024)
			result = append(result, fmt.Sprintf("  Disk %s: %.1fGB/%.1fGB (%.1f%%)",
				disk.MountPoint, usedGB, totalGB, disk.UsagePercent))
		}
	}

	// Network stats (top interfaces)
	for i, net := range metrics.Network {
		if i >= 2 { // Limit to top 2 interfaces
			break
		}
		recvMB := float64(net.BytesRecv) / (1024 * 1024)
		sentMB := float64(net.BytesSent) / (1024 * 1024)
		result = append(result, fmt.Sprintf("  Network %s: ↓%.1fMB ↑%.1fMB (Errors: %d)",
			net.Interface, recvMB, sentMB, net.ErrorsRecv+net.ErrorsSent))
	}

	return result
}

// fetchServiceMetrics fetches service status from the monitoring agent
func (m Model) fetchServiceMetrics() []string {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9090/api/v1/metrics/services")
	if err != nil {
		return []string{WarnStyle.Render("❌ Failed to fetch service metrics")}
	}
	defer resp.Body.Close()

	var services []monitor.ServiceStatus
	if err := json.NewDecoder(resp.Body).Decode(&services); err != nil {
		return []string{WarnStyle.Render("❌ Failed to parse service metrics")}
	}

	var result []string

	// Group services by category
	categories := map[string][]monitor.ServiceStatus{
		"database":  {},
		"webserver": {},
		"runtime":   {},
		"security":  {},
		"system":    {},
	}

	for _, service := range services {
		category := "system"
		if cat, exists := service.Metadata["category"]; exists {
			category = cat
		}
		categories[category] = append(categories[category], service)
	}

	// Display important categories first
	for _, category := range []string{"database", "webserver", "runtime", "security"} {
		if len(categories[category]) > 0 {
			for _, service := range categories[category] {
				status := "❌"
				if service.Active == "active" && service.Sub == "running" {
					status = "✅"
				} else if service.Active == "active" {
					status = "⚠️"
				}

				uptime := time.Since(service.Since)
				result = append(result, fmt.Sprintf("  %s %s (%s) - Up: %s",
					status, service.Name, service.Sub, formatDuration(uptime)))
			}
		}
	}

	// Show count of other services
	otherCount := len(categories["system"])
	if otherCount > 0 {
		result = append(result, fmt.Sprintf("  + %d other system services", otherCount))
	}

	return result
}

// fetchHTTPMetrics fetches HTTP check results from the monitoring agent
func (m Model) fetchHTTPMetrics() []string {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9090/api/v1/metrics/http")
	if err != nil {
		return []string{WarnStyle.Render("❌ Failed to fetch HTTP metrics")}
	}
	defer resp.Body.Close()

	var checks []monitor.HTTPCheckResult
	if err := json.NewDecoder(resp.Body).Decode(&checks); err != nil {
		return []string{WarnStyle.Render("❌ Failed to parse HTTP metrics")}
	}

	if len(checks) == 0 {
		return []string{
			ChoiceStyle.Render("  No HTTP checks configured"),
			ChoiceStyle.Render("  Enable in configs/monitor.yaml to monitor web endpoints"),
		}
	}

	var result []string
	for _, check := range checks {
		status := "❌"
		if check.Success {
			status = "✅"
		}

		result = append(result, fmt.Sprintf("  %s %s - %dms (Status: %d)",
			status, check.Name, check.ResponseTime.Milliseconds(), check.StatusCode))

		if check.Error != "" {
			// Simplify connection refused errors
			errorMsg := check.Error
			if strings.Contains(errorMsg, "connection refused") {
				errorMsg = "Connection refused - service not running"
			} else if strings.Contains(errorMsg, "no such host") {
				errorMsg = "Host not found"
			} else if strings.Contains(errorMsg, "timeout") {
				errorMsg = "Request timeout"
			}
			result = append(result, fmt.Sprintf("    %s", errorMsg))
		}
	}

	return result
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	} else {
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
