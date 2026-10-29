package request

import (
	"errors"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		req  Request
		want error
	}{
		{"ok", Request{Method: "GET", URL: "https://example.com"}, nil},
		{"no scheme", Request{Method: "GET", URL: "localhost:8080/users"}, nil},
		{"empty url", Request{Method: "GET", URL: "  "}, ErrNoURL},
		{"bad method", Request{Method: "FETCH", URL: "https://example.com"}, ErrMethod},
		{"lowercase method", Request{Method: "get", URL: "https://example.com"}, ErrMethod},
		{"ftp", Request{Method: "GET", URL: "ftp://example.com"}, ErrScheme},
		{"no host", Request{Method: "GET", URL: "http://"}, ErrNoHost},
		{"empty header name", Request{Method: "GET", URL: "x.io", Headers: []KV{{Value: "v"}}}, ErrEmptyKey},
		{"disabled empty header", Request{Method: "GET", URL: "x.io", Headers: []KV{{Value: "v", Disabled: true}}}, nil},
		{"header name with colon", Request{Method: "GET", URL: "x.io", Headers: []KV{{Key: "a:b"}}}, ErrInvalidHeader},
		{"header value with newline", Request{Method: "GET", URL: "x.io", Headers: []KV{{Key: "a", Value: "1\r\nX: y"}}}, ErrInvalidHeader},
		{"empty param name", Request{Method: "GET", URL: "x.io", Params: []KV{{Value: "v"}}}, ErrEmptyKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.want == nil && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("Validate() = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestFullURL(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		params []KV
		want   string
	}{
		{"plain", "https://example.com/a", nil, "https://example.com/a"},
		{"adds http", "example.com/a", nil, "http://example.com/a"},
		{"trims", "  https://example.com  ", nil, "https://example.com"},
		{"appends params", "https://x.io/s?q=1", []KV{{Key: "page", Value: "2"}}, "https://x.io/s?page=2&q=1"},
		{"repeats keys", "https://x.io", []KV{{Key: "a", Value: "1"}, {Key: "a", Value: "2"}}, "https://x.io?a=1&a=2"},
		{"skips disabled", "https://x.io", []KV{{Key: "a", Value: "1", Disabled: true}}, "https://x.io"},
		{"escapes", "https://x.io", []KV{{Key: "q", Value: "a b&c"}}, "https://x.io?q=a+b%26c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := Request{URL: tt.url, Params: tt.params}.FullURL()
			if err != nil {
				t.Fatal(err)
			}
			if got := u.String(); got != tt.want {
				t.Errorf("FullURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
