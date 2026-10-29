package format

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/skinleak/quest/internal/tui/theme"
)

// withColors makes Lip Gloss emit colors; without a terminal it renders
// none, and every style assertion would pass trivially.
func withColors(t testing.TB) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })
}

func allLines(d *Doc) []string {
	d.Highlight(0, d.Len())
	out := make([]string, d.Len())
	for i := range out {
		out[i] = d.Line(i)
	}
	return out
}

func TestNew(t *testing.T) {
	tests := []struct {
		name, body, ct string
		json           bool
		want           []string
	}{
		{"json", `{"a":1,"b":[true,null]}`, "application/json; charset=utf-8", true,
			[]string{"{", `  "a": 1,`, `  "b": [`, "    true,", "    null", "  ]", "}"}},
		{"json suffix", `{"a":1}`, "application/problem+json", true, []string{"{", `  "a": 1`, "}"}},
		{"invalid json is text", `{"a":`, "application/json", false, []string{`{"a":`}},
		{"text", "one\r\ntwo\n", "text/plain", false, []string{"one", "two"}},
		{"json as text", ` {"a":1}`, "text/plain; charset=utf-8", true, []string{"{", `  "a": 1`, "}"}},
		{"json untyped", `[1]`, "", true, []string{"[", "  1", "]"}},
		{"text that is not json", `[not json`, "text/plain", false, []string{`[not json`}},
		{"scalar text stays text", `42`, "text/plain", false, []string{`42`}},
		{"json-like html stays html", `{"a":1}`, "text/html", false, []string{`{"a":1}`}},
		{"blank lines", "a\n\nb\n\n", "", false, []string{"a", "", "b", ""}},
		{"controls", "a\x1b[31mred\tb\u009b", "", false, []string{"a?[31mred    b?"}},
		{"invalid utf8", "a\xffb", "", false, []string{"a�b"}},
		{"binary", "\x89PNG\x00\x00", "image/png", false, []string{"binary body, not shown"}},
		{"empty", "", "application/json", false, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := New([]byte(tt.body), tt.ct, theme.Default())
			if d.IsJSON() != tt.json {
				t.Errorf("IsJSON = %v, want %v", d.IsJSON(), tt.json)
			}
			got := allLines(d)
			if strings.Join(got, "\n") != strings.Join(tt.want, "\n") || len(got) != len(tt.want) {
				t.Errorf("lines = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLongLinesAreCut(t *testing.T) {
	d := New([]byte(strings.Repeat("é", maxLineBytes)), "", theme.Default())
	l := d.Line(0)
	if len(l) > maxLineBytes+len(theme.IconEllipsis) || !strings.HasSuffix(l, theme.IconEllipsis) || strings.ContainsRune(l, '�') {
		t.Errorf("cut line: %d bytes, ends %q", len(l), l[len(l)-8:])
	}
}

func TestHighlight(t *testing.T) {
	withColors(t)
	st := theme.Default()
	d := New([]byte(`{"key":"val: ue","n":-1.5e3,"ok":false,"esc":"a\"b:"}`), "application/json", st)
	got := allLines(d)
	key, str := theme.PaintOf(st.JSONKey), theme.PaintOf(st.JSONString)
	num, lit := theme.PaintOf(st.JSONNumber), theme.PaintOf(st.JSONLiteral)
	if key.Render("x") == "x" {
		t.Fatal("colors are off; every assertion below would pass trivially")
	}
	for _, want := range []struct {
		line int
		s    string
	}{
		{1, key.Render(`"key"`)},
		{1, str.Render(`"val: ue"`)},
		{2, num.Render("-1.5e3")},
		{3, lit.Render("false")},
		{4, key.Render(`"esc"`)},
		{4, str.Render(`"a\"b:"`)},
	} {
		if !strings.Contains(got[want.line], want.s) {
			t.Errorf("line %d = %q, missing %q", want.line, got[want.line], want.s)
		}
	}
	// Highlighting only adds styling.
	for i, l := range got {
		if ansi.Strip(l) != d.plain[i] {
			t.Errorf("line %d changed text: %q vs %q", i, ansi.Strip(l), d.plain[i])
		}
	}
}

func TestHighlightIsLazy(t *testing.T) {
	d := New(bigJSON(2000), "application/json", theme.Default())
	if d.Len() < 3*ChunkLines {
		t.Fatalf("only %d lines", d.Len())
	}
	d.Highlight(ChunkLines+10, ChunkLines+20)
	for c, done := range d.done {
		if done != (c == 1) {
			t.Errorf("chunk %d highlighted = %v", c, done)
		}
	}
	d.Highlight(ChunkLines-1, ChunkLines+1) // straddles chunks 0 and 1
	if !d.done[0] || d.done[2] {
		t.Errorf("done = %v", d.done[:3])
	}
}

func bigJSON(items int) []byte {
	type item struct {
		ID     int      `json:"id"`
		Name   string   `json:"name"`
		Active bool     `json:"active"`
		Score  float64  `json:"score"`
		Tags   []string `json:"tags"`
		Parent *int     `json:"parent"`
	}
	all := make([]item, items)
	for i := range all {
		all[i] = item{i, fmt.Sprintf("user %d", i), i%2 == 0, 1.5 * float64(i), []string{"a", "b"}, nil}
	}
	b, _ := json.Marshal(all)
	return b
}

// BenchmarkNew formats about 1 MiB of compact JSON: indenting, splitting
// and sanitizing, as done in the background for every response.
func BenchmarkNew(b *testing.B) {
	body := bigJSON(12000)
	st := theme.Default()
	b.SetBytes(int64(len(body)))
	for b.Loop() {
		New(body, "application/json", st)
	}
}

// BenchmarkHighlightChunk is the work a scroll into a new chunk adds on
// the UI goroutine.
func BenchmarkHighlightChunk(b *testing.B) {
	withColors(b)
	d := New(bigJSON(12000), "application/json", theme.Default())
	for b.Loop() {
		d.done[0] = false
		d.Highlight(0, ChunkLines)
	}
}
