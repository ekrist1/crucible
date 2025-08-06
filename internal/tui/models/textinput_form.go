package models

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// TextInputFormModel handles forms using Bubble Tea's textinput components
type TextInputFormModel struct {
	BaseModel
	title       string
	description string
	inputs      []textinput.Model
	focusIndex  int
	submitLabel string
	onSubmit    func([]string) tea.Cmd
	onCancel    func() tea.Cmd

	inputLabels  []string
	required     []bool
	validators   []func(string) error
	placeholders []string
	echoModes    []textinput.EchoMode

	errors     []string
	showErrors bool
}

// TextInputField represents a field configuration for the form
type TextInputField struct {
	Label       string
	Placeholder string
	Required    bool
	Validator   func(string) error
	EchoMode    textinput.EchoMode // Normal, Password, None
	MaxLength   int
}

// NewTextInputFormModel creates a new textinput-based form model
func NewTextInputFormModel(shared *SharedData, title, description string) *TextInputFormModel {
	return &TextInputFormModel{
		BaseModel:    NewBaseModel(shared),
		title:        title,
		description:  description,
		submitLabel:  "Submit",
		focusIndex:   0,
		inputs:       []textinput.Model{},
		inputLabels:  []string{},
		required:     []bool{},
		validators:   []func(string) error{},
		placeholders: []string{},
		echoModes:    []textinput.EchoMode{},
		errors:       []string{},
	}
}

// AddField adds a field to the form
func (f *TextInputFormModel) AddField(field TextInputField) {
	input := textinput.New()
	input.Placeholder = field.Placeholder
	input.EchoMode = field.EchoMode
	if field.MaxLength > 0 {
		input.CharLimit = field.MaxLength
	}

	// Focus the first input by default
	if len(f.inputs) == 0 {
		input.Focus()
	}

	f.inputs = append(f.inputs, input)
	f.inputLabels = append(f.inputLabels, field.Label)
	f.required = append(f.required, field.Required)
	f.validators = append(f.validators, field.Validator)
	f.placeholders = append(f.placeholders, field.Placeholder)
	f.echoModes = append(f.echoModes, field.EchoMode)
	f.errors = append(f.errors, "")
}

// SetSubmitLabel sets the label for the submit button
func (f *TextInputFormModel) SetSubmitLabel(label string) {
	f.submitLabel = label
}

// SetSubmitHandler sets the function to call when form is submitted
func (f *TextInputFormModel) SetSubmitHandler(handler func([]string) tea.Cmd) {
	f.onSubmit = handler
}

// SetCancelHandler sets the function to call when form is cancelled
func (f *TextInputFormModel) SetCancelHandler(handler func() tea.Cmd) {
	f.onCancel = handler
}

// Init initializes the form model
func (f *TextInputFormModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles form input and navigation
func (f *TextInputFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return f, tea.Quit

		case "esc":
			if f.onCancel != nil {
				return f, f.onCancel()
			}
			return f, f.GoBack()

		case "tab", "shift+tab", "enter", "up", "down":
			s := msg.String()

			// Cycle between inputs and submit button
			if s == "up" || s == "shift+tab" {
				f.focusIndex--
			} else {
				f.focusIndex++
			}

			// Handle focus wrapping
			if f.focusIndex > len(f.inputs) {
				f.focusIndex = 0
			} else if f.focusIndex < 0 {
				f.focusIndex = len(f.inputs)
			}

			cmds := make([]tea.Cmd, len(f.inputs))
			for i := 0; i <= len(f.inputs)-1; i++ {
				if i == f.focusIndex {
					// Set focused state
					cmds[i] = f.inputs[i].Focus()
					continue
				}
				// Remove focused state
				f.inputs[i].Blur()
			}

			// Handle submit button focus (when focusIndex == len(inputs))
			if f.focusIndex == len(f.inputs) && s == "enter" {
				return f.handleSubmit()
			}

			return f, tea.Batch(cmds...)
		}
	}

	// Handle input updates
	cmd := f.updateInputs(msg)
	return f, cmd
}

// updateInputs updates the currently focused input
func (f *TextInputFormModel) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(f.inputs))

	// Only update the focused input
	if f.focusIndex >= 0 && f.focusIndex < len(f.inputs) {
		f.inputs[f.focusIndex], cmds[f.focusIndex] = f.inputs[f.focusIndex].Update(msg)

		// Clear error for this field when user starts typing
		if keyMsg, ok := msg.(tea.KeyMsg); ok && len(keyMsg.String()) == 1 {
			f.errors[f.focusIndex] = ""
			f.showErrors = false
		}
	}

	return tea.Batch(cmds...)
}

// handleSubmit validates and submits the form
func (f *TextInputFormModel) handleSubmit() (tea.Model, tea.Cmd) {
	// Clear previous errors
	f.errors = make([]string, len(f.inputs))
	f.showErrors = false

	// Validate all fields
	valid := true
	values := make([]string, len(f.inputs))

	for i, input := range f.inputs {
		value := strings.TrimSpace(input.Value())
		values[i] = value

		// Check required fields
		if f.required[i] && value == "" {
			f.errors[i] = "This field is required"
			valid = false
			continue
		}

		// Run custom validator
		if f.validators[i] != nil {
			if err := f.validators[i](value); err != nil {
				f.errors[i] = err.Error()
				valid = false
				continue
			}
		}
	}

	if !valid {
		f.showErrors = true
		return f, nil
	}

	// Submit the form
	if f.onSubmit != nil {
		return f, f.onSubmit(values)
	}

	return f, nil
}

// View renders the form
func (f *TextInputFormModel) View() string {
	var s strings.Builder

	// Title
	s.WriteString(formTitleStyle.Render(f.title))
	s.WriteString("\n\n")

	// Description
	if f.description != "" {
		s.WriteString(f.description)
		s.WriteString("\n\n")
	}

	// Render form inputs
	for i := range f.inputs {
		// Label
		label := f.inputLabels[i]
		if f.required[i] {
			label += " *"
		}

		if i == f.focusIndex {
			label = fieldLabelStyle.Render("→ " + label)
		} else {
			label = fieldLabelStyle.Render("  " + label)
		}
		s.WriteString(label)
		s.WriteString("\n")

		// Input field
		inputView := f.inputs[i].View()
		if i == f.focusIndex {
			inputView = fieldValueStyle.Render(inputView)
		} else {
			inputView = choiceStyle.Render(inputView)
		}
		s.WriteString("  ")
		s.WriteString(inputView)
		s.WriteString("\n")

		// Error message
		if f.showErrors && f.errors[i] != "" {
			s.WriteString("  ")
			s.WriteString(errorStyle.Render("⚠ " + f.errors[i]))
			s.WriteString("\n")
		}

		s.WriteString("\n")
	}

	// Submit button
	submitButton := f.submitLabel
	if f.focusIndex == len(f.inputs) {
		submitButton = selectedStyle.Render("→ " + submitButton)
	} else {
		submitButton = choiceStyle.Render("  " + submitButton)
	}
	s.WriteString(submitButton)
	s.WriteString("\n\n")

	// Help text
	help := []string{
		"Tab/↑↓ - Navigate fields",
		"Enter  - Submit form",
		"Esc    - Cancel",
	}
	s.WriteString(helpStyle.Render(strings.Join(help, " • ")))

	return s.String()
}

// GetValues returns all form values
func (f *TextInputFormModel) GetValues() []string {
	values := make([]string, len(f.inputs))
	for i, input := range f.inputs {
		values[i] = strings.TrimSpace(input.Value())
	}
	return values
}

// SetValue sets a form value by index
func (f *TextInputFormModel) SetValue(index int, value string) {
	if index >= 0 && index < len(f.inputs) {
		f.inputs[index].SetValue(value)
	}
}

// GetValue gets a form value by index
func (f *TextInputFormModel) GetValue(index int) string {
	if index >= 0 && index < len(f.inputs) {
		return f.inputs[index].Value()
	}
	return ""
}
