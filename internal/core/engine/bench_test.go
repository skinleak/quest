package engine

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/skinleak/quest/internal/core/request"
)

// BenchmarkDo measures the engine's own overhead per request (tracing,
// header setup, capped read) against a local server on a pooled
// connection, with a 64 KiB body.
func BenchmarkDo(b *testing.B) {
	body := strings.Repeat("x", 64<<10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, body)
	}))
	defer srv.Close()
	e := New(Options{})
	r := request.Request{Method: "GET", URL: srv.URL, Headers: []request.KV{{Key: "Accept", Value: "application/json"}}}
	for b.Loop() {
		if _, err := e.Do(context.Background(), r); err != nil {
			b.Fatal(err)
		}
	}
}
