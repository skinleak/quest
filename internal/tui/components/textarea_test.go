package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/skinleak/quest/internal/tui/theme"
)

func typeInto(t *TextArea, keys ...tea.KeyMsg) {
	for _, k := range keys {
		t.Update(k)
	}
}

func runes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func TestTextAreaEditing(t *testing.T) {
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	bs := tea.KeyMsg{Type: tea.KeyBackspace}
	tests := []struct {
		name string
		init string
		keys []tea.KeyMsg
		want string
	}{
		{"type", "", []tea.KeyMsg{runes("ab"), {Type: tea.KeySpace}, runes("c")}, "ab c"},
		{"indent after bracket", "", []tea.KeyMsg{runes("{"), enter, runes(`"a": [`), enter, runes("1")}, "{\n  \"a\": [\n    1"},
		{"keeps indent", "  x", []tea.KeyMsg{{Type: tea.KeyEnd}, enter, runes("y")}, "  x\n  y"},
		{"backspace joins", "a\nb", []tea.KeyMsg{{Type: tea.KeyDown}, {Type: tea.KeyHome}, bs}, "ab"},
		{"delete joins", "a\nb", []tea.KeyMsg{{Type: tea.KeyEnd}, {Type: tea.KeyDelete}}, "ab"},
		{"delete word", "one two  ", []tea.KeyMsg{{Type: tea.KeyEnd}, {Type: tea.KeyCtrlW}}, "one "},
		{"kill to end", "abc", []tea.KeyMsg{{Type: tea.KeyRight}, {Type: tea.KeyCtrlK}}, "a"},
		{"paste with cr and tab", "[]", []tea.KeyMsg{{Type: tea.KeyRight}, runes("\r\t1,\r\n\t2\r")}, "[\n  1,\n  2\n]"},
		{"left wraps", "a\nb", []tea.KeyMsg{{Type: tea.KeyDown}, {Type: tea.KeyLeft}, runes("!")}, "a!\nb"},
		{"goal column", "abcd\n\nabcd", []tea.KeyMsg{{Type: tea.KeyEnd}, {Type: tea.KeyDown}, {Type: tea.KeyDown}, runes("!")}, "abcd\n\nabcd!"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ta := NewTextArea()
			ta.SetValue(tt.init)
			ta.SetSize(20, 5)
			ta.Focus()
			typeInto(&ta, tt.keys...)
			if got := ta.Value(); got != tt.want {
				t.Errorf("value = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTextAreaViewFits(t *testing.T) {
	ta := NewTextArea()
	ta.SetValue(strings.Repeat(strings.Repeat("é", 50)+"\n", 100) + "\x1b[31m")
	ta.SetSize(10, 4)
	ta.Focus()
	typeInto(&ta, tea.KeyMsg{Type: tea.KeyCtrlEnd}, tea.KeyMsg{Type: tea.KeyUp}, tea.KeyMsg{Type: tea.KeyEnd})
	lines := ta.View(theme.Default(), nil)
	if len(lines) != 4 {
		t.Fatalf("%d lines, want 4", len(lines))
	}
	for _, l := range lines {
		if w := ansi.StringWidth(l); w > 10 {
			t.Errorf("line %q is %d wide", l, w)
		}
		if strings.Contains(l, "\x1b[31m") {
			t.Errorf("escape sequence not sanitized: %q", l)
		}
	}
	// The cursor sits after the last é of line 99, in view.
	if got := ansi.Strip(lines[2]); got != strings.Repeat("é", 9)+" " {
		t.Errorf("cursor line = %q", got)
	}
}

func TestTextAreaLooksLikeJSON(t *testing.T) {
	for in, want := range map[string]bool{"": false, "\n  {": true, "[1]": true, "a=1": false} {
		ta := NewTextArea()
		ta.SetValue(in)
		if got := ta.LooksLikeJSON(); got != want {
			t.Errorf("LooksLikeJSON(%q) = %v", in, got)
		}
	}
}
