package app

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/skinleak/quest/internal/core/engine"
)

func newTestModel(t testing.TB) Model {
	t.Helper()
	m := New(engine.New(engine.Options{}), slog.New(slog.DiscardHandler))
	return pump(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
}

// pump delivers msg and synchronously runs the resulting commands.
func pump(t testing.TB, m Model, msg tea.Msg) Model {
	t.Helper()
	next, cmd := m.Update(msg)
	return drain(t, next.(Model), cmd)
}

func drain(t testing.TB, m Model, cmd tea.Cmd) Model {
	t.Helper()
	for steps := 0; cmd != nil && steps < 8; steps++ {
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				m = drain(t, m, c)
			}
			return m
		}
		if msg == nil || msg == tea.Quit() {
			return m
		}
		var next tea.Model
		next, cmd = m.Update(msg)
		m = next.(Model)
	}
	return m
}

func keyPress(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "ctrl+b":
		return tea.KeyMsg{Type: tea.KeyCtrlB}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	case "ctrl+t":
		return tea.KeyMsg{Type: tea.KeyCtrlT}
	case "f1":
		return tea.KeyMsg{Type: tea.KeyF1}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func typeText(t testing.TB, m Model, s string) Model {
	t.Helper()
	for _, r := range s {
		m = pump(t, m, keyPress(string(r)))
	}
	return m
}

func plainView(m Model) string { return ansi.Strip(m.View()) }

func jsonServer(t testing.TB, body string) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", "abc123")
		io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestSendShowsResponse(t *testing.T) {
	srv := jsonServer(t, `{"id":1,"name":"ada"}`)
	m := newTestModel(t)
	m = typeText(t, m, srv.URL+"/users/1")
	m = pump(t, m, keyPress("enter"))

	v := plainView(m)
	for _, want := range []string{"200 OK", `"name": "ada"`, "21 B", "body  headers  timing"} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q:\n%s", want, v)
		}
	}

	m = pump(t, m, keyPress("tab")) // response pane
	m = pump(t, m, keyPress("l"))
	if v := plainView(m); !strings.Contains(v, "X-Request-Id") || !strings.Contains(v, "abc123") {
		t.Errorf("headers tab:\n%s", v)
	}
	m = pump(t, m, keyPress("l"))
	if v := plainView(m); !strings.Contains(v, "total") || !strings.Contains(v, "connect") {
		t.Errorf("timing tab:\n%s", v)
	}
}

func TestMethodCycles(t *testing.T) {
	m := newTestModel(t)
	m = pump(t, m, keyPress("ctrl+t"))
	if r := m.editor.Request(); r.Method != http.MethodPost {
		t.Errorf("method = %s, want POST", r.Method)
	}
}

func TestTypingDoesNotTriggerKeys(t *testing.T) {
	m := newTestModel(t)
	m = typeText(t, m, "q?jk")
	if m.showHelp {
		t.Error("? opened help while typing a URL")
	}
	if got := m.editor.Request().URL; got != "q?jk" {
		t.Errorf("url = %q", got)
	}
}

func TestInvalidURLShowsError(t *testing.T) {
	m := newTestModel(t)
	m = typeText(t, m, "ftp://x")
	m = pump(t, m, keyPress("enter"))
	if v := plainView(m); !strings.Contains(v, "only http and https") {
		t.Errorf("no error in footer:\n%s", v)
	}
	if m.response.Sending() {
		t.Error("invalid request was sent")
	}
}

func TestCancel(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	m := newTestModel(t)
	m = typeText(t, m, srv.URL)
	next, cmd := m.Update(keyPress("enter"))
	m = next.(Model)
	if !m.response.Sending() {
		t.Fatal("not sending")
	}
	done := make(chan tea.Msg)
	go func() { done <- cmd() }()

	m = pump(t, m, keyPress("esc"))
	select {
	case msg := <-done:
		m = pump(t, m, msg)
	case <-time.After(2 * time.Second):
		t.Fatal("cancel did not stop the request")
	}
	if m.response.Sending() {
		t.Error("still sending after cancel")
	}
	if v := plainView(m); !strings.Contains(v, "request cancelled") || !strings.Contains(v, "nothing sent yet") {
		t.Errorf("after cancel:\n%s", v)
	}
}

func TestStaleResponseIsDropped(t *testing.T) {
	srv := jsonServer(t, `{"n":1}`)
	m := newTestModel(t)
	m = typeText(t, m, srv.URL)
	next, first := m.Update(keyPress("enter"))
	m = next.(Model)
	next, second := m.Update(keyPress("enter"))
	m = next.(Model)

	m = pump(t, m, first()) // replaced by the second: ignored
	if !m.response.Sending() {
		t.Fatal("stale response was applied")
	}
	m = pump(t, m, second())
	if m.response.Sending() {
		t.Fatal("current response was not applied")
	}
}

func TestSidebarToggle(t *testing.T) {
	m := newTestModel(t)
	if !strings.Contains(plainView(m), "collections") {
		t.Fatal("sidebar hidden in a wide window")
	}
	m = pump(t, m, keyPress("ctrl+b"))
	if strings.Contains(plainView(m), "collections") {
		t.Error("ctrl+b did not hide the sidebar")
	}
	m = pump(t, m, tea.WindowSizeMsg{Width: 60, Height: 20})
	m = pump(t, m, keyPress("ctrl+b"))
	v := plainView(m)
	if !strings.Contains(v, "collections") || strings.Contains(v, "request") {
		t.Errorf("narrow sidebar should cover the panes:\n%s", v)
	}
}

func TestViewFitsWindow(t *testing.T) {
	srv := jsonServer(t, `{"items":[`+strings.Repeat(`{"id":1,"name":"a long name that will not fit a narrow window"},`, 200)+`{}]}`)
	base := newTestModel(t)
	base = typeText(t, base, srv.URL+"/a/very/long/path/that/does/not/fit/anywhere?with=a&query=string")
	loaded := pump(t, base, keyPress("enter"))

	modes := map[string]func() Model{
		"empty":    func() Model { return base },
		"help":     func() Model { return pump(t, pump(t, base, keyPress("tab")), keyPress("?")) },
		"body":     func() Model { return pump(t, loaded, keyPress("tab")) },
		"scrolled": func() Model { return pump(t, pump(t, loaded, keyPress("tab")), keyPress("G")) },
		"headers":  func() Model { return pump(t, pump(t, loaded, keyPress("tab")), keyPress("l")) },
		"timing":   func() Model { return pump(t, pump(t, loaded, keyPress("tab")), keyPress("h")) },
		"sidebar":  func() Model { return pump(t, base, keyPress("shift+tab")) },
		"error": func() Model {
			m := typeText(t, newTestModel(t), "http://127.0.0.1:1/")
			return pump(t, m, keyPress("enter"))
		},
	}
	for _, size := range [][2]int{{100, 30}, {80, 24}, {60, 20}, {200, 60}, {40, 10}, {39, 10}, {30, 8}, {10, 2}, {1, 1}} {
		for name, mode := range modes {
			m := pump(t, mode(), tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			lines := strings.Split(plainView(m), "\n")
			if len(lines) != size[1] {
				t.Errorf("%dx%d %s: %d lines, want %d", size[0], size[1], name, len(lines), size[1])
			}
			for i, l := range lines {
				if w := ansi.StringWidth(l); w > size[0] {
					t.Errorf("%dx%d %s: line %d is %d wide: %q", size[0], size[1], name, i, w, l)
				}
			}
		}
	}
}

func TestHelpShowsEveryKey(t *testing.T) {
	m := newTestModel(t)
	m = pump(t, m, keyPress("f1"))
	if !m.showHelp {
		t.Fatal("f1 did not open help")
	}
	seen := plainView(m)
	for range 20 {
		m = pump(t, m, keyPress("j"))
		seen += plainView(m)
	}
	for _, g := range m.keys.FullHelp() {
		for _, b := range g {
			if !strings.Contains(seen, b.Help().Desc) {
				t.Errorf("help never shows %q", b.Help().Desc)
			}
		}
	}
	m = pump(t, m, keyPress("esc"))
	if m.showHelp {
		t.Error("esc did not close help")
	}
}
