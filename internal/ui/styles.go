// © Antony Monge López — Costa Rica — Céd. 604700548
package ui

import (
	"charm.land/lipgloss/v2"
)

// Paleta semántica (igual que backup-agent).
var (
	ColorGreen    = lipgloss.Color("#2ecc71")
	ColorYellow   = lipgloss.Color("#f1c40f")
	ColorRed      = lipgloss.Color("#e74c3c")
	ColorBlue     = lipgloss.Color("#3498db")
	ColorGray     = lipgloss.Color("#7f8c8d")
	ColorDarkGray = lipgloss.Color("#34495e")
	ColorWhite    = lipgloss.Color("#ecf0f1")
)

// Styles centraliza los estilos Lip Gloss del TUI.
type Styles struct {
	AppTitle      lipgloss.Style
	Subtitle      lipgloss.Style
	Box           lipgloss.Style
	SectionHeader lipgloss.Style
	Label         lipgloss.Style
	Value         lipgloss.Style
	Success       lipgloss.Style
	Warning       lipgloss.Style
	Error         lipgloss.Style
	Info          lipgloss.Style
	Muted         lipgloss.Style
	Key           lipgloss.Style
	Desc          lipgloss.Style
	HelpBar       lipgloss.Style
	Spinner       lipgloss.Style
	Cursor        lipgloss.Style
}

// DefaultStyles inicializa la paleta estándar.
func DefaultStyles() Styles {
	return Styles{
		AppTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWhite).
			Background(ColorBlue).
			Padding(0, 1),
		Subtitle: lipgloss.NewStyle().
			Foreground(ColorGray).
			Italic(true),
		Box: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBlue).
			Padding(0, 1),
		SectionHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorBlue).
			MarginTop(1).
			MarginBottom(0),
		Label: lipgloss.NewStyle().
			Foreground(ColorWhite).
			Bold(true).
			Width(16),
		Value: lipgloss.NewStyle().
			Foreground(ColorWhite),
		Success: lipgloss.NewStyle().
			Foreground(ColorGreen).
			Bold(true),
		Warning: lipgloss.NewStyle().
			Foreground(ColorYellow).
			Bold(true),
		Error: lipgloss.NewStyle().
			Foreground(ColorRed).
			Bold(true),
		Info: lipgloss.NewStyle().
			Foreground(ColorBlue),
		Muted: lipgloss.NewStyle().
			Foreground(ColorGray),
		Key: lipgloss.NewStyle().
			Foreground(ColorBlue).
			Bold(true),
		Desc: lipgloss.NewStyle().
			Foreground(ColorWhite),
		HelpBar: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorDarkGray).
			Padding(0, 1).
			MarginTop(1),
		Spinner: lipgloss.NewStyle().
			Foreground(ColorBlue),
		Cursor: lipgloss.NewStyle().
			Foreground(ColorBlue).
			Bold(true),
	}
}
