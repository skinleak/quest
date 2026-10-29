package components

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/skinleak/quest/internal/core/request"
	"github.com/skinleak/quest/internal/tui/theme"
)

func plain(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = ansi.Strip(l)
	}
	return out
}

func TestHeaderLinesWrapLongValues(t *testing.T) {
	h := http.Header{
		"Set-Cookie": {"session=abc; Path=/; HttpOnly; Secure; SameSite=Lax"},
		"X-Id":       {"1"},
	}
	got := plain(headerLines(theme.Default(), h, 36))
	want := []string{
		"Set-Cookie  session=abc; Path=/;",
		"            HttpOnly; Secure;",
		"            SameSite=Lax",
		"X-Id        1",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestHeaderLinesCutLongNames(t *testing.T) {
	long := strings.Repeat("X", maxHeaderName+10)
	h := http.Header{long: {"a"}, "Y": {"b"}}
	for _, l := range plain(headerLines(theme.Default(), h, 80)) {
		// Every value starts in the same column, even after a long name.
		if col := ansi.StringWidth(l[:strings.LastIndex(l, " ")+1]); col != maxHeaderName+2 {
			t.Errorf("value of %q starts at %d, want %d", l, col, maxHeaderName+2)
		}
	}
}

func TestTimingLines(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	resp := &request.Response{
		Proto:     "HTTP/2.0",
		Remote:    "93.184.216.34:443",
		URL:       "https://example.com/end",
		Redirects: 2,
		TLS: &request.TLS{
			Version: "TLS 1.3",
			Subject: "example.com",
			Issuer:  "Let's Encrypt",
			Expires: now.Add(10*24*time.Hour + time.Hour),
		},
		Timing: request.Timing{TTFB: 80 * time.Millisecond, Transfer: 20 * time.Millisecond, Total: 100 * time.Millisecond, Reused: true},
	}
	lines := plain(timingLines(theme.Default(), resp, 80, now))
	got := strings.Join(lines, "\n")
	for _, want := range []string{
		"wait", "transfer", "total",
		"server     93.184.216.34:443 · reused connection",
		"protocol   HTTP/2.0 · TLS 1.3",
		"cert       example.com",
		"issuer     Let's Encrypt",
		"expires    2026-11-08 · in 10 days",
		"redirects  2 → https://example.com/end",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	// A reused connection had no dns, connect or tls phase: no rows for them.
	for _, l := range lines {
		for _, unwanted := range []string{"dns ", "connect ", "tls "} {
			if strings.HasPrefix(l, unwanted) {
				t.Errorf("row %q for a phase that didn't happen", l)
			}
		}
	}
}

func TestExpiry(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	st := theme.Default()
	for _, tt := range []struct {
		in   time.Duration
		want string
	}{
		{-time.Hour, "expired"},
		{time.Hour, "today"},
		{25 * time.Hour, "in 1 day"},
		{400 * 24 * time.Hour, "in 400 days"},
	} {
		if got := ansi.Strip(expiry(st, now.Add(tt.in), now)); got != tt.want {
			t.Errorf("expiry(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
