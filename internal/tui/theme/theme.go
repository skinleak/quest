// Package theme holds every color, Lip Gloss style and icon used by the
// UI. Change the look of quest here and nowhere else.
package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Palette is the small set of colors the whole UI is built from. Colors
// are ANSI indices, so the terminal's own theme decides the exact shades,
// they work the same in 16-color, 256-color and truecolor terminals, and
// quest matches the rest of the user's CLI. Blue is the one UI accent; the
// other colors carry meaning only (status classes, methods, JSON tokens).
type Palette struct {
	Text    lipgloss.TerminalColor
	Muted   lipgloss.TerminalColor
	Accent  lipgloss.TerminalColor
	Success lipgloss.TerminalColor
	Warning lipgloss.TerminalColor
	Error   lipgloss.TerminalColor
	Info    lipgloss.TerminalColor
	Special lipgloss.TerminalColor
}

// The bright ANSI colors (SGR 90–97).
const (
	brightBlack  = lipgloss.Color("8")
	brightRed    = lipgloss.Color("9")
	brightGreen  = lipgloss.Color("10")
	brightYellow = lipgloss.Color("11")
	brightBlue   = lipgloss.Color("12")
	brightPurple = lipgloss.Color("13")
	brightCyan   = lipgloss.Color("14")
	brightWhite  = lipgloss.Color("15")
	regularBlack = lipgloss.Color("0")
)

// DefaultPalette uses the bright ANSI colors with one accent (blue), like
// qitlog. Text falls back to black on light backgrounds, where bright
// white would be invisible.
func DefaultPalette() Palette {
	return Palette{
		Text:    lipgloss.AdaptiveColor{Light: string(regularBlack), Dark: string(brightWhite)},
		Muted:   brightBlack,
		Accent:  brightBlue,
		Success: brightGreen,
		Warning: brightYellow,
		Error:   brightRed,
		Info:    brightCyan,
		Special: brightPurple,
	}
}

// Icons are the few symbols the UI draws. Plain Unicode only, so no
// special font is needed.
const (
	IconCollapsed = "▸"
	IconExpanded  = "▾"
	IconSep       = "·"
	IconBar       = "━"
	IconEllipsis  = "…"
)

// Styles are the concrete Lip Gloss styles derived from a Palette.
//
// Focus and selection never rely on color alone: the focused pane's title
// and the active tab are also bold, so they stay visible with NO_COLOR.
type Styles struct {
	Brand  lipgloss.Style
	Text   lipgloss.Style
	Muted  lipgloss.Style
	Subtle lipgloss.Style
	Strong lipgloss.Style

	Title      lipgloss.Style // a pane's title
	TitleFocus lipgloss.Style // the focused pane's title
	Cursor     lipgloss.Style

	Tab       lipgloss.Style
	TabActive lipgloss.Style

	Error   lipgloss.Style
	Warning lipgloss.Style
	Info    lipgloss.Style

	HelpKey lipgloss.Style
	HelpDsc lipgloss.Style
	HelpSep lipgloss.Style

	// Status codes by class: 2xx, 3xx, 4xx, 5xx.
	Status2xx lipgloss.Style
	Status3xx lipgloss.Style
	Status4xx lipgloss.Style
	Status5xx lipgloss.Style

	// Methods colored like most API tools; HEAD and OPTIONS are muted.
	MethodGet    lipgloss.Style
	MethodPost   lipgloss.Style
	MethodPut    lipgloss.Style
	MethodPatch  lipgloss.Style
	MethodDelete lipgloss.Style
	MethodOther  lipgloss.Style

	HeaderKey lipgloss.Style // response header names
	TimingBar lipgloss.Style

	// JSON tokens, colored like jq.
	JSONKey     lipgloss.Style
	JSONString  lipgloss.Style
	JSONNumber  lipgloss.Style
	JSONLiteral lipgloss.Style // true, false, null
	JSONPunct   lipgloss.Style
}

// New builds Styles from a Palette.
func New(p Palette) Styles {
	s := lipgloss.NewStyle
	return Styles{
		Brand:  s().Foreground(p.Accent).Bold(true),
		Text:   s().Foreground(p.Text),
		Muted:  s().Foreground(p.Muted),
		Subtle: s().Foreground(p.Muted).Faint(true),
		Strong: s().Foreground(p.Text).Bold(true),

		Title:      s().Foreground(p.Muted),
		TitleFocus: s().Foreground(p.Accent).Bold(true),
		Cursor:     s().Foreground(p.Accent).Bold(true),

		Tab:       s().Foreground(p.Muted),
		TabActive: s().Foreground(p.Text).Bold(true),

		Error:   s().Foreground(p.Error),
		Warning: s().Foreground(p.Warning),
		Info:    s().Foreground(p.Muted),

		HelpKey: s().Foreground(p.Accent),
		HelpDsc: s().Foreground(p.Muted),
		HelpSep: s().Foreground(p.Muted).Faint(true),

		Status2xx: s().Foreground(p.Success).Bold(true),
		Status3xx: s().Foreground(p.Info).Bold(true),
		Status4xx: s().Foreground(p.Warning).Bold(true),
		Status5xx: s().Foreground(p.Error).Bold(true),

		MethodGet:    s().Foreground(p.Success).Bold(true),
		MethodPost:   s().Foreground(p.Warning).Bold(true),
		MethodPut:    s().Foreground(p.Info).Bold(true),
		MethodPatch:  s().Foreground(p.Special).Bold(true),
		MethodDelete: s().Foreground(p.Error).Bold(true),
		MethodOther:  s().Foreground(p.Muted).Bold(true),

		HeaderKey: s().Foreground(p.Muted),
		TimingBar: s().Foreground(p.Accent),

		JSONKey:     s().Foreground(p.Accent),
		JSONString:  s().Foreground(p.Success),
		JSONNumber:  s().Foreground(p.Info),
		JSONLiteral: s().Foreground(p.Special),
		JSONPunct:   s().Foreground(p.Muted),
	}
}

// Default returns the styles for the default palette.
func Default() Styles { return New(DefaultPalette()) }

// Method returns the style for an HTTP method.
func (s Styles) Method(m string) lipgloss.Style {
	switch m {
	case "GET":
		return s.MethodGet
	case "POST":
		return s.MethodPost
	case "PUT":
		return s.MethodPut
	case "PATCH":
		return s.MethodPatch
	case "DELETE":
		return s.MethodDelete
	}
	return s.MethodOther
}

// Status returns the style for an HTTP status code.
func (s Styles) Status(code int) lipgloss.Style {
	switch {
	case code >= 500:
		return s.Status5xx
	case code >= 400:
		return s.Status4xx
	case code >= 300:
		return s.Status3xx
	}
	return s.Status2xx
}

// Paint is a style taken apart into the escape codes around its text, so
// hot paths can color many short runs without a Render call for each: a
// Render per run made qitlog's frames several times slower.
type Paint struct{ open, close string }

// PaintOf renders s once to learn its escape codes.
func PaintOf(s lipgloss.Style) Paint {
	var p Paint
	p.open, p.close, _ = strings.Cut(s.Render("\x00"), "\x00")
	return p
}

// Render wraps text in the paint's escape codes.
func (p Paint) Render(text string) string { return p.open + text + p.close }

// Append writes painted text to b.
func (p Paint) Append(b *strings.Builder, text string) {
	b.WriteString(p.open)
	b.WriteString(text)
	b.WriteString(p.close)
}
