package models

import (
	"github.com/charmbracelet/lipgloss"
)

// Centralized styles for all models to maintain consistency
var (
	// Basic styles
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")). // White
		Padding(0, 1)

	selectedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")). // Bright Green
		Bold(true)

	choiceStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")) // Gray

	infoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // Bright Green
	warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // Yellow
	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("9")). // Red
		Bold(true)

	helpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")). // Gray
		Italic(true)

	// Form-specific styles
	formTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		Padding(0, 1)

	fieldLabelStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("14")) // Cyan

	fieldValueStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")). // White
		Background(lipgloss.Color("0")).  // Black
		Padding(0, 1)

	progressStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")) // Bright Green

	// Input styles
	inputStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("14")). // Cyan
		Background(lipgloss.Color("0"))   // Black

	promptStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("11")). // Yellow
		Bold(true)

	// Status indicator styles
	activeStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")) // Bright Green

	inactiveStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("9")) // Red

	// Menu level styles
	menuTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")). // White
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8"))

	menuItemStyle = lipgloss.NewStyle().
		Padding(0, 2)

	// Processing styles
	processingStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("14")). // Cyan
		Bold(true)

	completedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")). // Bright Green
		Bold(true)
)