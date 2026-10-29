package engine

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/skinleak/quest/internal/core/request"
)

// echo answers with the request's method, query, selected headers and
// body, so tests can see exactly what was sent.
func echo(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	w.Header().Set("Content-Type", "text/plain")
	io.WriteString(w, r.Method+" "+r.URL.RawQuery+"|"+r.Header.Get("Content-Type")+"|"+
		r.Header.Get("User-Agent")+"|"+r.Header.Get("X-Test")+"|"+string(body))
}

func TestDo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(echo))
	defer srv.Close()

	tests := []struct {
		name string
		req  request.Request
		want string
	}{
		{"get", request.Request{Method: "GET", URL: srv.URL}, "GET ||quest||"},
		{"params", request.Request{Method: "GET", URL: srv.URL + "?a=1", Params: []request.KV{{Key: "b", Value: "2"}}}, "GET a=1&b=2||quest||"},
		{"headers", request.Request{Method: "DELETE", URL: srv.URL, Headers: []request.KV{
			{Key: "X-Test", Value: "yes"}, {Key: "X-Off", Value: "no", Disabled: true}, {Key: "User-Agent", Value: "me"},
		}}, "DELETE ||me|yes|"},
		{"json body", request.Request{Method: "POST", URL: srv.URL, Body: `{"a":1}`}, `POST |application/json|quest||{"a":1}`},
		{"text body", request.Request{Method: "PUT", URL: srv.URL, Body: "hi"}, "PUT |text/plain; charset=utf-8|quest||hi"},
		{"explicit content type", request.Request{Method: "PATCH", URL: srv.URL, Body: "{}",
			Headers: []request.KV{{Key: "Content-Type", Value: "application/x-custom"}}}, "PATCH |application/x-custom|quest||{}"},
	}
	e := New(Options{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := e.Do(context.Background(), tt.req)
			if err != nil {
				t.Fatal(err)
			}
			if got := string(resp.Body); got != tt.want {
				t.Errorf("body = %q, want %q", got, tt.want)
			}
			if resp.StatusCode != http.StatusOK || resp.Status != "200 OK" || resp.ContentType != "text/plain" {
				t.Errorf("status %d %q, content type %q", resp.StatusCode, resp.Status, resp.ContentType)
			}
			if resp.Size != int64(len(resp.Body)) || resp.Timing.Total <= 0 {
				t.Errorf("size %d, total %v", resp.Size, resp.Timing.Total)
			}
		})
	}
}

func TestDoHeadAndOptions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(echo))
	defer srv.Close()
	e := New(Options{})
	for _, m := range []string{"HEAD", "OPTIONS"} {
		resp, err := e.Do(context.Background(), request.Request{Method: m, URL: srv.URL})
		if err != nil {
			t.Fatal(m, err)
		}
		if m == "HEAD" && len(resp.Body) != 0 {
			t.Errorf("HEAD returned a body: %q", resp.Body)
		}
	}
}

func TestRedirects(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/end", http.StatusFound) })
	mux.HandleFunc("/end", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "end") })
	mux.HandleFunc("/loop", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/loop", http.StatusFound) })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	e := New(Options{MaxRedirects: 3})

	resp, err := e.Do(context.Background(), request.Request{Method: "GET", URL: srv.URL + "/start"})
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.Body) != "end" || resp.URL != srv.URL+"/end" {
		t.Errorf("followed: body %q, url %q", resp.Body, resp.URL)
	}

	resp, err = e.Do(context.Background(), request.Request{Method: "GET", URL: srv.URL + "/start", NoRedirects: true})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Errorf("not followed: status %d, want 302", resp.StatusCode)
	}

	_, err = e.Do(context.Background(), request.Request{Method: "GET", URL: srv.URL + "/loop"})
	if !errors.Is(err, ErrTooManyRedirects) {
		t.Errorf("loop: err = %v, want ErrTooManyRedirects", err)
	}
}

func TestBodyLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, strings.Repeat("x", 100))
	}))
	defer srv.Close()
	for _, tt := range []struct {
		limit     int64
		size      int
		truncated bool
	}{{10, 10, true}, {100, 100, false}, {1000, 100, false}} {
		resp, err := New(Options{MaxBodyBytes: tt.limit}).Do(context.Background(), request.Request{Method: "GET", URL: srv.URL})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Body) != tt.size || resp.Truncated != tt.truncated {
			t.Errorf("limit %d: %d bytes, truncated %v; want %d, %v", tt.limit, len(resp.Body), resp.Truncated, tt.size, tt.truncated)
		}
	}
}

func TestTimeoutAndCancel(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)
	e := New(Options{})

	_, err := e.Do(context.Background(), request.Request{Method: "GET", URL: srv.URL, Timeout: 50 * time.Millisecond})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("timeout: err = %v, want DeadlineExceeded", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	start := time.Now()
	_, err = e.Do(ctx, request.Request{Method: "GET", URL: srv.URL})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("cancel: err = %v, want Canceled", err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("cancel took %v", d)
	}
}

func TestErrors(t *testing.T) {
	e := New(Options{})
	if _, err := e.Do(context.Background(), request.Request{Method: "GET"}); !errors.Is(err, request.ErrNoURL) {
		t.Errorf("empty url: %v", err)
	}
	// Port 1 on loopback is closed; the error must not repeat the method
	// and URL the way *url.Error does.
	_, err := e.Do(context.Background(), request.Request{Method: "GET", URL: "http://127.0.0.1:1/x"})
	if err == nil || strings.Contains(err.Error(), "Get ") {
		t.Errorf("refused: err = %v", err)
	}
}

func TestTiming(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(20 * time.Millisecond)
		io.WriteString(w, "ok")
	}))
	defer srv.Close()
	e := New(Options{})
	e.transport.TLSClientConfig = srv.Client().Transport.(*http.Transport).TLSClientConfig

	resp, err := e.Do(context.Background(), request.Request{Method: "GET", URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	tm := resp.Timing
	if tm.Connect <= 0 || tm.TLS <= 0 || tm.TTFB < 20*time.Millisecond || tm.Reused {
		t.Errorf("first request: %+v", tm)
	}
	if sum := tm.DNS + tm.Connect + tm.TLS + tm.TTFB + tm.Transfer; sum > tm.Total {
		t.Errorf("phases %v exceed total %v", sum, tm.Total)
	}

	resp, err = e.Do(context.Background(), request.Request{Method: "GET", URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if tm := resp.Timing; !tm.Reused || tm.Connect != 0 || tm.TLS != 0 {
		t.Errorf("second request should reuse the connection: %+v", tm)
	}
}
