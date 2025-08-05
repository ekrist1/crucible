package models

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"crucible/internal/actions"
)

// Laravel message types
type laravelSitesLoadedMsg struct {
	sites []LaravelSite
	err   error
}

type laravelSiteCreatedMsg struct {
	name string
	err  error
}

// LaravelSite represents a Laravel site
type LaravelSite struct {
	Name       string
	Path       string
	Domain     string
	Repository string
	Branch     string
	IsActive   bool
}

// LaravelModel handles Laravel site management
type LaravelModel struct {
	BaseModel
	sites    []LaravelSite
	selected int
	message  string
	loading  bool
}

// LaravelFormModel handles Laravel site creation forms
type LaravelFormModel struct {
	BaseModel
	form *FormModel
}

// NewLaravelModel creates a new Laravel management model
func NewLaravelModel(shared *SharedData) *LaravelModel {
	return &LaravelModel{
		BaseModel: NewBaseModel(shared),
		sites:     []LaravelSite{},
		selected:  0,
		message:   "",
		loading:   false,
	}
}

// NewLaravelFormModel creates a new Laravel form model
func NewLaravelFormModel(shared *SharedData) *LaravelFormModel {
	model := &LaravelFormModel{
		BaseModel: NewBaseModel(shared),
	}
	model.setupForm()
	return model
}

// Init initializes the Laravel model
func (m *LaravelModel) Init() tea.Cmd {
	return m.loadSites()
}

// Update handles Laravel management updates
func (m *LaravelModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			return m, m.GoBack()
		case "c":
			// Create new site
			return m, m.NavigateTo(StateLaravelCreate, nil)
		case "r":
			// Refresh sites
			m.loading = true
			m.message = "Loading..."
			return m, m.loadSites()
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected < len(m.sites)-1 {
				m.selected++
			}
		case "enter", " ":
			if len(m.sites) > 0 && m.selected < len(m.sites) {
				return m.showSiteDetails()
			}
		}

	case laravelSitesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.message = fmt.Sprintf("Error loading sites: %v", msg.err)
		} else {
			m.sites = msg.sites
			m.message = fmt.Sprintf("Loaded %d Laravel sites", len(msg.sites))
		}
		return m, nil

	case laravelSiteCreatedMsg:
		if msg.err != nil {
			m.message = fmt.Sprintf("Error creating site: %v", msg.err)
			return m, nil
		} else {
			// Navigate to processing state to execute the commands
			return m, m.NavigateTo(StateProcessing, map[string]interface{}{
				"title": "Creating Laravel Site: " + msg.name,
			})
		}
		return m, nil
	}

	return m, nil
}

// View renders the Laravel management interface
func (m *LaravelModel) View() string {
	var s strings.Builder

	// Title
	s.WriteString(titleStyle.Render("🚀 Laravel Management"))
	s.WriteString("\n\n")

	// Loading or message
	if m.loading {
		s.WriteString("Loading Laravel sites...\n\n")
	} else if m.message != "" {
		if strings.Contains(m.message, "Error") {
			s.WriteString(errorStyle.Render(m.message))
		} else {
			s.WriteString(infoStyle.Render(m.message))
		}
		s.WriteString("\n\n")
	}

	// Sites list
	if len(m.sites) == 0 {
		s.WriteString(helpStyle.Render("No Laravel sites found."))
		s.WriteString("\n\n")
	} else {
		s.WriteString("Laravel Sites:\n\n")
		for i, site := range m.sites {
			cursor := "  "
			if i == m.selected {
				cursor = "→ "
			}

			// Check if Laravel site is actually running
			status := "🔴" // Default to inactive
			if isRunning, err := actions.GetLaravelSiteStatus(site.Name); err == nil && isRunning {
				status = "🟢" // Active and running
			} else if err != nil {
				status = "🟡" // Error checking status
			}

			siteName := site.Name
			if i == m.selected {
				siteName = selectedStyle.Render(siteName)
			} else {
				siteName = choiceStyle.Render(siteName)
			}

			s.WriteString(fmt.Sprintf("%s%s %s (Domain: %s, Path: %s)\n", 
				cursor, status, siteName, site.Domain, site.Path))
		}
		s.WriteString("\n")
	}

	// Help text
	s.WriteString(helpStyle.Render("Commands:"))
	s.WriteString("\n")
	s.WriteString(helpStyle.Render("  c - Create new site"))
	s.WriteString("\n")
	s.WriteString(helpStyle.Render("  r - Refresh sites"))
	s.WriteString("\n")
	s.WriteString(helpStyle.Render("  ↑/↓ - Navigate"))
	s.WriteString("\n")
	s.WriteString(helpStyle.Render("  Enter - Manage site"))
	s.WriteString("\n")
	s.WriteString(helpStyle.Render("  Esc - Back to main menu"))

	return s.String()
}

// loadSites loads Laravel sites asynchronously
func (m *LaravelModel) loadSites() tea.Cmd {
	return func() tea.Msg {
		// Use the actions package to discover Laravel sites
		siteNames, err := actions.ListLaravelSites()
		if err != nil {
			return laravelSitesLoadedMsg{sites: nil, err: err}
		}
		
		// Convert to LaravelSite structs
		sites := make([]LaravelSite, len(siteNames))
		for i, name := range siteNames {
			sites[i] = LaravelSite{
				Name:       name,
				Path:       "/var/www/" + name,
				Domain:     name + ".local", // Default domain
				Repository: "",              // Could be detected via Git
				Branch:     "main",          // Default branch
				IsActive:   true,            // Assume active for now
			}
		}
		
		return laravelSitesLoadedMsg{sites: sites, err: nil}
	}
}

// showSiteDetails shows details for the selected site
func (m *LaravelModel) showSiteDetails() (tea.Model, tea.Cmd) {
	if len(m.sites) == 0 || m.selected >= len(m.sites) {
		return m, nil
	}

	site := m.sites[m.selected]
	return m, m.NavigateTo(StateProcessing, map[string]interface{}{
		"action": "laravel-site-details",
		"site":   site,
	})
}

// Laravel Form Implementation

// Init initializes the Laravel form model
func (f *LaravelFormModel) Init() tea.Cmd {
	return f.form.Init()
}

// Update handles form updates
func (f *LaravelFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return f, tea.Quit
		case "esc":
			return f, f.GoBack()
		}
	}

	// Delegate to form
	newForm, cmd := f.form.Update(msg)
	if formModel, ok := newForm.(*FormModel); ok {
		f.form = formModel
	}
	return f, cmd
}

// View renders the Laravel form
func (f *LaravelFormModel) View() string {
	return f.form.View()
}

// setupForm configures the Laravel creation form
func (f *LaravelFormModel) setupForm() {
	f.form = NewFormModel(f.shared, "🚀 Create Laravel Site")

	// Step 1: Basic Information
	step1 := FormStep{
		Title:       "Basic Information",
		Description: "Provide basic site information",
		Fields: []FormField{
			{
				Name:        "siteName",
				Label:       "Site Name",
				FieldType:   FieldTypeText,
				Required:    true,
				Placeholder: "my-laravel-app",
				MinLength:   2,
				Validator:   validateLaravelSiteName,
			},
			{
				Name:        "domain",
				Label:       "Domain",
				FieldType:   FieldTypeText,
				Required:    true,
				Placeholder: "myapp.local",
				Validator:   validateDomain,
			},
		},
	}

	// Step 2: Installation Method  
	step2 := FormStep{
		Title:       "Installation Method",
		Description: "Choose how to create your Laravel application",
		Fields: []FormField{
			{
				Name:      "installMethod",
				Label:     "Installation Method",
				FieldType: FieldTypeSelect,
				Required:  true,
				Value:     "Fresh Laravel",
				Options:   []string{"Fresh Laravel", "Clone from Git"},
			},
		},
	}

	// Step 3: Git Repository (conditional)
	step3 := FormStep{
		Title:       "Git Repository",
		Description: "Configure the Git repository for your Laravel application",
		Fields: []FormField{
			{
				Name:        "gitRepo",
				Label:       "Git Repository URL",
				FieldType:   FieldTypeURL,
				Required:    false,
				Placeholder: "https://github.com/user/repo.git (leave empty for fresh Laravel)",
				Validator:   f.validateConditionalGitURL,
			},
			{
				Name:        "branch",
				Label:       "Branch",
				FieldType:   FieldTypeText,
				Required:    false,
				Value:       "main",
				Placeholder: "main",
			},
		},
	}

	// Add steps to form
	f.form.AddStep(step1)
	f.form.AddStep(step2)
	f.form.AddStep(step3)

	// Set handlers
	f.form.SetSubmitHandler(f.handleSubmit)
	f.form.SetCancelHandler(f.handleCancel)
}

// handleSubmit handles form submission
func (f *LaravelFormModel) handleSubmit(values map[string]string) tea.Cmd {
	return func() tea.Msg {

		// Create the Laravel site using the actions package
		config := actions.LaravelSiteConfig{
			SiteName: values["siteName"],
			Domain:   values["domain"],
		}
		
		// Check installation method
		installMethod := values["installMethod"]
		if installMethod == "Clone from Git" {
			// Only use Git repo if user chose to clone from Git
			config.GitRepo = values["gitRepo"]
			config.Branch = values["branch"]
			if config.Branch == "" {
				config.Branch = "main"
			}
		} else {
			// Fresh Laravel installation - no Git repository
			config.GitRepo = ""
			config.Branch = ""
		}
		
		// Get the commands for Laravel site creation
		commands, descriptions := actions.CreateLaravelSite(config)
		
		// Queue the commands for execution
		if f.shared.CommandQueue != nil {
			f.shared.CommandQueue.Reset()
			for i, cmd := range commands {
				desc := ""
				if i < len(descriptions) {
					desc = descriptions[i]
				}
				f.shared.CommandQueue.AddCommand(cmd, desc)
			}
			f.shared.CommandQueue.ServiceName = "Laravel Site: " + config.SiteName
		}
		
		return laravelSiteCreatedMsg{name: config.SiteName, err: nil}
	}
}

// handleCancel handles form cancellation
func (f *LaravelFormModel) handleCancel() tea.Cmd {
	return f.GoBack()
}

// validateConditionalGitURL validates Git URL conditionally based on installation method
func (f *LaravelFormModel) validateConditionalGitURL(url string) error {
	installMethod := f.form.GetValue("installMethod")
	
	// If "Clone from Git" is selected, Git repository is required
	if installMethod == "Clone from Git" {
		if strings.TrimSpace(url) == "" {
			return errors.New("Git repository URL is required when cloning from Git")
		}
		return validateGitURL(url)
	}
	
	// If "Fresh Laravel" is selected, Git repository is optional
	if strings.TrimSpace(url) != "" {
		// If provided, it must be valid
		return validateGitURL(url)
	}
	
	return nil
}

// Validation functions for Laravel
func validateLaravelSiteName(name string) error {
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

