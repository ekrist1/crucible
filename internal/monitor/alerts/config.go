package alerts

import (
	"fmt"
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// ConfigFile represents the YAML configuration file structure
type ConfigFile struct {
	EvaluationInterval string        `yaml:"evaluation_interval"`
	DefaultSeverity    AlertSeverity `yaml:"default_severity"`
	MaxAlertHistory    int           `yaml:"max_alert_history"`

	Email    EmailConfig     `yaml:"email"`
	Webhooks []WebhookConfig `yaml:"webhooks"`

	GlobalRateLimit struct {
		MaxPerHour   int    `yaml:"max_per_hour"`
		CooldownTime string `yaml:"cooldown_time"`
	} `yaml:"global_rate_limit"`

	Rules       []AlertRuleConfig `yaml:"rules"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

// AlertRuleConfig represents a rule configuration from YAML
type AlertRuleConfig struct {
	ID               string                `yaml:"id"`
	Name             string                `yaml:"name"`
	Type             AlertType             `yaml:"type"`
	Severity         AlertSeverity         `yaml:"severity"`
	Enabled          bool                  `yaml:"enabled"`
	Conditions       AlertConditionsConfig `yaml:"conditions"`
	NotifyEmails     []string              `yaml:"notify_emails"`
	NotifyWebhooks   []string              `yaml:"notify_webhooks"`
	MinInterval      string                `yaml:"min_interval"`
	MaxNotifications int                   `yaml:"max_notifications"`
	Labels           map[string]string     `yaml:"labels"`
	Annotations      map[string]string     `yaml:"annotations"`
}

// AlertConditionsConfig represents condition configuration from YAML
type AlertConditionsConfig struct {
	CPUThreshold    *float64 `yaml:"cpu_threshold,omitempty"`
	MemoryThreshold *float64 `yaml:"memory_threshold,omitempty"`
	DiskThreshold   *float64 `yaml:"disk_threshold,omitempty"`
	LoadThreshold   *float64 `yaml:"load_threshold,omitempty"`
	ServiceName     string   `yaml:"service_name,omitempty"`
	ServiceStatus   string   `yaml:"service_status,omitempty"`
	HTTPEndpoint    string   `yaml:"http_endpoint,omitempty"`
	ResponseTimeout string   `yaml:"response_timeout,omitempty"`
	ExpectedStatus  int      `yaml:"expected_status,omitempty"`
	Duration        string   `yaml:"duration,omitempty"`
}

// LoadConfig loads alert configuration from a YAML file
func LoadConfig(configPath string) (*Config, error) {
	// Read configuration file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	// Parse YAML
	var configFile ConfigFile
	if err := yaml.Unmarshal(data, &configFile); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %v", err)
	}

	// Convert to internal config structure
	config, err := convertConfig(&configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to convert config: %v", err)
	}

	// Load environment variables
	if err := loadEnvironmentOverrides(config); err != nil {
		return nil, fmt.Errorf("failed to load environment overrides: %v", err)
	}

	return config, nil
}

// convertConfig converts the YAML configuration to internal config structure
func convertConfig(configFile *ConfigFile) (*Config, error) {
	config := &Config{}

	// Parse evaluation interval
	if configFile.EvaluationInterval != "" {
		interval, err := time.ParseDuration(configFile.EvaluationInterval)
		if err != nil {
			return nil, fmt.Errorf("invalid evaluation_interval: %v", err)
		}
		config.EvaluationInterval = interval
	} else {
		config.EvaluationInterval = 30 * time.Second // Default
	}

	// Set default severity
	config.DefaultSeverity = configFile.DefaultSeverity
	if config.DefaultSeverity == "" {
		config.DefaultSeverity = SeverityWarning
	}

	// Set max alert history
	config.MaxAlertHistory = configFile.MaxAlertHistory
	if config.MaxAlertHistory == 0 {
		config.MaxAlertHistory = 1000 // Default
	}

	// Copy email configuration
	config.Email = configFile.Email

	// Copy webhook configuration
	config.Webhooks = configFile.Webhooks

	// Parse global rate limit
	if configFile.GlobalRateLimit.CooldownTime != "" {
		cooldown, err := time.ParseDuration(configFile.GlobalRateLimit.CooldownTime)
		if err != nil {
			return nil, fmt.Errorf("invalid global_rate_limit.cooldown_time: %v", err)
		}
		config.GlobalRateLimit.CooldownTime = cooldown
	} else {
		config.GlobalRateLimit.CooldownTime = 5 * time.Minute // Default
	}
	config.GlobalRateLimit.MaxPerHour = configFile.GlobalRateLimit.MaxPerHour
	if config.GlobalRateLimit.MaxPerHour == 0 {
		config.GlobalRateLimit.MaxPerHour = 50 // Default
	}

	return config, nil
}

// LoadRules loads alert rules from configuration
func LoadRules(configPath string) ([]*AlertRule, error) {
	// Read configuration file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	// Parse YAML
	var configFile ConfigFile
	if err := yaml.Unmarshal(data, &configFile); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %v", err)
	}

	// Convert rules
	rules := make([]*AlertRule, 0, len(configFile.Rules))
	for _, ruleConfig := range configFile.Rules {
		rule, err := convertRule(&ruleConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to convert rule %s: %v", ruleConfig.ID, err)
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

// convertRule converts a rule configuration to an AlertRule
func convertRule(ruleConfig *AlertRuleConfig) (*AlertRule, error) {
	rule := &AlertRule{
		ID:               ruleConfig.ID,
		Name:             ruleConfig.Name,
		Type:             ruleConfig.Type,
		Severity:         ruleConfig.Severity,
		Enabled:          ruleConfig.Enabled,
		NotifyEmails:     ruleConfig.NotifyEmails,
		NotifyWebhooks:   ruleConfig.NotifyWebhooks,
		MaxNotifications: ruleConfig.MaxNotifications,
		Labels:           ruleConfig.Labels,
		Annotations:      ruleConfig.Annotations,
	}

	// Parse minimum interval
	if ruleConfig.MinInterval != "" {
		interval, err := time.ParseDuration(ruleConfig.MinInterval)
		if err != nil {
			return nil, fmt.Errorf("invalid min_interval: %v", err)
		}
		rule.MinInterval = interval
	} else {
		rule.MinInterval = 5 * time.Minute // Default
	}

	// Convert conditions
	conditions, err := convertConditions(&ruleConfig.Conditions)
	if err != nil {
		return nil, fmt.Errorf("invalid conditions: %v", err)
	}
	rule.Conditions = *conditions

	// Set defaults
	if rule.Severity == "" {
		rule.Severity = SeverityWarning
	}
	if rule.MaxNotifications == 0 {
		rule.MaxNotifications = 10 // Default
	}
	if rule.Labels == nil {
		rule.Labels = make(map[string]string)
	}
	if rule.Annotations == nil {
		rule.Annotations = make(map[string]string)
	}

	return rule, nil
}

// convertConditions converts condition configuration to AlertConditions
func convertConditions(condConfig *AlertConditionsConfig) (*AlertConditions, error) {
	conditions := &AlertConditions{
		CPUThreshold:    condConfig.CPUThreshold,
		MemoryThreshold: condConfig.MemoryThreshold,
		DiskThreshold:   condConfig.DiskThreshold,
		LoadThreshold:   condConfig.LoadThreshold,
		ServiceName:     condConfig.ServiceName,
		ServiceStatus:   condConfig.ServiceStatus,
		HTTPEndpoint:    condConfig.HTTPEndpoint,
		ExpectedStatus:  condConfig.ExpectedStatus,
	}

	// Parse duration
	if condConfig.Duration != "" {
		duration, err := time.ParseDuration(condConfig.Duration)
		if err != nil {
			return nil, fmt.Errorf("invalid duration: %v", err)
		}
		conditions.Duration = duration
	}

	// Parse response timeout
	if condConfig.ResponseTimeout != "" {
		timeout, err := time.ParseDuration(condConfig.ResponseTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid response_timeout: %v", err)
		}
		conditions.ResponseTimeout = timeout
	}

	return conditions, nil
}

// loadEnvironmentOverrides loads sensitive configuration from environment variables
func loadEnvironmentOverrides(config *Config) error {
	// Load Resend API key from environment
	resendKey := os.Getenv("RESEND_API_KEY")
	log.Printf("DEBUG: loadEnvironmentOverrides - RESEND_API_KEY from env: '%s' (length: %d)", resendKey, len(resendKey))
	if resendKey != "" {
		config.Email.ResendAPIKey = resendKey
		log.Printf("DEBUG: loadEnvironmentOverrides - Set config.Email.ResendAPIKey to: '%s'", resendKey)
	} else {
		log.Printf("DEBUG: loadEnvironmentOverrides - RESEND_API_KEY is empty, not setting")
	}

	// Load email settings from environment if not set in config
	if fromEmail := os.Getenv("ALERT_FROM_EMAIL"); fromEmail != "" && config.Email.FromEmail == "" {
		config.Email.FromEmail = fromEmail
	}

	if fromName := os.Getenv("ALERT_FROM_NAME"); fromName != "" && config.Email.FromName == "" {
		config.Email.FromName = fromName
	}

	return nil
}

// CreateDefaultConfig creates a default configuration
func CreateDefaultConfig() *Config {
	return &Config{
		EvaluationInterval: 30 * time.Second,
		DefaultSeverity:    SeverityWarning,
		MaxAlertHistory:    1000,
		Email: EmailConfig{
			Enabled:   false, // Disabled by default until configured
			FromEmail: "alerts@localhost",
			FromName:  "Crucible Monitor",
			DefaultTo: []string{},
		},
		Webhooks: []WebhookConfig{},
		GlobalRateLimit: struct {
			MaxPerHour   int           `yaml:"max_per_hour"`
			CooldownTime time.Duration `yaml:"cooldown_time"`
		}{
			MaxPerHour:   50,
			CooldownTime: 5 * time.Minute,
		},
	}
}
