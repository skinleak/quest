package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/skinleak/quest/internal/tui/format"
	"github.com/skinleak/quest/internal/tui/keys"
	"github.com/skinleak/quest/internal/tui/theme"
)

// tabWidth is how many spaces a typed or pasted tab becomes, so every
// rune takes one cell and the cursor stays where it is drawn.
const tabWidth = 2

// TextArea edits multi-line text, such as a request body. Unlike bubbles'
// textarea it never wraps and renders only the visible lines, so a large
// pasted body costs the same per keypress as a small one.
type TextArea struct {
	lines         [][]rune
	row, col      int // the cursor
	goal          int // the column up and down aim for
	top, left     int // the first visible line and column
	width, height int
	focused       bool
	keys          keys.TextKeyMap
}

// NewTextArea returns an empty text area.
func NewTextArea() TextArea {
	return TextArea{lines: [][]rune{nil}, keys: keys.DefaultText()}
}

// SetValue replaces the text and moves the cursor to the start.
func (t *TextArea) SetValue(s string) {
	parts := strings.Split(cleanText(s), "\n")
	t.lines = make([][]rune, len(parts))
	for i, p := range parts {
		t.lines[i] = []rune(p)
	}
	t.row, t.col, t.goal, t.top, t.left = 0, 0, 0, 0, 0
}

// Value is the text, lines joined with "\n".
func (t *TextArea) Value() string {
	var b strings.Builder
	for i, l := range t.lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(string(l))
	}
	return b.String()
}

// Empty reports whether there is no text.
func (t *TextArea) Empty() bool { return len(t.lines) == 1 && len(t.lines[0]) == 0 }

// LineCount is the number of lines.
func (t *TextArea) LineCount() int { return len(t.lines) }

// LooksLikeJSON reports whether the first non-blank line starts an object
// or array. It reads no further, so it is cheap on every frame.
func (t *TextArea) LooksLikeJSON() bool {
	for _, l := range t.lines {
		if s := strings.TrimSpace(string(l)); s != "" {
			return format.LooksLikeJSON(s)
		}
	}
	return false
}

// SetSize sets the visible width and height.
func (t *TextArea) SetSize(width, height int) {
	t.width, t.height = max(1, width), max(0, height)
	t.follow()
}

// Focus shows the cursor and takes keys. A cursor scrolled out of view
// moves to the first visible line.
func (t *TextArea) Focus() {
	t.focused = true
	if t.row < t.top || t.row >= t.top+t.height {
		t.row, t.col, t.goal = t.top, 0, 0
	}
	t.follow()
}

// Blur hides the cursor.
func (t *TextArea) Blur() { t.focused = false }

// Focused reports whether the text area takes keys.
func (t *TextArea) Focused() bool { return t.focused }

// Scroll moves the view by delta lines without moving the cursor.
func (t *TextArea) Scroll(delta int) {
	t.top = min(max(0, t.top+delta), max(0, len(t.lines)-t.height))
}

// Update applies an editing key. Printable keys and pastes insert text.
func (t *TextArea) Update(msg tea.KeyMsg) {
	k := t.keys
	switch {
	case msg.Type == tea.KeyRunes:
		t.insert(string(msg.Runes))
	case msg.Type == tea.KeySpace:
		t.insert(" ")
	case key.Matches(msg, k.Newline):
		t.newline()
	case key.Matches(msg, k.DeleteWord):
		t.deleteWord()
	case key.Matches(msg, k.Backspace):
		t.backspace()
	case key.Matches(msg, k.DeleteChar):
		t.deleteChar()
	case key.Matches(msg, k.DeleteToStart):
		t.lines[t.row] = t.lines[t.row][t.col:]
		t.setCol(0)
	case key.Matches(msg, k.DeleteToEnd):
		t.lines[t.row] = t.lines[t.row][:t.col]
	case key.Matches(msg, k.Left):
		t.left1()
	case key.Matches(msg, k.Right):
		t.right1()
	case key.Matches(msg, k.Up):
		t.moveRow(-1)
	case key.Matches(msg, k.Down):
		t.moveRow(1)
	case key.Matches(msg, k.PageUp):
		t.moveRow(-max(1, t.height-1))
	case key.Matches(msg, k.PageDown):
		t.moveRow(max(1, t.height-1))
	case key.Matches(msg, k.LineStart):
		t.setCol(0)
	case key.Matches(msg, k.LineEnd):
		t.setCol(len(t.lines[t.row]))
	case key.Matches(msg, k.Top):
		t.row = 0
		t.setCol(0)
	case key.Matches(msg, k.Bottom):
		t.row = len(t.lines) - 1
		t.setCol(len(t.lines[t.row]))
	}
	t.follow()
}

// insert puts s at the cursor; a newline in s (from a paste) splits the
// line. It is linear in the size of s, so pasting a large body is cheap.
func (t *TextArea) insert(s string) {
	parts := strings.Split(cleanText(s), "\n")
	line := t.lines[t.row]
	tail := append([]rune(nil), line[t.col:]...)
	first := make([]rune, 0, t.col+len(parts[0]))
	first = append(append(first, line[:t.col]...), []rune(parts[0])...)
	if len(parts) == 1 {
		t.lines[t.row] = append(first, tail...)
		t.setCol(len(first))
		return
	}
	added := make([][]rune, 0, len(parts)-1)
	for _, p := range parts[1:] {
		added = append(added, []rune(p))
	}
	last := len(added) - 1
	col := len(added[last])
	added[last] = append(added[last], tail...)
	t.lines[t.row] = first
	t.lines = append(t.lines[:t.row+1], append(added, t.lines[t.row+1:]...)...)
	t.row += len(added)
	t.setCol(col)
}

// newline splits the line at the cursor and keeps its indentation, one
// level deeper after an opening bracket.
func (t *TextArea) newline() {
	line := t.lines[t.row]
	indent := 0
	for indent < t.col && line[indent] == ' ' {
		indent++
	}
	if t.col > 0 && (line[t.col-1] == '{' || line[t.col-1] == '[') {
		indent += tabWidth
	}
	t.insert("\n" + strings.Repeat(" ", indent))
}

func (t *TextArea) backspace() {
	switch {
	case t.col > 0:
		line := t.lines[t.row]
		t.lines[t.row] = append(line[:t.col-1], line[t.col:]...)
		t.setCol(t.col - 1)
	case t.row > 0:
		prev := t.lines[t.row-1]
		col := len(prev)
		t.lines[t.row-1] = append(prev, t.lines[t.row]...)
		t.lines = append(t.lines[:t.row], t.lines[t.row+1:]...)
		t.row--
		t.setCol(col)
	}
}

func (t *TextArea) deleteChar() {
	line := t.lines[t.row]
	switch {
	case t.col < len(line):
		t.lines[t.row] = append(line[:t.col], line[t.col+1:]...)
	case t.row < len(t.lines)-1:
		t.lines[t.row] = append(line, t.lines[t.row+1]...)
		t.lines = append(t.lines[:t.row+1], t.lines[t.row+2:]...)
	}
}

// deleteWord deletes back to the start of the word before the cursor, or
// joins the line with the previous one at its start.
func (t *TextArea) deleteWord() {
	if t.col == 0 {
		t.backspace()
		return
	}
	line := t.lines[t.row]
	i := t.col
	for i > 0 && line[i-1] == ' ' {
		i--
	}
	for i > 0 && line[i-1] != ' ' {
		i--
	}
	t.lines[t.row] = append(line[:i], line[t.col:]...)
	t.setCol(i)
}

func (t *TextArea) left1() {
	switch {
	case t.col > 0:
		t.setCol(t.col - 1)
	case t.row > 0:
		t.row--
		t.setCol(len(t.lines[t.row]))
	}
}

func (t *TextArea) right1() {
	switch {
	case t.col < len(t.lines[t.row]):
		t.setCol(t.col + 1)
	case t.row < len(t.lines)-1:
		t.row++
		t.setCol(0)
	}
}

// moveRow moves the cursor delta lines, aiming for the goal column.
func (t *TextArea) moveRow(delta int) {
	t.row = min(max(0, t.row+delta), len(t.lines)-1)
	t.col = min(t.goal, len(t.lines[t.row]))
}

// setCol moves the cursor within its line and makes that column the goal.
func (t *TextArea) setCol(col int) { t.col, t.goal = col, col }

// follow scrolls so the cursor is in view.
func (t *TextArea) follow() {
	if t.height > 0 {
		t.top = min(max(t.top, t.row-t.height+1), t.row)
	}
	t.top = max(0, min(t.top, len(t.lines)-1))
	// One cell is kept for the cursor after the end of a line.
	t.left = min(max(t.left, t.col-t.width+1), t.col)
}

// View renders the visible lines, colored as JSON when hl is set. The
// cursor's line is left plain so the cursor sits on the right cell.
func (t *TextArea) View(st *theme.Styles, hl *format.Highlighter) []string {
	end := min(t.top+t.height, len(t.lines))
	out := make([]string, 0, max(0, end-t.top))
	for i := t.top; i < end; i++ {
		line := t.lines[i]
		if t.focused && i == t.row {
			out = append(out, t.cursorLine(st, line))
			continue
		}
		s := format.Sanitize(string(line[min(t.left, len(line)):min(t.left+t.width, len(line))]))
		if hl != nil {
			s = hl.Line(s)
		}
		out = append(out, Fit(s, t.width, false))
	}
	return out
}

func (t *TextArea) cursorLine(st *theme.Styles, line []rune) string {
	from, to := min(t.left, len(line)), min(t.left+t.width, len(line))
	at := " "
	if t.col < len(line) {
		at = format.Sanitize(string(line[t.col]))
	}
	before := format.Sanitize(string(line[from:t.col]))
	after := ""
	if t.col+1 < to {
		after = format.Sanitize(string(line[t.col+1 : to]))
	}
	return Fit(before+st.Caret.Render(at)+after, t.width, false)
}

// cleanText turns "\r\n" and lone "\r" (which terminals paste for line
// breaks) into "\n", and tabs into spaces.
func cleanText(s string) string {
	if !strings.ContainsAny(s, "\r\t") {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.ReplaceAll(s, "\t", strings.Repeat(" ", tabWidth))
}
