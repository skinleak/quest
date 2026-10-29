package components

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/skinleak/quest/internal/core/request"
	"github.com/skinleak/quest/internal/tui/format"
	"github.com/skinleak/quest/internal/tui/theme"
)

// Response tabs.
const (
	TabBody = iota
	TabHeaders
	TabTiming
	tabCount
)

var tabNames = []string{"body", "headers", "timing"}

// responseChrome is the title line, the status line and a blank line.
const responseChrome = 3

// maxHeaderName caps the header name column so one long name can't push
// every value off screen; longer names are cut.
const maxHeaderName = 28

// minWrap is the narrowest value column header values are wrapped into;
// below it they are cut instead.
const minWrap = 16

// labelW is the label column of the timing tab.
const labelW = 11

type responseState int

const (
	stateIdle responseState = iota
	stateSending
	stateDone
	stateFailed
)

// Response shows the last response: a status line and the body, headers or
// timing in a scrollable window. Only visible lines are rendered, and body
// lines are highlighted chunk by chunk as they scroll into view.
type Response struct {
	state  responseState
	prev   responseState // the state before sending, restored on cancel
	resp   *request.Response
	err    string
	doc    *format.Doc
	media  string             // the media type, without parameters
	lines  [tabCount][]string // headers and timing, laid out for width
	tab    int
	offset [tabCount]int
	width  int
	height int // lines of the whole pane
	styles theme.Styles
}

// SetSending shows that a request is in flight. The previous response
// stays until the new one arrives, so there's no flicker.
func (r *Response) SetSending() {
	if r.state != stateSending {
		r.prev = r.state
	}
	r.state = stateSending
}

// SetResult shows a finished request: a response with its formatted body,
// or an error. A cancelled request brings back what was shown before.
func (r *Response) SetResult(st theme.Styles, resp *request.Response, doc *format.Doc, err error) {
	switch {
	case errors.Is(err, context.Canceled):
		r.state = r.prev
		return
	case errors.Is(err, context.DeadlineExceeded):
		r.state, r.err = stateFailed, "timed out"
		return
	case err != nil:
		r.state, r.err = stateFailed, format.Sanitize(err.Error())
		return
	}
	r.state, r.resp, r.doc, r.err = stateDone, resp, doc, ""
	r.styles = st
	r.media, _, _ = strings.Cut(resp.ContentType, ";")
	r.media = format.Sanitize(strings.TrimSpace(r.media))
	r.offset = [tabCount]int{}
	r.layout()
	r.prepare()
}

// layout renders the headers and timing tabs for the pane's width, so
// View only reads.
func (r *Response) layout() {
	if r.resp == nil {
		return
	}
	r.lines[TabHeaders] = headerLines(r.styles, r.resp.Header, r.width)
	r.lines[TabTiming] = timingLines(r.styles, r.resp, r.width, time.Now())
}

// Sending reports whether a request is in flight.
func (r *Response) Sending() bool { return r.state == stateSending }

// SetSize sets the pane's width and height.
func (r *Response) SetSize(w, h int) {
	if w != r.width {
		r.width = w
		r.layout()
	}
	r.height = h
	r.scroll(0)
}

// Tab is the active tab.
func (r *Response) Tab() int { return r.tab }

// MoveTab switches tabs by delta, wrapping around.
func (r *Response) MoveTab(delta int) {
	r.tab = (r.tab + delta + tabCount) % tabCount
	r.prepare()
}

// Scroll moves the window by delta lines.
func (r *Response) Scroll(delta int) { r.scroll(delta) }

// Page scrolls by a window's height, down for positive pages.
func (r *Response) Page(pages int) { r.scroll(pages * max(1, r.contentHeight()-1)) }

// Top scrolls to the first line.
func (r *Response) Top() { r.scroll(-r.offset[r.tab]) }

// Bottom scrolls to the last line.
func (r *Response) Bottom() { r.scroll(r.lineCount()) }

func (r *Response) scroll(delta int) {
	maxOff := max(0, r.lineCount()-r.contentHeight())
	r.offset[r.tab] = min(max(0, r.offset[r.tab]+delta), maxOff)
	r.prepare()
}

// prepare highlights the body lines about to be shown, so View only reads.
func (r *Response) prepare() {
	if r.doc != nil && r.tab == TabBody {
		off := r.offset[TabBody]
		r.doc.Highlight(off, off+r.contentHeight())
	}
}

func (r *Response) contentHeight() int { return max(0, r.height-responseChrome) }

func (r *Response) lineCount() int {
	if r.resp == nil {
		return 0
	}
	if r.tab == TabBody {
		return r.doc.Len()
	}
	return len(r.lines[r.tab])
}

// View renders the pane into at most its height lines.
func (r *Response) View(st theme.Styles, width int, focused bool) []string {
	if r.height <= 0 {
		return nil
	}
	out := make([]string, 0, r.height)
	out = append(out, Spread(Title(st, "response", focused), Tabs(st, tabNames, r.tab), width, 3))
	if r.height == 1 {
		return out
	}
	out = append(out, r.statusLine(st, width))
	if r.state == stateFailed {
		return append(out, r.errorLines(st, width)...)
	}
	if r.resp == nil || r.height < responseChrome {
		return out
	}
	out = append(out, "")
	h := r.contentHeight()
	switch r.tab {
	case TabBody:
		if r.doc.Len() == 0 {
			return append(out, st.Subtle.Render("empty body"))
		}
		off := r.offset[TabBody]
		for i := off; i < min(off+h, r.doc.Len()); i++ {
			out = append(out, Fit(r.doc.Line(i), width, false))
		}
	default:
		lines := r.lines[r.tab]
		off := min(r.offset[r.tab], len(lines))
		for _, l := range lines[off:min(off+h, len(lines))] {
			out = append(out, Fit(l, width, false))
		}
	}
	return out
}

func (r *Response) statusLine(st theme.Styles, width int) string {
	switch r.state {
	case stateIdle:
		return Fit(st.Subtle.Render("nothing sent yet"), width, false)
	case stateSending:
		return Fit(st.Muted.Render("sending"+theme.IconEllipsis), width, false)
	case stateFailed:
		return Fit(st.Error.Render("request failed"), width, false)
	}
	sep := st.Subtle.Render(" " + theme.IconSep + " ")
	resp := r.resp
	parts := []string{
		st.Status(resp.StatusCode).Render(format.Sanitize(resp.Status)),
		st.Text.Render(FormatDuration(resp.Timing.Total)),
		st.Text.Render(FormatSize(resp.Size)),
	}
	if resp.Truncated {
		parts = append(parts, st.Warning.Render("cut at "+FormatSize(resp.Size)))
	}
	if r.media != "" {
		parts = append(parts, st.Muted.Render(r.media))
	}
	var pos string
	if n, h := r.lineCount(), r.contentHeight(); n > h && h > 0 {
		last := min(r.offset[r.tab]+h, n)
		pos = st.Subtle.Render(strconv.Itoa(last) + "/" + strconv.Itoa(n))
	}
	return Spread(strings.Join(parts, sep), pos, width, 3)
}

// errorLines wraps the error message under the status line.
func (r *Response) errorLines(st theme.Styles, width int) []string {
	room := r.height - 2
	if room <= 0 {
		return nil
	}
	lines := Lines(ansi.Wordwrap(r.err, width, " /"))
	out := []string{""}
	for _, l := range lines[:min(len(lines), room-1)] {
		out = append(out, st.Text.Render(Fit(l, width, false)))
	}
	return out
}

// Tabs renders tab labels with the active one bold.
func Tabs(st theme.Styles, names []string, active int) string {
	var b strings.Builder
	for i, n := range names {
		if i > 0 {
			b.WriteString("  ")
		}
		if i == active {
			b.WriteString(st.TabActive.Render(n))
		} else {
			b.WriteString(st.Tab.Render(n))
		}
	}
	return b.String()
}

// headerLines renders the headers sorted by name, one per value, with
// the values aligned. Long values wrap under themselves, so cookies and
// policies can be read in full.
func headerLines(st theme.Styles, header http.Header, width int) []string {
	names := make([]string, 0, len(header))
	nameW := 0
	for n := range header {
		names = append(names, n)
		nameW = max(nameW, len(n))
	}
	slices.Sort(names)
	nameW = min(nameW, maxHeaderName)
	valW := width - nameW - 2
	indent := strings.Repeat(" ", nameW+2)
	key, val := theme.PaintOf(st.HeaderKey), theme.PaintOf(st.Text)
	var out []string
	for _, n := range names {
		label := Fit(ansi.Truncate(format.Sanitize(n), nameW, theme.IconEllipsis), nameW, true)
		for _, v := range header[n] {
			v = format.Sanitize(v)
			if valW < minWrap || len(v) <= valW {
				out = append(out, key.Render(label)+"  "+val.Render(v))
				continue
			}
			for i, l := range Lines(ansi.Wrap(v, valW, " ;,")) {
				if i == 0 {
					out = append(out, key.Render(label)+"  "+val.Render(l))
				} else {
					out = append(out, indent+val.Render(l))
				}
			}
		}
	}
	return out
}

// timingLines draws the phases that happened as a waterfall, then what
// is known about the connection: server, protocol, certificate and
// redirects.
func timingLines(st theme.Styles, resp *request.Response, width int, now time.Time) []string {
	out := waterfall(st, resp.Timing, width)
	label := func(s string) string { return st.Muted.Render(Fit(s, labelW, true)) }
	sep := st.Subtle.Render(" " + theme.IconSep + " ")
	row := func(name string, parts ...string) {
		out = append(out, Fit(label(name)+strings.Join(parts, sep), width, false))
	}
	out = append(out, "")

	if resp.Remote != "" {
		parts := []string{st.Text.Render(format.Sanitize(resp.Remote))}
		if resp.Timing.Reused {
			parts = append(parts, st.Muted.Render("reused connection"))
		}
		row("server", parts...)
	}
	proto := []string{st.Text.Render(format.Sanitize(resp.Proto))}
	if t := resp.TLS; t != nil {
		proto = append(proto, st.Text.Render(t.Version))
	}
	row("protocol", proto...)
	if t := resp.TLS; t != nil {
		if t.Subject != "" {
			row("cert", st.Text.Render(format.Sanitize(t.Subject)))
		}
		if t.Issuer != "" {
			row("issuer", st.Text.Render(format.Sanitize(t.Issuer)))
		}
		if !t.Expires.IsZero() {
			row("expires", st.Text.Render(t.Expires.Format(time.DateOnly)), expiry(st, t.Expires, now))
		}
	}
	if resp.Redirects > 0 {
		n := strconv.Itoa(resp.Redirects)
		row("redirects", st.Text.Render(n+" "+theme.IconArrow+" "+format.Sanitize(resp.URL)))
	}
	return out
}

// expiry says how long a certificate stays valid: a warning within two
// weeks, an error once expired.
func expiry(st theme.Styles, t, now time.Time) string {
	days := int(t.Sub(now).Hours() / 24)
	switch {
	case !t.After(now):
		return st.Error.Render("expired")
	case days == 0:
		return st.Warning.Render("today")
	case days == 1:
		return st.Warning.Render("in 1 day")
	case days <= 14:
		return st.Warning.Render("in " + strconv.Itoa(days) + " days")
	}
	return st.Muted.Render("in " + strconv.Itoa(days) + " days")
}

// waterfall draws each phase that happened as a bar starting where the
// phase started, scaled so the whole request spans the bar column. Wait
// and transfer are anchored at the end, which keeps them right when the
// request was sent some time after the connection was ready.
func waterfall(st theme.Styles, t request.Timing, width int) []string {
	const valueW, gap = 9, 2
	phases := []struct {
		name     string
		d, start int64
	}{
		{"dns", int64(t.DNS), 0},
		{"connect", int64(t.Connect), int64(t.DNS)},
		{"tls", int64(t.TLS), int64(t.DNS + t.Connect)},
		{"wait", int64(t.TTFB), int64(t.Total - t.Transfer - t.TTFB)},
		{"transfer", int64(t.Transfer), int64(t.Total - t.Transfer)},
	}
	total := max(1, int64(t.Total))
	barW := width - labelW - valueW - gap
	bar := theme.PaintOf(st.TimingBar)

	out := make([]string, 0, len(phases)+1)
	for _, p := range phases {
		if p.d == 0 {
			continue
		}
		line := st.Muted.Render(Fit(p.name, labelW, true)) +
			st.Text.Render(padLeft(FormatDuration(time.Duration(p.d)), valueW))
		if barW >= 10 {
			from := int(max(0, p.start) * int64(barW) / total)
			n := max(1, int(p.d*int64(barW)/total))
			from = min(from, barW-n)
			line += strings.Repeat(" ", gap+from) + bar.Render(strings.Repeat(theme.IconBar, n))
		}
		out = append(out, Fit(line, width, false))
	}
	sum := st.Muted.Render(Fit("total", labelW, true)) + st.Strong.Render(padLeft(FormatDuration(t.Total), valueW))
	return append(out, Fit(sum, width, false))
}

func padLeft(s string, width int) string {
	if n := width - ansi.StringWidth(s); n > 0 {
		return strings.Repeat(" ", n) + s
	}
	return s
}
