package alerts

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"crucible/internal/monitor/alerts/notifiers"
	"github.com/google/uuid"
)

// EmailNotifierWrapper wraps the notifiers.EmailNotifier to implement our Notifier interface
type EmailNotifierWrapper struct {
	notifier *notifiers.EmailNotifier
}

func (w *EmailNotifierWrapper) Name() string {
	return w.notifier.Name()
}

func (w *EmailNotifierWrapper) IsEnabled() bool {
	return w.notifier.IsEnabled()
}

func (w *EmailNotifierWrapper) Send(alert *Alert) error {
	// Convert our Alert to notifiers.Alert
	notifierAlert := &notifiers.Alert{
		ID:          alert.ID,
		Name:        alert.Name,
		Type:        notifiers.AlertType(alert.Type),
		Severity:    notifiers.AlertSeverity(alert.Severity),
		Message:     alert.Message,
		Details:     alert.Details,
		Labels:      alert.Labels,
		Annotations: alert.Annotations,
		StartsAt:    alert.StartsAt,
		EndsAt:      alert.EndsAt,
	}

	return w.notifier.Send(notifierAlert)
}

// NewAlertManager creates a new alert manager instance
func NewAlertManager(config *Config) *AlertManager {
	am := &AlertManager{
		rules:        make(map[string]*AlertRule),
		activeAlerts: make(map[string]*Alert),
		alertHistory: make([]*Alert, 0),
		notifiers:    make([]Notifier, 0),
		config:       config,
	}

	// Initialize notifiers based on configuration
	am.initializeNotifiers()

	return am
}

// initializeNotifiers sets up notification channels based on configuration
func (am *AlertManager) initializeNotifiers() {
	log.Printf("DEBUG: Initializing notifiers...")
	log.Printf("DEBUG: Email config - Enabled: %v", am.config.Email.Enabled)
	log.Printf("DEBUG: Email config - ResendAPIKey present: %v", am.config.Email.ResendAPIKey != "")
	log.Printf("DEBUG: Email config - ResendAPIKey length: %d", len(am.config.Email.ResendAPIKey))
	log.Printf("DEBUG: Email config - FromEmail: %s", am.config.Email.FromEmail)
	log.Printf("DEBUG: Email config - DefaultTo: %v", am.config.Email.DefaultTo)
	
	// Add email notifier if enabled
	if am.config.Email.Enabled {
		log.Printf("DEBUG: Email is enabled in config, creating email notifier...")
		log.Printf("DEBUG: am.config.Email.ResendAPIKey: '%s' (length: %d)", am.config.Email.ResendAPIKey, len(am.config.Email.ResendAPIKey))
		
		// Check environment variable directly here too
		envKey := os.Getenv("RESEND_API_KEY")
		log.Printf("DEBUG: Direct os.Getenv(RESEND_API_KEY): '%s' (length: %d)", envKey, len(envKey))
		
		// Convert config to notifiers package format
		notifierConfig := &notifiers.EmailConfig{
			Enabled:         am.config.Email.Enabled,
			ResendAPIKey:    am.config.Email.ResendAPIKey,
			FromEmail:       am.config.Email.FromEmail,
			FromName:        am.config.Email.FromName,
			DefaultTo:       am.config.Email.DefaultTo,
			SubjectTemplate: am.config.Email.SubjectTemplate,
			BodyTemplate:    am.config.Email.BodyTemplate,
		}
		log.Printf("DEBUG: notifierConfig.ResendAPIKey: '%s' (length: %d)", notifierConfig.ResendAPIKey, len(notifierConfig.ResendAPIKey))
		
		// If config is empty but env var has value, use env var directly
		if notifierConfig.ResendAPIKey == "" && envKey != "" {
			log.Printf("DEBUG: Config ResendAPIKey is empty but env var has value, using env var directly")
			notifierConfig.ResendAPIKey = envKey
			log.Printf("DEBUG: Updated notifierConfig.ResendAPIKey: '%s' (length: %d)", notifierConfig.ResendAPIKey, len(notifierConfig.ResendAPIKey))
		}
		emailNotifier := notifiers.NewEmailNotifier(notifierConfig)
		am.notifiers = append(am.notifiers, &EmailNotifierWrapper{notifier: emailNotifier})
		log.Printf("DEBUG: Email notifier added to notifiers list")
	} else {
		log.Printf("DEBUG: Email is disabled in config, skipping email notifier")
	}

	log.Printf("DEBUG: Total notifiers initialized: %d", len(am.notifiers))

	// TODO: Add webhook notifiers when implemented
	// for _, webhookConfig := range am.config.Webhooks {
	// 	if webhookConfig.Enabled {
	// 		webhookNotifier := notifiers.NewWebhookNotifier(&webhookConfig)
	// 		am.notifiers = append(am.notifiers, webhookNotifier)
	// 	}
	// }
}

// AddRule adds a new alert rule
func (am *AlertManager) AddRule(rule *AlertRule) {
	am.rules[rule.ID] = rule
}

// RemoveRule removes an alert rule
func (am *AlertManager) RemoveRule(ruleID string) {
	delete(am.rules, ruleID)
}

// GetActiveAlerts returns all currently active alerts
func (am *AlertManager) GetActiveAlerts() []*Alert {
	alerts := make([]*Alert, 0, len(am.activeAlerts))
	for _, alert := range am.activeAlerts {
		alerts = append(alerts, alert)
	}
	return alerts
}

// GetAlertHistory returns recent alert history
func (am *AlertManager) GetAlertHistory() []*Alert {
	return am.alertHistory
}

// EvaluateRules evaluates all rules against current metrics
func (am *AlertManager) EvaluateRules(ctx *EvaluationContext) error {
	am.lastEvaluation = ctx.CurrentTime

	var wg sync.WaitGroup
	for _, rule := range am.rules {
		if !rule.Enabled {
			continue
		}

		wg.Add(1)
		go func(r *AlertRule) {
			defer wg.Done()
			am.evaluateRule(r, ctx)
		}(rule)
	}

	wg.Wait()
	return nil
}

// evaluateRule evaluates a single rule against the current context
func (am *AlertManager) evaluateRule(rule *AlertRule, ctx *EvaluationContext) {
	alertID := fmt.Sprintf("%s-%s", rule.ID, "current")

	// Check if condition is met
	conditionMet, details := am.checkCondition(rule, ctx)

	existingAlert, exists := am.activeAlerts[alertID]

	if conditionMet {
		if !exists {
			// Create new alert
			alert := &Alert{
				ID:                alertID,
				Name:              rule.Name,
				Type:              rule.Type,
				Severity:          rule.Severity,
				Status:            StatusFiring,
				Message:           am.generateAlertMessage(rule, details),
				Details:           details,
				Labels:            rule.Labels,
				Annotations:       rule.Annotations,
				StartsAt:          ctx.CurrentTime,
				RuleID:            rule.ID,
				NotificationsSent: 0,
				SentTo:            make([]string, 0),
			}

			am.activeAlerts[alertID] = alert
			am.sendNotifications(alert, rule)

			log.Printf("Alert fired: %s - %s", alert.Name, alert.Message)
		} else {
			// Update existing alert
			existingAlert.Details = details
			existingAlert.Message = am.generateAlertMessage(rule, details)

			// Check if we should send another notification
			if am.shouldSendNotification(existingAlert, rule) {
				am.sendNotifications(existingAlert, rule)
			}
		}
	} else {
		if exists {
			// Resolve alert
			existingAlert.Status = StatusResolved
			existingAlert.EndsAt = &ctx.CurrentTime

			// Move to history
			am.addToHistory(existingAlert)
			delete(am.activeAlerts, alertID)

			log.Printf("Alert resolved: %s", existingAlert.Name)
		}
	}
}

// checkCondition evaluates whether an alert condition is met
func (am *AlertManager) checkCondition(rule *AlertRule, ctx *EvaluationContext) (bool, map[string]interface{}) {
	details := make(map[string]interface{})

	switch rule.Type {
	case AlertTypeSystem:
		return am.checkSystemCondition(rule, ctx, details)
	case AlertTypeService:
		return am.checkServiceCondition(rule, ctx, details)
	case AlertTypeHTTP:
		return am.checkHTTPCondition(rule, ctx, details)
	default:
		return false, details
	}
}

// checkSystemCondition checks system metric thresholds
func (am *AlertManager) checkSystemCondition(rule *AlertRule, ctx *EvaluationContext, details map[string]interface{}) (bool, map[string]interface{}) {
	conditions := rule.Conditions

	// CPU threshold check
	if conditions.CPUThreshold != nil {
		if cpuMetric, exists := ctx.SystemMetrics["cpu_usage"]; exists {
			details["cpu_usage"] = cpuMetric.Value
			if cpuMetric.Value > *conditions.CPUThreshold {
				details["threshold"] = *conditions.CPUThreshold
				details["metric"] = "CPU usage"
				return true, details
			}
		}
	}

	// Memory threshold check
	if conditions.MemoryThreshold != nil {
		if memMetric, exists := ctx.SystemMetrics["memory_usage"]; exists {
			details["memory_usage"] = memMetric.Value
			if memMetric.Value > *conditions.MemoryThreshold {
				details["threshold"] = *conditions.MemoryThreshold
				details["metric"] = "Memory usage"
				return true, details
			}
		}
	}

	// Disk threshold check
	if conditions.DiskThreshold != nil {
		if diskMetric, exists := ctx.SystemMetrics["disk_usage"]; exists {
			details["disk_usage"] = diskMetric.Value
			if diskMetric.Value > *conditions.DiskThreshold {
				details["threshold"] = *conditions.DiskThreshold
				details["metric"] = "Disk usage"
				return true, details
			}
		}
	}

	// Load threshold check
	if conditions.LoadThreshold != nil {
		if loadMetric, exists := ctx.SystemMetrics["load_average"]; exists {
			details["load_average"] = loadMetric.Value
			if loadMetric.Value > *conditions.LoadThreshold {
				details["threshold"] = *conditions.LoadThreshold
				details["metric"] = "Load average"
				return true, details
			}
		}
	}

	return false, details
}

// checkServiceCondition checks service status
func (am *AlertManager) checkServiceCondition(rule *AlertRule, ctx *EvaluationContext, details map[string]interface{}) (bool, map[string]interface{}) {
	conditions := rule.Conditions

	if conditions.ServiceName != "" {
		if status, exists := ctx.ServiceStates[conditions.ServiceName]; exists {
			details["service_name"] = conditions.ServiceName
			details["current_status"] = status
			details["expected_status"] = conditions.ServiceStatus

			// If expected status is specified, check for exact match
			if conditions.ServiceStatus != "" {
				return status != conditions.ServiceStatus, details
			}

			// Default: alert if service is not active
			return status != "active", details
		}
	}

	return false, details
}

// checkHTTPCondition checks HTTP endpoint health
func (am *AlertManager) checkHTTPCondition(rule *AlertRule, ctx *EvaluationContext, details map[string]interface{}) (bool, map[string]interface{}) {
	conditions := rule.Conditions

	if conditions.HTTPEndpoint != "" {
		if result, exists := ctx.HTTPResults[conditions.HTTPEndpoint]; exists {
			details["endpoint"] = conditions.HTTPEndpoint
			details["status_code"] = result.StatusCode
			details["response_time"] = result.ResponseTime.Milliseconds()
			details["success"] = result.Success

			// Check if endpoint is failing
			if !result.Success {
				details["error"] = result.Error
				return true, details
			}

			// Check response time threshold
			if conditions.ResponseTimeout > 0 && result.ResponseTime > conditions.ResponseTimeout {
				details["timeout_threshold"] = conditions.ResponseTimeout.Milliseconds()
				return true, details
			}

			// Check expected status code
			if conditions.ExpectedStatus > 0 && result.StatusCode != conditions.ExpectedStatus {
				details["expected_status"] = conditions.ExpectedStatus
				return true, details
			}
		}
	}

	return false, details
}

// generateAlertMessage creates a human-readable alert message
func (am *AlertManager) generateAlertMessage(rule *AlertRule, details map[string]interface{}) string {
	switch rule.Type {
	case AlertTypeSystem:
		if metric, ok := details["metric"].(string); ok {
			if threshold, ok := details["threshold"].(float64); ok {
				if value, ok := details[getMetricKey(metric)].(float64); ok {
					return fmt.Sprintf("%s is %.1f%%, exceeding threshold of %.1f%%",
						metric, value, threshold)
				}
			}
		}
	case AlertTypeService:
		if serviceName, ok := details["service_name"].(string); ok {
			if status, ok := details["current_status"].(string); ok {
				return fmt.Sprintf("Service %s is %s", serviceName, status)
			}
		}
	case AlertTypeHTTP:
		if endpoint, ok := details["endpoint"].(string); ok {
			if !details["success"].(bool) {
				if errMsg, ok := details["error"].(string); ok {
					return fmt.Sprintf("HTTP check failed for %s: %s", endpoint, errMsg)
				}
				return fmt.Sprintf("HTTP check failed for %s", endpoint)
			}
			if responseTime, ok := details["response_time"].(int64); ok {
				if threshold, ok := details["timeout_threshold"].(int64); ok {
					return fmt.Sprintf("HTTP response time for %s (%dms) exceeds threshold (%dms)",
						endpoint, responseTime, threshold)
				}
			}
		}
	}

	return fmt.Sprintf("Alert condition met for rule: %s", rule.Name)
}

// getMetricKey returns the detail key for a metric name
func getMetricKey(metric string) string {
	switch metric {
	case "CPU usage":
		return "cpu_usage"
	case "Memory usage":
		return "memory_usage"
	case "Disk usage":
		return "disk_usage"
	case "Load average":
		return "load_average"
	default:
		return "value"
	}
}

// shouldSendNotification determines if a notification should be sent for an existing alert
func (am *AlertManager) shouldSendNotification(alert *Alert, rule *AlertRule) bool {
	now := time.Now()

	// Check minimum interval
	if alert.LastSent != nil && now.Sub(*alert.LastSent) < rule.MinInterval {
		return false
	}

	// Check maximum notifications
	if rule.MaxNotifications > 0 && alert.NotificationsSent >= rule.MaxNotifications {
		return false
	}

	return true
}

// sendNotifications sends the alert through all configured notifiers
func (am *AlertManager) sendNotifications(alert *Alert, rule *AlertRule) {
	now := time.Now()

	log.Printf("DEBUG: Attempting to send notifications for alert: %s", alert.Name)
	log.Printf("DEBUG: Number of configured notifiers: %d", len(am.notifiers))

	for i, notifier := range am.notifiers {
		log.Printf("DEBUG: Checking notifier %d: %s (enabled: %v)", i, notifier.Name(), notifier.IsEnabled())
		
		if !notifier.IsEnabled() {
			log.Printf("DEBUG: Notifier %s is disabled, skipping", notifier.Name())
			continue
		}

		log.Printf("DEBUG: Sending alert via %s...", notifier.Name())
		err := notifier.Send(alert)
		if err != nil {
			log.Printf("Failed to send alert via %s: %v", notifier.Name(), err)
			continue
		}

		log.Printf("DEBUG: Successfully sent alert via %s", notifier.Name())
		alert.SentTo = append(alert.SentTo, notifier.Name())
	}

	alert.NotificationsSent++
	alert.LastSent = &now
	
	log.Printf("DEBUG: Notification attempt completed. Sent to: %v", alert.SentTo)
}

// addToHistory adds an alert to the history buffer
func (am *AlertManager) addToHistory(alert *Alert) {
	am.alertHistory = append(am.alertHistory, alert)

	// Maintain history size limit
	if len(am.alertHistory) > am.config.MaxAlertHistory {
		am.alertHistory = am.alertHistory[1:]
	}
}

// AcknowledgeAlert acknowledges an active alert
func (am *AlertManager) AcknowledgeAlert(alertID string) error {
	if alert, exists := am.activeAlerts[alertID]; exists {
		alert.Status = StatusAcknowledged
		return nil
	}
	return fmt.Errorf("alert not found: %s", alertID)
}

// GetAlert returns a specific alert by ID
func (am *AlertManager) GetAlert(alertID string) (*Alert, error) {
	if alert, exists := am.activeAlerts[alertID]; exists {
		return alert, nil
	}

	// Check alert history as well
	for _, alert := range am.alertHistory {
		if alert.ID == alertID {
			return alert, nil
		}
	}

	return nil, fmt.Errorf("alert not found: %s", alertID)
}

// ResolveAlert manually resolves an active alert
func (am *AlertManager) ResolveAlert(alertID string) error {
	if alert, exists := am.activeAlerts[alertID]; exists {
		alert.Status = StatusResolved
		now := time.Now()
		alert.EndsAt = &now

		// Move to history
		am.addToHistory(alert)
		delete(am.activeAlerts, alertID)

		return nil
	}
	return fmt.Errorf("alert not found: %s", alertID)
}

// GetRules returns all configured alert rules
func (am *AlertManager) GetRules() map[string]*AlertRule {
	return am.rules
}

// GenerateID generates a unique ID for alerts and rules
func GenerateID() string {
	return uuid.New().String()
}
