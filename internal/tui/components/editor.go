package components

import (
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/skinleak/quest/internal/core/request"
	"github.com/skinleak/quest/internal/tui/theme"
)

// EditorHeight is the editor's height in lines: a title, the method row
// and the URL.
const EditorHeight = 3

// EditorFocus is the part of the editor that has the keyboard.
type EditorFocus int

// Editor parts.
const (
	EditorBlurred EditorFocus = iota
	EditorMethod
	EditorURL
)

// Method label variants, rendered once in NewEditor.
const (
	labelIdle = iota
	labelActive
	labelPicking // active, while the method row has focus
	labelCount
)

// Editor edits a request: its method and URL. Every method is shown in a
// row with the active one in its color, so the choice is always in view.
type Editor struct {
	method int // index into request.Methods
	url    textinput.Model
	labels [labelCount][]string
}

// NewEditor returns an editor for an empty GET request.
func NewEditor(st theme.Styles) Editor {
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = "https://api.example.com/users"
	in.PlaceholderStyle = st.Subtle
	in.TextStyle = st.Text
	in.Cursor.Style = st.Cursor
	// A static cursor is calmer and avoids a redraw every blink.
	in.Cursor.SetMode(cursor.CursorStatic)
	e := Editor{url: in}
	for v := range e.labels {
		e.labels[v] = make([]string, len(request.Methods))
	}
	for i, m := range request.Methods {
		e.labels[labelIdle][i] = st.Tab.Render(m)
		e.labels[labelActive][i] = st.Method(m).Render(m)
		e.labels[labelPicking][i] = st.MethodPicked(m).Render(m)
	}
	return e
}

// Focus gives the URL input the keyboard.
func (e *Editor) Focus() tea.Cmd { return e.url.Focus() }

// Blur takes the keyboard away from the URL input.
func (e *Editor) Blur() { e.url.Blur() }

// Update passes a message to the URL input.
func (e *Editor) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	e.url, cmd = e.url.Update(msg)
	return cmd
}

// MoveMethod picks the method delta steps away in the row, wrapping
// around.
func (e *Editor) MoveMethod(delta int) {
	n := len(request.Methods)
	e.method = (e.method + delta%n + n) % n
}

// SetRequest loads r into the editor.
func (e *Editor) SetRequest(r request.Request) {
	e.method = max(0, slices.Index(request.Methods, r.Method))
	e.url.SetValue(r.URL)
	e.url.CursorEnd()
}

// Request is the request being edited.
func (e *Editor) Request() request.Request {
	return request.Request{Method: request.Methods[e.method], URL: e.url.Value()}
}

// SetWidth sizes the URL input for a pane width cells wide. textinput
// draws one cell more than its Width, for the cursor.
func (e *Editor) SetWidth(width int) {
	e.url.Width = max(1, width-1)
}

// View renders the title, the method row and the URL.
func (e *Editor) View(st theme.Styles, width int, focus EditorFocus) []string {
	return []string{
		Title(st, "request", focus != EditorBlurred),
		e.methodRow(width, focus == EditorMethod),
		Fit(e.url.View(), width, false),
	}
}

// methodRow lists every method, the active one in its color. It closes
// up when the row is too wide and shows only the active method when even
// that doesn't fit.
func (e *Editor) methodRow(width int, picking bool) string {
	active := labelActive
	if picking {
		active = labelPicking
	}
	names := 0
	for _, m := range request.Methods {
		names += len(m)
	}
	gaps := len(request.Methods) - 1
	var sep string
	switch {
	case names+2*gaps <= width:
		sep = "  "
	case names+gaps <= width:
		sep = " "
	default:
		return Fit(e.labels[active][e.method], width, false)
	}
	var b strings.Builder
	for i := range request.Methods {
		if i > 0 {
			b.WriteString(sep)
		}
		v := labelIdle
		if i == e.method {
			v = active
		}
		b.WriteString(e.labels[v][i])
	}
	return b.String()
}

// Title renders a pane title, bold in the accent color when focused.
func Title(st theme.Styles, title string, focused bool) string {
	if focused {
		return st.TitleFocus.Render(title)
	}
	return st.Title.Render(title)
}
