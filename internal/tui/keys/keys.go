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

	// On the method row.
	PrevMethod key.Binding
	NextMethod key.Binding
	EditURL    key.Binding

	// On the request's params, headers and body.
	Edit   key.Binding
	Add    key.Binding
	Delete key.Binding
	Toggle key.Binding
	Done   key.Binding

	// In the endpoint list, where printable keys filter.
	Endpoints key.Binding
	Choose    key.Binding
	PickUp    key.Binding
	PickDown  key.Binding
	CloseList key.Binding

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
func Default() *KeyMap {
	return &KeyMap{
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
			key.WithHelp("ctrl+t", "cycle method"),
		),
		PrevMethod: key.NewBinding(
			key.WithKeys("h", "left"),
			key.WithHelp("←/h", "previous method"),
		),
		NextMethod: key.NewBinding(
			key.WithKeys("l", "right"),
			key.WithHelp("→/l", "next method"),
		),
		EditURL: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "edit url"),
		),
		Edit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "edit"),
		),
		Add: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d", "delete"),
			key.WithHelp("d", "delete"),
		),
		Toggle: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "on/off"),
		),
		Done: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "done editing"),
		),
		Endpoints: key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "endpoints"),
		),
		Choose: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "load endpoint"),
		),
		PickUp: key.NewBinding(
			key.WithKeys("up", "ctrl+k"),
			key.WithHelp("↑", "previous endpoint"),
		),
		PickDown: key.NewBinding(
			key.WithKeys("down", "ctrl+j"),
			key.WithHelp("↓", "next endpoint"),
		),
		CloseList: key.NewBinding(
			key.WithKeys("esc", "ctrl+p"),
			key.WithHelp("esc", "close list"),
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
// narrow, hints are dropped from the right, except the last two. Keys
// whose action is plain from their name (arrows, page up and down) are
// left to the help overlay.

// URLHelp is the hint while the URL input has focus.
func (k *KeyMap) URLHelp() []key.Binding {
	return []key.Binding{k.Submit, k.Endpoints, k.Method, k.NextPane, k.InputHelp, k.InputQuit}
}

// MethodHelp is the hint while the method row has focus.
func (k *KeyMap) MethodHelp() []key.Binding {
	return []key.Binding{k.NextMethod, k.EditURL, k.Send, k.Endpoints, k.Help, k.Quit}
}

// ListHelp is the hint on the params and headers tabs.
func (k *KeyMap) ListHelp() []key.Binding {
	return []key.Binding{k.Edit, k.Add, k.Toggle, k.Delete, k.NextTab, k.Send, k.Help, k.Quit}
}

// BodyHelp is the hint on the body tab.
func (k *KeyMap) BodyHelp() []key.Binding {
	return []key.Binding{k.Edit, k.NextTab, k.Send, k.Help, k.Quit}
}

// EditingHelp is the hint while a row or the body is being edited.
func (k *KeyMap) EditingHelp() []key.Binding {
	return []key.Binding{k.Done, k.Send, k.NextPane, k.InputHelp, k.InputQuit}
}

// ResponseHelp is the hint while the response pane has focus.
func (k *KeyMap) ResponseHelp() []key.Binding {
	return []key.Binding{k.Send, k.NextTab, k.NextPane, k.Endpoints, k.Help, k.Quit}
}

// PickerHelp is the hint while the endpoint list is open.
func (k *KeyMap) PickerHelp() []key.Binding {
	return []key.Binding{k.Choose, k.CloseList, k.InputHelp, k.InputQuit}
}

// SendingHelp is the hint while a request is in flight.
func (k *KeyMap) SendingHelp() []key.Binding {
	return []key.Binding{k.Cancel}
}

// FullHelp is the grouped list shown in the help overlay.
func (k *KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Send, k.Submit, k.Cancel, k.NextPane, k.PrevPane},
		{k.Method, k.PrevMethod, k.NextMethod, k.EditURL},
		{k.Edit, k.Add, k.Delete, k.Toggle, k.Done},
		{k.Endpoints, k.Choose, k.PickUp, k.PickDown, k.CloseList},
		{k.Up, k.Down, k.PageUp, k.PageDown, k.Top, k.Bottom, k.PrevTab, k.NextTab},
		{k.Help, k.InputHelp, k.Quit, k.InputQuit},
	}
}

// TextKeyMap holds the editing keys of the multi-line body editor. They are
// the usual text-editing keys, like those of the URL input, so the help
// overlay leaves them out.
type TextKeyMap struct {
	Left, Right, Up, Down      key.Binding
	LineStart, LineEnd         key.Binding
	PageUp, PageDown           key.Binding
	Top, Bottom                key.Binding
	Newline                    key.Binding
	Backspace, DeleteChar      key.Binding
	DeleteWord                 key.Binding
	DeleteToStart, DeleteToEnd key.Binding
}

// DefaultText returns the default editing keys.
func DefaultText() TextKeyMap {
	return TextKeyMap{
		Left:          key.NewBinding(key.WithKeys("left", "ctrl+b")),
		Right:         key.NewBinding(key.WithKeys("right", "ctrl+f")),
		Up:            key.NewBinding(key.WithKeys("up")),
		Down:          key.NewBinding(key.WithKeys("down")),
		LineStart:     key.NewBinding(key.WithKeys("home", "ctrl+a")),
		LineEnd:       key.NewBinding(key.WithKeys("end", "ctrl+e")),
		PageUp:        key.NewBinding(key.WithKeys("pgup")),
		PageDown:      key.NewBinding(key.WithKeys("pgdown")),
		Top:           key.NewBinding(key.WithKeys("ctrl+home")),
		Bottom:        key.NewBinding(key.WithKeys("ctrl+end")),
		Newline:       key.NewBinding(key.WithKeys("enter")),
		Backspace:     key.NewBinding(key.WithKeys("backspace", "ctrl+h")),
		DeleteChar:    key.NewBinding(key.WithKeys("delete", "ctrl+d")),
		DeleteWord:    key.NewBinding(key.WithKeys("ctrl+w", "alt+backspace")),
		DeleteToStart: key.NewBinding(key.WithKeys("ctrl+u")),
		DeleteToEnd:   key.NewBinding(key.WithKeys("ctrl+k")),
	}
}
