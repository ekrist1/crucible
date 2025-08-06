package models

import (
	"errors"

	"crucible/internal/actions"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// LaravelTextInputFormModel handles Laravel site creation using textinput components
type LaravelTextInputFormModel struct {
	BaseModel
	form *TextInputFormModel
}

// NewLaravelTextInputFormModel creates a new Laravel form using textinput components
func NewLaravelTextInputFormModel(shared *SharedData) *LaravelTextInputFormModel {
	model := &LaravelTextInputFormModel{
		BaseModel: NewBaseModel(shared),
	}
	model.setupForm()
	return model
}

// setupForm configures the Laravel creation form with textinput fields
func (f *LaravelTextInputFormModel) setupForm() {
	f.form = NewTextInputFormModel(
		f.shared,
		"🚀 Create Laravel Site",
		"Fill out all fields and press Submit to create your Laravel site",
	)

	// Site Name field
	f.form.AddField(TextInputField{
		Label:       "Site Name",
		Placeholder: "my-laravel-app",
		Required:    true,
		Validator:   validateLaravelSiteName,
		EchoMode:    textinput.EchoNormal,
		MaxLength:   50,
	})

	// Domain field
	f.form.AddField(TextInputField{
		Label:       "Domain",
		Placeholder: "myapp.local",
		Required:    true,
		Validator:   validateDomain,
		EchoMode:    textinput.EchoNormal,
		MaxLength:   100,
	})

	// Installation Method field (we'll use a simple text field that accepts specific values)
	f.form.AddField(TextInputField{
		Label:       "Installation Method",
		Placeholder: "fresh or git (default: fresh)",
		Required:    false,
		Validator:   validateInstallationMethod,
		EchoMode:    textinput.EchoNormal,
		MaxLength:   10,
	})

	// Git Repository field
	f.form.AddField(TextInputField{
		Label:       "Git Repository URL",
		Placeholder: "https://github.com/user/repo.git (optional)",
		Required:    false,
		Validator:   f.validateConditionalGitURL,
		EchoMode:    textinput.EchoNormal,
		MaxLength:   200,
	})

	// Branch field
	f.form.AddField(TextInputField{
		Label:       "Branch",
		Placeholder: "main (default if using Git)",
		Required:    false,
		Validator:   nil, // No specific validation needed
		EchoMode:    textinput.EchoNormal,
		MaxLength:   50,
	})

	// Set default values for optional fields
	f.form.SetValue(2, "fresh") // Installation method
	f.form.SetValue(4, "main")  // Branch

	// Set submit label
	f.form.SetSubmitLabel("Create Laravel Site")

	// Set handlers
	f.form.SetSubmitHandler(f.handleSubmit)
	f.form.SetCancelHandler(f.handleCancel)
}

// Init initializes the form
func (f *LaravelTextInputFormModel) Init() tea.Cmd {
	return f.form.Init()
}

// Update handles form updates
func (f *LaravelTextInputFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	newModel, cmd := f.form.Update(msg)
	if textInputModel, ok := newModel.(*TextInputFormModel); ok {
		f.form = textInputModel
	}
	return f, cmd
}

// View renders the form
func (f *LaravelTextInputFormModel) View() string {
	return f.form.View()
}

// handleSubmit handles form submission
func (f *LaravelTextInputFormModel) handleSubmit(values []string) tea.Cmd {
	return func() tea.Msg {
		// Extract values by position
		siteName := values[0]
		domain := values[1]
		installMethod := values[2]
		gitRepo := values[3]
		branch := values[4]

		// Set defaults
		if installMethod == "" {
			installMethod = "fresh"
		}
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
			return laravelSiteCreatedMsg{
				name: siteName,
				err:  errors.New("no commands generated for site creation"),
			}
		}

		// Execute commands using the new command execution framework
		// Note: This returns a CmdCompletedMsg, not laravelSiteCreatedMsg
		// The processing model should handle CmdCompletedMsg and convert it appropriately
		result := ExecuteCommandBatchAsync(commands, descriptions, "laravel-"+siteName)()

		// Convert the result to the expected message type
		if cmdResult, ok := result.(CmdCompletedMsg); ok {
			return laravelSiteCreatedMsg{
				name: siteName,
				err:  cmdResult.Result.Error,
			}
		}

		return laravelSiteCreatedMsg{
			name: siteName,
			err:  errors.New("unexpected result type from command execution"),
		}
	}
}

// handleCancel handles form cancellation
func (f *LaravelTextInputFormModel) handleCancel() tea.Cmd {
	return f.GoBack()
}

// validateConditionalGitURL validates Git URL based on installation method
func (f *LaravelTextInputFormModel) validateConditionalGitURL(gitURL string) error {
	if gitURL == "" {
		return nil // Optional field
	}

	// Get the installation method from the form
	values := f.form.GetValues()
	if len(values) < 3 {
		return nil // Not enough fields yet
	}

	installMethod := values[2]
	if installMethod == "" {
		installMethod = "fresh"
	}

	// Only validate if using git method
	if installMethod == "git" {
		return validateGitURL(gitURL)
	}

	return nil
}

// Validation functions
