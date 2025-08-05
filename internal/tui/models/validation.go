package models

import (
	"errors"
	"net/url"
	"strings"
	"unicode"
)

// Shared validation functions for forms

func validateGitURL(gitURL string) error {
	if gitURL == "" {
		return errors.New("git URL is required")
	}

	// Parse URL
	parsedURL, err := url.Parse(gitURL)
	if err != nil {
		return errors.New("invalid URL format")
	}

	// Check scheme
	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" && parsedURL.Scheme != "git" {
		return errors.New("URL must use https, http, or git scheme")
	}

	// Check if it looks like a git repository
	if !strings.Contains(gitURL, "git") && !strings.HasSuffix(gitURL, ".git") {
		return errors.New("URL does not appear to be a Git repository")
	}

	return nil
}

func validateDomain(domain string) error {
	if domain == "" {
		return errors.New("domain is required")
	}

	// Basic domain validation
	if len(domain) > 253 {
		return errors.New("domain name too long")
	}

	// Check for valid characters
	if strings.Contains(domain, " ") {
		return errors.New("domain cannot contain spaces")
	}

	// Must contain at least one dot
	if !strings.Contains(domain, ".") {
		return errors.New("domain must contain at least one dot")
	}

	return nil
}

func validateSiteName(name string) error {
	if len(name) < 2 {
		return errors.New("site name must be at least 2 characters")
	}
	if len(name) > 50 {
		return errors.New("site name must be less than 50 characters")
	}
	// Check for valid characters (alphanumeric, hyphens, underscores)
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return errors.New("site name can only contain letters, numbers, hyphens, and underscores")
		}
	}
	return nil
}