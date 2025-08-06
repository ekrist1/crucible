package notifiers

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/resend/resend-go/v2"
)

// EmailNotifier implements email notifications using Resend API
type EmailNotifier struct {
	config *EmailConfig
	client *resend.Client
}

// NewEmailNotifier creates a new email notifier using Resend
func NewEmailNotifier(config *EmailConfig) *EmailNotifier {
	var client *resend.Client
	if config.ResendAPIKey != "" {
		client = resend.NewClient(config.ResendAPIKey)
	}

	return &EmailNotifier{
		config: config,
		client: client,
	}
}

// Name returns the notifier name
func (e *EmailNotifier) Name() string {
	return "email"
}

// IsEnabled returns whether email notifications are enabled
func (e *EmailNotifier) IsEnabled() bool {
	enabled := e.config.Enabled && e.config.ResendAPIKey != "" && e.client != nil
	// Debug logging
	fmt.Printf("DEBUG: EmailNotifier.IsEnabled() check:\n")
	fmt.Printf("  config.Enabled: %v\n", e.config.Enabled)
	fmt.Printf("  config.ResendAPIKey present: %v\n", e.config.ResendAPIKey != "")
	fmt.Printf("  config.ResendAPIKey length: %d\n", len(e.config.ResendAPIKey))
	fmt.Printf("  client initialized: %v\n", e.client != nil)
	fmt.Printf("  Final enabled result: %v\n", enabled)
	return enabled
}

// Send sends an alert notification via email using Resend
func (e *EmailNotifier) Send(alert *Alert) error {
	if !e.IsEnabled() {
		// Log detailed information about why email is disabled
		enabled := e.config.Enabled
		hasAPIKey := e.config.ResendAPIKey != ""
		return fmt.Errorf("email notifier is not enabled or configured (enabled: %v, has_api_key: %v)", enabled, hasAPIKey)
	}

	// Determine recipients
	recipients := e.getRecipients(alert)
	if len(recipients) == 0 {
		return fmt.Errorf("no email recipients configured")
	}

	// Generate email content
	subject, err := e.generateSubject(alert)
	if err != nil {
		return fmt.Errorf("failed to generate subject: %v", err)
	}

	htmlBody, textBody, err := e.generateBody(alert)
	if err != nil {
		return fmt.Errorf("failed to generate email body: %v", err)
	}

	// Prepare email request using Resend SDK
	params := &resend.SendEmailRequest{
		From:    e.getFromAddress(),
		To:      recipients,
		Subject: subject,
		Html:    htmlBody,
		Text:    textBody,
		Headers: map[string]string{
			"X-Alert-ID":       alert.ID,
			"X-Alert-Severity": string(alert.Severity),
			"X-Alert-Type":     string(alert.Type),
		},
		Tags: []resend.Tag{
			{
				Name:  "alert_type",
				Value: string(alert.Type),
			},
			{
				Name:  "alert_severity",
				Value: string(alert.Severity),
			},
		},
	}

	// Send email via Resend API
	return e.sendViaResendSDK(params)
}

// getRecipients determines who should receive the alert email
func (e *EmailNotifier) getRecipients(alert *Alert) []string {
	// Use default recipients from config
	if len(e.config.DefaultTo) > 0 {
		return e.config.DefaultTo
	}

	// If no default recipients, check if alert has specific recipients
	// This would come from the alert rule configuration
	return []string{}
}

// getFromAddress returns the formatted from address
func (e *EmailNotifier) getFromAddress() string {
	if e.config.FromName != "" {
		return fmt.Sprintf("%s <%s>", e.config.FromName, e.config.FromEmail)
	}
	return e.config.FromEmail
}

// generateSubject creates the email subject using template or default format
func (e *EmailNotifier) generateSubject(alert *Alert) (string, error) {
	if e.config.SubjectTemplate != "" {
		return e.executeTemplate(e.config.SubjectTemplate, alert)
	}

	// Default subject format
	severityIcon := e.getSeverityIcon(alert.Severity)
	return fmt.Sprintf("%s [%s] %s", severityIcon, strings.ToUpper(string(alert.Severity)), alert.Name), nil
}

// generateBody creates both HTML and text versions of the email body
func (e *EmailNotifier) generateBody(alert *Alert) (string, string, error) {
	if e.config.BodyTemplate != "" {
		body, err := e.executeTemplate(e.config.BodyTemplate, alert)
		return body, body, err // Use same content for both HTML and text
	}

	// Generate default email body
	htmlBody := e.generateDefaultHTMLBody(alert)
	textBody := e.generateDefaultTextBody(alert)

	return htmlBody, textBody, nil
}

// generateDefaultHTMLBody creates a default HTML email body
func (e *EmailNotifier) generateDefaultHTMLBody(alert *Alert) string {
	severityColor := e.getSeverityColor(alert.Severity)
	severityIcon := e.getSeverityIcon(alert.Severity)

	var html strings.Builder
	html.WriteString(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Crucible Alert</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 20px; background-color: #f5f5f5; }
        .container { max-width: 600px; margin: 0 auto; background-color: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .header { border-bottom: 2px solid ` + severityColor + `; padding-bottom: 10px; margin-bottom: 20px; }
        .severity { color: ` + severityColor + `; font-weight: bold; font-size: 18px; }
        .details { background-color: #f8f9fa; padding: 15px; border-radius: 4px; margin: 15px 0; }
        .details-table { width: 100%; border-collapse: collapse; }
        .details-table td { padding: 8px; border-bottom: 1px solid #dee2e6; }
        .details-table td:first-child { font-weight: bold; width: 30%; }
        .footer { margin-top: 20px; padding-top: 15px; border-top: 1px solid #dee2e6; font-size: 12px; color: #6c757d; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>` + severityIcon + ` Crucible Alert</h1>
            <p class="severity">` + strings.ToUpper(string(alert.Severity)) + ` - ` + alert.Name + `</p>
        </div>`)

	html.WriteString(`
        <div class="content">
            <p><strong>Message:</strong> ` + alert.Message + `</p>
            
            <div class="details">
                <h3>Alert Details</h3>
                <table class="details-table">
                    <tr><td>Alert ID</td><td>` + alert.ID + `</td></tr>
                    <tr><td>Type</td><td>` + string(alert.Type) + `</td></tr>
                    <tr><td>Severity</td><td>` + string(alert.Severity) + `</td></tr>
                    <tr><td>Started At</td><td>` + alert.StartsAt.Format("2006-01-02 15:04:05 MST") + `</td></tr>`)

	// Add custom details
	if len(alert.Details) > 0 {
		for key, value := range alert.Details {
			html.WriteString(fmt.Sprintf(`<tr><td>%s</td><td>%v</td></tr>`, key, value))
		}
	}

	html.WriteString(`
                </table>
            </div>
        </div>
        
        <div class="footer">
            <p>This alert was generated by Crucible Server Management System.</p>
        </div>
    </div>
</body>
</html>`)

	return html.String()
}

// generateDefaultTextBody creates a default plain text email body
func (e *EmailNotifier) generateDefaultTextBody(alert *Alert) string {
	var text strings.Builder

	severityIcon := e.getSeverityIcon(alert.Severity)
	text.WriteString(fmt.Sprintf("%s CRUCIBLE ALERT - %s\n", severityIcon, strings.ToUpper(string(alert.Severity))))
	text.WriteString(strings.Repeat("=", 50) + "\n\n")

	text.WriteString(fmt.Sprintf("Alert: %s\n", alert.Name))
	text.WriteString(fmt.Sprintf("Message: %s\n\n", alert.Message))

	text.WriteString("DETAILS:\n")
	text.WriteString(fmt.Sprintf("- Alert ID: %s\n", alert.ID))
	text.WriteString(fmt.Sprintf("- Type: %s\n", string(alert.Type)))
	text.WriteString(fmt.Sprintf("- Severity: %s\n", string(alert.Severity)))
	text.WriteString(fmt.Sprintf("- Started At: %s\n", alert.StartsAt.Format("2006-01-02 15:04:05 MST")))

	// Add custom details
	if len(alert.Details) > 0 {
		text.WriteString("\nADDITIONAL DETAILS:\n")
		for key, value := range alert.Details {
			text.WriteString(fmt.Sprintf("- %s: %v\n", key, value))
		}
	}

	text.WriteString("\n" + strings.Repeat("-", 50) + "\n")
	text.WriteString("Generated by Crucible Server Management System\n")

	return text.String()
}

// getSeverityIcon returns an appropriate icon for the severity level
func (e *EmailNotifier) getSeverityIcon(severity AlertSeverity) string {
	switch severity {
	case SeverityInfo:
		return "ℹ️"
	case SeverityWarning:
		return "⚠️"
	case SeverityCritical:
		return "🚨"
	default:
		return "📋"
	}
}

// getSeverityColor returns an appropriate color for the severity level
func (e *EmailNotifier) getSeverityColor(severity AlertSeverity) string {
	switch severity {
	case SeverityInfo:
		return "#17a2b8"
	case SeverityWarning:
		return "#ffc107"
	case SeverityCritical:
		return "#dc3545"
	default:
		return "#6c757d"
	}
}

// executeTemplate executes a template with alert data
func (e *EmailNotifier) executeTemplate(tmplStr string, alert *Alert) (string, error) {
	tmpl, err := template.New("alert").Parse(tmplStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, alert)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// sendViaResendSDK sends the email using Resend Go SDK
func (e *EmailNotifier) sendViaResendSDK(params *resend.SendEmailRequest) error {
	// Log email attempt for debugging
	fmt.Printf("DEBUG: ===== ATTEMPTING EMAIL SEND =====\n")
	fmt.Printf("DEBUG: From: %s\n", params.From)
	fmt.Printf("DEBUG: To: %v\n", params.To)
	fmt.Printf("DEBUG: Subject: %s\n", params.Subject)
	fmt.Printf("DEBUG: HTML length: %d chars\n", len(params.Html))
	fmt.Printf("DEBUG: Text length: %d chars\n", len(params.Text))
	fmt.Printf("DEBUG: Headers: %v\n", params.Headers)
	fmt.Printf("DEBUG: Tags: %v\n", params.Tags)
	fmt.Printf("DEBUG: Client initialized: %v\n", e.client != nil)
	fmt.Printf("DEBUG: ==================================\n")

	if e.client == nil {
		return fmt.Errorf("resend client not initialized")
	}

	// Validate email parameters
	if params.From == "" {
		return fmt.Errorf("from email is required")
	}
	if len(params.To) == 0 {
		return fmt.Errorf("recipient email is required")
	}
	if params.Subject == "" {
		return fmt.Errorf("email subject is required")
	}

	fmt.Printf("DEBUG: Validation passed, sending email...\n")

	// Send email using Resend SDK
	fmt.Printf("DEBUG: About to call client.Emails.Send()...\n")
	sent, err := e.client.Emails.Send(params)
	fmt.Printf("DEBUG: client.Emails.Send() returned, err = %v\n", err)

	if err != nil {
		fmt.Printf("DEBUG: ===== RESEND SDK ERROR DETAILS =====\n")
		fmt.Printf("DEBUG: Error occurred: %v\n", err)

		// Handle potential nil error safely
		if err != nil {
			fmt.Printf("DEBUG: Error type: %T\n", err)
			fmt.Printf("DEBUG: Error string: '%s'\n", err.Error())

			// Log additional context that might help debug
			errorStr := err.Error()
			fmt.Printf("DEBUG: Error message analysis:\n")

			if strings.Contains(errorStr, "domain") {
				fmt.Printf("DEBUG: → Domain-related error detected\n")
				if len(strings.Split(params.From, "@")) > 1 {
					domain := strings.Split(params.From, "@")[1]
					fmt.Printf("DEBUG: → Domain '%s' may not be verified in Resend\n", domain)
				}
			}
			if strings.Contains(errorStr, "api_key") || strings.Contains(errorStr, "unauthorized") {
				fmt.Printf("DEBUG: → API key issue detected\n")
			}
			if strings.Contains(errorStr, "validation") {
				fmt.Printf("DEBUG: → Validation error detected\n")
			}
		}
		fmt.Printf("DEBUG: =====================================\n")

		return fmt.Errorf("failed to send email via Resend SDK: %v", err)
	}

	fmt.Printf("DEBUG: Email sent successfully! Resend ID: %s\n", sent.Id)
	return nil
}
