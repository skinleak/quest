package discover

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"

	"github.com/skinleak/quest/internal/core/request"
)

// maxGoFileBytes skips generated giants; route files are small.
const maxGoFileBytes = 1 << 20

// routerImports mark the Go files worth parsing for routes; any other file
// is skipped after a byte search.
var routerImports = [][]byte{
	[]byte(`"net/http"`),
	[]byte(`"github.com/gin-gonic/gin"`),
	[]byte(`"github.com/labstack/echo`),
	[]byte(`"github.com/go-chi/chi`),
}

// ginImport marks files where a bare r.Run() means gin's default :8080.
var ginImport = []byte(`"github.com/gin-gonic/gin"`)

// conflict marks a function parameter that is passed different prefixes.
const conflict = "\x00"

type goFile struct {
	rel string
	ast *ast.File
	gin bool
}

// goRoutes reads the routes registered in Go source: gin, echo, chi and
// net/http's ServeMux. It reads only literal paths, follows router groups
// (also when a group is passed to another function), and takes the base
// URL from the address the server listens on, if it is a literal.
func goRoutes(ctx context.Context, root string, paths []string) ([]Endpoint, int, error) {
	fset := token.NewFileSet()
	var files []goFile
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		data, err := readFile(path)
		if err != nil || len(data) > maxGoFileBytes || !importsRouter(data) {
			continue
		}
		f, err := parser.ParseFile(fset, path, data, parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		files = append(files, goFile{rel: relPath(root, path), ast: f, gin: bytes.Contains(data, ginImport)})
	}

	// A group passed to a function prefixes the routes registered there.
	// Each pass learns those prefixes from the calls; a few passes resolve
	// chains of such functions.
	var w routeWalker
	incoming := map[string]map[int]string{}
	for range 4 {
		w = routeWalker{fset: fset, calls: map[string]map[int]string{}, seen: map[string]bool{}}
		for _, f := range files {
			w.file = f
			for _, d := range f.ast.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
					w.walk(fd.Body, w.params(fd.Type, incoming[fd.Name.Name]))
				}
			}
		}
		if sameCalls(w.calls, incoming) {
			break
		}
		incoming = w.calls
	}

	base := w.base
	if base == "" {
		base = BaseURLVar
	}
	withRoutes := map[string]bool{}
	for i := range w.out {
		w.out[i].Request.URL = base + w.out[i].Path
		file, _, _ := strings.Cut(w.out[i].Source, ":")
		withRoutes[file] = true
	}
	return w.out, len(withRoutes), nil
}

func importsRouter(data []byte) bool {
	for _, imp := range routerImports {
		if bytes.Contains(data, imp) {
			return true
		}
	}
	return false
}

type routeWalker struct {
	fset  *token.FileSet
	file  goFile
	out   []Endpoint
	seen  map[string]bool           // "METHOD path" already found
	calls map[string]map[int]string // function name → argument → prefix
	base  string                    // the first listen address found
}

// params gives the parameters of a function the prefixes it is called
// with.
func (w *routeWalker) params(ft *ast.FuncType, incoming map[int]string) map[string]string {
	prefixes := map[string]string{}
	i := 0
	for _, field := range ft.Params.List {
		if len(field.Names) == 0 {
			i++
			continue
		}
		for _, name := range field.Names {
			if p, ok := incoming[i]; ok && p != conflict {
				prefixes[name.Name] = p
			}
			i++
		}
	}
	return prefixes
}

// walk finds groups, routes, calls and listen addresses under n. prefixes
// maps router variables ("api", "s.router") to their path prefix.
func (w *routeWalker) walk(n ast.Node, prefixes map[string]string) {
	ast.Inspect(n, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			for i, rhs := range n.Rhs {
				if i < len(n.Lhs) {
					w.assign(n.Lhs[i], rhs, prefixes)
				}
			}
		case *ast.ValueSpec:
			for i, v := range n.Values {
				if i < len(n.Names) {
					w.assign(n.Names[i], v, prefixes)
				}
			}
		case *ast.CompositeLit:
			// &http.Server{Addr: ":8080"}
			for _, e := range n.Elts {
				if kv, ok := e.(*ast.KeyValueExpr); ok && exprKey(kv.Key) == "Addr" {
					w.listen(kv.Value, false)
				}
			}
		case *ast.CallExpr:
			return w.call(n, prefixes)
		}
		return true
	})
}

// assign records a variable that holds a router group, or another name
// for a router with a prefix.
func (w *routeWalker) assign(lhs, rhs ast.Expr, prefixes map[string]string) {
	key := exprKey(lhs)
	if key == "" {
		return
	}
	if p, known := prefixOf(rhs, prefixes); known {
		prefixes[key] = p
	}
}

// call handles one call: a route, a chi Route or Group block, a listen
// address, or a call that passes routers to another function. It returns
// whether to look inside the call.
func (w *routeWalker) call(c *ast.CallExpr, prefixes map[string]string) bool {
	if name := calleeName(c.Fun); name != "" {
		for i, arg := range c.Args {
			if p, known := prefixOf(arg, prefixes); known && p != "" {
				w.passPrefix(name, i, p)
			}
		}
	}
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return true
	}
	name := sel.Sel.Name
	prefix, _ := prefixOf(sel.X, prefixes)

	// chi: r.Route("/users", func(r chi.Router) { … }) and
	// r.Group(func(r chi.Router) { … }).
	if fn := lastFuncLit(c.Args); fn != nil && (name == "Route" || name == "Group") {
		inner := prefix
		if name == "Route" {
			path, ok := stringLit(c.Args[0])
			if !ok {
				return true
			}
			inner = joinPath(prefix, path)
		}
		scoped := make(map[string]string, len(prefixes)+1)
		for k, v := range prefixes {
			scoped[k] = v
		}
		if params := fn.Type.Params.List; len(params) > 0 && len(params[0].Names) > 0 {
			scoped[params[0].Names[0].Name] = inner
		}
		w.walk(fn.Body, scoped)
		return false
	}

	switch name {
	case "Run", "Start", "ListenAndServe", "ListenAndServeTLS", "RunTLS", "StartTLS":
		tls := strings.HasSuffix(name, "TLS")
		if len(c.Args) > 0 {
			w.listen(c.Args[0], tls)
		} else if name == "Run" && w.file.gin {
			w.setBase("http://localhost:8080")
		}
		return true
	}

	// An empty path is the group's own: users.POST("", h).
	if method, path, ok := route(name, c.Args); ok && (path != "" || prefix != "") {
		w.add(method, joinPath(prefix, path), c)
	}
	return true
}

// route reads a route registration: r.GET("/p", h) (gin, echo),
// r.Get("/p", h) (chi), r.Handle("GET", "/p", h) (gin), e.Add("GET",
// "/p", h) (echo), r.Method("GET", "/p", h) (chi) and
// mux.HandleFunc("GET /p", h) (net/http, chi).
func route(name string, args []ast.Expr) (method, path string, ok bool) {
	switch name {
	case "Handle", "Add", "Method", "MethodFunc":
		if len(args) >= 3 {
			m, ok1 := stringLit(args[0])
			p, ok2 := stringLit(args[1])
			if ok1 && ok2 && isMethod(strings.ToUpper(m)) && (p == "" || strings.HasPrefix(p, "/")) {
				return strings.ToUpper(m), p, true
			}
			return "", "", false
		}
		fallthrough
	case "HandleFunc":
		if len(args) != 2 {
			return "", "", false
		}
		pattern, ok := stringLit(args[0])
		if !ok {
			return "", "", false
		}
		return muxPattern(pattern)
	case "Any":
		name = "GET"
	}
	upper := strings.ToUpper(name)
	if !isMethod(upper) || (name != upper && name != strings.ToUpper(name[:1])+strings.ToLower(name[1:])) {
		return "", "", false
	}
	if len(args) < 2 {
		return "", "", false
	}
	p, ok := stringLit(args[0])
	if !ok || (p != "" && !strings.HasPrefix(p, "/")) {
		return "", "", false
	}
	return upper, p, true
}

// muxPattern reads a ServeMux pattern, "[METHOD ][HOST]/[PATH]". Without a
// method it matches every method; it is listed as a GET.
func muxPattern(pattern string) (method, path string, ok bool) {
	method = "GET"
	if m, rest, found := strings.Cut(pattern, " "); found && isMethod(m) {
		method, pattern = m, strings.TrimSpace(rest)
	}
	i := strings.IndexByte(pattern, '/')
	if i < 0 {
		return "", "", false
	}
	return method, pattern[i:], true
}

func (w *routeWalker) add(method, path string, c *ast.CallExpr) {
	path = normalizePath(path)
	if w.seen[method+" "+path] {
		return
	}
	w.seen[method+" "+path] = true
	line := w.fset.Position(c.Pos()).Line
	w.out = append(w.out, Endpoint{
		Name:    handlerName(c.Args[len(c.Args)-1]),
		Path:    path,
		Source:  w.file.rel + ":" + strconv.Itoa(line),
		Request: request.Request{Method: method},
	})
}

func (w *routeWalker) passPrefix(fn string, arg int, prefix string) {
	m := w.calls[fn]
	if m == nil {
		m = map[int]string{}
		w.calls[fn] = m
	}
	if old, ok := m[arg]; ok && old != prefix {
		prefix = conflict
	}
	m[arg] = prefix
}

// listen takes the base URL from a literal listen address.
func (w *routeWalker) listen(addr ast.Expr, tls bool) {
	a, ok := stringLit(addr)
	if !ok {
		return
	}
	host, port, found := strings.Cut(a, ":")
	if !found || port == "" || strings.Contains(port, ":") {
		return
	}
	if host == "" || host == "0.0.0.0" || host == "[::]" || host == "::" {
		host = "localhost"
	}
	scheme := "http"
	if tls {
		scheme = "https"
	}
	w.setBase(scheme + "://" + host + ":" + port)
}

func (w *routeWalker) setBase(u string) {
	if w.base == "" {
		w.base = u
	}
}

// prefixOf is the path prefix of a router expression, and whether it is
// known: a variable with a recorded prefix, or a Group("/p") call.
func prefixOf(e ast.Expr, prefixes map[string]string) (string, bool) {
	switch e := e.(type) {
	case *ast.ParenExpr:
		return prefixOf(e.X, prefixes)
	case *ast.UnaryExpr:
		return prefixOf(e.X, prefixes)
	case *ast.Ident, *ast.SelectorExpr:
		p, ok := prefixes[exprKey(e)]
		return p, ok
	case *ast.CallExpr:
		sel, ok := e.Fun.(*ast.SelectorExpr)
		if !ok {
			return "", false
		}
		switch sel.Sel.Name {
		case "Group":
			if len(e.Args) > 0 {
				if path, ok := stringLit(e.Args[0]); ok {
					p, _ := prefixOf(sel.X, prefixes)
					return joinPath(p, path), true
				}
			}
		case "With": // chi middleware keeps the prefix
			return prefixOf(sel.X, prefixes)
		}
	}
	return "", false
}

// exprKey names a variable or field: "r", "s.router"; "" otherwise.
func exprKey(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		if x := exprKey(e.X); x != "" {
			return x + "." + e.Sel.Name
		}
	}
	return ""
}

// calleeName is the name of the function called: "Register" for
// Register(r), routes.Register(r) and h.Register(r).
func calleeName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

// handlerName names a route's handler: "ping", "GetUser" for h.GetUser,
// "" for a function literal.
func handlerName(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	case *ast.CallExpr:
		// An adapter, such as gin.WrapF(serveFile), is named after the
		// handler it wraps; any other call, such as a constructor taking
		// data, after the function called.
		if len(e.Args) == 1 && handlerAdapters[calleeName(e.Fun)] {
			return handlerName(e.Args[0])
		}
		return handlerName(e.Fun)
	}
	return ""
}

// handlerAdapters turn a function into a handler without naming it.
var handlerAdapters = map[string]bool{"WrapF": true, "WrapH": true, "HandlerFunc": true, "WrapHandler": true}

// lastFuncLit is the last argument when it is a function literal.
func lastFuncLit(args []ast.Expr) *ast.FuncLit {
	if len(args) == 0 {
		return nil
	}
	fn, _ := args[len(args)-1].(*ast.FuncLit)
	return fn
}

func stringLit(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

// joinPath puts a group's prefix before a route's path.
func joinPath(prefix, path string) string {
	switch {
	case prefix == "":
		return path
	case path == "":
		return prefix
	}
	return strings.TrimSuffix(prefix, "/") + "/" + strings.TrimPrefix(path, "/")
}

// normalizePath writes path parameters the OpenAPI way, so every source
// looks the same and the editor asks to fill them in: gin and echo's
// ":id" and "*path", chi's "{id:[0-9]+}" and net/http's "{path...}" all
// become "{id}" or "{path}"; net/http's "{$}" is dropped.
func normalizePath(p string) string {
	if p == "" {
		return "/"
	}
	segs := strings.Split(p, "/")
	for i, s := range segs {
		switch {
		case len(s) > 1 && (s[0] == ':' || s[0] == '*'):
			segs[i] = "{" + s[1:] + "}"
		case s == "{$}":
			segs[i] = ""
		case strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}"):
			name, _, _ := strings.Cut(s[1:len(s)-1], ":")
			segs[i] = "{" + strings.TrimSuffix(name, "...") + "}"
		}
	}
	return strings.Join(segs, "/")
}

func sameCalls(a, b map[string]map[int]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, am := range a {
		bm, ok := b[k]
		if !ok || len(am) != len(bm) {
			return false
		}
		for i, v := range am {
			if bm[i] != v {
				return false
			}
		}
	}
	return true
}
