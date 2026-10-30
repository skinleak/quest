package components

import (
	"net/http"
	"slices"
	"testing"

	"github.com/skinleak/quest/internal/core/request"
	"github.com/skinleak/quest/internal/tui/theme"
)

func TestParseRow(t *testing.T) {
	tests := []struct {
		in   string
		tab  int
		want request.KV
	}{
		{"Authorization: Bearer a:b", ReqHeaders, request.KV{Key: "Authorization", Value: "Bearer a:b"}},
		{"X-Empty", ReqHeaders, request.KV{Key: "X-Empty"}},
		{"q=a b&c#d", ReqParams, request.KV{Key: "q", Value: "a%20b%26c%23d"}},
		{"a=b=c", ReqParams, request.KV{Key: "a", Value: "b=c"}},
		{"ü=ö", ReqParams, request.KV{Key: "ü", Value: "ö"}},
	}
	for _, tt := range tests {
		if got := parseRow(tt.in, tt.tab); got != tt.want {
			t.Errorf("parseRow(%q) = %+v, want %+v", tt.in, got, tt.want)
		}
	}
}

func TestMergeParams(t *testing.T) {
	off := request.KV{Key: "off", Disabled: true}
	old := []request.KV{{Key: "a"}, off, {Key: "b"}}
	got := mergeParams(old, []request.KV{{Key: "x"}, {Key: "y"}, {Key: "z"}})
	want := []request.KV{{Key: "x"}, off, {Key: "y"}, {Key: "z"}}
	if !slices.Equal(got, want) {
		t.Errorf("merge = %+v, want %+v", got, want)
	}
	if got := mergeParams(old, nil); !slices.Equal(got, []request.KV{off}) {
		t.Errorf("merge with no query = %+v", got)
	}
}

func TestSetRequestAddsParamsToURL(t *testing.T) {
	e := NewEditor(theme.Default())
	e.SetRequest(request.Request{
		Method: http.MethodPut,
		URL:    "https://x.io/a?k=1#f",
		Params: []request.KV{{Key: "q", Value: "a b"}, {Key: "off", Disabled: true}},
		Body:   "{}",
	})
	r := e.Request()
	if r.Method != http.MethodPut || r.URL != "https://x.io/a?k=1&q=a%20b#f" || r.Body != "{}" {
		t.Errorf("request = %+v", r)
	}
	if n := len(e.params.rows); n != 3 {
		t.Errorf("%d param rows, want 3", n)
	}
}
