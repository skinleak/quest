// Package engine sends requests with net/http and measures their phases.
// It knows nothing about the UI: everything runs under the caller's
// context, so a cancelled context aborts the exchange at once.
package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"time"

	"github.com/skinleak/quest/internal/core/request"
)

// Defaults for Options fields left at zero.
const (
	DefaultTimeout      = 30 * time.Second
	DefaultMaxBodyBytes = 10 << 20
	DefaultMaxRedirects = 10
	userAgent           = "quest"
)

// Options configures an Engine. Zero fields take the defaults above.
type Options struct {
	Timeout      time.Duration
	MaxBodyBytes int64
	MaxRedirects int
}

// Engine sends requests. It is safe for concurrent use; its connection
// pool is shared by all requests.
type Engine struct {
	opts      Options
	transport *http.Transport
}

// New returns an engine with its own connection pool.
func New(opts Options) *Engine {
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultTimeout
	}
	if opts.MaxBodyBytes <= 0 {
		opts.MaxBodyBytes = DefaultMaxBodyBytes
	}
	if opts.MaxRedirects <= 0 {
		opts.MaxRedirects = DefaultMaxRedirects
	}
	t := http.DefaultTransport.(*http.Transport).Clone()
	return &Engine{opts: opts, transport: t}
}

// ErrTooManyRedirects is returned when a request redirects more often
// than Options.MaxRedirects allows.
var ErrTooManyRedirects = errors.New("too many redirects")

// Do validates and sends r and reads its response body, up to the body
// limit. The returned error wraps context.Canceled when ctx was cancelled
// and context.DeadlineExceeded when the request timed out.
func (e *Engine) Do(ctx context.Context, r request.Request) (*request.Response, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	u, err := r.FullURL()
	if err != nil {
		return nil, err
	}

	timeout := r.Timeout
	if timeout <= 0 {
		timeout = e.opts.Timeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var tr tracer
	ctx = httptrace.WithClientTrace(ctx, tr.trace())

	var body io.Reader
	if r.Body != "" {
		body = strings.NewReader(r.Body)
	}
	req, err := http.NewRequestWithContext(ctx, r.Method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	setHeaders(req, r)

	client := &http.Client{Transport: e.transport, CheckRedirect: e.redirectPolicy(r.NoRedirects)}
	tr.start = time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return nil, unwrapURLError(err)
	}
	defer resp.Body.Close()

	data, truncated, err := readCapped(resp.Body, e.opts.MaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return &request.Response{
		Status:      resp.Status,
		StatusCode:  resp.StatusCode,
		Proto:       resp.Proto,
		Header:      resp.Header,
		Body:        data,
		Truncated:   truncated,
		Size:        int64(len(data)),
		ContentType: resp.Header.Get("Content-Type"),
		URL:         resp.Request.URL.String(),
		Timing:      tr.timing(time.Now()),
	}, nil
}

// setHeaders copies the enabled headers and fills in a User-Agent and,
// for a body without one, a Content-Type: JSON when it parses, else text.
func setHeaders(req *http.Request, r request.Request) {
	for _, h := range r.Headers {
		if !h.Disabled {
			req.Header.Add(h.Key, h.Value)
		}
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", userAgent)
	}
	if r.Body != "" && req.Header.Get("Content-Type") == "" {
		ct := "text/plain; charset=utf-8"
		if json.Valid([]byte(r.Body)) {
			ct = "application/json"
		}
		req.Header.Set("Content-Type", ct)
	}
}

func (e *Engine) redirectPolicy(off bool) func(*http.Request, []*http.Request) error {
	return func(_ *http.Request, via []*http.Request) error {
		if off {
			return http.ErrUseLastResponse
		}
		if len(via) > e.opts.MaxRedirects {
			return fmt.Errorf("%w (%d)", ErrTooManyRedirects, e.opts.MaxRedirects)
		}
		return nil
	}
}

// readCapped reads up to limit bytes and reports whether there was more.
func readCapped(r io.Reader, limit int64) ([]byte, bool, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) > limit {
		return data[:limit], true, nil
	}
	return data, false, nil
}

// unwrapURLError drops the *url.Error wrapper net/http adds, whose message
// repeats the method and the whole URL, and keeps the cause.
func unwrapURLError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
		return fmt.Errorf("host not found: %s", dnsErr.Name)
	}
	return err
}
