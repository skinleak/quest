package components

import (
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/skinleak/quest/internal/core/discover"
	"github.com/skinleak/quest/internal/core/request"
	"github.com/skinleak/quest/internal/tui/format"
	"github.com/skinleak/quest/internal/tui/theme"
)

// pickerChrome is the title, the filter and a blank line above the rows.
const pickerChrome = 3

// Picker lists the endpoints found in the project, filtered as you type:
// every word typed must appear in the method, path, name or file.
type Picker struct {
	all       []discover.Endpoint
	hay       []string // what the filter searches, lowercased, per endpoint
	shown     []int    // indexes into all that match the filter
	cursor    int      // index into shown
	top       int      // first visible row
	height    int      // lines of the whole list
	filter    textinput.Model
	methods   []string // method labels, colored and padded to one width
	methodW   int
	searching bool
	searched  bool // a search has finished
	result    discover.Result
	root      string // the directory searched, as shown
}

// NewPicker returns an empty picker for endpoints found in root.
func NewPicker(st *theme.Styles, root string) Picker {
	p := Picker{filter: newInput(st, "type to filter"), root: root}
	for _, m := range request.Methods {
		p.methodW = max(p.methodW, len(m))
	}
	for _, m := range request.Methods {
		p.methods = append(p.methods, st.Method(m).Render(m)+strings.Repeat(" ", p.methodW-len(m)))
	}
	return p
}

// Open clears the filter and gives it the keyboard.
func (p *Picker) Open() tea.Cmd {
	p.filter.SetValue("")
	p.refilter()
	return p.filter.Focus()
}

// SetSearching shows that a search is running; the last result stays.
func (p *Picker) SetSearching() { p.searching = true }

// SetResult replaces the endpoints with a search's result, keeping the
// filter.
func (p *Picker) SetResult(res discover.Result) {
	p.searching, p.searched, p.result = false, true, res
	p.all = res.Endpoints
	p.hay = make([]string, len(p.all))
	for i, ep := range p.all {
		p.hay[i] = strings.ToLower(ep.Request.Method + " " + ep.Path + " " + ep.Name + " " + ep.Source)
	}
	p.refilter()
}

// Count is the number of endpoints found.
func (p *Picker) Count() int { return len(p.all) }

// SetHeight sets the lines of the whole list.
func (p *Picker) SetHeight(h int) {
	p.height = h
	p.follow()
}

// SetWidth sizes the filter input.
func (p *Picker) SetWidth(w int) { p.filter.Width = max(1, w-1) }

// Update passes a key to the filter.
func (p *Picker) Update(msg tea.Msg) tea.Cmd {
	before := p.filter.Value()
	var cmd tea.Cmd
	p.filter, cmd = p.filter.Update(msg)
	if p.filter.Value() != before {
		p.refilter()
	}
	return cmd
}

// Move selects the row delta rows away.
func (p *Picker) Move(delta int) {
	p.cursor = min(max(0, p.cursor+delta), max(0, len(p.shown)-1))
	p.follow()
}

// Selected is the endpoint under the cursor.
func (p *Picker) Selected() (discover.Endpoint, bool) {
	if len(p.shown) == 0 {
		return discover.Endpoint{}, false
	}
	return p.all[p.shown[p.cursor]], true
}

func (p *Picker) refilter() {
	words := strings.Fields(strings.ToLower(p.filter.Value()))
	p.shown = p.shown[:0]
outer:
	for i, h := range p.hay {
		for _, w := range words {
			if !strings.Contains(h, w) {
				continue outer
			}
		}
		p.shown = append(p.shown, i)
	}
	p.cursor, p.top = 0, 0
}

func (p *Picker) follow() {
	if rows := p.height - pickerChrome; rows > 0 {
		p.top = min(max(p.top, p.cursor-rows+1), p.cursor)
	}
	p.top = max(0, p.top)
}

// View renders the list into at most height lines.
func (p *Picker) View(st *theme.Styles, width, height int) []string {
	if height <= 0 {
		return nil
	}
	out := []string{Spread(Title(st, "endpoints", true), p.summary(st), width, 3)}
	if height == 1 {
		return out
	}
	out = append(out, Fit(p.filter.View(), width, false))
	rows := height - pickerChrome
	if rows <= 0 {
		return out
	}
	out = append(out, "")
	note := func(s string) []string { return append(out, Fit(st.Muted.Render(s), width, false)) }
	switch {
	case len(p.all) == 0 && !p.searched:
		return note("looking for endpoints" + theme.IconEllipsis)
	case len(p.all) == 0:
		out = note("no endpoints found")
		if rows > 1 {
			out = append(out, Fit(st.P.Subtle.Render("quest reads openapi specs and .http files in "+p.root), width, false))
		}
		return out
	case len(p.shown) == 0:
		return note("nothing matches")
	}
	blank := strings.Repeat(" ", ansi.StringWidth(theme.IconPointer)+1)
	end := min(p.top+rows, len(p.shown))
	// Names line up after the longest visible path, up to half the width.
	pathW := 0
	for i := p.top; i < end; i++ {
		pathW = max(pathW, ansi.StringWidth(p.all[p.shown[i]].Path))
	}
	pathW = min(pathW, width/2)
	for i := p.top; i < end; i++ {
		ep := p.all[p.shown[i]]
		lead, pathStyle := blank, st.Text
		if i == p.cursor {
			lead, pathStyle = st.Pointer.Render(theme.IconPointer)+" ", st.Strong
		}
		path := pathStyle.Render(Fit(ansi.Truncate(format.Sanitize(ep.Path), pathW, theme.IconEllipsis), pathW, ep.Name != ""))
		left := lead + p.method(st, ep.Request.Method) + "  " + path
		if ep.Name != "" {
			left += "  " + st.Muted.Render(format.Sanitize(ep.Name))
		}
		out = append(out, Spread(left, st.P.Subtle.Render(format.Sanitize(ep.Source)), width, 3))
	}
	return out
}

func (p *Picker) method(st *theme.Styles, m string) string {
	if i := slices.Index(request.Methods, m); i >= 0 {
		return p.methods[i]
	}
	return st.MethodOther.Render(Fit(m, p.methodW, true))
}

// summary is the count shown next to the title, with what went wrong.
func (p *Picker) summary(st *theme.Styles) string {
	var parts []string
	switch n := len(p.all); {
	case len(p.shown) < n:
		parts = append(parts, strconv.Itoa(len(p.shown))+" of "+strconv.Itoa(n))
	case n == 1:
		parts = append(parts, "1 endpoint")
	case n > 1:
		parts = append(parts, strconv.Itoa(n)+" endpoints")
	}
	if f := len(p.result.Failed); f == 1 {
		parts = append(parts, "1 file unreadable")
	} else if f > 1 {
		parts = append(parts, strconv.Itoa(f)+" files unreadable")
	}
	if p.result.Partial {
		parts = append(parts, "search stopped early")
	}
	if p.searching && p.searched {
		parts = append(parts, "searching"+theme.IconEllipsis)
	}
	return st.Muted.Render(strings.Join(parts, " "+theme.IconSep+" "))
}
