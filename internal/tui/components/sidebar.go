package components

import "github.com/skinleak/quest/internal/tui/theme"

// Sidebar lists collections, with the active environment at the bottom.
// Collections and environments arrive with the store; until then it shows
// their empty states.
type Sidebar struct{}

// View renders the sidebar into exactly height lines.
func (s Sidebar) View(st theme.Styles, width, height int, focused bool) []string {
	if height <= 0 {
		return nil
	}
	out := make([]string, height)
	out[0] = Title(st, "collections", focused)
	if height > 2 {
		out[1] = Fit(st.Subtle.Render("no collections yet"), width, false)
	}
	if height > 4 {
		out[height-1] = Fit(st.Muted.Render("env ")+st.Subtle.Render("none"), width, false)
	}
	return out
}
