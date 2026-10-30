// Package discover finds a project's API endpoints in OpenAPI specs and
// .http files, so they can be picked instead of typed. It reads files
// only; nothing is sent.
package discover

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/skinleak/quest/internal/core/request"
)

// Limits that keep a search fast even when started in a home directory.
const (
	maxEntries   = 20000    // files and directories looked at
	maxFileBytes = 16 << 20 // larger files are skipped
	sniffBytes   = 1024     // read to tell a spec from other JSON or YAML
)

// skipDirs are never searched: dependencies, build output, virtualenvs.
// Directories starting with "." are skipped too.
var skipDirs = map[string]bool{
	"node_modules": true, "vendor": true, "dist": true, "build": true,
	"target": true, "venv": true, "__pycache__": true, "bin": true, "obj": true,
}

// Endpoint is one request found in a project.
type Endpoint struct {
	Name    string // the summary, operationId or .http request name; may be empty
	Path    string // the URL's path, as shown in a list
	Source  string // the file, relative to the root; "file:line" for .http
	Request request.Request
}

// Result is what a search found.
type Result struct {
	Endpoints []Endpoint
	Files     int      // files endpoints came from
	Failed    []string // files that looked like specs but could not be read
	Partial   bool     // the search stopped at its limit
}

var errNotSpec = errors.New("not an openapi spec")

// Find searches root for OpenAPI specs (JSON or YAML) and .http files and
// returns their endpoints, file by file in path order. Unreadable files
// are skipped; only a cancelled ctx is an error.
func Find(ctx context.Context, root string) (Result, error) {
	var res Result
	var specs, httpFiles []string
	entries := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			return nil //nolint:nilerr // an unreadable entry is skipped, not fatal
		}
		if entries++; entries > maxEntries {
			res.Partial = true
			return fs.SkipAll
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (strings.HasPrefix(name, ".") || skipDirs[name]) {
				return fs.SkipDir
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(name)) {
		case ".http", ".rest":
			httpFiles = append(httpFiles, path)
		case ".json", ".yaml", ".yml":
			if isSpec(path, name) {
				specs = append(specs, path)
			}
		}
		return nil
	})
	if err != nil {
		return res, err
	}

	// Specs first, then .http files, each in path order.
	for _, path := range append(specs, httpFiles...) {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)
		data, err := readFile(path)
		var eps []Endpoint
		if err == nil {
			if ext := strings.ToLower(filepath.Ext(path)); ext == ".http" || ext == ".rest" {
				eps = parseHTTPFile(data, rel)
			} else {
				eps, err = parseOpenAPI(data, rel)
			}
		}
		switch {
		case errors.Is(err, errNotSpec):
		case err != nil:
			res.Failed = append(res.Failed, rel)
		case len(eps) > 0:
			res.Endpoints = append(res.Endpoints, eps...)
			res.Files++
		}
	}
	return res, nil
}

// isSpec tells an OpenAPI spec from other JSON or YAML: by its name, or
// by an "openapi" or "swagger" key near the top.
func isSpec(path, name string) bool {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "openapi") || strings.Contains(lower, "swagger") {
		return true
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, sniffBytes)
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	for _, key := range []string{"openapi", "swagger"} {
		if bytes.Contains(head, []byte(`"`+key+`"`)) || bytes.HasPrefix(head, []byte(key+":")) ||
			bytes.Contains(head, []byte("\n"+key+":")) {
			return true
		}
	}
	return false
}

func readFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxFileBytes {
		return nil, errors.New("file too large")
	}
	return os.ReadFile(path)
}

// displayPath is the part of a URL shown in a list: from the path on,
// without scheme, host or a leading {{variable}}.
func displayPath(u string) string {
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
		if j := strings.IndexByte(u, '/'); j >= 0 {
			return u[j:]
		}
		return "/"
	}
	if strings.HasPrefix(u, "{{") {
		if i := strings.Index(u, "}}"); i >= 0 {
			u = u[i+2:]
		}
	}
	if u == "" {
		return "/"
	}
	return u
}
