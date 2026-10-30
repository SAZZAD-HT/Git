package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"anvil/internal/api"
	"anvil/internal/config"
	"anvil/internal/logging"
)

type fakeCheck struct{ err error }

func (f fakeCheck) Name() string                    { return "fake" }
func (f fakeCheck) Ready(ctx context.Context) error { return f.err }

func newTestServer(t *testing.T, logs *bytes.Buffer, check error) http.Handler {
	t.Helper()
	spa := fstest.MapFS{
		"index.html":         {Data: []byte("<!doctype html><title>Anvil</title>")},
		"assets/app-1a2b.js": {Data: []byte("console.log(1)")},
	}
	return New(Deps{
		Config:   config.Default(),
		Logger:   logging.NewWithWriter(logs, "json", slog.LevelInfo),
		Checkers: []ReadyChecker{fakeCheck{check}},
		API:      api.Deps{GitVersion: "2.53.0"},
		SPA:      spa,
	}).Handler()
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestReadyz(t *testing.T) {
	var logs bytes.Buffer
	if rec := get(newTestServer(t, &logs, nil), "/readyz"); rec.Code != 200 {
		t.Fatalf("ready: %d %s", rec.Code, rec.Body)
	}
	rec := get(newTestServer(t, &logs, errors.New("db down")), "/readyz")
	if rec.Code != 503 || !strings.Contains(rec.Body.String(), "db down") {
		t.Fatalf("not ready: %d %s", rec.Code, rec.Body)
	}
}

func TestVersion(t *testing.T) {
	var logs bytes.Buffer
	rec := get(newTestServer(t, &logs, nil), "/api/v1/version")
	var body map[string]string
	json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != 200 || body["git_version"] != "2.53.0" || body["version"] == "" || body["go_version"] == "" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestUnknownAPIRouteIsJSON404(t *testing.T) {
	var logs bytes.Buffer
	rec := get(newTestServer(t, &logs, nil), "/api/v1/nope")
	var body struct {
		Error     struct{ Code string } `json:"error"`
		RequestID string                `json:"request_id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != 404 || body.Error.Code != "not_found" || body.RequestID != rec.Header().Get("X-Request-ID") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestSPAFallback(t *testing.T) {
	var logs bytes.Buffer
	h := newTestServer(t, &logs, nil)
	if rec := get(h, "/alice/demo/pulls"); rec.Code != 200 || !strings.Contains(rec.Body.String(), "<title>Anvil") {
		t.Fatalf("client route: %d %s", rec.Code, rec.Body)
	}
	rec := get(h, "/assets/app-1a2b.js")
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("asset: %d %v", rec.Code, rec.Header())
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("security headers missing")
	}
}

// TC-NFR-11: request log lines carry request_id and trace_id.
func TestAccessLogHasIDs(t *testing.T) {
	var logs bytes.Buffer
	h := newTestServer(t, &logs, nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	h.ServeHTTP(httptest.NewRecorder(), req)
	var line map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &line); err != nil {
		t.Fatalf("%v: %s", err, logs.String())
	}
	if line["request_id"] == "" || line["trace_id"] != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("log line: %v", line)
	}
}

func TestRecover(t *testing.T) {
	var logs bytes.Buffer
	h := Trace(logging.NewWithWriter(&logs, "text", slog.LevelInfo))(Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})))
	rec := get(h, "/")
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestRequestIDsSort(t *testing.T) {
	a := api.NewRequestID()
	if len(a) != 26 {
		t.Fatalf("len %d: %s", len(a), a)
	}
}
