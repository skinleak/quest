// Package request holds the request and response models shared by the
// engine, the store and the UI. It has no dependencies on either.
package request

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode"
)

// Methods are the supported HTTP methods, in the order the UI cycles them.
var Methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}

// KV is one query parameter or header. Disabled pairs are kept in the
// request but not sent.
type KV struct {
	Key      string
	Value    string
	Disabled bool
}

// Request is everything needed to send one HTTP request.
type Request struct {
	Method  string
	URL     string
	Params  []KV
	Headers []KV
	Body    string
	// Timeout bounds the whole exchange, body included. Zero means the
	// engine's default.
	Timeout time.Duration
	// NoRedirects returns a redirect response as is instead of following it.
	NoRedirects bool
}

// Errors returned by Validate.
var (
	ErrNoURL         = errors.New("url is empty")
	ErrMethod        = errors.New("unsupported method")
	ErrScheme        = errors.New("only http and https urls are supported")
	ErrNoHost        = errors.New("url has no host")
	ErrEmptyKey      = errors.New("empty name")
	ErrInvalidHeader = errors.New("invalid header")
	ErrPlaceholder   = errors.New("fill in")
)

// Validate reports the first problem that would stop r from being sent.
func (r Request) Validate() error {
	if !slices.Contains(Methods, r.Method) {
		return fmt.Errorf("%w: %q", ErrMethod, r.Method)
	}
	if _, err := r.FullURL(); err != nil {
		return err
	}
	for _, h := range r.Headers {
		if h.Disabled {
			continue
		}
		if h.Key == "" {
			return fmt.Errorf("header: %w", ErrEmptyKey)
		}
		if strings.ContainsAny(h.Key, " \t\r\n:") || strings.ContainsAny(h.Value, "\r\n") {
			return fmt.Errorf("%w: %q", ErrInvalidHeader, h.Key)
		}
	}
	for _, p := range r.Params {
		if !p.Disabled && p.Key == "" {
			return fmt.Errorf("param: %w", ErrEmptyKey)
		}
	}
	return nil
}

// FullURL parses the URL and appends the enabled params to its query.
// A URL without a scheme gets http://, like curl.
func (r Request) FullURL() (*url.URL, error) {
	raw := strings.TrimSpace(r.URL)
	if raw == "" {
		return nil, ErrNoURL
	}
	if p := placeholder(raw); p != "" {
		return nil, fmt.Errorf("%w %s", ErrPlaceholder, p)
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("%w: %q", ErrScheme, u.Scheme)
	}
	if u.Host == "" {
		return nil, ErrNoHost
	}
	// net/http sends the query as typed, and a space in it breaks the
	// request line (servers answer 400). The path is escaped already.
	u.RawQuery = strings.ReplaceAll(u.RawQuery, " ", "%20")
	if len(r.Params) == 0 {
		return u, nil
	}
	q := u.Query()
	for _, p := range r.Params {
		if !p.Disabled {
			q.Add(p.Key, p.Value)
		}
	}
	u.RawQuery = q.Encode()
	return u, nil
}

// placeholder returns the first placeholder left in a URL: a {{variable}}
// anywhere, or an OpenAPI path parameter such as {id} before the query.
// Sent as is, they would fail with a confusing parse error or a 404.
func placeholder(u string) string {
	if i := strings.Index(u, "{{"); i >= 0 {
		if j := strings.Index(u[i:], "}}"); j >= 0 {
			return u[i : i+j+2]
		}
	}
	path, _, _ := strings.Cut(u, "?")
	path, _, _ = strings.Cut(path, "#")
	for {
		i := strings.IndexByte(path, '{')
		if i < 0 {
			return ""
		}
		j := strings.IndexByte(path[i:], '}')
		if j < 0 {
			return ""
		}
		if name := path[i+1 : i+j]; name != "" && strings.IndexFunc(name, notNameRune) < 0 {
			return path[i : i+j+1]
		}
		path = path[i+1:]
	}
}

func notNameRune(r rune) bool {
	return r != '_' && r != '-' && r != '.' && !unicode.IsLetter(r) && !unicode.IsDigit(r)
}
