package main

import (
	"fmt"
	"os"

	"crucible/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

// Build information - can be set via ldflags
var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	model := tui.NewModel()
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}
