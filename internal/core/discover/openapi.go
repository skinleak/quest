package discover

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/skinleak/quest/internal/core/request"
)

// BaseURLVar stands in for the server of a spec that names none, or only
// a relative one; an environment can fill it in.
const BaseURLVar = "{{baseUrl}}"

// maxDepth bounds schema recursion, for self-referencing schemas.
const maxDepth = 8

// spec is a parsed OpenAPI document, version 2 (Swagger) or 3.
type spec struct {
	root *yaml.Node
}

// parseOpenAPI reads the endpoints of an OpenAPI 2 or 3 document, JSON or
// YAML. It works on yaml.Node rather than maps, so examples and generated
// bodies keep the key order of the spec.
func parseOpenAPI(data []byte, rel string) ([]Endpoint, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, errNotSpec
	}
	s := spec{root: doc.Content[0]}
	if get(s.root, "openapi") == nil && get(s.root, "swagger") == nil {
		return nil, errNotSpec
	}
	base := s.baseURL()
	var out []Endpoint
	pairs(get(s.root, "paths"), func(path string, item *yaml.Node) {
		item = s.resolve(item)
		shared := get(item, "parameters")
		pairs(item, func(method string, op *yaml.Node) {
			m := strings.ToUpper(method)
			if !isMethod(m) {
				return
			}
			name := scalar(get(op, "summary"))
			if name == "" {
				name = scalar(get(op, "operationId"))
			}
			out = append(out, Endpoint{
				Name:    name,
				Path:    path,
				Source:  rel,
				Request: s.request(m, base+path, shared, op),
			})
		})
	})
	return out, nil
}

// baseURL is the first server's URL, with its variables set to their
// defaults. A relative server, or none, is put after BaseURLVar.
func (s spec) baseURL() string {
	var u string
	if servers := get(s.root, "servers"); servers != nil && servers.Kind == yaml.SequenceNode && len(servers.Content) > 0 {
		srv := servers.Content[0]
		u = scalar(get(srv, "url"))
		pairs(get(srv, "variables"), func(name string, v *yaml.Node) {
			u = strings.ReplaceAll(u, "{"+name+"}", scalar(get(v, "default")))
		})
	} else if host := scalar(get(s.root, "host")); host != "" {
		scheme := "https"
		if schemes := get(s.root, "schemes"); schemes != nil && len(schemes.Content) > 0 {
			scheme = scalar(schemes.Content[0])
		}
		u = scheme + "://" + host + scalar(get(s.root, "basePath"))
	} else {
		u = scalar(get(s.root, "basePath"))
	}
	u = strings.TrimSuffix(u, "/")
	switch {
	case strings.HasPrefix(u, "http://"), strings.HasPrefix(u, "https://"):
		return u
	case strings.HasPrefix(u, "//"):
		return "https:" + u
	}
	return BaseURLVar + u
}

// request builds the request for one operation: query and header
// parameters (required ones on, optional ones off) and an example body.
func (s spec) request(method, url string, shared, op *yaml.Node) request.Request {
	r := request.Request{Method: method, URL: url}
	var body *yaml.Node // a Swagger 2 "in: body" schema
	for _, p := range s.params(shared, get(op, "parameters")) {
		kv := request.KV{
			Key:      scalar(get(p, "name")),
			Value:    s.paramValue(p),
			Disabled: scalar(get(p, "required")) != "true",
		}
		switch scalar(get(p, "in")) {
		case "query":
			r.Params = append(r.Params, kv)
		case "header":
			r.Headers = append(r.Headers, kv)
		case "body":
			body = get(p, "schema")
		}
	}
	if body != nil {
		r.Body = s.sampleJSON(nil, body)
	}
	if rb := s.resolve(get(op, "requestBody")); rb != nil {
		mediaType, media := jsonContent(get(rb, "content"))
		switch {
		case media == nil:
		case isJSONType(mediaType):
			r.Body = s.sampleJSON(media, get(media, "schema"))
		default:
			r.Headers = append(r.Headers, request.KV{Key: "Content-Type", Value: mediaType})
		}
	}
	return r
}

// params merges path-level and operation parameters; an operation's
// parameter replaces a shared one with the same name and location.
func (s spec) params(shared, own *yaml.Node) []*yaml.Node {
	var out []*yaml.Node
	add := func(list *yaml.Node) {
		if list == nil || list.Kind != yaml.SequenceNode {
			return
		}
		for _, p := range list.Content {
			p = s.resolve(p)
			key := scalar(get(p, "in")) + ":" + scalar(get(p, "name"))
			replaced := false
			for i, q := range out {
				if scalar(get(q, "in"))+":"+scalar(get(q, "name")) == key {
					out[i], replaced = p, true
				}
			}
			if !replaced {
				out = append(out, p)
			}
		}
	}
	add(shared)
	add(own)
	return out
}

// paramValue is a parameter's example or default, if it has one.
func (s spec) paramValue(p *yaml.Node) string {
	for _, n := range []*yaml.Node{get(p, "example"), get(p, "default")} {
		if v := text(n); v != "" {
			return v
		}
	}
	schema := s.resolve(get(p, "schema"))
	for _, n := range []*yaml.Node{get(schema, "example"), get(schema, "default"), first(get(schema, "enum"))} {
		if v := text(n); v != "" {
			return v
		}
	}
	return ""
}

// jsonContent picks a request body's media type: JSON when offered, else
// the first one.
func jsonContent(content *yaml.Node) (string, *yaml.Node) {
	if content == nil || content.Kind != yaml.MappingNode || len(content.Content) < 2 {
		return "", nil
	}
	for i := 0; i+1 < len(content.Content); i += 2 {
		if isJSONType(content.Content[i].Value) {
			return content.Content[i].Value, content.Content[i+1]
		}
	}
	return content.Content[0].Value, content.Content[1]
}

func isJSONType(mt string) bool {
	mt, _, _ = strings.Cut(mt, ";")
	return mt == "application/json" || strings.HasSuffix(mt, "+json")
}

// sampleJSON is an indented JSON example body: the media type's example,
// its first named example, or one made from the schema.
func (s spec) sampleJSON(media, schema *yaml.Node) string {
	var b bytes.Buffer
	switch {
	case get(media, "example") != nil:
		writeJSON(&b, get(media, "example"))
	case first(values(get(media, "examples"))) != nil:
		writeJSON(&b, get(s.resolve(first(values(get(media, "examples")))), "value"))
	case schema != nil:
		s.writeSample(&b, schema, 0)
	default:
		return ""
	}
	var out bytes.Buffer
	if json.Indent(&out, b.Bytes(), "", "  ") != nil {
		return ""
	}
	return out.String()
}

// writeSample writes a compact JSON value for schema: its example,
// default or first enum value, else a zero value of its type, with every
// property of an object.
func (s spec) writeSample(b *bytes.Buffer, schema *yaml.Node, depth int) {
	schema = s.resolve(schema)
	if schema == nil || depth > maxDepth {
		b.WriteString("null")
		return
	}
	for _, n := range []*yaml.Node{get(schema, "example"), get(schema, "default"), first(get(schema, "enum"))} {
		if n != nil {
			writeJSON(b, n)
			return
		}
	}
	if all := get(schema, "allOf"); all != nil {
		s.writeObject(b, s.allProperties(all, depth), depth)
		return
	}
	for _, key := range []string{"oneOf", "anyOf"} {
		if alt := first(get(schema, key)); alt != nil {
			s.writeSample(b, alt, depth+1)
			return
		}
	}
	switch schemaType(schema) {
	case "object":
		var props []*yaml.Node
		if p := get(schema, "properties"); p != nil {
			props = p.Content
		}
		s.writeObject(b, props, depth)
	case "array":
		b.WriteByte('[')
		s.writeSample(b, get(schema, "items"), depth+1)
		b.WriteByte(']')
	case "string":
		b.WriteString(`""`)
	case "integer", "number":
		b.WriteByte('0')
	case "boolean":
		b.WriteString("false")
	default:
		b.WriteString("null")
	}
}

// writeObject writes properties, given as key and schema nodes in turn.
func (s spec) writeObject(b *bytes.Buffer, props []*yaml.Node, depth int) {
	b.WriteByte('{')
	for i := 0; i+1 < len(props); i += 2 {
		if i > 0 {
			b.WriteByte(',')
		}
		writeString(b, props[i].Value)
		b.WriteByte(':')
		s.writeSample(b, props[i+1], depth+1)
	}
	b.WriteByte('}')
}

// allProperties collects the properties of every schema in an allOf.
func (s spec) allProperties(all *yaml.Node, depth int) []*yaml.Node {
	var out []*yaml.Node
	if depth > maxDepth {
		return nil
	}
	for _, sub := range all.Content {
		sub = s.resolve(sub)
		if nested := get(sub, "allOf"); nested != nil {
			out = append(out, s.allProperties(nested, depth+1)...)
		}
		if props := get(sub, "properties"); props != nil {
			out = append(out, props.Content...)
		}
	}
	return out
}

// schemaType is a schema's type; for a list of types (OpenAPI 3.1), the
// first that isn't null.
func schemaType(schema *yaml.Node) string {
	t := get(schema, "type")
	if t != nil && t.Kind == yaml.SequenceNode {
		for _, n := range t.Content {
			if n.Value != "null" {
				return n.Value
			}
		}
		return ""
	}
	if t == nil && get(schema, "properties") != nil {
		return "object"
	}
	return scalar(t)
}

// resolve follows local $refs ("#/components/schemas/User"), a few levels
// deep. Refs to other files are left unresolved.
func (s spec) resolve(n *yaml.Node) *yaml.Node {
	for range maxDepth {
		n = deref(n)
		ref := scalar(get(n, "$ref"))
		if !strings.HasPrefix(ref, "#/") {
			return n
		}
		target := s.root
		for part := range strings.SplitSeq(ref[2:], "/") {
			part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
			target = get(target, part)
		}
		if target == nil {
			return nil
		}
		n = target
	}
	return n
}

// get returns the value of key in a mapping node, or nil.
func get(n *yaml.Node, key string) *yaml.Node {
	n = deref(n)
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return deref(n.Content[i+1])
		}
	}
	return nil
}

// pairs calls fn for each key and value of a mapping node, in order.
func pairs(n *yaml.Node, fn func(key string, value *yaml.Node)) {
	n = deref(n)
	if n == nil || n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		fn(n.Content[i].Value, deref(n.Content[i+1]))
	}
}

// values is the values of a mapping node, in order.
func values(n *yaml.Node) *yaml.Node {
	n = deref(n)
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	out := &yaml.Node{Kind: yaml.SequenceNode}
	for i := 1; i < len(n.Content); i += 2 {
		out.Content = append(out.Content, n.Content[i])
	}
	return out
}

// first is the first item of a sequence node, or nil.
func first(n *yaml.Node) *yaml.Node {
	n = deref(n)
	if n == nil || n.Kind != yaml.SequenceNode || len(n.Content) == 0 {
		return nil
	}
	return deref(n.Content[0])
}

func deref(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
}

// scalar is a scalar node's value, or "".
func scalar(n *yaml.Node) string {
	if n = deref(n); n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

// text is a value as it would be typed: a scalar as is, anything else as
// compact JSON.
func text(n *yaml.Node) string {
	n = deref(n)
	if n == nil {
		return ""
	}
	if n.Kind == yaml.ScalarNode {
		if n.Tag == "!!null" {
			return ""
		}
		return n.Value
	}
	var b bytes.Buffer
	writeJSON(&b, n)
	return b.String()
}

// writeJSON writes a YAML node as compact JSON, keeping key order.
func writeJSON(b *bytes.Buffer, n *yaml.Node) {
	n = deref(n)
	if n == nil {
		b.WriteString("null")
		return
	}
	switch n.Kind {
	case yaml.MappingNode:
		b.WriteByte('{')
		for i := 0; i+1 < len(n.Content); i += 2 {
			if i > 0 {
				b.WriteByte(',')
			}
			writeString(b, n.Content[i].Value)
			b.WriteByte(':')
			writeJSON(b, n.Content[i+1])
		}
		b.WriteByte('}')
	case yaml.SequenceNode:
		b.WriteByte('[')
		for i, c := range n.Content {
			if i > 0 {
				b.WriteByte(',')
			}
			writeJSON(b, c)
		}
		b.WriteByte(']')
	case yaml.ScalarNode:
		switch n.Tag {
		case "!!null":
			b.WriteString("null")
		case "!!bool", "!!int", "!!float":
			if json.Valid([]byte(n.Value)) {
				b.WriteString(n.Value)
				return
			}
			writeString(b, n.Value)
		default:
			writeString(b, n.Value)
		}
	default:
		b.WriteString("null")
	}
}

func writeString(b *bytes.Buffer, s string) {
	q, _ := json.Marshal(s)
	b.Write(q)
}

func isMethod(m string) bool { return slices.Contains(request.Methods, m) }
