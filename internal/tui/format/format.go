// Package format turns a response body into display lines: JSON is
// pretty-printed and highlighted, anything else is shown as plain text.
//
// New does the expensive work (indenting, splitting, sanitizing) and runs
// inside a tea.Cmd. Highlighting is lazy and per chunk of lines: Highlight
// styles only the chunks a view is about to show, and each chunk once, so
// a multi-MB body costs the same per frame as a small one.
package format

import (
	"bytes"
	"encoding/json"
	"mime"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/skinleak/quest/internal/tui/theme"
)

const (
	// ChunkLines is how many lines Highlight styles at a time.
	ChunkLines = 256
	// maxLineBytes cuts very long lines, such as minified HTML: nothing
	// wider than a terminal is shown, and cut lines keep frames cheap.
	maxLineBytes = 4096
	// sniffBytes is how much of a body is checked for NUL bytes to tell
	// binary data from text.
	sniffBytes = 512
)

// Doc is a formatted body. Its methods are called from Update and View
// only, so it needs no locking; New may run on any goroutine.
type Doc struct {
	plain  []string // sanitized lines
	styled []string // highlighted lines, filled per chunk
	done   []bool   // chunks already highlighted
	json   bool
	paints paints
}

type paints struct {
	key, str, num, lit, punct theme.Paint
}

// New formats body. JSON (by content type, including +json types) is
// indented; JSON that doesn't parse is shown as plain text. Binary bodies
// become a one-line note instead of noise.
func New(body []byte, contentType string, st theme.Styles) *Doc {
	d := &Doc{paints: paints{
		key:   theme.PaintOf(st.JSONKey),
		str:   theme.PaintOf(st.JSONString),
		num:   theme.PaintOf(st.JSONNumber),
		lit:   theme.PaintOf(st.JSONLiteral),
		punct: theme.PaintOf(st.JSONPunct),
	}}
	switch {
	case len(body) == 0:
		return d
	case isBinary(body):
		d.plain = []string{"binary body, not shown"}
		return d
	case isJSON(contentType):
		var buf bytes.Buffer
		buf.Grow(len(body) + len(body)/2)
		if json.Indent(&buf, bytes.TrimSpace(body), "", "  ") == nil {
			d.json = true
			body = buf.Bytes()
		}
	}
	d.plain = splitLines(body)
	if d.json {
		d.styled = make([]string, len(d.plain))
		d.done = make([]bool, (len(d.plain)+ChunkLines-1)/ChunkLines)
	}
	return d
}

// Len is the number of lines.
func (d *Doc) Len() int { return len(d.plain) }

// IsJSON reports whether the body was parsed and indented as JSON.
func (d *Doc) IsJSON() bool { return d.json }

// Line returns line i, highlighted when its chunk has been.
func (d *Doc) Line(i int) string {
	if d.json && d.done[i/ChunkLines] {
		return d.styled[i]
	}
	return d.plain[i]
}

// Highlight styles every chunk that overlaps lines [from, to).
func (d *Doc) Highlight(from, to int) {
	if !d.json {
		return
	}
	from, to = max(0, from), min(to, len(d.plain))
	for c := from / ChunkLines; c*ChunkLines < to; c++ {
		if d.done[c] {
			continue
		}
		end := min((c+1)*ChunkLines, len(d.plain))
		var b strings.Builder
		for i := c * ChunkLines; i < end; i++ {
			b.Reset()
			d.paints.highlight(&b, d.plain[i])
			d.styled[i] = b.String()
		}
		d.done[c] = true
	}
}

// highlight colors one line of indented JSON. Indented JSON never splits
// a token across lines, so each line can be colored on its own, in any
// order: a string is a key when a colon follows it.
func (p paints) highlight(b *strings.Builder, line string) {
	b.Grow(len(line) + 32)
	for i := 0; i < len(line); {
		c := line[i]
		switch {
		case c == ' ':
			j := i + 1
			for j < len(line) && line[j] == ' ' {
				j++
			}
			b.WriteString(line[i:j])
			i = j
		case c == '"':
			j := stringEnd(line, i)
			if k := skipSpaces(line, j); k < len(line) && line[k] == ':' {
				p.key.Append(b, line[i:j])
			} else {
				p.str.Append(b, line[i:j])
			}
			i = j
		case c == '-' || (c >= '0' && c <= '9'):
			j := wordEnd(line, i)
			p.num.Append(b, line[i:j])
			i = j
		case c >= 'a' && c <= 'z':
			j := wordEnd(line, i)
			p.lit.Append(b, line[i:j])
			i = j
		default:
			// Punctuation, and the "…" of a cut line.
			_, n := utf8.DecodeRuneInString(line[i:])
			p.punct.Append(b, line[i:i+n])
			i += n
		}
	}
}

// stringEnd returns the index after the string starting at i, or the end
// of the line for a string cut off by maxLineBytes.
func stringEnd(s string, i int) int {
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case '"':
			return j + 1
		}
	}
	return len(s)
}

func skipSpaces(s string, i int) int {
	for i < len(s) && s[i] == ' ' {
		i++
	}
	return i
}

// wordEnd returns the end of a number or literal starting at i.
func wordEnd(s string, i int) int {
	for i < len(s) && !strings.ContainsRune(" ,:]}", rune(s[i])) {
		i++
	}
	return i
}

func isJSON(contentType string) bool {
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return mt == "application/json" || strings.HasSuffix(mt, "+json")
}

func isBinary(body []byte) bool {
	return bytes.IndexByte(body[:min(len(body), sniffBytes)], 0) >= 0
}

// splitLines splits body into sanitized lines, dropping one final newline.
func splitLines(body []byte) []string {
	body = bytes.TrimSuffix(body, []byte("\n"))
	out := make([]string, 0, bytes.Count(body, []byte("\n"))+1)
	for {
		line, rest, found := bytes.Cut(body, []byte("\n"))
		out = append(out, sanitizeLine(line))
		if !found {
			return out
		}
		body = rest
	}
}

// sanitizeLine cuts a line to maxLineBytes and makes it safe to print.
func sanitizeLine(line []byte) string {
	cut := len(line) > maxLineBytes
	if cut {
		line = line[:maxLineBytes]
	}
	s := Sanitize(string(line))
	if cut {
		s = strings.TrimRight(s, "�") + theme.IconEllipsis
	}
	return s
}

// Sanitize makes untrusted text safe and measurable on screen: tabs become
// four spaces, carriage returns are dropped, other control characters
// (including the C1 range, which some terminals act on) become "?", and
// invalid UTF-8 becomes U+FFFD. Without this, a response could move the
// cursor or recolor the terminal, and width math would break.
func Sanitize(s string) string {
	if utf8.ValidString(s) && strings.IndexFunc(s, unicode.IsControl) < 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteString("    ")
		case r == '\r':
		case unicode.IsControl(r):
			b.WriteByte('?')
		default:
			b.WriteRune(r) // invalid bytes decode to U+FFFD
		}
	}
	return b.String()
}
