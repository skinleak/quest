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

// EditorHeight is the editor's height in lines: a title and the request
// line.
const EditorHeight = 2

// methodWidth fits the longest method, OPTIONS, so the URL never shifts.
const methodWidth = 7

// Editor edits a request: its method and URL.
type Editor struct {
	method int // index into request.Methods
	url    textinput.Model
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
	return Editor{url: in}
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

// NextMethod cycles to the next method.
func (e *Editor) NextMethod() {
	e.method = (e.method + 1) % len(request.Methods)
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
	e.url.Width = max(1, width-methodWidth-2-1)
}

// View renders the title and the request line.
func (e *Editor) View(st theme.Styles, width int, focused bool) []string {
	m := request.Methods[e.method]
	method := st.Method(m).Render(m) + strings.Repeat(" ", methodWidth-len(m)+2)
	return []string{
		Title(st, "request", focused),
		Fit(method+e.url.View(), width, false),
	}
}

// Title renders a pane title, bold in the accent color when focused.
func Title(st theme.Styles, title string, focused bool) string {
	if focused {
		return st.TitleFocus.Render(title)
	}
	return st.Title.Render(title)
}
