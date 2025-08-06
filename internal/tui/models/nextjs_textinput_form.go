package models

import (
	"errors"
	"regexp"
	"strings"

	"crucible/internal/nextjs"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// NextJSTextInputFormModel handles NextJS site creation using textinput components
type NextJSTextInputFormModel struct {
	BaseModel
	form *TextInputFormModel
}

// NewNextJSTextInputFormModel creates a new NextJS form using textinput components
func NewNextJSTextInputFormModel(shared *SharedData) *NextJSTextInputFormModel {
	model := &NextJSTextInputFormModel{
		BaseModel: NewBaseModel(shared),
	}
	model.setupForm()
	return model
}

// setupForm configures the NextJS creation form with textinput fields
func (f *NextJSTextInputFormModel) setupForm() {
	f.form = NewTextInputFormModel(
		f.shared,
		"⚡ Create NextJS Site",
		"Fill out all fields and press Submit to create your NextJS site",
	)

	// Site Name field
	f.form.AddField(TextInputField{
		Label:       "Site Name",
		Placeholder: "my-nextjs-app",
		Required:    true,
		Validator:   validateNextJSSiteName,
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

	// Installation Method field
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
		Validator:   validateBranch,
		EchoMode:    textinput.EchoNormal,
		MaxLength:   50,
	})

	// Set default values for optional fields
	f.form.SetValue(2, "fresh") // Installation method
	f.form.SetValue(4, "main")  // Branch

	// Set submit label
	f.form.SetSubmitLabel("Create NextJS Site")

	// Set handlers
	f.form.SetSubmitHandler(f.handleSubmit)
	f.form.SetCancelHandler(f.handleCancel)
}

// Init initializes the form
func (f *NextJSTextInputFormModel) Init() tea.Cmd {
	return f.form.Init()
}

// Update handles form updates
func (f *NextJSTextInputFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	newModel, cmd := f.form.Update(msg)
	if textInputModel, ok := newModel.(*TextInputFormModel); ok {
		f.form = textInputModel
	}
	return f, cmd
}

// View renders the form
func (f *NextJSTextInputFormModel) View() string {
	return f.form.View()
}

// handleSubmit handles form submission
func (f *NextJSTextInputFormModel) handleSubmit(values []string) tea.Cmd {
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
func (f *NextJSTextInputFormModel) handleCancel() tea.Cmd {
	return f.GoBack()
}

// validateConditionalGitURL validates Git URL based on installation method
func (f *NextJSTextInputFormModel) validateConditionalGitURL(gitURL string) error {
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

// validateNextJSSiteName validates NextJS site names
func validateNextJSSiteName(name string) error {
	if len(name) < 2 {
		return errors.New("site name must be at least 2 characters")
	}

	if len(name) > 50 {
		return errors.New("site name must be less than 50 characters")
	}

	// Check for valid characters (alphanumeric, hyphens, underscores)
	validName := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validName.MatchString(name) {
		return errors.New("site name can only contain letters, numbers, hyphens, and underscores")
	}

	// Can't start or end with hyphen or underscore
	if strings.HasPrefix(name, "-") || strings.HasPrefix(name, "_") ||
		strings.HasSuffix(name, "-") || strings.HasSuffix(name, "_") {
		return errors.New("site name cannot start or end with hyphen or underscore")
	}

	return nil
}
