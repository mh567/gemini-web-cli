package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor   = lipgloss.Color("#7C3AED")
	secondaryColor = lipgloss.Color("#06B6D4")
	mutedColor     = lipgloss.Color("#6B7280")
	errorColor     = lipgloss.Color("#EF4444")
	successColor   = lipgloss.Color("#10B981")

	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Padding(0, 1)

	userMsgStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	modelMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB"))

	statusStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Padding(0, 1)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	mutedStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true)
)
