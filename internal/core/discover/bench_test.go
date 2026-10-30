package discover

import (
	"fmt"
	"strings"
	"testing"
)

// BenchmarkParseOpenAPI parses a spec of 500 paths with two operations
// each (about 0.5 MiB of YAML), as done in the background on every search.
func BenchmarkParseOpenAPI(b *testing.B) {
	var sb strings.Builder
	sb.WriteString("openapi: 3.0.0\nservers: [{url: 'https://api.example.com'}]\npaths:\n")
	for i := range 500 {
		fmt.Fprintf(&sb, `  /things%d/{id}:
    get:
      summary: Get thing %d
      parameters:
        - {name: id, in: path, required: true, schema: {type: string}}
        - {name: expand, in: query, schema: {type: string, enum: [all, none]}}
    put:
      summary: Replace thing %d
      requestBody:
        content:
          application/json:
            schema: {$ref: '#/components/schemas/Thing'}
`, i, i, i)
	}
	sb.WriteString(`components:
  schemas:
    Thing:
      properties:
        id: {type: string, example: abc}
        tags: {type: array, items: {type: string}}
        owner: {properties: {name: {type: string}, email: {type: string}}}
`)
	data := []byte(sb.String())
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		if eps, err := parseOpenAPI(data, "openapi.yaml"); err != nil || len(eps) != 1000 {
			b.Fatal(err, len(eps))
		}
	}
}
