package collectors

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	"crucible/internal/monitor"
)

// HTTPCollector performs HTTP health checks
type HTTPCollector struct {
	client *http.Client
}

// NewHTTPCollector creates a new HTTP health check collector
func NewHTTPCollector() *HTTPCollector {
	// Create HTTP client with reasonable timeouts
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false, // Enable SSL verification by default
			},
		},
	}

	return &HTTPCollector{
		client: client,
	}
}

// PerformCheck performs a single HTTP health check
func (h *HTTPCollector) PerformCheck(check monitor.HTTPCheck) monitor.HTTPCheckResult {
	startTime := time.Now()
	result := monitor.HTTPCheckResult{
		Name:      check.Name,
		URL:       check.URL,
		Timestamp: startTime,
		Success:   false,
	}

	// Create request
	req, err := http.NewRequest("GET", check.URL, nil)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create request: %v", err)
		result.ResponseTime = time.Since(startTime)
		return result
	}

	// Set User-Agent
	req.Header.Set("User-Agent", "Crucible-Monitor/1.0.0")

	// Set custom timeout for this check
	timeout := check.GetTimeout()
	if timeout > 0 {
		h.client.Timeout = timeout
	}

	// Perform the request
	resp, err := h.client.Do(req)
	responseTime := time.Since(startTime)
	result.ResponseTime = responseTime

	if err != nil {
		result.Error = fmt.Sprintf("Request failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	// Read response body to get content length
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to read response body: %v", err)
		return result
	}

	result.StatusCode = resp.StatusCode
	result.ContentLength = int64(len(body))

	// Check if status code matches expected
	expectedStatus := check.ExpectedStatus
	if expectedStatus == 0 {
		expectedStatus = 200 // Default expected status
	}

	if resp.StatusCode == expectedStatus {
		result.Success = true
	} else {
		result.Error = fmt.Sprintf("Unexpected status code: got %d, expected %d", resp.StatusCode, expectedStatus)
	}

	// Check SSL certificate expiry if HTTPS
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		cert := resp.TLS.PeerCertificates[0]
		result.SSLExpiry = &cert.NotAfter
	}

	return result
}

// ValidateCheck validates an HTTP check configuration
func (h *HTTPCollector) ValidateCheck(check monitor.HTTPCheck) error {
	if check.Name == "" {
		return fmt.Errorf("check name is required")
	}
	if check.URL == "" {
		return fmt.Errorf("check URL is required")
	}

	// Validate timeout
	if check.Timeout != "" {
		if _, err := time.ParseDuration(check.Timeout); err != nil {
			return fmt.Errorf("invalid timeout format: %w", err)
		}
	}

	// Validate interval
	if check.Interval != "" {
		if _, err := time.ParseDuration(check.Interval); err != nil {
			return fmt.Errorf("invalid interval format: %w", err)
		}
	}

	// Validate expected status code
	if check.ExpectedStatus < 0 || check.ExpectedStatus > 599 {
		return fmt.Errorf("invalid expected status code: %d", check.ExpectedStatus)
	}

	return nil
}

// GetSSLCertificateInfo returns SSL certificate information for HTTPS URLs
func (h *HTTPCollector) GetSSLCertificateInfo(url string) (map[string]interface{}, error) {
	resp, err := h.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.TLS == nil {
		return nil, fmt.Errorf("no TLS connection information available")
	}

	if len(resp.TLS.PeerCertificates) == 0 {
		return nil, fmt.Errorf("no peer certificates found")
	}

	cert := resp.TLS.PeerCertificates[0]
	info := map[string]interface{}{
		"subject":           cert.Subject.String(),
		"issuer":            cert.Issuer.String(),
		"not_before":        cert.NotBefore,
		"not_after":         cert.NotAfter,
		"dns_names":         cert.DNSNames,
		"is_expired":        time.Now().After(cert.NotAfter),
		"days_until_expiry": int(time.Until(cert.NotAfter).Hours() / 24),
	}

	return info, nil
}
