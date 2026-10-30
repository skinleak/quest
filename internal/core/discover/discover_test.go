package discover

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/skinleak/quest/internal/core/request"
)

const openapi3 = `openapi: 3.0.3
info: {title: Users, version: "1"}
servers:
  - url: https://{env}.example.com/v1/
    variables:
      env: {default: api}
paths:
  /users:
    parameters:
      - {name: X-Tenant, in: header, required: true, schema: {type: string, example: acme}}
    get:
      summary: List users
      parameters:
        - {name: page, in: query, required: true, schema: {type: integer, default: 1}}
        - {name: q, in: query, schema: {type: string}}
    post:
      operationId: createUser
      requestBody:
        $ref: '#/components/requestBodies/NewUser'
  /users/{id}:
    delete:
      parameters:
        - {name: id, in: path, required: true}
    put:
      requestBody:
        content:
          application/x-www-form-urlencoded:
            schema: {type: object}
components:
  requestBodies:
    NewUser:
      content:
        application/json:
          schema:
            allOf:
              - $ref: '#/components/schemas/Base'
              - properties:
                  name: {type: string}
                  tags: {type: array, items: {type: string, enum: [admin, dev]}}
                  boss: {$ref: '#/components/schemas/User'}
  schemas:
    Base:
      properties:
        id: {type: integer, example: 7}
        active: {type: boolean}
    User:
      type: object
      properties:
        name: {type: string}
        boss: {$ref: '#/components/schemas/User'}
`

const swagger2 = `{
  "swagger": "2.0",
  "host": "petstore.example.com",
  "basePath": "/v2",
  "schemes": ["http"],
  "paths": {
    "/pets": {
      "post": {
        "summary": "Add a pet",
        "parameters": [{"name": "body", "in": "body", "schema": {"$ref": "#/definitions/Pet"}}]
      }
    }
  },
  "definitions": {
    "Pet": {"type": "object", "properties": {"name": {"type": "string", "example": "rex"}, "age": {"type": "integer"}}}
  }
}`

func TestOpenAPI3(t *testing.T) {
	eps, err := parseOpenAPI([]byte(openapi3), "api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 4 {
		t.Fatalf("%d endpoints, want 4: %+v", len(eps), eps)
	}

	list := eps[0]
	if list.Name != "List users" || list.Path != "/users" || list.Source != "api/openapi.yaml" {
		t.Errorf("list: %+v", list)
	}
	if list.Request.Method != http.MethodGet || list.Request.URL != "https://api.example.com/v1/users" {
		t.Errorf("list request: %s %s", list.Request.Method, list.Request.URL)
	}
	wantParams := []request.KV{{Key: "page", Value: "1"}, {Key: "q", Disabled: true}}
	if !slices.Equal(list.Request.Params, wantParams) {
		t.Errorf("params = %+v", list.Request.Params)
	}
	if !slices.Equal(list.Request.Headers, []request.KV{{Key: "X-Tenant", Value: "acme"}}) {
		t.Errorf("headers = %+v", list.Request.Headers)
	}

	create := eps[1]
	if create.Name != "createUser" {
		t.Errorf("name = %q", create.Name)
	}
	// Spec order is kept; a self-referencing schema ends at maxDepth.
	wantBody := `{
  "id": 7,
  "active": false,
  "name": "",
  "tags": [
    "admin"
  ],
  "boss": {`
	if !strings.HasPrefix(create.Request.Body, wantBody) {
		t.Errorf("body:\n%s", create.Request.Body)
	}

	if del := eps[2]; del.Request.Method != http.MethodDelete || del.Path != "/users/{id}" {
		t.Errorf("delete: %+v", del)
	}
	put := eps[3]
	if !slices.Equal(put.Request.Headers, []request.KV{{Key: "Content-Type", Value: "application/x-www-form-urlencoded"}}) || put.Request.Body != "" {
		t.Errorf("form body: %+v", put.Request)
	}
}

func TestSwagger2(t *testing.T) {
	eps, err := parseOpenAPI([]byte(swagger2), "swagger.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 {
		t.Fatalf("%d endpoints", len(eps))
	}
	r := eps[0].Request
	if r.URL != "http://petstore.example.com/v2/pets" || r.Body != "{\n  \"name\": \"rex\",\n  \"age\": 0\n}" {
		t.Errorf("request: %s\n%s", r.URL, r.Body)
	}
}

func TestBaseURL(t *testing.T) {
	for spec, want := range map[string]string{
		"openapi: 3.0.0\npaths: {/a: {get: {}}}":                                            BaseURLVar + "/a",
		"openapi: 3.0.0\nservers: [{url: /api/}]\npaths: {/a: {get: {}}}":                   BaseURLVar + "/api/a",
		"openapi: 3.0.0\nservers: [{url: //x.io}]\npaths: {/a: {get: {}}}":                  "https://x.io/a",
		"swagger: '2.0'\nbasePath: /v1\npaths: {/a: {get: {}}}":                             BaseURLVar + "/v1/a",
		"openapi: 3.0.0\nservers: [{url: 'http://localhost:8080'}]\npaths: {/a: {get: {}}}": "http://localhost:8080/a",
	} {
		eps, err := parseOpenAPI([]byte(spec), "s.yaml")
		if err != nil || len(eps) != 1 {
			t.Fatalf("%q: %v, %d endpoints", spec, err, len(eps))
		}
		if got := eps[0].Request.URL; got != want {
			t.Errorf("%q: url %q, want %q", spec, got, want)
		}
	}
}

func TestNotASpec(t *testing.T) {
	for _, data := range []string{`{"name": "pkg"}`, "", "- a\n- b", "a: [unclosed"} {
		if _, err := parseOpenAPI([]byte(data), "x.json"); err == nil {
			t.Errorf("%q parsed as a spec", data)
		}
	}
}

const httpFile = `@host = https://api.example.com
@base = {{host}}/v1

### List users
GET {{base}}/users?page=1 HTTP/1.1
Accept: application/json

### Create
# @name createUser
// a comment
POST {{base}}/users
Content-Type: application/json
Authorization: Bearer {{token}}

{
  "name": "ada"
}

> {% client.global.set("id", response.body.id); %}

###
{{host}}/health
    ?verbose=true
    &x=1

### not a request
just some notes
`

func TestHTTPFile(t *testing.T) {
	eps := parseHTTPFile([]byte(httpFile), "requests/api.http")
	if len(eps) != 3 {
		t.Fatalf("%d endpoints, want 3: %+v", len(eps), eps)
	}
	list := eps[0]
	if list.Name != "List users" || list.Source != "requests/api.http:5" || list.Path != "/v1/users?page=1" {
		t.Errorf("list: %+v", list)
	}
	if r := list.Request; r.Method != http.MethodGet || r.URL != "https://api.example.com/v1/users?page=1" ||
		!slices.Equal(r.Headers, []request.KV{{Key: "Accept", Value: "application/json"}}) || r.Body != "" {
		t.Errorf("list request: %+v", r)
	}
	create := eps[1]
	if create.Name != "createUser" || create.Request.Method != http.MethodPost {
		t.Errorf("create: %+v", create)
	}
	if h := create.Request.Headers[1]; h.Value != "Bearer {{token}}" {
		t.Errorf("unknown variable should stay: %q", h.Value)
	}
	if create.Request.Body != "{\n  \"name\": \"ada\"\n}" {
		t.Errorf("body = %q", create.Request.Body)
	}
	health := eps[2]
	if health.Name != "" || health.Request.Method != http.MethodGet || health.Request.URL != "https://api.example.com/health?verbose=true&x=1" {
		t.Errorf("health: %+v", health)
	}
}

func TestDisplayPath(t *testing.T) {
	for in, want := range map[string]string{
		"https://x.io/a?b=1": "/a?b=1",
		"https://x.io":       "/",
		"{{host}}/a":         "/a",
		"/a":                 "/a",
		"":                   "/",
	} {
		if got := displayPath(in); got != want {
			t.Errorf("displayPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFind(t *testing.T) {
	root := t.TempDir()
	write := func(path, data string) {
		t.Helper()
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("docs/openapi.yaml", openapi3)
	write("spec/petstore.json", swagger2) // found by its content
	write("api.http", "GET https://x.io/a\n")
	write("package.json", `{"name": "app"}`)
	write("broken-openapi.yaml", "openapi: [")
	write("node_modules/lib/openapi.yaml", openapi3)
	write(".git/swagger.json", swagger2)

	res, err := Find(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	var sources []string
	for _, ep := range res.Endpoints {
		if len(sources) == 0 || sources[len(sources)-1] != ep.Source {
			sources = append(sources, ep.Source)
		}
	}
	want := []string{"docs/openapi.yaml", "spec/petstore.json", "api.http:1"}
	if !slices.Equal(sources, want) {
		t.Errorf("sources = %q, want %q", sources, want)
	}
	if len(res.Endpoints) != 6 || res.Files != 3 || res.Partial {
		t.Errorf("%d endpoints from %d files, partial %v", len(res.Endpoints), res.Files, res.Partial)
	}
	if !slices.Equal(res.Failed, []string{"broken-openapi.yaml"}) {
		t.Errorf("failed = %q", res.Failed)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Find(ctx, root); err == nil {
		t.Error("cancelled search returned no error")
	}
}
