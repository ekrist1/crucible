package tui

import (
	"crucible/internal/logging"
	"crucible/internal/tui/models"
	tea "github.com/charmbracelet/bubbletea"
)

// Re-export types from models package for backward compatibility
type AppState = models.AppState

const (
	StateMenu                   = models.StateMenu
	StateSubmenu                = models.StateSubmenu
	StateInput                  = models.StateInput
	StateProcessing             = models.StateProcessing
	StateLogViewer              = models.StateLogViewer
	StateServiceList            = models.StateServiceList
	StateServiceActions         = models.StateServiceActions
	StateNextJSMenu             = models.StateNextJSMenu
	StateNextJSCreate           = models.StateNextJSCreate
	StateNextJSCreateTextInput  = models.StateNextJSCreateTextInput
	StateNextJSCreateHybrid     = models.StateNextJSCreateHybrid
	StateLaravel                = models.StateLaravel
	StateLaravelCreate          = models.StateLaravelCreate
	StateLaravelCreateTextInput = models.StateLaravelCreateTextInput
	StateLaravelCreateHybrid    = models.StateLaravelCreateHybrid
	StateMonitoring             = models.StateMonitoring
	StateSettings               = models.StateSettings
)

// NewModel creates a new model using the composed architecture
// This maintains backward compatibility with main.go
func NewModel() tea.Model {
	logger, err := logging.NewLogger(logging.DefaultLogPath())
	if err != nil {
		// Fallback logger
		logger = &logging.Logger{}
	}

	app := models.NewAppModel(logger)
	return app
}

// NewMainMenuModel is a legacy alias for NewModel
func NewMainMenuModel() tea.Model {
	return NewModel()
}
