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
	paneGap      = 3  // between side-by-side panes
	chromeHeight = 5  // blank, header, blank above the body; blank, footer below
	minWidth     = 40 // below this, only a "too small" note is shown
	minHeight    = 10
	wideWidth    = 80 // inner width from which the sidebar sits beside the panes
	minSidebar   = 22
	maxSidebar   = 34
)

type pane int

const (
	paneSidebar pane = iota
	paneRequest
	paneResponse
	paneCount
)

// Model is the root Bubble Tea model.
type Model struct {
	engine *engine.Engine
	log    *slog.Logger
	styles theme.Styles
	keys   keys.KeyMap

	width, height int
	focus         pane
	sidebarOn     bool
	showHelp      bool

	sidebar  components.Sidebar
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
		engine:    eng,
		log:       log,
		styles:    st,
		keys:      keys.Default(),
		focus:     paneRequest,
		sidebarOn: true,
		editor:    components.NewEditor(st),
		help:      components.NewHelp(st),
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
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		m.editor.NextMethod()
		return m, nil
	case key.Matches(msg, k.NextPane):
		return m, m.cycleFocus(1)
	case key.Matches(msg, k.PrevPane):
		return m, m.cycleFocus(-1)
	case key.Matches(msg, k.Sidebar):
		return m, m.toggleSidebar()
	case key.Matches(msg, k.InputHelp):
		m.openHelp()
		return m, nil
	}

	if m.focus == paneRequest {
		if key.Matches(msg, k.Submit) {
			return m, m.send()
		}
		return m, m.editor.Update(msg)
	}

	switch {
	case key.Matches(msg, k.Quit):
		return m, m.quit()
	case key.Matches(msg, k.Help):
		m.openHelp()
	case m.focus == paneResponse:
		m.handleResponseKey(msg)
	}
	return m, nil
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

// cycleFocus moves focus by delta panes, skipping the sidebar when it is
// off.
func (m *Model) cycleFocus(delta int) tea.Cmd {
	next := m.focus
	for {
		next = (next + pane(delta) + paneCount) % paneCount
		if next != paneSidebar || m.sidebarOn {
			break
		}
	}
	return m.setFocus(next)
}

func (m *Model) setFocus(p pane) tea.Cmd {
	m.focus = p
	if p == paneRequest {
		return m.editor.Focus()
	}
	m.editor.Blur()
	return nil
}

// toggleSidebar shows or hides the sidebar. In a narrow window it never
// sits beside the panes: it is shown alone while it has focus, so turning
// it on focuses it.
func (m *Model) toggleSidebar() tea.Cmd {
	m.sidebarOn = !m.sidebarOn
	m.resize()
	switch {
	case !m.sidebarOn && m.focus == paneSidebar:
		return m.setFocus(paneRequest)
	case m.sidebarOn && !m.wide():
		return m.setFocus(paneSidebar)
	}
	return nil
}

func (m *Model) setMessage(info string, err error) {
	if err != nil {
		m.message, m.isErr = format.Sanitize(err.Error()), true
		return
	}
	m.message, m.isErr = info, false
}

func (m Model) innerWidth() int     { return max(1, m.width-2*padX) }
func (m Model) bodyHeight() int     { return max(1, m.height-chromeHeight) }
func (m Model) wide() bool          { return m.innerWidth() >= wideWidth }
func (m Model) tooSmall() bool      { return m.width < minWidth || m.height < minHeight }
func (m Model) sidebarBeside() bool { return m.sidebarOn && m.wide() }

// layout is the width of the sidebar and of the request and response
// column. A zero width means the part is not shown.
func (m Model) layout() (sideW, mainW int) {
	w := m.innerWidth()
	switch {
	case m.sidebarBeside():
		sideW = min(max(w/4, minSidebar), maxSidebar)
		return sideW, w - sideW - paneGap
	case m.focus == paneSidebar:
		return w, 0
	}
	return 0, w
}

// resize sizes the panes for the current window.
func (m *Model) resize() {
	_, mainW := m.layout()
	if mainW == 0 {
		mainW = m.innerWidth() // keep the column sized while the sidebar covers it
	}
	m.editor.SetWidth(mainW)
	// The editor, a blank line, then the response.
	m.response.SetHeight(max(0, m.bodyHeight()-components.EditorHeight-1))
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
		body = m.bodyLines(bodyH)
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

// bodyLines renders the sidebar and the request and response column.
func (m Model) bodyLines(height int) []string {
	sideW, mainW := m.layout()
	var side, main []string
	if sideW > 0 {
		side = m.sidebar.View(m.styles, sideW, height, m.focus == paneSidebar)
	}
	if mainW > 0 {
		main = m.editor.View(m.styles, mainW, m.focus == paneRequest)
		main = append(main, "")
		main = append(main, m.response.View(m.styles, mainW, m.focus == paneResponse)...)
	}
	switch {
	case mainW == 0:
		return side
	case sideW == 0:
		return main
	}
	gap := strings.Repeat(" ", paneGap)
	out := make([]string, height)
	for i := range out {
		var s, r string
		if i < len(side) {
			s = side[i]
		}
		if i < len(main) {
			r = main[i]
		}
		if r == "" {
			out[i] = s
			continue
		}
		out[i] = components.Fit(s, sideW, true) + gap + r
	}
	return out
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
	case paneRequest:
		return m.help.Hints(m.keys.URLHelp(), w)
	case paneResponse:
		return m.help.Hints(m.keys.ResponseHelp(), w)
	}
	return m.help.Hints(m.keys.SidebarHelp(), w)
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
