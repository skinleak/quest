package components

import (
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/skinleak/quest/internal/core/request"
	"github.com/skinleak/quest/internal/tui/format"
	"github.com/skinleak/quest/internal/tui/theme"
)

// EditorHeight is the editor's height in lines without its tab content:
// the title with the tabs, the method row and the URL.
const EditorHeight = 3

// EditorFocus is the part of the editor that has the keyboard.
type EditorFocus int

// Editor parts.
const (
	EditorBlurred EditorFocus = iota
	EditorMethod
	EditorURL
	EditorTabs // the params, headers or body below the URL
)

// Request tabs.
const (
	ReqParams = iota
	ReqHeaders
	ReqBody
	reqTabCount
)

var reqTabNames = []string{"params", "headers", "body"}

// Method label variants, rendered once in NewEditor.
const (
	labelIdle = iota
	labelActive
	labelPicking // active, while the method row has focus
	labelCount
)

// Editor edits a request: its method, URL, params, headers and body.
// Every method is shown in a row with the active one in its color, so the
// choice is always in view.
//
// The URL is the truth for the params: the params tab lists its query,
// and editing a param rewrites the query. Disabled params keep their
// place in the list but leave the URL.
type Editor struct {
	method  int // index into request.Methods
	url     textinput.Model
	labels  [labelCount][]string
	tab     int
	params  kvList
	headers kvList
	body    TextArea
	cell    textinput.Model // edits the selected row as "name: value"
	editing bool
	area    int // lines for the tab content; 0 hides it
	width   int
	hl      format.Highlighter
}

// NewEditor returns an editor for an empty GET request.
func NewEditor(st *theme.Styles) Editor {
	e := Editor{
		url:  newInput(st, "https://api.example.com/users"),
		cell: newInput(st, ""),
		body: NewTextArea(),
		hl:   format.NewHighlighter(st),
	}
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

func newInput(st *theme.Styles, placeholder string) textinput.Model {
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = placeholder
	in.PlaceholderStyle = st.Subtle
	in.TextStyle = st.Text
	in.Cursor.Style = st.Cursor
	// A static cursor is calmer and avoids a redraw every blink.
	in.Cursor.SetMode(cursor.CursorStatic)
	return in
}

// Focus gives the URL input the keyboard.
func (e *Editor) Focus() tea.Cmd { return e.url.Focus() }

// Blur takes the keyboard away from the URL input.
func (e *Editor) Blur() { e.url.Blur() }

// Update passes a message to the URL input and lists its query as params.
func (e *Editor) Update(msg tea.Msg) tea.Cmd {
	before := e.url.Value()
	var cmd tea.Cmd
	e.url, cmd = e.url.Update(msg)
	if e.url.Value() != before {
		e.readParams()
	}
	return cmd
}

// MoveMethod picks the method delta steps away in the row, wrapping
// around.
func (e *Editor) MoveMethod(delta int) {
	n := len(request.Methods)
	e.method = (e.method + delta%n + n) % n
}

// SetRequest loads r into the editor. Its params are added to the URL.
func (e *Editor) SetRequest(r request.Request) {
	e.StopEdit()
	e.method = max(0, slices.Index(request.Methods, r.Method))
	e.url.SetValue(r.URL)
	e.url.CursorEnd()
	e.params = kvList{}
	e.readParams()
	for _, p := range r.Params {
		e.params.rows = append(e.params.rows, request.KV{
			Key: escapeQuery(p.Key, "&#= "), Value: escapeQuery(p.Value, "&# "), Disabled: p.Disabled,
		})
	}
	e.writeParams()
	e.headers = kvList{rows: slices.Clone(r.Headers)}
	e.body.SetValue(r.Body)
}

// Request is the request being edited, including a row being typed.
func (e *Editor) Request() request.Request {
	r := request.Request{Method: request.Methods[e.method], URL: e.url.Value(), Body: e.body.Value()}
	for _, h := range e.headers.rows {
		if h.Key != "" || h.Value != "" {
			r.Headers = append(r.Headers, h)
		}
	}
	return r
}

// SetWidth sizes the inputs for a pane width cells wide. textinput draws
// one cell more than its Width, for the cursor.
func (e *Editor) SetWidth(width int) {
	e.width = width
	e.url.Width = max(1, width-1)
	// The row's pointer and a space, and one cell for the cursor.
	e.cell.Width = max(1, width-ansi.StringWidth(theme.IconPointer)-2)
	e.body.SetSize(width, e.area)
}

// SetArea sets how many lines the tab content may use.
func (e *Editor) SetArea(lines int) {
	e.area = lines
	e.body.SetSize(e.width, lines)
	e.params.follow(lines)
	e.headers.follow(lines)
}

// ContentHeight is how many lines the tab content would like: the longest
// tab, so switching tabs doesn't move the panes below.
func (e *Editor) ContentHeight() int {
	return max(1, len(e.params.rows), len(e.headers.rows), e.body.LineCount())
}

// Height is the editor's height for the current area.
func (e *Editor) Height() int {
	if e.area == 0 {
		return EditorHeight
	}
	return EditorHeight + 1 + e.area
}

// Tab is the active request tab.
func (e *Editor) Tab() int { return e.tab }

// MoveTab switches tabs by delta, wrapping around.
func (e *Editor) MoveTab(delta int) {
	e.tab = (e.tab + delta + reqTabCount) % reqTabCount
}

// Editing reports whether a row or the body is being typed in.
func (e *Editor) Editing() bool { return e.editing }

// EditingBody reports whether the body is being typed in, where enter is
// a newline rather than the end of the edit.
func (e *Editor) EditingBody() bool { return e.editing && e.tab == ReqBody }

// MoveRow selects another row, or scrolls the body.
func (e *Editor) MoveRow(delta int) {
	if l := e.list(); l != nil {
		l.cursor = min(max(0, l.cursor+delta), max(0, len(l.rows)-1))
		l.follow(e.area)
		return
	}
	e.body.Scroll(delta)
}

// Edit starts typing in the selected row, or in the body. An empty list
// gets a new row.
func (e *Editor) Edit() tea.Cmd {
	l := e.list()
	if l == nil {
		e.editing = true
		e.body.Focus()
		return nil
	}
	if len(l.rows) == 0 {
		return e.Add()
	}
	e.editing = true
	e.cell.SetValue(rowText(l.rows[l.cursor], e.tab))
	e.cell.CursorEnd()
	return e.cell.Focus()
}

// Add appends an empty row and starts typing in it.
func (e *Editor) Add() tea.Cmd {
	l := e.list()
	if l == nil {
		return nil
	}
	l.rows = append(l.rows, request.KV{})
	l.cursor = len(l.rows) - 1
	l.follow(e.area)
	e.editing = true
	e.cell.SetValue("")
	return e.cell.Focus()
}

// Delete removes the selected row.
func (e *Editor) Delete() {
	l := e.list()
	if l == nil || len(l.rows) == 0 {
		return
	}
	l.rows = slices.Delete(l.rows, l.cursor, l.cursor+1)
	l.cursor = max(0, min(l.cursor, len(l.rows)-1))
	l.follow(e.area)
	e.rowsChanged()
}

// Toggle turns the selected row on or off.
func (e *Editor) Toggle() {
	l := e.list()
	if l == nil || len(l.rows) == 0 {
		return
	}
	l.rows[l.cursor].Disabled = !l.rows[l.cursor].Disabled
	e.rowsChanged()
}

// UpdateEditing passes a key to the row or body being edited. A row
// changes as it is typed, so a request sent meanwhile has it.
func (e *Editor) UpdateEditing(msg tea.Msg) tea.Cmd {
	if e.tab == ReqBody {
		if k, ok := msg.(tea.KeyMsg); ok {
			e.body.Update(k)
		}
		return nil
	}
	var cmd tea.Cmd
	e.cell, cmd = e.cell.Update(msg)
	l := e.list()
	row := parseRow(e.cell.Value(), e.tab)
	row.Disabled = l.rows[l.cursor].Disabled
	l.rows[l.cursor] = row
	e.rowsChanged()
	return cmd
}

// StopEdit ends typing. A row left empty is removed.
func (e *Editor) StopEdit() {
	if !e.editing {
		return
	}
	e.editing = false
	e.body.Blur()
	e.cell.Blur()
	if l := e.list(); l != nil && len(l.rows) > 0 && l.rows[l.cursor] == (request.KV{}) {
		e.Delete()
	}
}

// list is the active tab's rows, or nil on the body tab.
func (e *Editor) list() *kvList {
	switch e.tab {
	case ReqParams:
		return &e.params
	case ReqHeaders:
		return &e.headers
	}
	return nil
}

func (e *Editor) rowsChanged() {
	if e.tab == ReqParams {
		e.writeParams()
	}
}

// readParams lists the URL's query, keeping disabled params in place.
func (e *Editor) readParams() {
	_, query, _ := splitURL(e.url.Value())
	e.params.rows = mergeParams(e.params.rows, parseQuery(query))
	e.params.cursor = max(0, min(e.params.cursor, len(e.params.rows)-1))
}

// writeParams rewrites the URL's query from the enabled params.
func (e *Editor) writeParams() {
	base, _, frag := splitURL(e.url.Value())
	var q strings.Builder
	for _, p := range e.params.rows {
		if p.Disabled || (p.Key == "" && p.Value == "") {
			continue
		}
		if q.Len() > 0 {
			q.WriteByte('&')
		}
		q.WriteString(p.Key)
		q.WriteByte('=')
		q.WriteString(p.Value)
	}
	u := base
	if q.Len() > 0 {
		u += "?" + q.String()
	}
	e.url.SetValue(u + frag)
}

// View renders the title with the tabs, the method row, the URL and, when
// there is room, the active tab's content.
func (e *Editor) View(st *theme.Styles, width int, focus EditorFocus) []string {
	tabs := make([]string, reqTabCount)
	copy(tabs, reqTabNames)
	if n := e.params.enabled(); n > 0 {
		tabs[ReqParams] += " " + strconv.Itoa(n)
	}
	if n := e.headers.enabled(); n > 0 {
		tabs[ReqHeaders] += " " + strconv.Itoa(n)
	}
	out := []string{
		Spread(Title(st, "request", focus != EditorBlurred), Tabs(st, tabs, e.tab), width, 3),
		e.methodRow(width, focus == EditorMethod),
		Fit(e.url.View(), width, false),
	}
	if e.area == 0 {
		return out
	}
	out = append(out, "")
	focused := focus == EditorTabs
	switch e.tab {
	case ReqParams:
		return append(out, e.listView(st, &e.params, "no params", width, focused)...)
	case ReqHeaders:
		return append(out, e.listView(st, &e.headers, "no headers", width, focused)...)
	}
	if e.body.Empty() && !e.editing {
		return append(out, st.P.Subtle.Render("no body"))
	}
	var hl *format.Highlighter
	if e.body.LooksLikeJSON() {
		hl = &e.hl
	}
	return append(out, e.body.View(st, hl)...)
}

// listView renders the visible rows, names in one column. The selected
// row is marked while the tabs have focus, so focus shows without color.
func (e *Editor) listView(st *theme.Styles, l *kvList, empty string, width int, focused bool) []string {
	if len(l.rows) == 0 {
		return []string{st.P.Subtle.Render(empty)}
	}
	nameW := 0
	for _, r := range l.rows {
		nameW = max(nameW, ansi.StringWidth(r.Key))
	}
	nameW = min(nameW, maxHeaderName)
	blank := strings.Repeat(" ", ansi.StringWidth(theme.IconPointer)+1)
	key, val, off := theme.PaintOf(st.HeaderKey), theme.PaintOf(st.Text), theme.PaintOf(st.Off)
	out := make([]string, 0, e.area)
	for i := l.top; i < min(l.top+e.area, len(l.rows)); i++ {
		lead := blank
		if focused && i == l.cursor {
			lead = st.Pointer.Render(theme.IconPointer) + " "
		}
		if e.editing && i == l.cursor {
			out = append(out, Fit(lead+e.cell.View(), width, false))
			continue
		}
		r := l.rows[i]
		name := Fit(ansi.Truncate(format.Sanitize(r.Key), nameW, theme.IconEllipsis), nameW, true)
		value := format.Sanitize(r.Value)
		if r.Disabled {
			out = append(out, Fit(lead+off.Render(name)+"  "+off.Render(value), width, false))
			continue
		}
		out = append(out, Fit(lead+key.Render(name)+"  "+val.Render(value), width, false))
	}
	return out
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
func Title(st *theme.Styles, title string, focused bool) string {
	if focused {
		return st.P.TitleFocus.Render(title)
	}
	return st.P.Title.Render(title)
}

// kvList is the rows of the params or headers tab, with one selected.
type kvList struct {
	rows   []request.KV
	cursor int
	top    int // first visible row
}

// follow scrolls so the selected row is among height visible ones.
func (l *kvList) follow(height int) {
	if height > 0 {
		l.top = min(max(l.top, l.cursor-height+1), l.cursor)
	}
	l.top = max(0, l.top)
}

func (l *kvList) enabled() int {
	n := 0
	for _, r := range l.rows {
		if !r.Disabled && (r.Key != "" || r.Value != "") {
			n++
		}
	}
	return n
}

// rowText is a row as it is typed: "name: value" for headers, "name=value"
// for params, as they appear on the wire.
func rowText(r request.KV, tab int) string {
	if r.Key == "" && r.Value == "" {
		return ""
	}
	if tab == ReqParams {
		return r.Key + "=" + r.Value
	}
	return r.Key + ": " + r.Value
}

// parseRow reads a typed row. A param's name and value are escaped where
// they would otherwise end the pair or the query.
func parseRow(s string, tab int) request.KV {
	if tab == ReqParams {
		k, v, _ := strings.Cut(s, "=")
		return request.KV{Key: escapeQuery(strings.TrimSpace(k), "&#= "), Value: escapeQuery(v, "&# ")}
	}
	k, v, _ := strings.Cut(s, ":")
	return request.KV{Key: strings.TrimSpace(k), Value: strings.TrimSpace(v)}
}

// escapeQuery percent-encodes the characters in special, and nothing else,
// so what was typed stays readable in the URL.
func escapeQuery(s, special string) string {
	if !strings.ContainsAny(s, special) {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if r < 0x80 && strings.ContainsRune(special, r) {
			b.WriteString("%" + strings.ToUpper(strconv.FormatInt(int64(r), 16)))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// splitURL cuts a URL into what comes before its query, the query, and
// the fragment including its "#".
func splitURL(u string) (base, query, frag string) {
	if i := strings.IndexByte(u, '#'); i >= 0 {
		u, frag = u[:i], u[i:]
	}
	base, query, _ = strings.Cut(u, "?")
	return base, query, frag
}

// parseQuery splits a raw query into pairs, as typed: nothing is decoded.
func parseQuery(q string) []request.KV {
	var out []request.KV
	for pair := range strings.SplitSeq(q, "&") {
		if pair == "" {
			continue
		}
		k, v, _ := strings.Cut(pair, "=")
		out = append(out, request.KV{Key: k, Value: v})
	}
	return out
}

// mergeParams puts the URL's params in the places of the enabled ones in
// old, so disabled params keep their position; extra ones go at the end.
func mergeParams(old, enabled []request.KV) []request.KV {
	out := make([]request.KV, 0, len(old)+len(enabled))
	i := 0
	for _, r := range old {
		if r.Disabled {
			out = append(out, r)
			continue
		}
		if i < len(enabled) {
			out = append(out, enabled[i])
			i++
		}
	}
	return append(out, enabled[i:]...)
}
