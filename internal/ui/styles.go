// © Antony Monge López — Costa Rica — Céd. 604700548
package ui

import (
	"charm.land/lipgloss/v2"
)

// Semantic colors (alineado con backup-agent)
var (
	ColorGreen         = lipgloss.Color("#4ade80") // éxito (emerald)
	ColorYellow        = lipgloss.Color("#fbbf24") // advertencia / pendiente (amber)
	ColorRed           = lipgloss.Color("#f87171") // error (coral)
	ColorBlue          = lipgloss.Color("#38bdf8") // información / primario (sky)
	ColorIndigo        = lipgloss.Color("#818cf8") // secundario de acento (indigo)
	ColorGray          = lipgloss.Color("#94a3b8") // secundario / muted (slate-400)
	ColorDarkGray      = lipgloss.Color("#334155") // bordes secundarios (slate-700)
	ColorWhite         = lipgloss.Color("#f8fafc") // texto principal (slate-50)
	ColorBgCard        = lipgloss.Color("#1e293b") // fondo panel (slate-800)
	ColorBadgeGreenBg  = lipgloss.Color("#14532d")
	ColorBadgeYellowBg = lipgloss.Color("#713f12")
	ColorBadgeRedBg    = lipgloss.Color("#7f1d1d")
	ColorBadgeBlueBg   = lipgloss.Color("#0c4a6e")
	ColorBadgeMutedBg  = lipgloss.Color("#1e293b")
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

	// Estilos modulares inspirados en backup-agent
	Panel        lipgloss.Style
	PanelActive  lipgloss.Style
	CardHeader   lipgloss.Style
	BadgeSuccess lipgloss.Style
	BadgeWarning lipgloss.Style
	BadgeError   lipgloss.Style
	BadgeInfo    lipgloss.Style
	BadgeMuted   lipgloss.Style
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

		Panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorDarkGray).
			Padding(0, 1),
		PanelActive: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBlue).
			Padding(0, 1),
		CardHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorBlue),
		BadgeSuccess: lipgloss.NewStyle().
			Foreground(ColorGreen).
			Background(ColorBadgeGreenBg).
			Bold(true).
			Padding(0, 1),
		BadgeWarning: lipgloss.NewStyle().
			Foreground(ColorYellow).
			Background(ColorBadgeYellowBg).
			Bold(true).
			Padding(0, 1),
		BadgeError: lipgloss.NewStyle().
			Foreground(ColorRed).
			Background(ColorBadgeRedBg).
			Bold(true).
			Padding(0, 1),
		BadgeInfo: lipgloss.NewStyle().
			Foreground(ColorBlue).
			Background(ColorBadgeBlueBg).
			Bold(true).
			Padding(0, 1),
		BadgeMuted: lipgloss.NewStyle().
			Foreground(ColorGray).
			Background(ColorBadgeMutedBg).
			Padding(0, 1),
	}
}
