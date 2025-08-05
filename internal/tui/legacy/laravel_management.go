package tui

import (
	"fmt"
	"os"

	"crucible/internal/actions"
	tea "github.com/charmbracelet/bubbletea"
)

// Laravel management functions

// handleLaravelSiteForm handles Laravel site form processing
func (m Model) handleLaravelSiteForm() (tea.Model, tea.Cmd) {
	newModel, cmd := m.HandleLaravelSiteForm()
	return newModel, cmd
}

// handleUpdateSiteForm handles Laravel site update form processing
func (m Model) handleUpdateSiteForm() (tea.Model, tea.Cmd) {
	newModel, cmd := m.HandleUpdateSiteForm()
	return newModel, cmd
}

// handleQueueWorkerForm handles Laravel queue worker form processing
func (m Model) handleQueueWorkerForm() (tea.Model, tea.Cmd) {
	newModel, cmd := m.HandleQueueWorkerForm()
	return newModel, cmd
}

// handleBackupForm handles MySQL backup form processing
func (m Model) handleBackupForm() (tea.Model, tea.Cmd) {
	newModel, cmd := m.HandleBackupForm()
	return newModel, cmd
}

// showLaravelSiteList displays available Laravel sites for updating
func (m Model) showLaravelSiteList() (tea.Model, tea.Cmd) {
	// Use the actions package to list Laravel sites
	sites, err := actions.ListLaravelSites()
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{
			WarnStyle.Render(fmt.Sprintf("❌ Error scanning for Laravel sites: %v", err)),
			"",
			InfoStyle.Render("Make sure /var/www exists and is accessible"),
		}
		return m, tea.ClearScreen
	}

	if len(sites) == 0 {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{
			WarnStyle.Render("❌ No Laravel sites found in /var/www"),
			"",
			InfoStyle.Render("Create a Laravel site first using 'Create a new Laravel Site'"),
		}
		return m, tea.ClearScreen
	}

	// Build the report showing available sites
	m.State = StateProcessing
	m.ProcessingMsg = ""
	m.Report = []string{
		TitleStyle.Render("📂 Available Laravel Sites"),
		"",
		InfoStyle.Render("Found the following Laravel sites in /var/www:"),
		"",
	}

	for i, site := range sites {
		sitePath := fmt.Sprintf("/var/www/%s", site)
		// Check if it's a git repository
		gitStatus := "📁 Regular site"
		if _, err := os.Stat(fmt.Sprintf("%s/.git", sitePath)); err == nil {
			gitStatus = "📦 Git repository"
		}

		m.Report = append(m.Report,
			InfoStyle.Render(fmt.Sprintf("%d. %s", i+1, site)),
			ChoiceStyle.Render(fmt.Sprintf("   Path: %s", sitePath)),
			ChoiceStyle.Render(fmt.Sprintf("   Type: %s", gitStatus)),
			"",
		)
	}

	// Store sites for later use and ask for selection
	m.FormData["availableSites"] = fmt.Sprintf("%v", sites) // Convert to string for storage

	m.Report = append(m.Report,
		InfoStyle.Render("Select a site to update:"),
	)

	newModel, cmd := m.startInput("Enter site number (1-"+fmt.Sprintf("%d", len(sites))+"):", "siteIndex", 101)
	return newModel, cmd
}

// showLaravelSiteListForQueue displays available Laravel sites for queue worker setup
func (m Model) showLaravelSiteListForQueue() (tea.Model, tea.Cmd) {
	// Use the actions package to list Laravel sites
	sites, err := actions.ListLaravelSites()
	if err != nil {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{
			WarnStyle.Render(fmt.Sprintf("❌ Error scanning for Laravel sites: %v", err)),
			"",
			InfoStyle.Render("Make sure /var/www exists and is accessible"),
		}
		return m, tea.ClearScreen
	}

	if len(sites) == 0 {
		m.State = StateProcessing
		m.ProcessingMsg = ""
		m.Report = []string{
			WarnStyle.Render("❌ No Laravel sites found in /var/www"),
			"",
			InfoStyle.Render("Create a Laravel site first using 'Create a new Laravel Site'"),
		}
		return m, tea.ClearScreen
	}

	// Build the report showing available sites
	m.State = StateProcessing
	m.ProcessingMsg = ""
	m.Report = []string{
		TitleStyle.Render("🚀 Setup Queue Worker"),
		"",
		InfoStyle.Render("Select a Laravel site to setup queue worker for:"),
		"",
	}

	for i, site := range sites {
		sitePath := fmt.Sprintf("/var/www/%s", site)
		m.Report = append(m.Report,
			InfoStyle.Render(fmt.Sprintf("%d. %s", i+1, site)),
			ChoiceStyle.Render(fmt.Sprintf("   Path: %s", sitePath)),
			"",
		)
	}

	m.Report = append(m.Report,
		InfoStyle.Render("Select a site for queue worker setup:"),
	)

	newModel, cmd := m.startInput("Enter site number (1-"+fmt.Sprintf("%d", len(sites))+"):", "queueSiteIndex", 102)
	return newModel, cmd
}

// setupCaddyLaravelConfig sets up Caddy configuration for Laravel
func (m *Model) setupCaddyLaravelConfig() {
	// This will be properly connected when we finish the refactor
}
