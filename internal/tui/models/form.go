package models

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// FormFieldType represents the type of form field
type FormFieldType int

const (
	FieldTypeText FormFieldType = iota
	FieldTypePassword
	FieldTypeEmail
	FieldTypeURL
	FieldTypeNumber
	FieldTypeSelect
	FieldTypeMultiline
)

// FormField represents a single form field
type FormField struct {
	Name        string
	Label       string
	Value       string
	FieldType   FormFieldType
	Required    bool
	Placeholder string
	Options     []string // For select fields
	Validator   func(string) error
	MaxLength   int
	MinLength   int
}

// FormStep represents a step in a multi-step form
type FormStep struct {
	Title       string
	Description string
	Fields      []FormField
}

// FormModel handles generic form input and validation
type FormModel struct {
	BaseModel
	title        string
	steps        []FormStep
	currentStep  int
	currentField int
	inputCursor  int
	values       map[string]string
	errors       map[string]string
	onSubmit     func(map[string]string) tea.Cmd
	onCancel     func() tea.Cmd
	showHelp     bool
}

// Styles are now centralized in styles.go

// NewFormModel creates a new form model
func NewFormModel(shared *SharedData, title string) *FormModel {
	return &FormModel{
		BaseModel:    NewBaseModel(shared),
		title:        title,
		currentStep:  0,
		currentField: 0,
		inputCursor:  0,
		values:       make(map[string]string),
		errors:       make(map[string]string),
	}
}

// AddStep adds a step to the form
func (f *FormModel) AddStep(step FormStep) {
	f.steps = append(f.steps, step)
}

// SetSubmitHandler sets the function to call when form is submitted
func (f *FormModel) SetSubmitHandler(handler func(map[string]string) tea.Cmd) {
	f.onSubmit = handler
}

// SetCancelHandler sets the function to call when form is cancelled
func (f *FormModel) SetCancelHandler(handler func() tea.Cmd) {
	f.onCancel = handler
}

// Init initializes the form model
func (f *FormModel) Init() tea.Cmd {
	// Initialize default values
	for _, step := range f.steps {
		for _, field := range step.Fields {
			if f.values[field.Name] == "" && field.Value != "" {
				f.values[field.Name] = field.Value
			}
		}
	}
	return nil
}

// Update handles form input and navigation
func (f *FormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if f.onCancel != nil {
				return f, f.onCancel()
			}
			return f, tea.Quit

		case "esc":
			if f.onCancel != nil {
				return f, f.onCancel()
			}
			return f, f.GoBack()

		case "tab", "down":
			f.nextField()

		case "shift+tab", "up":
			f.previousField()

		case "enter":
			return f.handleEnter()

		case "ctrl+n":
			if f.canGoToNextStep() {
				f.nextStep()
			}

		case "ctrl+p":
			if f.canGoToPreviousStep() {
				f.previousStep()
			}

		case "ctrl+h":
			f.showHelp = !f.showHelp

		case "left":
			// For select fields, move to previous option
			if f.isCurrentFieldSelect() {
				f.cyclePrevSelectOption()
			} else {
				f.moveCursorLeft()
			}

		case "right":
			// For select fields, move to next option
			if f.isCurrentFieldSelect() {
				f.cycleSelectOption(*f.getCurrentField())
			} else {
				f.moveCursorRight()
			}

		case "home":
			f.inputCursor = 0

		case "end":
			f.inputCursor = len(f.getCurrentFieldValue())

		case "backspace":
			f.handleBackspace()

		case "delete":
			f.handleDelete()

		case "ctrl+v":
			// For now, show a message about paste
			// In a real terminal, Ctrl+V should trigger the system paste
			// which will appear as regular character input
			return f, nil

		default:
			// Handle regular character input (including pasted text)
			if len(msg.String()) == 1 {
				f.handleCharacterInput(msg.String())
			} else if len(msg.String()) > 1 {
				// This could be pasted text - handle it as such
				f.handlePaste(msg.String())
			}
		}
	}

	return f, nil
}

// View renders the form
func (f *FormModel) View() string {
	if len(f.steps) == 0 {
		return "No form steps defined"
	}

	var s strings.Builder

	// Title
	s.WriteString(formTitleStyle.Render(f.title))
	s.WriteString("\n\n")

	// Progress indicator
	if len(f.steps) > 1 {
		progress := fmt.Sprintf("Step %d of %d: %s",
			f.currentStep+1, len(f.steps), f.steps[f.currentStep].Title)
		s.WriteString(progressStyle.Render(progress))
		s.WriteString("\n\n")
	}

	// Step description
	if f.steps[f.currentStep].Description != "" {
		s.WriteString(f.steps[f.currentStep].Description)
		s.WriteString("\n\n")
	}

	// Form fields
	step := f.steps[f.currentStep]
	for i, field := range step.Fields {
		s.WriteString(f.renderField(field, i == f.currentField))
		s.WriteString("\n")
	}

	// Help text
	s.WriteString("\n")
	if f.showHelp {
		s.WriteString(f.renderHelp())
	} else {
		s.WriteString(helpStyle.Render("Press Ctrl+H for help"))
	}

	return s.String()
}

// renderField renders a single form field
func (f *FormModel) renderField(field FormField, isCurrent bool) string {
	var s strings.Builder

	// Field label
	label := field.Label
	if field.Required {
		label += " *"
	}
	if isCurrent {
		label = fieldLabelStyle.Render("→ " + label)
	} else {
		label = fieldLabelStyle.Render("  " + label)
	}
	s.WriteString(label)
	s.WriteString("\n")

	// Field value
	value := f.values[field.Name]
	if value == "" && field.Placeholder != "" && !isCurrent {
		value = helpStyle.Render(field.Placeholder)
	}

	// Handle different field types
	switch field.FieldType {
	case FieldTypePassword:
		if isCurrent {
			value = f.renderPasswordField(value)
		} else {
			value = strings.Repeat("*", len(value))
		}
	case FieldTypeSelect:
		value = f.renderSelectField(field, isCurrent)
	default:
		if isCurrent {
			value = f.renderTextFieldWithCursor(value)
		}
	}

	// Style the field
	if isCurrent {
		value = fieldValueStyle.Render(value)
	} else {
		value = choiceStyle.Render(value)
	}
	s.WriteString("  ")
	s.WriteString(value)

	// Error message
	if err, hasError := f.errors[field.Name]; hasError {
		s.WriteString("\n  ")
		s.WriteString(errorStyle.Render("⚠ " + err))
	}

	return s.String()
}

// renderTextFieldWithCursor renders a text field with cursor
func (f *FormModel) renderTextFieldWithCursor(value string) string {
	if f.inputCursor > len(value) {
		f.inputCursor = len(value)
	}
	if f.inputCursor < 0 {
		f.inputCursor = 0
	}

	if f.inputCursor == len(value) {
		return value + "█"
	}

	return value[:f.inputCursor] + "█" + value[f.inputCursor+1:]
}

// renderPasswordField renders a password field with cursor
func (f *FormModel) renderPasswordField(value string) string {
	maskedValue := strings.Repeat("*", len(value))
	return f.renderTextFieldWithCursor(maskedValue)
}

// renderSelectField renders a select field
func (f *FormModel) renderSelectField(field FormField, isCurrent bool) string {
	value := f.values[field.Name]
	if !isCurrent {
		return value
	}

	// Show dropdown options
	var s strings.Builder
	s.WriteString(selectedStyle.Render(value))
	s.WriteString(" (Use ←/→/Space to change)\n")
	for _, option := range field.Options {
		prefix := "    "
		optionText := option
		if option == value {
			prefix = "  → "
			optionText = selectedStyle.Render(option)
		} else {
			optionText = choiceStyle.Render(option)
		}
		s.WriteString(prefix + optionText + "\n")
	}
	return s.String()
}

// renderHelp renders help text
func (f *FormModel) renderHelp() string {
	help := []string{
		"Navigation:",
		"  Tab/↓      - Next field",
		"  Shift+Tab/↑ - Previous field",
		"  Ctrl+N     - Next step (if valid)",
		"  Ctrl+P     - Previous step",
		"  Enter      - Submit/Next step",
		"  Esc        - Cancel",
		"",
		"Editing:",
		"  ←/→        - Move cursor (or select options)",
		"  Space      - Cycle select options",
		"  Home/End   - Start/End of field",
		"  Backspace  - Delete char before cursor",
		"  Delete     - Delete char at cursor",
		"  Ctrl+V     - Paste from clipboard",
		"",
		"Press Ctrl+H to hide help",
	}
	return helpStyle.Render(strings.Join(help, "\n"))
}

// Field navigation methods
func (f *FormModel) nextField() {
	if f.currentStep >= len(f.steps) {
		return
	}
	step := f.steps[f.currentStep]
	if f.currentField < len(step.Fields)-1 {
		f.currentField++
		f.inputCursor = len(f.getCurrentFieldValue())
	}
}

func (f *FormModel) previousField() {
	if f.currentField > 0 {
		f.currentField--
		f.inputCursor = len(f.getCurrentFieldValue())
	}
}

func (f *FormModel) nextStep() {
	if f.currentStep < len(f.steps)-1 {
		f.currentStep++
		f.currentField = 0
		f.inputCursor = 0
	}
}

func (f *FormModel) previousStep() {
	if f.currentStep > 0 {
		f.currentStep--
		f.currentField = 0
		f.inputCursor = 0
	}
}

func (f *FormModel) canGoToNextStep() bool {
	return f.currentStep < len(f.steps)-1 && f.validateCurrentStep()
}

func (f *FormModel) canGoToPreviousStep() bool {
	return f.currentStep > 0
}

// Input handling methods
func (f *FormModel) handleEnter() (tea.Model, tea.Cmd) {
	// Validate current field
	if !f.validateCurrentField() {
		return f, nil
	}

	// If we're at the last field of the last step, submit
	if f.isAtLastField() {
		if f.validateAllSteps() && f.onSubmit != nil {
			return f, f.onSubmit(f.values)
		}
		return f, nil
	}

	// If we're at the last field of current step, go to next step
	if f.isAtLastFieldOfStep() && f.validateCurrentStep() {
		f.nextStep()
		return f, nil
	}

	// Otherwise, go to next field
	f.nextField()
	return f, nil
}

func (f *FormModel) handleCharacterInput(char string) {
	if f.currentStep >= len(f.steps) {
		return
	}

	step := f.steps[f.currentStep]
	if f.currentField >= len(step.Fields) {
		return
	}

	field := step.Fields[f.currentField]
	currentValue := f.values[field.Name]

	// Handle select fields
	if field.FieldType == FieldTypeSelect {
		// Space toggles through options
		if char == " " {
			f.cycleSelectOption(field)
		}
		return
	}

	// Check max length
	if field.MaxLength > 0 && len(currentValue) >= field.MaxLength {
		return
	}

	// Insert character at cursor position
	newValue := currentValue[:f.inputCursor] + char + currentValue[f.inputCursor:]
	f.values[field.Name] = newValue
	f.inputCursor++

	// Clear error for this field
	delete(f.errors, field.Name)
}

func (f *FormModel) handlePaste(pastedText string) {
	if f.currentStep >= len(f.steps) {
		return
	}

	step := f.steps[f.currentStep]
	if f.currentField >= len(step.Fields) {
		return
	}

	field := step.Fields[f.currentField]
	currentValue := f.values[field.Name]

	// Handle select fields - ignore paste for select fields
	if field.FieldType == FieldTypeSelect {
		return
	}

	// Clean pasted text (remove newlines and control characters for single-line fields)
	cleanText := strings.ReplaceAll(pastedText, "\n", "")
	cleanText = strings.ReplaceAll(cleanText, "\r", "")
	cleanText = strings.ReplaceAll(cleanText, "\t", " ")

	// Check max length constraint
	if field.MaxLength > 0 {
		maxInsertLength := field.MaxLength - len(currentValue)
		if maxInsertLength <= 0 {
			return // Can't insert anything
		}
		if len(cleanText) > maxInsertLength {
			cleanText = cleanText[:maxInsertLength]
		}
	}

	// Insert pasted text at cursor position
	newValue := currentValue[:f.inputCursor] + cleanText + currentValue[f.inputCursor:]
	f.values[field.Name] = newValue
	f.inputCursor += len(cleanText)

	// Clear error for this field
	delete(f.errors, field.Name)
}

func (f *FormModel) handleBackspace() {
	if f.inputCursor > 0 {
		currentValue := f.getCurrentFieldValue()
		newValue := currentValue[:f.inputCursor-1] + currentValue[f.inputCursor:]
		f.setCurrentFieldValue(newValue)
		f.inputCursor--
	}
}

func (f *FormModel) handleDelete() {
	currentValue := f.getCurrentFieldValue()
	if f.inputCursor < len(currentValue) {
		newValue := currentValue[:f.inputCursor] + currentValue[f.inputCursor+1:]
		f.setCurrentFieldValue(newValue)
	}
}

func (f *FormModel) moveCursorLeft() {
	if f.inputCursor > 0 {
		f.inputCursor--
	}
}

func (f *FormModel) moveCursorRight() {
	currentValue := f.getCurrentFieldValue()
	if f.inputCursor < len(currentValue) {
		f.inputCursor++
	}
}

func (f *FormModel) cycleSelectOption(field FormField) {
	if len(field.Options) == 0 {
		return
	}

	currentValue := f.values[field.Name]
	currentIndex := -1

	// Find current option index
	for i, option := range field.Options {
		if option == currentValue {
			currentIndex = i
			break
		}
	}

	// Move to next option
	nextIndex := (currentIndex + 1) % len(field.Options)
	f.values[field.Name] = field.Options[nextIndex]
}

func (f *FormModel) cyclePrevSelectOption() {
	field := f.getCurrentField()
	if field == nil || len(field.Options) == 0 {
		return
	}

	currentValue := f.values[field.Name]
	currentIndex := -1

	// Find current option index
	for i, option := range field.Options {
		if option == currentValue {
			currentIndex = i
			break
		}
	}

	// Move to previous option
	prevIndex := currentIndex - 1
	if prevIndex < 0 {
		prevIndex = len(field.Options) - 1
	}
	f.values[field.Name] = field.Options[prevIndex]
}

func (f *FormModel) isCurrentFieldSelect() bool {
	field := f.getCurrentField()
	return field != nil && field.FieldType == FieldTypeSelect
}

// Utility methods
func (f *FormModel) getCurrentField() *FormField {
	if f.currentStep >= len(f.steps) || f.currentField >= len(f.steps[f.currentStep].Fields) {
		return nil
	}
	return &f.steps[f.currentStep].Fields[f.currentField]
}

func (f *FormModel) getCurrentFieldValue() string {
	field := f.getCurrentField()
	if field == nil {
		return ""
	}
	return f.values[field.Name]
}

func (f *FormModel) setCurrentFieldValue(value string) {
	field := f.getCurrentField()
	if field != nil {
		f.values[field.Name] = value
	}
}

func (f *FormModel) isAtLastField() bool {
	return f.currentStep == len(f.steps)-1 &&
		f.currentField == len(f.steps[f.currentStep].Fields)-1
}

func (f *FormModel) isAtLastFieldOfStep() bool {
	if f.currentStep >= len(f.steps) {
		return false
	}
	return f.currentField == len(f.steps[f.currentStep].Fields)-1
}

// Validation methods
func (f *FormModel) validateCurrentField() bool {
	field := f.getCurrentField()
	if field == nil {
		return true
	}

	value := f.values[field.Name]

	// Check required fields
	if field.Required && strings.TrimSpace(value) == "" {
		f.errors[field.Name] = "This field is required"
		return false
	}

	// Check minimum length
	if field.MinLength > 0 && len(value) < field.MinLength {
		f.errors[field.Name] = fmt.Sprintf("Minimum length is %d characters", field.MinLength)
		return false
	}

	// Run custom validator
	if field.Validator != nil {
		if err := field.Validator(value); err != nil {
			f.errors[field.Name] = err.Error()
			return false
		}
	}

	// Clear any previous error
	delete(f.errors, field.Name)
	return true
}

func (f *FormModel) validateCurrentStep() bool {
	step := f.steps[f.currentStep]
	valid := true

	for _, field := range step.Fields {
		value := f.values[field.Name]

		// Check required fields
		if field.Required && strings.TrimSpace(value) == "" {
			f.errors[field.Name] = "This field is required"
			valid = false
			continue
		}

		// Check minimum length
		if field.MinLength > 0 && len(value) < field.MinLength {
			f.errors[field.Name] = fmt.Sprintf("Minimum length is %d characters", field.MinLength)
			valid = false
			continue
		}

		// Run custom validator
		if field.Validator != nil {
			if err := field.Validator(value); err != nil {
				f.errors[field.Name] = err.Error()
				valid = false
				continue
			}
		}

		// Clear any previous error
		delete(f.errors, field.Name)
	}

	return valid
}

func (f *FormModel) validateAllSteps() bool {
	for i := range f.steps {
		originalStep := f.currentStep
		f.currentStep = i
		if !f.validateCurrentStep() {
			f.currentStep = originalStep
			return false
		}
		f.currentStep = originalStep
	}
	return true
}

// GetValues returns all form values
func (f *FormModel) GetValues() map[string]string {
	return f.values
}

// SetValue sets a form value
func (f *FormModel) SetValue(name, value string) {
	f.values[name] = value
}

// GetValue gets a form value
func (f *FormModel) GetValue(name string) string {
	return f.values[name]
}
