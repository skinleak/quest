package discover

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// The routes file of a gin project, as a user had it, with the group
// made in main.go and passed in.
const ginRoutes = `// Package routes registers the API's HTTP handlers.
package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Register attaches all routes to r.
func Register(r gin.IRouter) {
	r.GET("/ping", ping)
	r.GET("/health", health)
	users := r.Group("/users")
	{
		users.GET("/:id", h.GetUser)
		users.POST("", h.CreateUser)
		users.Handle("DELETE", "/:id", h.DeleteUser)
	}
	r.Group("/files").GET("/*path", gin.WrapF(serveFile))
}

func ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "pong"})
	_ = c.GetString("user")   // not a route: one argument
	_, _ = http.Get("https://example.com/x")
}

func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
`

const ginMain = `package main

import (
	"github.com/gin-gonic/gin"
	"example.com/app/routes"
)

func main() {
	r := gin.Default()
	routes.Register(r.Group("/api/v1"))
	r.Run(":9090")
}
`

const chiRoutes = `package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) routes() {
	s.router.Get("/", s.index)
	s.router.Route("/articles", func(r chi.Router) {
		r.With(paginate).Get("/", listArticles)
		r.Route("/{articleID:[0-9]+}", func(r chi.Router) {
			r.Put("/", updateArticle)
			r.Method("PATCH", "/", http.HandlerFunc(patchArticle))
		})
	})
	s.router.Group(func(r chi.Router) {
		r.Post("/login", s.login)
	})
}
`

const stdlibRoutes = `package main

import "net/http"

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /items/{id}", getItem)
	mux.HandleFunc("POST example.com/items", addItem)
	mux.Handle("/static/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/{$}", home)
	mux.Handle("/trace", newTraceHandler(parsed))
	srv := &http.Server{Addr: "0.0.0.0:3000", Handler: mux}
	srv.ListenAndServe()
}
`

const echoRoutes = `package main

import "github.com/labstack/echo/v4"

func main() {
	e := echo.New()
	g := e.Group("/admin")
	g.GET("/stats", stats)
	e.Add("PUT", "/config", saveConfig)
	e.Start(":1323")
}
`

func goProject(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, data := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// routes lists endpoints as "METHOD url name source".
func routes(t *testing.T, root string) []string {
	t.Helper()
	res, err := Find(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, ep := range res.Endpoints {
		out = append(out, ep.Request.Method+" "+ep.Request.URL+" "+ep.Name+" "+ep.Source)
	}
	return out
}

func checkRoutes(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("got %d routes, want %d", len(got), len(want))
	}
	for i := range max(len(got), len(want)) {
		var g, w string
		if i < len(got) {
			g = got[i]
		}
		if i < len(want) {
			w = want[i]
		}
		if g != w {
			t.Errorf("route %d:\n got %q\nwant %q", i, g, w)
		}
	}
}

func TestGinRoutes(t *testing.T) {
	root := goProject(t, map[string]string{
		"routes/routes.go":      ginRoutes,
		"main.go":               ginMain,
		"routes/routes_test.go": `package routes; import "net/http"; func T() { r.GET("/test-only", h) }`,
	})
	checkRoutes(t, routes(t, root), []string{
		"GET http://localhost:9090/api/v1/ping ping routes/routes.go:12",
		"GET http://localhost:9090/api/v1/health health routes/routes.go:13",
		"GET http://localhost:9090/api/v1/users/{id} GetUser routes/routes.go:16",
		"POST http://localhost:9090/api/v1/users CreateUser routes/routes.go:17",
		"DELETE http://localhost:9090/api/v1/users/{id} DeleteUser routes/routes.go:18",
		"GET http://localhost:9090/api/v1/files/{path} serveFile routes/routes.go:20",
	})
}

func TestGinRoutesAlone(t *testing.T) {
	// Without main.go: no prefix, no address, so {{baseUrl}}.
	root := goProject(t, map[string]string{"routes.go": ginRoutes})
	got := routes(t, root)
	if len(got) == 0 || got[0] != "GET {{baseUrl}}/ping ping routes.go:12" {
		t.Errorf("routes = %q", got)
	}
}

func TestChiRoutes(t *testing.T) {
	root := goProject(t, map[string]string{"api/routes.go": chiRoutes})
	checkRoutes(t, routes(t, root), []string{
		"GET {{baseUrl}}/ index api/routes.go:10",
		"GET {{baseUrl}}/articles/ listArticles api/routes.go:12",
		"PUT {{baseUrl}}/articles/{articleID}/ updateArticle api/routes.go:14",
		"PATCH {{baseUrl}}/articles/{articleID}/ patchArticle api/routes.go:15",
		"POST {{baseUrl}}/login login api/routes.go:19",
	})
}

func TestStdlibRoutes(t *testing.T) {
	root := goProject(t, map[string]string{"main.go": stdlibRoutes})
	checkRoutes(t, routes(t, root), []string{
		"GET http://localhost:3000/items/{id} getItem main.go:7",
		"POST http://localhost:3000/items addItem main.go:8",
		"GET http://localhost:3000/static/ FileServer main.go:9",
		"GET http://localhost:3000/ home main.go:10",
		"GET http://localhost:3000/trace newTraceHandler main.go:11",
	})
}

func TestEchoRoutes(t *testing.T) {
	root := goProject(t, map[string]string{"main.go": echoRoutes})
	checkRoutes(t, routes(t, root), []string{
		"GET http://localhost:1323/admin/stats stats main.go:8",
		"PUT http://localhost:1323/config saveConfig main.go:9",
	})
}

func TestGoFilesWithoutRouters(t *testing.T) {
	root := goProject(t, map[string]string{
		"client.go": `package c; import "net/http"; func f() { http.Get("/a"); c.Get("/b") }`,
		"cache.go":  `package c; func f() { cache.Get("/a", 1); m.GET("/b", h) }`, // no router import
		"broken.go": `package c; import "net/http"; func f() { r.GET("/x", h`,
	})
	if got := routes(t, root); len(got) != 0 {
		t.Errorf("routes = %q", got)
	}
}

func TestNormalizePath(t *testing.T) {
	for in, want := range map[string]string{
		"/users/:id":        "/users/{id}",
		"/files/*filepath":  "/files/{filepath}",
		"/a/{id:[0-9]+}/b":  "/a/{id}/b",
		"/static/{path...}": "/static/{path}",
		"/{$}":              "/",
		"":                  "/",
		"/plain/*":          "/plain/*",
	} {
		if got := normalizePath(in); got != want {
			t.Errorf("normalizePath(%q) = %q, want %q", in, got, want)
		}
	}
}
