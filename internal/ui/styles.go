package ui

import "charm.land/lipgloss/v2"

// Palette. Color is layered on top of text markers and is never the only
// carrier of meaning: warning and exhausted rows always show "⚠ WARN" /
// "✗ FULL" in plain text, so the output reads correctly when piped or run
// under NO_COLOR.
var (
	colorHealthy   = lipgloss.Color("#7aa87b")
	colorWarning   = lipgloss.Color("#d8a657")
	colorExhausted = lipgloss.Color("#d9635f")
	colorMuted     = lipgloss.Color("#6e7681")

	styleProvider  = lipgloss.NewStyle().Bold(true)
	styleMuted     = lipgloss.NewStyle().Foreground(colorMuted)
	styleHealthy   = lipgloss.NewStyle().Foreground(colorHealthy)
	styleWarning   = lipgloss.NewStyle().Foreground(colorWarning)
	styleExhausted = lipgloss.NewStyle().Foreground(colorExhausted)
)
