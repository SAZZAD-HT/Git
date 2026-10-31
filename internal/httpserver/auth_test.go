package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"anvil/internal/api"
	"anvil/internal/auth"
	"anvil/internal/config"
	"anvil/internal/logging"
	"anvil/internal/store"
	"anvil/internal/users"
)

const testPassword = "tangerine-orbit-42"

// newIdentityServer starts a real server (SQLite, auth, users) with one admin and one user.
func newIdentityServer(t *testing.T, allowRegistration bool) *httptest.Server {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, config.DatabaseConfig{Driver: "sqlite"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Migrate(ctx, false); err != nil {
		t.Fatal(err)
	}
	a := auth.NewService(db, nil, auth.Config{SessionTTL: time.Hour})
	u := users.NewService(db, a, nil, users.Config{AllowRegistration: allowRegistration})
	for _, c := range []users.CreateUser{
		{Username: "admin1", Email: "admin@example.com", Password: testPassword, IsAdmin: true},
		{Username: "alice", Email: "alice@example.com", Password: testPassword},
	} {
		if _, err := u.Create(ctx, auth.SystemActor, c); err != nil {
			t.Fatal(err)
		}
	}
	h := New(Deps{
		Config: config.Default(),
		Logger: logging.NewWithWriter(io.Discard, "text", slog.LevelError),
		API: api.Deps{Auth: a, Users: u, AllowRegistration: allowRegistration, SessionTTL: time.Hour,
			ClientIP: ClientIP(nil)},
		Auth: a,
		SPA:  fstest.MapFS{"index.html": {Data: []byte("<html>")}},
	}).Handler()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

type client struct {
	t    *testing.T
	base string
	http *http.Client
	csrf string
}

func newClient(t *testing.T, srv *httptest.Server) *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: t, base: srv.URL, http: &http.Client{Jar: jar}}
}

func (c *client) do(method, path string, body any, headers ...string) (int, map[string]any) {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.base+path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.csrf != "" {
		req.Header.Set("X-CSRF-Token", c.csrf)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	raw, _ := io.ReadAll(res.Body)
	_ = json.Unmarshal(raw, &out)
	if out == nil && len(raw) > 0 && raw[0] == '[' {
		var list []any
		json.Unmarshal(raw, &list)
		out = map[string]any{"list": list}
	}
	return res.StatusCode, out
}

func (c *client) login(username string) {
	c.t.Helper()
	code, body := c.do("POST", "/api/v1/auth/login", map[string]string{"username": username, "password": testPassword})
	if code != 200 {
		c.t.Fatalf("login %s: %d %v", username, code, body)
	}
	c.csrf = body["csrf_token"].(string)
}

func errCode(body map[string]any) string {
	e, _ := body["error"].(map[string]any)
	s, _ := e["code"].(string)
	return s
}

func TestLoginFlowAndCSRF(t *testing.T) {
	srv := newIdentityServer(t, false)
	c := newClient(t, srv)
	if code, _ := c.do("GET", "/api/v1/user", nil); code != 401 {
		t.Fatalf("anonymous /user: %d", code)
	}
	c.login("alice")
	code, body := c.do("GET", "/api/v1/user", nil)
	if code != 200 || body["csrf_token"] != c.csrf {
		t.Fatalf("%d %v", code, body)
	}
	// Unsafe request without the CSRF header is rejected even with a valid cookie.
	csrf := c.csrf
	c.csrf = ""
	if code, body := c.do("PATCH", "/api/v1/user", map[string]string{"full_name": "Alice"}); code != 403 {
		t.Fatalf("missing CSRF: %d %v", code, body)
	}
	c.csrf = "wrong"
	if code, _ := c.do("PATCH", "/api/v1/user", map[string]string{"full_name": "Alice"}); code != 403 {
		t.Fatalf("wrong CSRF: %d", code)
	}
	c.csrf = csrf
	if code, body := c.do("PATCH", "/api/v1/user", map[string]string{"full_name": "Alice A."}); code != 200 || body["full_name"] != "Alice A." {
		t.Fatalf("with CSRF: %d %v", code, body)
	}
	// Form posts (cross-site) are rejected: JSON content type is required.
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/auth/login", strings.NewReader("username=alice&password="+testPassword))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if res, _ := http.DefaultClient.Do(req); res.StatusCode != 422 {
		t.Fatalf("form login: %d", res.StatusCode)
	}
	if code, _ := c.do("POST", "/api/v1/auth/logout", nil); code != 204 {
		t.Fatalf("logout: %d", code)
	}
	if code, _ := c.do("GET", "/api/v1/user", nil); code != 401 {
		t.Fatalf("after logout: %d", code)
	}
}

func TestTokenFlow(t *testing.T) {
	srv := newIdentityServer(t, false)
	c := newClient(t, srv)
	c.login("alice")
	code, body := c.do("POST", "/api/v1/user/tokens", map[string]any{"name": "laptop", "scopes": []string{"repo:read"}})
	if code != 201 {
		t.Fatalf("%d %v", code, body)
	}
	token := body["token"].(string)

	tc := newClient(t, srv) // no cookies: token only
	if code, body := tc.do("GET", "/api/v1/user", nil, "Authorization", "Bearer "+token); code != 200 || body["csrf_token"] != nil {
		t.Fatalf("token auth: %d %v", code, body)
	}
	// Token callers need no CSRF header, but cannot mint more tokens.
	if code, _ := tc.do("POST", "/api/v1/user/tokens", map[string]any{"name": "x", "scopes": []string{"repo:read"}}, "Authorization", "Bearer "+token); code != 403 {
		t.Fatalf("token created a token: %d", code)
	}
	if code, _ := tc.do("GET", "/api/v1/user", nil, "Authorization", "Bearer anv_pat_bogus"); code != 401 {
		t.Fatalf("bogus token: %d", code)
	}
	// The listing never includes the secret.
	_, list := c.do("GET", "/api/v1/user/tokens", nil)
	if b, _ := json.Marshal(list); strings.Contains(string(b), token) {
		t.Fatal("token secret listed")
	}
}

func TestLoginRateLimit(t *testing.T) {
	srv := newIdentityServer(t, false)
	c := newClient(t, srv)
	var last int
	for i := 0; i < 11; i++ {
		last, _ = c.do("POST", "/api/v1/auth/login", map[string]string{"username": "alice", "password": "wrong-password-1"})
	}
	if last != 429 {
		t.Fatalf("11th failed login: %d", last)
	}
	// Even the right password is refused while limited.
	if code, _ := c.do("POST", "/api/v1/auth/login", map[string]string{"username": "alice", "password": testPassword}); code != 429 {
		t.Fatalf("login while limited: %d", code)
	}
}

func TestAdminEndpoints(t *testing.T) {
	srv := newIdentityServer(t, false)
	user := newClient(t, srv)
	user.login("alice")
	if code, _ := user.do("GET", "/api/v1/admin/users", nil); code != 403 {
		t.Fatalf("non-admin listed users: %d", code)
	}
	admin := newClient(t, srv)
	admin.login("admin1")
	code, body := admin.do("POST", "/api/v1/admin/users", map[string]any{"username": "bob", "email": "bob@example.com", "password": testPassword})
	if code != 201 {
		t.Fatalf("%d %v", code, body)
	}
	if code, body := admin.do("GET", "/api/v1/admin/users?per_page=2", nil); code != 200 || len(body["list"].([]any)) != 2 {
		t.Fatalf("%d %v", code, body)
	}
	if code, body := admin.do("PATCH", "/api/v1/admin/users/bob", map[string]any{"is_active": false}); code != 200 || body["is_active"] != false {
		t.Fatalf("%d %v", code, body)
	}
	bob := newClient(t, srv)
	if code, body := bob.do("POST", "/api/v1/auth/login", map[string]string{"username": "bob", "password": testPassword}); code != 401 || errCode(body) != "unauthenticated" {
		t.Fatalf("deactivated login: %d %v", code, body)
	}
}

func TestRegistration(t *testing.T) {
	closed := newClient(t, newIdentityServer(t, false))
	if code, _ := closed.do("POST", "/api/v1/auth/register", map[string]string{"username": "zed", "email": "z@example.com", "password": testPassword}); code != 403 {
		t.Fatalf("registration while disabled: %d", code)
	}
	open := newClient(t, newIdentityServer(t, true))
	if code, body := open.do("POST", "/api/v1/auth/register", map[string]string{"username": "zed", "email": "z@example.com", "password": testPassword}); code != 201 || body["csrf_token"] == nil {
		t.Fatalf("%d %v", code, body)
	}
	if code, _ := open.do("GET", "/api/v1/user", nil); code != 200 {
		t.Fatal("not signed in after registering")
	}
}

func TestPublicProfileHidesEmail(t *testing.T) {
	srv := newIdentityServer(t, false)
	anon := newClient(t, srv)
	code, body := anon.do("GET", "/api/v1/users/ALICE", nil)
	if code != 200 || body["username"] != "alice" || body["email"] != nil {
		t.Fatalf("%d %v", code, body)
	}
}

func TestClientIP(t *testing.T) {
	f := ClientIP([]string{"10.0.0.0/8"})
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.1.2.3:5000"
	r.Header.Set("X-Forwarded-For", "6.6.6.6, 203.0.113.9, 10.0.0.5")
	if got := f(r); got != "203.0.113.9" {
		t.Fatalf("trusted proxy: %s", got)
	}
	r.RemoteAddr = "198.51.100.1:5000" // untrusted peer: header ignored
	if got := f(r); got != "198.51.100.1" {
		t.Fatalf("untrusted proxy: %s", got)
	}
}
