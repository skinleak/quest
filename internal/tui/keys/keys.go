// Package keys defines every keybinding in one place. The status bar hints
// and the help overlay are generated from these bindings, so a binding
// added here is documented automatically once it is listed in FullHelp.
package keys

import "github.com/charmbracelet/bubbles/key"

// KeyMap holds all bindings.
//
// While the URL input has focus, printable keys are text, so only bindings
// with a modifier (and tab, enter, esc, f1) act there. Submit, InputHelp
// and InputQuit are the forms of Send, Help and Quit shown in that state.
type KeyMap struct {
	NextPane key.Binding
	PrevPane key.Binding
	Send     key.Binding
	Submit   key.Binding
	Cancel   key.Binding
	Method   key.Binding
	Sidebar  key.Binding

	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Top      key.Binding
	Bottom   key.Binding
	PrevTab  key.Binding
	NextTab  key.Binding

	Help      key.Binding
	InputHelp key.Binding
	Close     key.Binding
	Quit      key.Binding
	InputQuit key.Binding
}

// Default returns the default bindings.
func Default() KeyMap {
	return KeyMap{
		NextPane: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next pane"),
		),
		PrevPane: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "previous pane"),
		),
		Send: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "send"),
		),
		Submit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "send"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc", "ctrl+c"),
			key.WithHelp("esc", "cancel request"),
		),
		Method: key.NewBinding(
			key.WithKeys("ctrl+t"),
			key.WithHelp("ctrl+t", "next method"),
		),
		Sidebar: key.NewBinding(
			key.WithKeys("ctrl+b"),
			key.WithHelp("ctrl+b", "sidebar"),
		),
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("↓/j", "down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("ctrl+u", "pgup"),
			key.WithHelp("ctrl+u", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("ctrl+d", "pgdown"),
			key.WithHelp("ctrl+d", "page down"),
		),
		Top: key.NewBinding(
			key.WithKeys("g", "home"),
			key.WithHelp("g", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G", "end"),
			key.WithHelp("G", "bottom"),
		),
		PrevTab: key.NewBinding(
			key.WithKeys("h", "left"),
			key.WithHelp("←/h", "previous tab"),
		),
		NextTab: key.NewBinding(
			key.WithKeys("l", "right"),
			key.WithHelp("→/l", "next tab"),
		),
		Help: key.NewBinding(
			key.WithKeys("?", "f1"),
			key.WithHelp("?", "help"),
		),
		InputHelp: key.NewBinding(
			key.WithKeys("f1"),
			key.WithHelp("f1", "help"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc", "?", "q", "f1"),
			key.WithHelp("esc", "close"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		InputQuit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
	}
}

// The hints below are ordered by importance: when the terminal is too
// narrow, hints are dropped from the right, except the last two.

// URLHelp is the hint while the URL input has focus.
func (k KeyMap) URLHelp() []key.Binding {
	return []key.Binding{k.Submit, k.Method, k.NextPane, k.Sidebar, k.InputHelp, k.InputQuit}
}

// ResponseHelp is the hint while the response pane has focus.
func (k KeyMap) ResponseHelp() []key.Binding {
	return []key.Binding{k.Send, k.NextTab, k.Down, k.Up, k.PageDown, k.PageUp, k.NextPane, k.Help, k.Quit}
}

// SidebarHelp is the hint while the sidebar has focus.
func (k KeyMap) SidebarHelp() []key.Binding {
	return []key.Binding{k.Send, k.NextPane, k.Sidebar, k.Help, k.Quit}
}

// SendingHelp is the hint while a request is in flight.
func (k KeyMap) SendingHelp() []key.Binding {
	return []key.Binding{k.Cancel}
}

// FullHelp is the grouped list shown in the help overlay.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Send, k.Submit, k.Cancel, k.Method, k.NextPane, k.PrevPane, k.Sidebar},
		{k.Up, k.Down, k.PageUp, k.PageDown, k.Top, k.Bottom, k.PrevTab, k.NextTab},
		{k.Help, k.InputHelp, k.Quit, k.InputQuit},
	}
}
