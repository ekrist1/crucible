package models

import (
	"crucible/internal/nextjs"
	tea "github.com/charmbracelet/bubbletea"
)

// NextJSHybridFormModel handles NextJS site creation using hybrid form (text + selection)
type NextJSHybridFormModel struct {
	BaseModel
	form *HybridFormModel
}

// NewNextJSHybridFormModel creates a new NextJS form using hybrid components
func NewNextJSHybridFormModel(shared *SharedData) *NextJSHybridFormModel {
	model := &NextJSHybridFormModel{
		BaseModel: NewBaseModel(shared),
	}
	model.setupForm()
	return model
}

// setupForm configures the NextJS creation form with hybrid fields
func (f *NextJSHybridFormModel) setupForm() {
	f.form = NewHybridFormModel(
		f.shared,
		"⚡ Create NextJS Site",
		"Fill out all fields and press Submit to create your NextJS site",
	)

	// Site Name field
	f.form.AddField(HybridFormField{
		Label:       "Site Name",
		FieldType:   HybridFieldTypeText,
		Placeholder: "my-nextjs-app",
		Required:    true,
		Validator:   validateNextJSSiteName,
		MaxLength:   50,
	})

	// Domain field
	f.form.AddField(HybridFormField{
		Label:       "Domain",
		FieldType:   HybridFieldTypeText,
		Placeholder: "myapp.local",
		Required:    true,
		Validator:   validateDomain,
		MaxLength:   100,
	})

	// Installation Method selection field
	f.form.AddField(HybridFormField{
		Label:     "Installation Method",
		FieldType: HybridFieldTypeSelection,
		Required:  true,
		Options: []SelectionOption{
			{
				Value:       "fresh",
				Description: "Create a fresh NextJS installation",
			},
			{
				Value:       "git",
				Description: "Clone from Git repository",
			},
		},
		SelectedIndex: 0, // Default to "fresh"
	})

	// Git Repository field
	f.form.AddField(HybridFormField{
		Label:       "Git Repository URL",
		FieldType:   HybridFieldTypeText,
		Placeholder: "https://github.com/user/repo.git (leave empty for fresh install)",
		Required:    false,
		Validator:   f.validateConditionalGitURL,
		MaxLength:   200,
	})

	// Branch field
	f.form.AddField(HybridFormField{
		Label:       "Branch",
		FieldType:   HybridFieldTypeText,
		Placeholder: "main",
		Required:    false,
		Validator:   validateBranch,
		MaxLength:   50,
	})

	// Set default value for branch
	f.form.SetValue(4, "main")

	// Set submit label
	f.form.SetSubmitLabel("Create NextJS Site")

	// Set handlers
	f.form.SetSubmitHandler(f.handleSubmit)
	f.form.SetCancelHandler(f.handleCancel)
}

// Init initializes the form
func (f *NextJSHybridFormModel) Init() tea.Cmd {
	return f.form.Init()
}

// Update handles form updates
func (f *NextJSHybridFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	newModel, cmd := f.form.Update(msg)
	if hybridModel, ok := newModel.(*HybridFormModel); ok {
		f.form = hybridModel
	}
	return f, cmd
}

// View renders the form
func (f *NextJSHybridFormModel) View() string {
	return f.form.View()
}

// handleSubmit handles form submission
func (f *NextJSHybridFormModel) handleSubmit(values []string) tea.Cmd {
	return func() tea.Msg {
		// Extract values by position
		siteName := values[0]
		domain := values[1]
		installMethod := values[2]
		gitRepo := values[3]
		branch := values[4]

		// Set defaults
		if branch == "" {
			branch = "main"
		}

		// Create the NextJS site using the nextjs manager
		site := &nextjs.Site{
			Name:   siteName,
			Domain: domain,
			Branch: branch,
		}

		// Check installation method
		if installMethod == "git" && gitRepo != "" {
			site.Repository = gitRepo
		}

		// Create NextJS site using the manager
		manager := nextjs.NewNextJSManager()
		err := manager.CreateSite(site)

		return nextjsSiteCreatedMsg{
			name: siteName,
			err:  err,
		}
	}
}

// handleCancel handles form cancellation
func (f *NextJSHybridFormModel) handleCancel() tea.Cmd {
	return f.GoBack()
}

// validateConditionalGitURL validates Git URL based on installation method
func (f *NextJSHybridFormModel) validateConditionalGitURL(gitURL string) error {
	if gitURL == "" {
		return nil // Optional field
	}

	// Get the installation method from the form
	values := f.form.GetValues()
	if len(values) < 3 {
		return nil // Not enough fields yet
	}

	installMethod := values[2]

	// Only validate if using git method
	if installMethod == "git" {
		return validateGitURL(gitURL)
	}

	return nil
}
