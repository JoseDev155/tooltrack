// Package styles holds the Lip Gloss palette shared by every view.
package styles

import "github.com/charmbracelet/lipgloss"

// Adaptive colours work on light and dark terminals.
var (
	colorAccent = lipgloss.AdaptiveColor{Light: "#5B3FD1", Dark: "#9D8CFF"}
	colorOK     = lipgloss.AdaptiveColor{Light: "#1A7F37", Dark: "#56D364"}
	colorWarn   = lipgloss.AdaptiveColor{Light: "#9A6700", Dark: "#E3B341"}
	colorErr    = lipgloss.AdaptiveColor{Light: "#CF222E", Dark: "#FF7B72"}
	colorMuted  = lipgloss.AdaptiveColor{Light: "#6E7781", Dark: "#8B949E"}
)

var (
	Title    = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	Header   = lipgloss.NewStyle().Bold(true).Underline(true)
	Cursor   = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	Muted    = lipgloss.NewStyle().Foreground(colorMuted)
	Help     = lipgloss.NewStyle().Foreground(colorMuted)
	OK       = lipgloss.NewStyle().Foreground(colorOK)
	Warn     = lipgloss.NewStyle().Foreground(colorWarn)
	Err      = lipgloss.NewStyle().Foreground(colorErr)
	Selected = lipgloss.NewStyle().Bold(true)
	Frame    = lipgloss.NewStyle().Padding(1, 2)
)
