// Package app is the root of the TUI: it owns the panes, routes keys,
// lays out the frame and runs requests as tea.Cmds.
package app

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/skinleak/quest/internal/core/engine"
	"github.com/skinleak/quest/internal/tui/components"
	"github.com/skinleak/quest/internal/tui/format"
	"github.com/skinleak/quest/internal/tui/keys"
	"github.com/skinleak/quest/internal/tui/theme"
)

// Layout constants, in cells.
const (
	padX         = 2  // left and right margin of the frame
	chromeHeight = 5  // blank, header, blank above the body; blank, footer below
	minWidth     = 40 // below this, only a "too small" note is shown
	minHeight    = 10
	// maxArea caps the lines of params, headers or body shown, so the
	// response keeps most of a tall window.
	maxArea = 12
)

// pane is a focus stop; tab moves through them in this order.
type pane int

const (
	paneMethod pane = iota
	paneURL
	paneRequest // the params, headers and body
	paneResponse
	paneCount
)

// Model is the root Bubble Tea model.
type Model struct {
	engine *engine.Engine
	log    *slog.Logger
	styles *theme.Styles
	keys   *keys.KeyMap

	width, height int
	focus         pane
	showHelp      bool

	editor   components.Editor
	response components.Response
	help     components.Help

	seq    int                // id of the latest request; older results are dropped
	cancel context.CancelFunc // cancels the request in flight, if any

	message string
	isErr   bool
}

// New returns the root model. Requests go through eng; log receives
// internal diagnostics and never writes to the terminal.
func New(eng *engine.Engine, log *slog.Logger) Model {
	st := theme.Default()
	m := Model{
		engine: eng,
		log:    log,
		styles: st,
		keys:   keys.Default(),
		focus:  paneURL,
		editor: components.NewEditor(st),
		help:   components.NewHelp(st),
	}
	m.editor.Focus()
	return m
}

// Init implements tea.Model. Nothing is loaded at startup, so the first
// frame is drawn at once.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil
	case responseMsg:
		m.applyResponse(msg)
		return m, nil
	case tea.KeyMsg:
		m, cmd := m.handleKey(msg)
		// Keys can add or remove rows, which moves the response.
		m.resize()
		return m, cmd
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	m.message, m.isErr = "", false
	k := m.keys

	if m.showHelp {
		switch {
		case key.Matches(msg, k.InputQuit):
			return m, m.quit()
		case key.Matches(msg, k.Close):
			m.showHelp = false
		case key.Matches(msg, k.Down):
			m.help.Scroll(m.styles, k.FullHelp(), 1, m.innerWidth(), m.bodyHeight())
		case key.Matches(msg, k.Up):
			m.help.Scroll(m.styles, k.FullHelp(), -1, m.innerWidth(), m.bodyHeight())
		}
		return m, nil
	}

	if m.response.Sending() && key.Matches(msg, k.Cancel) {
		m.cancelRequest()
		return m, nil
	}

	// Bindings with a modifier work everywhere, even while typing.
	switch {
	case key.Matches(msg, k.InputQuit):
		return m, m.quit()
	case key.Matches(msg, k.Send):
		return m, m.send()
	case key.Matches(msg, k.Method):
		m.editor.MoveMethod(1)
		return m, nil
	case key.Matches(msg, k.NextPane):
		return m, m.setFocus((m.focus + 1) % paneCount)
	case key.Matches(msg, k.PrevPane):
		return m, m.setFocus((m.focus + paneCount - 1) % paneCount)
	case key.Matches(msg, k.InputHelp):
		m.openHelp()
		return m, nil
	}

	if m.focus == paneURL {
		if key.Matches(msg, k.Submit) {
			return m, m.send()
		}
		return m, m.editor.Update(msg)
	}
	if m.editor.Editing() {
		if key.Matches(msg, k.Done) || (!m.editor.EditingBody() && key.Matches(msg, k.Submit)) {
			m.editor.StopEdit()
			return m, nil
		}
		return m, m.editor.UpdateEditing(msg)
	}

	switch {
	case key.Matches(msg, k.Quit):
		return m, m.quit()
	case key.Matches(msg, k.Help):
		m.openHelp()
	case m.focus == paneMethod:
		return m, m.handleMethodKey(msg)
	case m.focus == paneRequest:
		return m, m.handleRequestKey(msg)
	case m.focus == paneResponse:
		m.handleResponseKey(msg)
	}
	return m, nil
}

func (m *Model) handleMethodKey(msg tea.KeyMsg) tea.Cmd {
	k := m.keys
	switch {
	case key.Matches(msg, k.PrevMethod):
		m.editor.MoveMethod(-1)
	case key.Matches(msg, k.NextMethod):
		m.editor.MoveMethod(1)
	case key.Matches(msg, k.EditURL):
		return m.setFocus(paneURL)
	}
	return nil
}

// handleRequestKey works the params, headers and body tabs.
func (m *Model) handleRequestKey(msg tea.KeyMsg) tea.Cmd {
	k, e := m.keys, &m.editor
	switch {
	case key.Matches(msg, k.NextTab):
		e.MoveTab(1)
	case key.Matches(msg, k.PrevTab):
		e.MoveTab(-1)
	case key.Matches(msg, k.Down):
		e.MoveRow(1)
	case key.Matches(msg, k.Up):
		e.MoveRow(-1)
	case key.Matches(msg, k.Edit):
		return e.Edit()
	case key.Matches(msg, k.Add):
		return e.Add()
	case key.Matches(msg, k.Delete):
		e.Delete()
	case key.Matches(msg, k.Toggle):
		e.Toggle()
	}
	return nil
}

func (m *Model) handleResponseKey(msg tea.KeyMsg) {
	k, r := m.keys, &m.response
	switch {
	case key.Matches(msg, k.Down):
		r.Scroll(1)
	case key.Matches(msg, k.Up):
		r.Scroll(-1)
	case key.Matches(msg, k.PageDown):
		r.Page(1)
	case key.Matches(msg, k.PageUp):
		r.Page(-1)
	case key.Matches(msg, k.Top):
		r.Top()
	case key.Matches(msg, k.Bottom):
		r.Bottom()
	case key.Matches(msg, k.NextTab):
		r.MoveTab(1)
	case key.Matches(msg, k.PrevTab):
		r.MoveTab(-1)
	}
}

// send validates the edited request and starts it, replacing any request
// still in flight.
func (m *Model) send() tea.Cmd {
	r := m.editor.Request()
	if err := r.Validate(); err != nil {
		m.setMessage("", err)
		return nil
	}
	if m.cancel != nil {
		m.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.seq++
	m.response.SetSending()
	return send(ctx, m.engine, m.log, m.styles, m.seq, r)
}

func (m *Model) cancelRequest() {
	if m.cancel != nil {
		m.cancel()
	}
}

func (m *Model) applyResponse(msg responseMsg) {
	if msg.seq != m.seq {
		return
	}
	m.cancel()
	m.cancel = nil
	m.response.SetResult(m.styles, msg.resp, msg.doc, msg.err)
	if errors.Is(msg.err, context.Canceled) {
		m.setMessage("request cancelled", nil)
	}
}

func (m *Model) quit() tea.Cmd {
	m.cancelRequest()
	return tea.Quit
}

func (m *Model) openHelp() {
	m.showHelp = true
	m.help.Reset()
}

func (m *Model) setFocus(p pane) tea.Cmd {
	m.editor.StopEdit()
	m.focus = p
	if p == paneURL {
		return m.editor.Focus()
	}
	m.editor.Blur()
	return nil
}

func (m *Model) setMessage(info string, err error) {
	if err != nil {
		m.message, m.isErr = format.Sanitize(err.Error()), true
		return
	}
	m.message, m.isErr = info, false
}

func (m Model) innerWidth() int { return max(1, m.width-2*padX) }
func (m Model) bodyHeight() int { return max(1, m.height-chromeHeight) }
func (m Model) tooSmall() bool  { return m.width < minWidth || m.height < minHeight }

// resize sizes the panes for the current window. The request's tab
// content gets the lines it needs, up to two fifths of the room left
// after the fixed lines, and the response gets the rest.
func (m *Model) resize() {
	w := m.innerWidth()
	m.editor.SetWidth(w)
	// The editor, a blank line, its tab content, a blank line, then at
	// least the response's title.
	room := m.bodyHeight() - components.EditorHeight - 2
	m.editor.SetArea(max(0, min(m.editor.ContentHeight(), maxArea, room*2/5)))
	m.response.SetSize(w, max(0, m.bodyHeight()-m.editor.Height()-1))
}

// View implements tea.Model. The frame is assembled line by line from
// parts already cut to their width: Lip Gloss's Width, Height and Join
// measure every cell again and made qitlog's frames 3× slower. Lines are
// not padded on the right; the renderer clears the rest of each.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	if m.tooSmall() {
		return m.tooSmallView()
	}
	bodyH := m.bodyHeight()
	var body []string
	if m.showHelp {
		body = m.help.Overlay(m.styles, m.keys.FullHelp(), m.innerWidth(), bodyH)
	} else {
		body = m.bodyLines()
	}

	margin := strings.Repeat(" ", padX)
	var b strings.Builder
	b.Grow(m.width * m.height)
	line := func(l string) {
		if l != "" {
			b.WriteString(margin)
			b.WriteString(l)
		}
		b.WriteByte('\n')
	}
	line("")
	line(m.headerView())
	line("")
	for i := range bodyH {
		if i < len(body) {
			line(body[i])
		} else {
			line("")
		}
	}
	line("")
	b.WriteString(margin)
	b.WriteString(m.footerView())
	return b.String()
}

// bodyLines renders the request editor above the response.
func (m Model) bodyLines() []string {
	w := m.innerWidth()
	focus := components.EditorBlurred
	switch m.focus {
	case paneMethod:
		focus = components.EditorMethod
	case paneURL:
		focus = components.EditorURL
	case paneRequest:
		focus = components.EditorTabs
	}
	out := m.editor.View(m.styles, w, focus)
	out = append(out, "")
	return append(out, m.response.View(m.styles, w, m.focus == paneResponse)...)
}

func (m Model) headerView() string {
	return m.styles.Brand.Render("quest")
}

func (m Model) footerView() string {
	w := m.innerWidth()
	switch {
	case m.message != "" && m.isErr:
		return components.Fit(m.styles.Error.Render(m.message), w, false)
	case m.message != "":
		return components.Fit(m.styles.Info.Render(m.message), w, false)
	case m.response.Sending():
		return m.help.Hints(m.keys.SendingHelp(), w)
	}
	switch m.focus {
	case paneMethod:
		return m.help.Hints(m.keys.MethodHelp(), w)
	case paneURL:
		return m.help.Hints(m.keys.URLHelp(), w)
	case paneRequest:
		switch {
		case m.editor.Editing():
			return m.help.Hints(m.keys.EditingHelp(), w)
		case m.editor.Tab() == components.ReqBody:
			return m.help.Hints(m.keys.BodyHelp(), w)
		}
		return m.help.Hints(m.keys.ListHelp(), w)
	}
	return m.help.Hints(m.keys.ResponseHelp(), w)
}

// tooSmallView replaces the frame in a window below the minimum size.
func (m Model) tooSmallView() string {
	lines := []string{
		m.styles.Strong.Render("window too small"),
		m.styles.Muted.Render(strconv.Itoa(m.width) + "×" + strconv.Itoa(m.height) + ", needs " +
			strconv.Itoa(minWidth) + "×" + strconv.Itoa(minHeight)),
	}
	out := make([]string, 0, m.height)
	for range max(0, (m.height-len(lines))/2) {
		out = append(out, "")
	}
	for _, l := range lines {
		pad := max(0, (m.width-ansi.StringWidth(l))/2)
		out = append(out, ansi.Truncate(strings.Repeat(" ", pad)+l, m.width, ""))
	}
	for len(out) < m.height {
		out = append(out, "")
	}
	return strings.Join(out[:m.height], "\n")
}
