package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// BenchmarkScrollFrame is a keypress that scrolls a ~2 MiB JSON response
// plus the full frame at 220×60: the work on the UI goroutine per key.
func BenchmarkScrollFrame(b *testing.B) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI)
	b.Cleanup(func() { lipgloss.SetColorProfile(prev) })

	item := `{"id":12345,"name":"a fairly long user name","email":"user@example.com","active":true,"tags":["a","b","c"],"manager":null},`
	srv := jsonServer(b, "["+strings.Repeat(item, 2<<20/len(item))+"{}]")
	m := newTestModel(b)
	m = typeText(b, m, srv.URL)
	m = pump(b, m, keyPress("enter"))
	m = pump(b, m, tea.WindowSizeMsg{Width: 220, Height: 60})
	m = pump(b, m, keyPress("tab"))
	down, up := keyPress("ctrl+d"), keyPress("ctrl+u")
	if v := plainView(m); !strings.Contains(v, "200 OK") || !strings.Contains(v, "MB") {
		b.Fatalf("response not loaded:\n%s", v)
	}

	i := 0
	for b.Loop() {
		k := down
		if i%400 >= 200 {
			k = up
		}
		i++
		next, _ := m.Update(k)
		m = next.(Model)
		_ = m.View()
	}
}
