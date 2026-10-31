package api

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"anvil/internal/auth"
	"anvil/internal/errs"
	"anvil/internal/issues"
	"anvil/internal/logging"
	"anvil/internal/pulls"
	"anvil/internal/ratelimit"
	"anvil/internal/repos"
	"anvil/internal/users"
	"anvil/internal/version"
)

// Deps are the services the API handlers call. It grows phase by phase; every field is
// set by `anvil serve`.
type Deps struct {
	// GitVersion is the version of the git binary found at startup, e.g. "2.53.0".
	GitVersion string
	Auth       *auth.Service
	Users      *users.Service
	Repos      *repos.Service
	Issues     *issues.Service
	Pulls      *pulls.Service
	// PublicURL is server.public_url, used for clone URLs.
	PublicURL string
	// AllowRegistration mirrors auth.allow_registration so the web UI can show or hide sign-up.
	AllowRegistration bool
	// SecureCookies sets the Secure flag on the session cookie (when served over HTTPS).
	SecureCookies bool
	// SessionTTL is how long a new session cookie lives.
	SessionTTL time.Duration
	// ClientIP returns the caller's IP address, honouring trusted proxies.
	ClientIP func(*http.Request) string
}

// handlers holds Deps plus per-process state such as the sign-in failure limiter.
type handlers struct {
	Deps
	// loginFailures allows 10 failed sign-ins per minute per IP and per username (docs/reference/06 §1).
	loginFailuresIP   *ratelimit.Limiter
	loginFailuresUser *ratelimit.Limiter
}

// Mount registers every /api/v1 route on r. r is already scoped to /api/v1.
func Mount(r chi.Router, d Deps) {
	h := &handlers{Deps: d,
		loginFailuresIP:   ratelimit.New(10, time.Minute),
		loginFailuresUser: ratelimit.New(10, time.Minute),
	}
	r.Get("/version", h.getVersion)
	r.Get("/meta", h.getMeta)
	if d.Auth == nil { // health-only server (tests)
		return
	}
	h.mountAuth(r)
	h.mountUsers(r)
	h.mountAdmin(r)
	if d.Repos != nil {
		h.mountRepos(r)
	}
}

// versionResponse is the body of GET /api/v1/version.
type versionResponse struct {
	version.Info
	GitVersion string `json:"git_version"`
}

// getVersion serves GET /api/v1/version. It needs no authentication.
func (h *handlers) getVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, versionResponse{Info: version.Get(), GitVersion: h.GitVersion})
}

// getMeta serves GET /api/v1/meta: public instance settings the web UI needs before sign-in.
func (h *handlers) getMeta(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, map[string]any{"allow_registration": h.AllowRegistration})
}

// writeJSON writes v as a JSON response with the given status.
func writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logging.FromContext(r.Context()).Warn("write response", "err", err)
	}
}

// maxJSONBody is the largest JSON request body accepted.
const maxJSONBody = 1 << 20

// decodeJSON reads a JSON body into dst: 1 MiB limit, unknown fields rejected, and the
// Content-Type must be application/json. Requiring that type also blocks cross-site
// form posts, which cannot set it without a CORS preflight. Errors wrap errs.Invalid.
func decodeJSON(r *http.Request, dst any) error {
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" {
		return fmt.Errorf("Content-Type must be application/json: %w", errs.Invalid)
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, maxJSONBody+1))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid JSON body: %v: %w", err, errs.Invalid)
	}
	return nil
}

// page reads ?page= and ?per_page= (docs/reference/06 §1).
func page(r *http.Request) users.Page {
	p, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pp, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	return users.Page{Page: p, PerPage: pp}.Normalize()
}

// writePaged writes items with X-Total-Count and a Link header for next/last pages.
func writePaged(w http.ResponseWriter, r *http.Request, items any, p users.Page, total int) {
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	last := max(1, (total+p.PerPage-1)/p.PerPage)
	link := func(n int, rel string) string {
		q := r.URL.Query()
		q.Set("page", strconv.Itoa(n))
		q.Set("per_page", strconv.Itoa(p.PerPage))
		return fmt.Sprintf(`<%s?%s>; rel="%s"`, r.URL.Path, q.Encode(), rel)
	}
	l := link(last, "last")
	if p.Page < last {
		l = link(p.Page+1, "next") + ", " + l
	}
	w.Header().Set("Link", l)
	writeJSON(w, r, http.StatusOK, items)
}

// requireUser returns the signed-in actor, or writes 401 and returns false.
func requireUser(w http.ResponseWriter, r *http.Request) (auth.Actor, bool) {
	a := auth.ActorFrom(r.Context())
	if !a.IsUser() {
		WriteError(w, r, fmt.Errorf("sign in required: %w", errs.Unauthenticated))
		return a, false
	}
	return a, true
}
