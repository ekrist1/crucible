package monitor

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultConfigPaths defines the default locations to search for configuration files
var DefaultConfigPaths = []string{
	"/etc/crucible/monitor.yaml",
	"/usr/local/etc/crucible/monitor.yaml",
	"./configs/monitor.yaml",
	"./monitor.yaml",
}

// LoadConfig loads the monitoring configuration from the specified path or default locations
func LoadConfig(configPath string) (*Config, error) {
	var config Config
	var configFile string
	var err error

	// If specific path is provided, use it
	if configPath != "" {
		configFile = configPath
	} else {
		// Search for config file in default locations
		configFile, err = findConfigFile()
		if err != nil {
			return nil, fmt.Errorf("config file not found in default locations: %w", err)
		}
	}

	// Read config file
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configFile, err)
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configFile, err)
	}

	// Validate and set defaults
	if err := validateAndSetDefaults(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// findConfigFile searches for a config file in default locations
func findConfigFile() (string, error) {
	for _, path := range DefaultConfigPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no config file found in default paths: %v", DefaultConfigPaths)
}

// validateAndSetDefaults validates the configuration and sets default values
func validateAndSetDefaults(config *Config) error {
	// Agent defaults
	if config.Agent.ListenAddr == "" {
		config.Agent.ListenAddr = "127.0.0.1:9090"
	}
	if config.Agent.DataRetention == "" {
		config.Agent.DataRetention = "30d"
	}
	if config.Agent.CollectInterval == "" {
		config.Agent.CollectInterval = "30s"
	}

	// Validate agent intervals
	if _, err := time.ParseDuration(config.Agent.CollectInterval); err != nil {
		return fmt.Errorf("invalid collect_interval: %w", err)
	}

	// Storage defaults
	if config.Storage.Type == "" {
		config.Storage.Type = "memory"
	}
	if config.Storage.SQLite.Path == "" {
		config.Storage.SQLite.Path = "/var/lib/crucible/monitor.db"
	}
	if config.Storage.Aggregation.RawRetention == "" {
		config.Storage.Aggregation.RawRetention = "24h"
	}
	if config.Storage.Aggregation.MinuteRetention == "" {
		config.Storage.Aggregation.MinuteRetention = "7d"
	}
	if config.Storage.Aggregation.HourRetention == "" {
		config.Storage.Aggregation.HourRetention = "30d"
	}

	// Collector defaults
	if config.Collectors.System.Interval == "" {
		config.Collectors.System.Interval = "30s"
	}
	if config.Collectors.Services.Interval == "" {
		config.Collectors.Services.Interval = "60s"
	}

	// Validate collector intervals
	if config.Collectors.System.Enabled {
		if _, err := time.ParseDuration(config.Collectors.System.Interval); err != nil {
			return fmt.Errorf("invalid system collector interval: %w", err)
		}
	}
	if config.Collectors.Services.Enabled {
		if _, err := time.ParseDuration(config.Collectors.Services.Interval); err != nil {
			return fmt.Errorf("invalid services collector interval: %w", err)
		}
	}

	// Validate HTTP checks
	for i, check := range config.Collectors.HTTPChecks.Checks {
		if check.Name == "" {
			return fmt.Errorf("HTTP check %d: name is required", i)
		}
		if check.URL == "" {
			return fmt.Errorf("HTTP check %s: URL is required", check.Name)
		}
		if check.Interval == "" {
			config.Collectors.HTTPChecks.Checks[i].Interval = "60s"
		}
		if check.Timeout == "" {
			config.Collectors.HTTPChecks.Checks[i].Timeout = "10s"
		}
		if check.ExpectedStatus == 0 {
			config.Collectors.HTTPChecks.Checks[i].ExpectedStatus = 200
		}

		// Validate interval and timeout
		if _, err := time.ParseDuration(check.Interval); err != nil {
			return fmt.Errorf("HTTP check %s: invalid interval: %w", check.Name, err)
		}
		if _, err := time.ParseDuration(check.Timeout); err != nil {
			return fmt.Errorf("HTTP check %s: invalid timeout: %w", check.Name, err)
		}
	}

	// Alert defaults
	if config.Alerts.CheckInterval == "" {
		config.Alerts.CheckInterval = "60s"
	}
	if config.Alerts.Enabled {
		if _, err := time.ParseDuration(config.Alerts.CheckInterval); err != nil {
			return fmt.Errorf("invalid alert check_interval: %w", err)
		}

		// Set default thresholds if not provided
		if config.Alerts.Thresholds.CPUPercent == 0 {
			config.Alerts.Thresholds.CPUPercent = 80.0
		}
		if config.Alerts.Thresholds.MemoryPercent == 0 {
			config.Alerts.Thresholds.MemoryPercent = 90.0
		}
		if config.Alerts.Thresholds.DiskPercent == 0 {
			config.Alerts.Thresholds.DiskPercent = 85.0
		}
		if config.Alerts.Thresholds.LoadAverage == 0 {
			config.Alerts.Thresholds.LoadAverage = 5.0
		}
		if config.Alerts.Thresholds.ResponseTimeMs == 0 {
			config.Alerts.Thresholds.ResponseTimeMs = 5000
		}
	}

	// Notification defaults
	if config.Notifications.Email.SMTPPort == 0 {
		config.Notifications.Email.SMTPPort = 587
	}
	if config.Notifications.Webhook.Timeout == "" {
		config.Notifications.Webhook.Timeout = "10s"
	}

	return nil
}

// SaveConfig saves the configuration to the specified file
func SaveConfig(config *Config, configPath string) error {
	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", configPath, err)
	}

	return nil
}

// GetCollectInterval parses and returns the collect interval as a duration
func (c *Config) GetCollectInterval() time.Duration {
	duration, _ := time.ParseDuration(c.Agent.CollectInterval)
	return duration
}

// GetSystemCollectorInterval parses and returns the system collector interval as a duration
func (c *Config) GetSystemCollectorInterval() time.Duration {
	duration, _ := time.ParseDuration(c.Collectors.System.Interval)
	return duration
}

// GetServicesCollectorInterval parses and returns the services collector interval as a duration
func (c *Config) GetServicesCollectorInterval() time.Duration {
	duration, _ := time.ParseDuration(c.Collectors.Services.Interval)
	return duration
}

// GetAlertCheckInterval parses and returns the alert check interval as a duration
func (c *Config) GetAlertCheckInterval() time.Duration {
	duration, _ := time.ParseDuration(c.Alerts.CheckInterval)
	return duration
}

// IsSystemMetricEnabled checks if a specific system metric is enabled
func (c *Config) IsSystemMetricEnabled(metric string) bool {
	if !c.Collectors.System.Enabled {
		return false
	}

	// If no specific metrics are configured, enable all
	if len(c.Collectors.System.Metrics) == 0 {
		return true
	}

	// Check if the metric is in the enabled list
	for _, enabledMetric := range c.Collectors.System.Metrics {
		if enabledMetric == metric {
			return true
		}
	}

	return false
}

// GetHTTPCheckInterval parses and returns the HTTP check interval as a duration
func (check *HTTPCheck) GetInterval() time.Duration {
	duration, _ := time.ParseDuration(check.Interval)
	return duration
}

// GetHTTPCheckTimeout parses and returns the HTTP check timeout as a duration
func (check *HTTPCheck) GetTimeout() time.Duration {
	duration, _ := time.ParseDuration(check.Timeout)
	return duration
}
