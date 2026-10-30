package components

import (
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/x/ansi"

	"github.com/skinleak/quest/internal/tui/theme"
)

// Help renders key hints for the status bar and the help overlay.
type Help struct {
	model  help.Model
	offset int // overlay scroll position
}

// NewHelp returns a Help styled from st.
func NewHelp(st *theme.Styles) Help {
	h := help.New()
	h.ShortSeparator = " " + theme.IconSep + " "
	h.FullSeparator = "      "
	h.Styles = help.Styles{
		ShortKey:       st.HelpKey,
		ShortDesc:      st.HelpDsc,
		ShortSeparator: st.HelpSep,
		FullKey:        st.HelpKey,
		FullDesc:       st.HelpDsc,
		FullSeparator:  st.HelpSep,
		Ellipsis:       st.HelpSep,
	}
	return Help{model: h}
}

// Hints renders the hints that fit width. They are ordered by importance,
// so the least important are dropped from the right; the last two always
// stay. help.ShortHelpView can exceed its Width when not even "…" fits,
// so the result is cut explicitly.
func (h Help) Hints(bs []key.Binding, width int) string {
	m := h.model
	m.Width = 0 // measure untruncated
	for len(bs) > 2 {
		if v := m.ShortHelpView(bs); ansi.StringWidth(v) <= width {
			return v
		}
		bs = slices.Delete(slices.Clone(bs), len(bs)-3, len(bs)-2)
	}
	return ansi.Truncate(m.ShortHelpView(bs), width, theme.IconEllipsis)
}

// Reset scrolls the overlay back to the top.
func (h *Help) Reset() { h.offset = 0 }

// Scroll scrolls the overlay when it doesn't fit the window.
func (h *Help) Scroll(st *theme.Styles, groups [][]key.Binding, delta, width, height int) {
	_, scroll := h.lines(st, groups, width, height)
	h.offset = min(max(0, h.offset+delta), scroll)
}

// Overlay renders every binding in a width × height body, centered when
// it fits. Nothing is ever cut off: groups that don't fit side by side go
// below each other, and a list taller than the window scrolls with j/k.
func (h Help) Overlay(st *theme.Styles, groups [][]key.Binding, width, height int) []string {
	if height <= 0 {
		return nil
	}
	all, scroll := h.lines(st, groups, width, height)
	var body []string
	if scroll == 0 {
		body = append(slices.Clip(all), "", st.Subtle.Render("press ? or esc to close"))
	} else {
		offset := min(h.offset, scroll)
		hint := "↓ j/k scroll " + theme.IconSep + " esc close"
		if offset == scroll {
			hint = "↑ j/k scroll " + theme.IconSep + " esc close"
		}
		body = append(slices.Clone(all[offset:offset+height-1]), st.Subtle.Render(hint))
	}

	// Measured over every line, not the visible ones, so scrolling never
	// shifts the list sideways.
	contentW := max(blockWidth(all), ansi.StringWidth(body[len(body)-1]))
	margin := strings.Repeat(" ", max(0, (width-contentW)/2))
	out := make([]string, max(0, (height-len(body))/2), height)
	for _, l := range body {
		if l != "" {
			l = Fit(margin+l, width, false)
		}
		out = append(out, l)
	}
	return out
}

// lines lays out the title and the groups: as many side by side as fit,
// the rest below. It also returns how far the lines scroll, which is 0
// when they fit along with the closing hint.
func (h Help) lines(st *theme.Styles, groups [][]key.Binding, width, height int) ([]string, int) {
	m := h.model
	m.Width = 0 // never cut a group; the layout makes room
	blocks := make([][]string, 0, len(groups))
	for _, g := range groups {
		blocks = append(blocks, Lines(m.FullHelpView([][]key.Binding{g})))
	}
	gap := ansi.StringWidth(m.FullSeparator)

	all := []string{st.Strong.Render("keybindings")}
	for i := 0; i < len(blocks); {
		widths := []int{blockWidth(blocks[i])}
		total := widths[0]
		j := i + 1
		for ; j < len(blocks); j++ {
			w := blockWidth(blocks[j])
			if total+gap+w > width {
				break
			}
			widths, total = append(widths, w), total+gap+w
		}
		all = append(all, "")
		all = append(all, sideBySide(blocks[i:j], widths, gap)...)
		i = j
	}
	if len(all)+2 <= height {
		return all, 0
	}
	// One line is kept for the scroll hint.
	return all, max(0, len(all)-(height-1))
}

func blockWidth(block []string) int {
	w := 0
	for _, l := range block {
		w = max(w, ansi.StringWidth(l))
	}
	return w
}

// sideBySide joins blocks into lines, each block padded to its width.
func sideBySide(blocks [][]string, widths []int, gap int) []string {
	rows := 0
	for _, b := range blocks {
		rows = max(rows, len(b))
	}
	sep := strings.Repeat(" ", gap)
	out := make([]string, rows)
	for r := range out {
		var b strings.Builder
		for i, block := range blocks {
			last := i == len(blocks)-1
			var l string
			if r < len(block) {
				l = block[r]
			}
			b.WriteString(Fit(l, widths[i], !last))
			if !last {
				b.WriteString(sep)
			}
		}
		out[r] = strings.TrimRight(b.String(), " ")
	}
	return out
}
