package models

import (
	tea "github.com/charmbracelet/bubbletea"
	"crucible/internal/services"
)

// Node.js message types
type nodejsInstallCompleteMsg struct {
	err error
}

// NodeJSFormModel handles Node.js installation forms
type NodeJSFormModel struct {
	BaseModel
	form *FormModel
}

// NewNodeJSFormModel creates a new Node.js installation form model
func NewNodeJSFormModel(shared *SharedData) *NodeJSFormModel {
	model := &NodeJSFormModel{
		BaseModel: NewBaseModel(shared),
	}
	model.setupForm()
	return model
}

// Init initializes the Node.js form model
func (f *NodeJSFormModel) Init() tea.Cmd {
	return f.form.Init()
}

// Update handles Node.js form updates
func (f *NodeJSFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return f, tea.Quit
		case "esc":
			return f, f.GoBack()
		}
	
	case nodejsInstallCompleteMsg:
		if msg.err != nil {
			// Handle error case - could add error display here
			return f, f.GoBack()
		} else {
			// Navigate to processing state to execute the commands
			return f, f.NavigateTo(StateProcessing, map[string]interface{}{
				"title": "Installing Node.js and npm",
			})
		}
	}

	// Delegate to form
	newForm, cmd := f.form.Update(msg)
	if formModel, ok := newForm.(*FormModel); ok {
		f.form = formModel
	}
	return f, cmd
}

// View renders the Node.js form
func (f *NodeJSFormModel) View() string {
	return f.form.View()
}

// setupForm configures the Node.js installation form
func (f *NodeJSFormModel) setupForm() {
	f.form = NewFormModel(f.shared, "📦 Install Node.js and npm")

	// Step 1: PM2 Configuration
	step1 := FormStep{
		Title:       "Installation Options",
		Description: "Configure Node.js installation options",
		Fields: []FormField{
			{
				Name:        "installPM2",
				Label:       "Install PM2 Process Manager?",
				FieldType:   FieldTypeSelect,
				Required:    true,
				Value:       "Yes",
				Options:     []string{"Yes", "No"},
			},
		},
	}

	// Add steps to form
	f.form.AddStep(step1)

	// Set handlers
	f.form.SetSubmitHandler(f.handleSubmit)
	f.form.SetCancelHandler(f.handleCancel)
}

// handleSubmit handles form submission
func (f *NodeJSFormModel) handleSubmit(values map[string]string) tea.Cmd {
	return func() tea.Msg {
		// Determine if PM2 should be installed
		installPM2 := values["installPM2"] == "Yes"
		
		// Get the commands for Node.js installation
		commands, descriptions, err := services.InstallNodeWithPM2(installPM2)
		if err != nil {
			return nodejsInstallCompleteMsg{err: err}
		}
		
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
			f.shared.CommandQueue.ServiceName = "Node.js Installation"
		}
		
		return nodejsInstallCompleteMsg{err: nil}
	}
}

// handleCancel handles form cancellation
func (f *NodeJSFormModel) handleCancel() tea.Cmd {
	return f.GoBack()
}