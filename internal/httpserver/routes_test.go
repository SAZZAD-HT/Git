package httpserver

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/go-chi/chi/v5"

	"anvil/internal/api"
	"anvil/internal/auth"
	"anvil/internal/config"
	"anvil/internal/git/smarthttp"
	"anvil/internal/issues"
	"anvil/internal/logging"
	"anvil/internal/pulls"
	"anvil/internal/repos"
	"anvil/internal/users"
)

// TestFullRouterBuilds assembles the router with every optional service present, as
// `anvil serve` does. chi panics when a pattern is mounted twice, which unit tests that
// leave services nil would never notice. Handlers are not called, so empty services suffice.
func TestFullRouterBuilds(t *testing.T) {
	h := New(Deps{
		Config: config.Default(),
		Logger: logging.NewWithWriter(io.Discard, "text", slog.LevelError),
		API: api.Deps{Auth: &auth.Service{}, Users: &users.Service{}, Repos: &repos.Service{},
			Issues: &issues.Service{}, Pulls: &pulls.Service{}, ClientIP: ClientIP(nil)},
		Auth: &auth.Service{},
		Git:  smarthttp.New(smarthttp.Deps{}),
		SPA:  fstest.MapFS{"index.html": {Data: []byte("<html>")}},
	}).Handler()
	want := map[string]bool{
		"POST /api/v1/repos/{owner}/{repo}/pulls/{n}/merge": false,
		"GET /api/v1/repos/{owner}/{repo}/issues":           false,
		"GET /{owner}/{repo}/info/refs":                     false,
	}
	err := chi.Walk(h.(chi.Routes), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		key := method + " " + strings.ReplaceAll(route, "/*/", "/")
		if _, ok := want[key]; ok {
			want[key] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for route, found := range want {
		if !found {
			t.Errorf("route %s not registered", route)
		}
	}
}
