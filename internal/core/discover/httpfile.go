package discover

import (
	"strconv"
	"strings"

	"github.com/skinleak/quest/internal/core/request"
)

// parseHTTPFile reads the requests of a .http or .rest file, the format of
// the VS Code REST Client and JetBrains HTTP Client:
//
//	@host = https://api.example.com
//
//	### List users
//	GET {{host}}/users?page=1 HTTP/1.1
//	Accept: application/json
//
//	### Create a user
//	# @name createUser
//	POST {{host}}/users
//	Content-Type: application/json
//
//	{"name": "ada"}
//
// Requests are separated by lines starting with "###". File variables
// (@name = value) are filled in; other {{variables}} are left for an
// environment. Response handlers (lines from "> " or "<> " on) are
// ignored.
func parseHTTPFile(data []byte, rel string) []Endpoint {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	vars := fileVars(lines)
	var out []Endpoint
	start := 0
	for i := 0; i <= len(lines); i++ {
		if i < len(lines) && !strings.HasPrefix(lines[i], "###") {
			continue
		}
		name := ""
		if start > 0 {
			name = strings.TrimSpace(strings.TrimLeft(lines[start-1], "#"))
		}
		if ep, ok := parseHTTPRequest(lines[start:i], start, name, vars); ok {
			ep.Source = rel + ":" + strconv.Itoa(ep.line)
			out = append(out, ep.Endpoint)
		}
		start = i + 1
	}
	return out
}

type httpRequest struct {
	Endpoint
	line int // 1-based line of the request line
}

// parseHTTPRequest reads one block: comments and variables, the request
// line, headers, a blank line and the body. offset is the block's first
// line in the file.
func parseHTTPRequest(lines []string, offset int, name string, vars map[string]string) (httpRequest, bool) {
	i := 0
	for ; i < len(lines); i++ {
		l := strings.TrimSpace(lines[i])
		if l == "" || strings.HasPrefix(l, "@") {
			continue
		}
		if c, ok := comment(l); ok {
			if n, ok := strings.CutPrefix(c, "@name"); ok && strings.TrimSpace(n) != "" {
				name = strings.TrimSpace(n)
			}
			continue
		}
		break
	}
	if i == len(lines) {
		return httpRequest{}, false
	}
	method, url, ok := requestLine(strings.TrimSpace(lines[i]))
	if !ok {
		return httpRequest{}, false
	}
	line := offset + i + 1
	// A query can go on over indented lines starting with ? or &.
	for i++; i < len(lines); i++ {
		l := strings.TrimSpace(lines[i])
		if lines[i] == l || (!strings.HasPrefix(l, "?") && !strings.HasPrefix(l, "&")) {
			break
		}
		url += l
	}

	r := request.Request{Method: method, URL: expand(url, vars)}
	for ; i < len(lines) && strings.TrimSpace(lines[i]) != ""; i++ {
		l := strings.TrimSpace(lines[i])
		if _, ok := comment(l); ok {
			continue
		}
		k, v, ok := strings.Cut(l, ":")
		if !ok {
			continue
		}
		r.Headers = append(r.Headers, request.KV{Key: strings.TrimSpace(k), Value: expand(strings.TrimSpace(v), vars)})
	}

	var body []string
	for i++; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "> ") || strings.HasPrefix(lines[i], "<> ") {
			break
		}
		body = append(body, lines[i])
	}
	for len(body) > 0 && strings.TrimSpace(body[len(body)-1]) == "" {
		body = body[:len(body)-1]
	}
	r.Body = expand(strings.Join(body, "\n"), vars)

	return httpRequest{
		Endpoint: Endpoint{Name: name, Path: displayPath(r.URL), Request: r},
		line:     line,
	}, true
}

// requestLine reads "METHOD url [HTTP/1.1]", or a bare URL, which is a GET.
func requestLine(l string) (method, url string, ok bool) {
	fields := strings.Fields(l)
	if len(fields) == 0 {
		return "", "", false
	}
	if isMethod(fields[0]) {
		if len(fields) < 2 {
			return "", "", false
		}
		return fields[0], fields[1], true
	}
	u := fields[0]
	if strings.Contains(u, "://") || strings.HasPrefix(u, "{{") || strings.HasPrefix(u, "/") {
		return "GET", u, true
	}
	return "", "", false
}

// comment returns the text of a "#" or "//" comment line.
func comment(l string) (string, bool) {
	if c, ok := strings.CutPrefix(l, "//"); ok {
		return strings.TrimSpace(c), true
	}
	if c, ok := strings.CutPrefix(l, "#"); ok {
		return strings.TrimSpace(c), true
	}
	return "", false
}

// fileVars collects "@name = value" lines.
func fileVars(lines []string) map[string]string {
	vars := map[string]string{}
	for _, l := range lines {
		rest, ok := strings.CutPrefix(strings.TrimSpace(l), "@")
		if !ok {
			continue
		}
		k, v, ok := strings.Cut(rest, "=")
		if k = strings.TrimSpace(k); ok && k != "" && !strings.ContainsAny(k, " \t") {
			vars[k] = strings.TrimSpace(v)
		}
	}
	// Variables may use others: @url = {{host}}/v1. A few passes resolve
	// chains whatever the map order; cycles stay unresolved.
	for range 4 {
		for k, v := range vars {
			vars[k] = expand(v, vars)
		}
	}
	return vars
}

// expand fills in the {{name}} variables known in vars and leaves the
// rest, such as {{$uuid}} or ones an environment sets.
func expand(s string, vars map[string]string) string {
	if len(vars) == 0 || !strings.Contains(s, "{{") {
		return s
	}
	var b strings.Builder
	for {
		i := strings.Index(s, "{{")
		if i < 0 {
			break
		}
		j := strings.Index(s[i:], "}}")
		if j < 0 {
			break
		}
		name := strings.TrimSpace(s[i+2 : i+j])
		b.WriteString(s[:i])
		if v, ok := vars[name]; ok {
			b.WriteString(v)
		} else {
			b.WriteString(s[i : i+j+2])
		}
		s = s[i+j+2:]
	}
	b.WriteString(s)
	return b.String()
}
