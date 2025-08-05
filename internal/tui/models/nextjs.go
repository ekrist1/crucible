package models

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"crucible/internal/nextjs"
)

// NextJS message types
type nextjsSitesLoadedMsg struct {
	sites []*nextjs.Site
	err   error
}

type nextjsSiteCreatedMsg struct {
	name string
	err  error
}

// NextJSModel handles Next.js site management
type NextJSModel struct {
	BaseModel
	manager  *nextjs.NextJSManager
	sites    []*nextjs.Site
	selected int
	message  string
	loading  bool
}

// NextJSFormModel handles Next.js site creation forms
type NextJSFormModel struct {
	BaseModel
	form    *FormModel
	manager *nextjs.NextJSManager
}

// NewNextJSModel creates a new NextJS management model
func NewNextJSModel(shared *SharedData) *NextJSModel {
	return &NextJSModel{
		BaseModel: NewBaseModel(shared),
		manager:   nextjs.NewNextJSManager(),
		sites:     []*nextjs.Site{},
		selected:  0,
		message:   "",
		loading:   false,
	}
}

// NewNextJSFormModel creates a new NextJS form model
func NewNextJSFormModel(shared *SharedData) *NextJSFormModel {
	model := &NextJSFormModel{
		BaseModel: NewBaseModel(shared),
		manager:   nextjs.NewNextJSManager(),
	}
	model.setupForm()
	return model
}

// Init initializes the NextJS model
func (m *NextJSModel) Init() tea.Cmd {
	return m.loadSites()
}

// Update handles NextJS management updates
func (m *NextJSModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			return m, m.GoBack()
		case "c":
			// Create new site
			return m, m.NavigateTo(StateNextJSCreate, nil)
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
				// Show site details or management options
				return m.showSiteDetails()
			}
		}

	case nextjsSitesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.message = fmt.Sprintf("Error loading sites: %v", msg.err)
		} else {
			m.sites = msg.sites
			m.message = fmt.Sprintf("Loaded %d Next.js sites", len(msg.sites))
		}
		return m, nil

	case nextjsSiteCreatedMsg:
		if msg.err != nil {
			m.message = fmt.Sprintf("Error creating site: %v", msg.err)
			return m, nil
		} else {
			// Navigate to processing state to execute the commands
			return m, m.NavigateTo(StateProcessing, map[string]interface{}{
				"title": "Creating Next.js Site: " + msg.name,
			})
		}
		return m, nil
	}

	return m, nil
}

// View renders the NextJS management interface
func (m *NextJSModel) View() string {
	var s strings.Builder

	// Title
	s.WriteString(titleStyle.Render("🚀 Next.js Management"))
	s.WriteString("\n\n")

	// Loading or message
	if m.loading {
		s.WriteString("Loading Next.js sites...\n\n")
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
		s.WriteString(helpStyle.Render("No Next.js sites found."))
		s.WriteString("\n\n")
	} else {
		s.WriteString("Next.js Sites:\n\n")
		for i, site := range m.sites {
			cursor := "  "
			if i == m.selected {
				cursor = "→ "
			}

			// Check if site is actually running
			status := "🔴" // Default to stopped
			if siteStatus, err := m.manager.GetSiteStatus(site.Name); err == nil {
				switch siteStatus.PM2Status {
				case "online":
					status = "🟢" // Running
				case "stopped":
					status = "🔴" // Stopped  
				case "errored":
					status = "🟡" // Error state
				default:
					status = "⚪" // Unknown state
				}
			}

			siteName := site.Name
			if i == m.selected {
				siteName = selectedStyle.Render(siteName)
			} else {
				siteName = choiceStyle.Render(siteName)
			}

			s.WriteString(fmt.Sprintf("%s%s %s (Port: %d, Domain: %s)\n", 
				cursor, status, siteName, site.Port, site.Domain))
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

// loadSites loads NextJS sites asynchronously
func (m *NextJSModel) loadSites() tea.Cmd {
	return func() tea.Msg {
		sites, err := m.manager.ListSites()
		return nextjsSitesLoadedMsg{sites: sites, err: err}
	}
}

// showSiteDetails shows details for the selected site
func (m *NextJSModel) showSiteDetails() (tea.Model, tea.Cmd) {
	if len(m.sites) == 0 || m.selected >= len(m.sites) {
		return m, nil
	}

	site := m.sites[m.selected]
	// Navigate to processing state to show site management options
	return m, m.NavigateTo(StateProcessing, map[string]interface{}{
		"action": "nextjs-site-details",
		"site":   site,
	})
}

// NextJS Form Implementation

// Init initializes the NextJS form model
func (f *NextJSFormModel) Init() tea.Cmd {
	return f.form.Init()
}

// Update handles form updates
func (f *NextJSFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

// View renders the NextJS form
func (f *NextJSFormModel) View() string {
	return f.form.View()
}

// setupForm configures the NextJS creation form
func (f *NextJSFormModel) setupForm() {
	f.form = NewFormModel(f.shared, "🚀 Create Next.js Site")

	// Step 1: Repository and Basic Info
	step1 := FormStep{
		Title:       "Repository & Basic Info",
		Description: "Provide the Git repository and basic site information",
		Fields: []FormField{
			{
				Name:        "name",
				Label:       "Site Name",
				FieldType:   FieldTypeText,
				Required:    true,
				Placeholder: "my-nextjs-app",
				MinLength:   2,
				Validator:   validateSiteName,
			},
			{
				Name:        "repository",
				Label:       "Git Repository URL",
				FieldType:   FieldTypeURL,
				Required:    true,
				Placeholder: "https://github.com/user/repo.git",
				Validator:   validateGitURL,
			},
			{
				Name:        "branch",
				Label:       "Branch",
				FieldType:   FieldTypeText,
				Required:    false,
				Value:       "main",
				Placeholder: "main",
			},
			{
				Name:        "domain",
				Label:       "Domain",
				FieldType:   FieldTypeText,
				Required:    true,
				Placeholder: "myapp.example.com",
				Validator:   validateDomain,
			},
		},
	}

	// Step 2: Build Configuration
	step2 := FormStep{
		Title:       "Build Configuration",
		Description: "Configure how your Next.js application should be built and run",
		Fields: []FormField{
			{
				Name:      "nodeVersion",
				Label:     "Node.js Version",
				FieldType: FieldTypeSelect,
				Required:  true,
				Value:     "18",
				Options:   []string{"16", "18", "20", "21"},
			},
			{
				Name:      "packageManager",
				Label:     "Package Manager",
				FieldType: FieldTypeSelect,
				Required:  true,
				Value:     "npm",
				Options:   []string{"npm", "yarn", "pnpm"},
			},
			{
				Name:        "buildCommand",
				Label:       "Build Command",
				FieldType:   FieldTypeText,
				Required:    true,
				Value:       "npm run build",
				Placeholder: "npm run build",
			},
			{
				Name:        "startCommand",
				Label:       "Start Command",
				FieldType:   FieldTypeText,
				Required:    true,
				Value:       "npm start",
				Placeholder: "npm start",
			},
		},
	}

	// Step 3: PM2 Configuration
	step3 := FormStep{
		Title:       "PM2 Configuration",
		Description: "Configure PM2 process management for your Next.js application",
		Fields: []FormField{
			{
				Name:      "instances",
				Label:     "Number of Instances",
				FieldType: FieldTypeSelect,
				Required:  true,
				Value:     "1",
				Options:   []string{"1", "2", "4", "max"},
			},
			{
				Name:      "environment",
				Label:     "Environment",
				FieldType: FieldTypeSelect,
				Required:  true,
				Value:     "production",
				Options:   []string{"development", "production"},
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
func (f *NextJSFormModel) handleSubmit(values map[string]string) tea.Cmd {
	return func() tea.Msg {
		// Create the NextJS site using the nextjs package
		site := &nextjs.Site{
			Name:           values["name"],
			Repository:     values["repository"],
			Branch:         values["branch"],
			Domain:         values["domain"],
			NodeVersion:    values["nodeVersion"],
			PackageManager: values["packageManager"],
			BuildCommand:   values["buildCommand"],
			StartCommand:   values["startCommand"],
			Environment:    values["environment"],
		}
		
		// Parse PM2 instances
		if instances := values["instances"]; instances == "max" {
			site.PM2Instances = 0 // PM2 will use max instances
		} else {
			// Convert string to int (simplified parsing)
			switch instances {
			case "1":
				site.PM2Instances = 1
			case "2":
				site.PM2Instances = 2
			case "4":
				site.PM2Instances = 4
			default:
				site.PM2Instances = 1
			}
		}
		
		// Create the site using NextJSManager
		if err := f.manager.CreateSite(site); err != nil {
			return nextjsSiteCreatedMsg{name: site.Name, err: err}
		}
		
		return nextjsSiteCreatedMsg{name: site.Name, err: nil}
	}
}

// handleCancel handles form cancellation
func (f *NextJSFormModel) handleCancel() tea.Cmd {
	return f.GoBack()
}

// Validation functions are now in validation.go