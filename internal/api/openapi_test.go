package api_test

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"

	"anvil/internal/api"
	"anvil/internal/auth"
	"anvil/internal/config"
	"anvil/internal/httpserver"
	"anvil/internal/logging"
	"anvil/internal/repos"
	"anvil/internal/users"
)

// specPath is api/openapi.yaml relative to this package directory.
var specPath = filepath.Join("..", "..", "api", "openapi.yaml")

// httpMethods are the OpenAPI path-item keys that are operations.
var httpMethods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

// notREST are router patterns that are catch-alls, not REST endpoints: the JSON 404 for
// unknown /api paths and the web app fallback.
var notREST = []string{"/api/*", "/*"}

// paramRE matches a path parameter in either chi ({owner}) or OpenAPI ({owner}) form.
var paramRE = regexp.MustCompile(`\{[^}]*\}`)

// normalise maps a chi route pattern or an OpenAPI path to a common form, so the two can
// be compared. The rule:
//
//  1. A trailing "/" is dropped (chi reports r.Route("/x", ...) + r.Get("/") as "/x/").
//  2. A trailing chi wildcard "/*" is one path parameter. The spec names it by meaning
//     ({refpath} for tree/blob/raw, {basehead} for compare, {archive} for archive) and
//     documents that it may span several segments.
//  3. Parameter names are ignored: every "{name}" becomes "{}", so only the position of
//     parameters must agree.
func normalise(p string) string {
	if len(p) > 1 {
		p = strings.TrimSuffix(p, "/")
	}
	if strings.HasSuffix(p, "/*") {
		p = strings.TrimSuffix(p, "*") + "{}"
	}
	return paramRE.ReplaceAllString(p, "{}")
}

// routerOperations returns "METHOD /normalised/path" for every REST route of the main
// router. Mount registers every route only when Auth and Repos are non-nil; the handlers
// are never called, so empty services are enough.
func routerOperations(t *testing.T) map[string]string {
	t.Helper()
	srv := httpserver.New(httpserver.Deps{
		Config: config.Default(),
		Logger: logging.NewWithWriter(io.Discard, "text", slog.LevelError),
		API:    api.Deps{Auth: &auth.Service{}, Users: &users.Service{}, Repos: &repos.Service{}},
		SPA:    fstest.MapFS{"index.html": {Data: []byte("<html>")}},
	})
	routes, ok := srv.Handler().(chi.Routes)
	if !ok {
		t.Fatalf("server handler is %T, not a chi router", srv.Handler())
	}
	ops := map[string]string{}
	err := chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if slices.Contains(notREST, route) {
			return nil
		}
		ops[method+" "+normalise(route)] = method + " " + route
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return ops
}

// specOperations returns "METHOD /normalised/path" for every operation in the spec, after
// checking that each operation has a summary, tags, security and responses.
func specOperations(t *testing.T) map[string]string {
	t.Helper()
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		OpenAPI string                          `yaml:"openapi"`
		Paths   map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatalf("parse %s: %v", specPath, err)
	}
	if !strings.HasPrefix(spec.OpenAPI, "3.1") {
		t.Errorf("openapi version %q, want 3.1.x", spec.OpenAPI)
	}
	ops := map[string]string{}
	for path, item := range spec.Paths {
		for key, node := range item {
			if !slices.Contains(httpMethods, key) {
				continue // path-level parameters, summary, ...
			}
			var op map[string]any
			if err := node.Decode(&op); err != nil {
				t.Fatalf("spec %s %s: %v", key, path, err)
			}
			method := strings.ToUpper(key)
			for _, field := range []string{"summary", "tags", "security", "responses"} {
				if _, ok := op[field]; !ok {
					t.Errorf("spec %s %s has no %s", method, path, field)
				}
			}
			ops[method+" "+normalise(path)] = method + " " + path
		}
	}
	return ops
}

// TestOpenAPIMatchesRoutes fails if a REST route is missing from api/openapi.yaml or the
// spec documents an operation the router does not serve (docs/reference/15 §1).
func TestOpenAPIMatchesRoutes(t *testing.T) {
	routes, spec := routerOperations(t), specOperations(t)
	for key, route := range routes {
		if _, ok := spec[key]; !ok {
			t.Errorf("route %s is not documented in api/openapi.yaml", route)
		}
	}
	for key, op := range spec {
		if _, ok := routes[key]; !ok {
			t.Errorf("api/openapi.yaml documents %s, but no route serves it", op)
		}
	}
	if len(routes) == 0 {
		t.Fatal("router has no routes")
	}
}

// TestOpenAPIRefsResolve fails if a "$ref" in api/openapi.yaml points at a component that
// does not exist, so typos are caught without an external OpenAPI linter.
func TestOpenAPIRefsResolve(t *testing.T) {
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	var walk func(v any)
	walk = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			if ref, ok := v["$ref"].(string); ok {
				var cur any = root
				for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
					m, _ := cur.(map[string]any)
					cur = m[part]
				}
				if !strings.HasPrefix(ref, "#/") || cur == nil {
					t.Errorf("unresolved $ref %q", ref)
				}
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(root)
}
