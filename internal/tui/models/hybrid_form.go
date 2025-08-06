package models

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// FormFieldType represents the type of form field
type HybridFormFieldType int

const (
	HybridFieldTypeText HybridFormFieldType = iota
	HybridFieldTypePassword
	HybridFieldTypeSelection
)

// SelectionOption represents an option in a selection field
type SelectionOption struct {
	Value       string
	Description string
}

// HybridFormField represents a field that can be either text input or selection
type HybridFormField struct {
	Label       string
	FieldType   HybridFormFieldType
	Placeholder string
	Required    bool
	Validator   func(string) error
	MaxLength   int

	// For selection fields
	Options       []SelectionOption
	SelectedIndex int
}

// HybridFormModel handles forms with mixed text inputs and selections
type HybridFormModel struct {
	BaseModel
	title       string
	description string
	fields      []HybridFormField
	textInputs  []textinput.Model
	focusIndex  int
	submitLabel string
	onSubmit    func([]string) tea.Cmd
	onCancel    func() tea.Cmd

	errors     []string
	showErrors bool
}

// NewHybridFormModel creates a new hybrid form model
func NewHybridFormModel(shared *SharedData, title, description string) *HybridFormModel {
	return &HybridFormModel{
		BaseModel:   NewBaseModel(shared),
		title:       title,
		description: description,
		submitLabel: "Submit",
		focusIndex:  0,
		fields:      []HybridFormField{},
		textInputs:  []textinput.Model{},
		errors:      []string{},
	}
}

// AddField adds a field to the form
func (f *HybridFormModel) AddField(field HybridFormField) {
	f.fields = append(f.fields, field)
	f.errors = append(f.errors, "")

	// Only create textinput for text fields
	if field.FieldType == HybridFieldTypeText || field.FieldType == HybridFieldTypePassword {
		input := textinput.New()
		input.Placeholder = field.Placeholder
		if field.FieldType == HybridFieldTypePassword {
			input.EchoMode = textinput.EchoPassword
		}
		if field.MaxLength > 0 {
			input.CharLimit = field.MaxLength
		}

		// Focus the first input by default
		if len(f.textInputs) == 0 && f.focusIndex == 0 {
			input.Focus()
		}

		f.textInputs = append(f.textInputs, input)
	}
}

// SetSubmitLabel sets the label for the submit button
func (f *HybridFormModel) SetSubmitLabel(label string) {
	f.submitLabel = label
}

// SetSubmitHandler sets the function to call when form is submitted
func (f *HybridFormModel) SetSubmitHandler(handler func([]string) tea.Cmd) {
	f.onSubmit = handler
}

// SetCancelHandler sets the function to call when form is cancelled
func (f *HybridFormModel) SetCancelHandler(handler func() tea.Cmd) {
	f.onCancel = handler
}

// Init initializes the form model
func (f *HybridFormModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles form input and navigation
func (f *HybridFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			return f.handleNavigation(msg.String())

		case "left", "right":
			// Handle selection field navigation
			if f.focusIndex < len(f.fields) && f.fields[f.focusIndex].FieldType == HybridFieldTypeSelection {
				return f.handleSelectionNavigation(msg.String())
			}
			// Fall through to text input handling
		}
	}

	// Handle text input updates for focused field
	if f.focusIndex < len(f.fields) {
		field := f.fields[f.focusIndex]
		if field.FieldType == HybridFieldTypeText || field.FieldType == HybridFieldTypePassword {
			// Find the corresponding textinput index
			textInputIndex := f.getTextInputIndex(f.focusIndex)
			if textInputIndex >= 0 && textInputIndex < len(f.textInputs) {
				var cmd tea.Cmd
				f.textInputs[textInputIndex], cmd = f.textInputs[textInputIndex].Update(msg)

				// Clear error when user starts typing
				if keyMsg, ok := msg.(tea.KeyMsg); ok && len(keyMsg.String()) == 1 {
					f.errors[f.focusIndex] = ""
					f.showErrors = false
				}

				return f, cmd
			}
		}
	}

	return f, nil
}

// handleNavigation handles tab, enter, up, down navigation
func (f *HybridFormModel) handleNavigation(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "shift+tab":
		f.focusIndex--
	case "down", "tab":
		f.focusIndex++
	case "enter":
		// Submit button handling
		if f.focusIndex == len(f.fields) {
			return f.handleSubmit()
		}
		// For other fields, just move to next
		f.focusIndex++
	}

	// Handle focus wrapping
	if f.focusIndex > len(f.fields) {
		f.focusIndex = 0
	} else if f.focusIndex < 0 {
		f.focusIndex = len(f.fields)
	}

	// Update text input focus
	f.updateTextInputFocus()

	return f, nil
}

// handleSelectionNavigation handles left/right for selection fields
func (f *HybridFormModel) handleSelectionNavigation(key string) (tea.Model, tea.Cmd) {
	if f.focusIndex >= len(f.fields) {
		return f, nil
	}

	field := &f.fields[f.focusIndex]
	if field.FieldType != HybridFieldTypeSelection || len(field.Options) == 0 {
		return f, nil
	}

	switch key {
	case "left":
		if field.SelectedIndex > 0 {
			field.SelectedIndex--
		}
	case "right":
		if field.SelectedIndex < len(field.Options)-1 {
			field.SelectedIndex++
		}
	}

	// Clear error for this field
	f.errors[f.focusIndex] = ""
	f.showErrors = false

	return f, nil
}

// updateTextInputFocus updates which text input has focus
func (f *HybridFormModel) updateTextInputFocus() {
	// Blur all text inputs first
	for i := range f.textInputs {
		f.textInputs[i].Blur()
	}

	// Focus the current field if it's a text input
	if f.focusIndex < len(f.fields) {
		field := f.fields[f.focusIndex]
		if field.FieldType == HybridFieldTypeText || field.FieldType == HybridFieldTypePassword {
			textInputIndex := f.getTextInputIndex(f.focusIndex)
			if textInputIndex >= 0 && textInputIndex < len(f.textInputs) {
				f.textInputs[textInputIndex].Focus()
			}
		}
	}
}

// getTextInputIndex maps field index to textinput index
func (f *HybridFormModel) getTextInputIndex(fieldIndex int) int {
	textInputIndex := 0
	for i := 0; i < fieldIndex && i < len(f.fields); i++ {
		field := f.fields[i]
		if field.FieldType == HybridFieldTypeText || field.FieldType == HybridFieldTypePassword {
			textInputIndex++
		}
	}

	// Check if current field is a text input
	if fieldIndex < len(f.fields) {
		field := f.fields[fieldIndex]
		if field.FieldType == HybridFieldTypeText || field.FieldType == HybridFieldTypePassword {
			return textInputIndex
		}
	}

	return -1
}

// handleSubmit validates and submits the form
func (f *HybridFormModel) handleSubmit() (tea.Model, tea.Cmd) {
	// Clear previous errors
	f.errors = make([]string, len(f.fields))
	f.showErrors = false

	// Validate all fields
	valid := true
	values := make([]string, len(f.fields))

	textInputIndex := 0
	for i, field := range f.fields {
		var value string

		switch field.FieldType {
		case HybridFieldTypeText, HybridFieldTypePassword:
			if textInputIndex < len(f.textInputs) {
				value = strings.TrimSpace(f.textInputs[textInputIndex].Value())
				textInputIndex++
			}
		case HybridFieldTypeSelection:
			if field.SelectedIndex >= 0 && field.SelectedIndex < len(field.Options) {
				value = field.Options[field.SelectedIndex].Value
			}
		}

		values[i] = value

		// Check required fields
		if field.Required && value == "" {
			f.errors[i] = "This field is required"
			valid = false
			continue
		}

		// Run custom validator
		if field.Validator != nil {
			if err := field.Validator(value); err != nil {
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
func (f *HybridFormModel) View() string {
	var s strings.Builder

	// Title
	s.WriteString(formTitleStyle.Render(f.title))
	s.WriteString("\n\n")

	// Description
	if f.description != "" {
		s.WriteString(f.description)
		s.WriteString("\n\n")
	}

	// Render form fields
	textInputIndex := 0
	for i, field := range f.fields {
		// Label
		label := field.Label
		if field.Required {
			label += " *"
		}

		if i == f.focusIndex {
			label = fieldLabelStyle.Render("→ " + label)
		} else {
			label = fieldLabelStyle.Render("  " + label)
		}
		s.WriteString(label)
		s.WriteString("\n")

		// Field content
		var fieldView string
		switch field.FieldType {
		case HybridFieldTypeText, HybridFieldTypePassword:
			if textInputIndex < len(f.textInputs) {
				fieldView = f.textInputs[textInputIndex].View()
				textInputIndex++
			}
		case HybridFieldTypeSelection:
			fieldView = f.renderSelection(field, i == f.focusIndex)
		}

		if i == f.focusIndex {
			fieldView = fieldValueStyle.Render(fieldView)
		} else {
			fieldView = choiceStyle.Render(fieldView)
		}
		s.WriteString("  ")
		s.WriteString(fieldView)
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
	if f.focusIndex == len(f.fields) {
		submitButton = selectedStyle.Render("→ " + submitButton)
	} else {
		submitButton = choiceStyle.Render("  " + submitButton)
	}
	s.WriteString(submitButton)
	s.WriteString("\n\n")

	// Help text
	help := []string{
		"Tab/↑↓ - Navigate fields",
		"←/→    - Change selection",
		"Enter  - Submit form",
		"Esc    - Cancel",
	}
	s.WriteString(helpStyle.Render(strings.Join(help, " • ")))

	return s.String()
}

// renderSelection renders a selection field
func (f *HybridFormModel) renderSelection(field HybridFormField, isFocused bool) string {
	if len(field.Options) == 0 {
		return "No options available"
	}

	if !isFocused {
		// Show only selected value when not focused
		if field.SelectedIndex >= 0 && field.SelectedIndex < len(field.Options) {
			option := field.Options[field.SelectedIndex]
			return option.Value + " - " + option.Description
		}
		return "No selection"
	}

	// Show all options when focused
	var s strings.Builder
	for i, option := range field.Options {
		if i == field.SelectedIndex {
			s.WriteString(selectedStyle.Render("● " + option.Value + " - " + option.Description))
		} else {
			s.WriteString(choiceStyle.Render("○ " + option.Value + " - " + option.Description))
		}
		if i < len(field.Options)-1 {
			s.WriteString("  ")
		}
	}
	return s.String()
}

// GetValues returns all form values
func (f *HybridFormModel) GetValues() []string {
	values := make([]string, len(f.fields))
	textInputIndex := 0

	for i, field := range f.fields {
		switch field.FieldType {
		case HybridFieldTypeText, HybridFieldTypePassword:
			if textInputIndex < len(f.textInputs) {
				values[i] = strings.TrimSpace(f.textInputs[textInputIndex].Value())
				textInputIndex++
			}
		case HybridFieldTypeSelection:
			if field.SelectedIndex >= 0 && field.SelectedIndex < len(field.Options) {
				values[i] = field.Options[field.SelectedIndex].Value
			}
		}
	}

	return values
}

// SetValue sets a form value by index
func (f *HybridFormModel) SetValue(index int, value string) {
	if index < 0 || index >= len(f.fields) {
		return
	}

	field := &f.fields[index]
	switch field.FieldType {
	case HybridFieldTypeText, HybridFieldTypePassword:
		textInputIndex := f.getTextInputIndex(index)
		if textInputIndex >= 0 && textInputIndex < len(f.textInputs) {
			f.textInputs[textInputIndex].SetValue(value)
		}
	case HybridFieldTypeSelection:
		// Find matching option
		for i, option := range field.Options {
			if option.Value == value {
				field.SelectedIndex = i
				break
			}
		}
	}
}
