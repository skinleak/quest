// Package components holds the panes and widgets the app is made of. Each
// renders itself into lines already cut to the width it is given, so the
// app can assemble a frame line by line without measuring it again.
package components

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Fit cuts s to width cells and, when pad is set, fills it up to width.
func Fit(s string, width int, pad bool) string {
	w := ansi.StringWidth(s)
	if w > width {
		return ansi.Truncate(s, width, "")
	}
	if pad && w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

// Lines splits rendered text into lines; empty text has none.
func Lines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// Spread puts left and right on one line of width cells, right-aligned,
// and drops right when both don't fit with at least gap cells between.
func Spread(left, right string, width, gap int) string {
	lw, rw := ansi.StringWidth(left), ansi.StringWidth(right)
	if right == "" || lw+gap+rw > width {
		return Fit(left, width, false)
	}
	return left + strings.Repeat(" ", width-lw-rw) + right
}
