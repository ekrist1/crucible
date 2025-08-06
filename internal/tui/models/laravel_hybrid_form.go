package models

import (
	"errors"

	"crucible/internal/actions"
	tea "github.com/charmbracelet/bubbletea"
)

// LaravelHybridFormModel handles Laravel site creation using hybrid form (text + selection)
type LaravelHybridFormModel struct {
	BaseModel
	form *HybridFormModel
}

// NewLaravelHybridFormModel creates a new Laravel form using hybrid components
func NewLaravelHybridFormModel(shared *SharedData) *LaravelHybridFormModel {
	model := &LaravelHybridFormModel{
		BaseModel: NewBaseModel(shared),
	}
	model.setupForm()
	return model
}

// setupForm configures the Laravel creation form with hybrid fields
func (f *LaravelHybridFormModel) setupForm() {
	f.form = NewHybridFormModel(
		f.shared,
		"🚀 Create Laravel Site",
		"Fill out all fields and press Submit to create your Laravel site",
	)

	// Site Name field
	f.form.AddField(HybridFormField{
		Label:       "Site Name",
		FieldType:   HybridFieldTypeText,
		Placeholder: "my-laravel-app",
		Required:    true,
		Validator:   validateLaravelSiteName,
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
				Description: "Create a fresh Laravel installation",
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
	f.form.SetSubmitLabel("Create Laravel Site")

	// Set handlers
	f.form.SetSubmitHandler(f.handleSubmit)
	f.form.SetCancelHandler(f.handleCancel)
}

// Init initializes the form
func (f *LaravelHybridFormModel) Init() tea.Cmd {
	return f.form.Init()
}

// Update handles form updates
func (f *LaravelHybridFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	newModel, cmd := f.form.Update(msg)
	if hybridModel, ok := newModel.(*HybridFormModel); ok {
		f.form = hybridModel
	}
	return f, cmd
}

// View renders the form
func (f *LaravelHybridFormModel) View() string {
	return f.form.View()
}

// handleSubmit handles form submission
func (f *LaravelHybridFormModel) handleSubmit(values []string) tea.Cmd {
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

	// Create the Laravel site using the actions package
	config := actions.LaravelSiteConfig{
		SiteName: siteName,
		Domain:   domain,
	}

	// Check installation method
	if installMethod == "git" && gitRepo != "" {
		config.GitRepo = gitRepo
		config.Branch = branch
	}

	// Generate commands for site creation
	commands, descriptions := actions.CreateLaravelSite(config)
	if len(commands) == 0 {
		return func() tea.Msg {
			return laravelSiteCreatedMsg{
				name: siteName,
				err:  errors.New("no commands generated for site creation"),
			}
		}
	}

	// Navigate to processing state to execute Laravel site creation
	return f.NavigateTo(StateProcessing, map[string]interface{}{
		"action":      "laravel-create",
		"commands":    commands,
		"descriptions": descriptions,
		"serviceName": "laravel-" + siteName,
		"siteName":    siteName,
		"domain":      domain,
	})
}

// handleCancel handles form cancellation
func (f *LaravelHybridFormModel) handleCancel() tea.Cmd {
	return f.GoBack()
}

// validateConditionalGitURL validates Git URL based on installation method
func (f *LaravelHybridFormModel) validateConditionalGitURL(gitURL string) error {
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
