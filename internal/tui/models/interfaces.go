package models

import tea "github.com/charmbracelet/bubbletea"

// BaseModel provides common functionality for all models
type BaseModel struct {
	shared *SharedData
}

// NewBaseModel creates a new base model
func NewBaseModel(shared *SharedData) BaseModel {
	return BaseModel{shared: shared}
}

// GetShared returns the shared data
func (b *BaseModel) GetShared() *SharedData {
	return b.shared
}

// NavigateToMsg creates a navigation message
func (b *BaseModel) NavigateTo(state AppState, data interface{}) tea.Cmd {
	return func() tea.Msg {
		return NavigationMsg{State: state, Data: data}
	}
}

// GoBackMsg creates a back navigation message
func (b *BaseModel) GoBack() tea.Cmd {
	return func() tea.Msg {
		return BackNavigationMsg{}
	}
}

// All models are now implemented in their respective files:
// - MenuModel: menu.go
// - FormModel: form.go
// - NextJSModel & NextJSFormModel: nextjs.go
// - LaravelModel & LaravelFormModel: laravel.go
// - MonitoringModel: monitoring.go
// - ServiceModel: service.go
// - ProcessingModel: processing.go
// - LogViewerModel: logviewer.go
