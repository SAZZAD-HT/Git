package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"anvil/internal/auth"
	"anvil/internal/errs"
	"anvil/internal/store"
	"anvil/internal/users"
)

// SessionCookie is the name of the browser session cookie.
const SessionCookie = "anvil_session"

func (h *handlers) setSessionCookie(w http.ResponseWriter, s *auth.Session) {
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookie, Value: s.Cookie, Path: "/", Expires: s.ExpiresAt,
		HttpOnly: true, Secure: h.SecureCookies, SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie tells the browser to drop the session cookie.
func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
}

// userJSON is the public representation of a user. Email and flags are included only for
// the user themself and site admins (private=true).
type userJSON struct {
	ID        int64   `json:"id"`
	Username  string  `json:"username"`
	FullName  string  `json:"full_name"`
	Email     *string `json:"email,omitempty"`
	IsAdmin   bool    `json:"is_admin"`
	IsActive  *bool   `json:"is_active,omitempty"`
	CreatedAt string  `json:"created_at"`
}

func toUserJSON(u *store.User, private bool) userJSON {
	j := userJSON{ID: u.ID, Username: u.Username, FullName: u.FullName, IsAdmin: u.IsAdmin, CreatedAt: rfc3339(u.CreatedAt)}
	if private {
		email, active := u.Email, u.IsActive
		j.Email, j.IsActive = &email, &active
	}
	return j
}

// rfc3339 formats unix milliseconds as an RFC 3339 UTC timestamp, or "" for zero.
func rfc3339(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

func (h *handlers) mountAuth(r chi.Router) {
	r.Post("/auth/login", h.login)
	r.Post("/auth/logout", h.logout)
	r.Post("/auth/register", h.register)
	r.Get("/user", h.currentUser)
	r.Patch("/user", h.updateCurrentUser)
	r.Put("/user/password", h.changePassword)
	r.Get("/user/tokens", h.listTokens)
	r.Post("/user/tokens", h.createToken)
	r.Delete("/user/tokens/{id}", h.revokeToken)
	r.Get("/user/sessions", h.listSessions)
	r.Delete("/user/sessions/{id}", h.revokeSession)
}

// login serves POST /api/v1/auth/login: {username, password} → session cookie + user + CSRF token.
// Failed attempts are limited to 10 a minute per IP and per username (docs/reference/10 §3).
func (h *handlers) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &in); err != nil {
		WriteError(w, r, err)
		return
	}
	ip := h.ClientIP(r)
	userKey := strings.ToLower(strings.TrimSpace(in.Username))
	for _, l := range []struct {
		lim interface {
			Exceeded(string) (bool, time.Duration)
		}
		key string
	}{{h.loginFailuresIP, ip}, {h.loginFailuresUser, userKey}} {
		if over, retry := l.lim.Exceeded(l.key); over {
			w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
			WriteError(w, r, fmt.Errorf("too many failed sign-ins; try again later: %w", errs.RateLimited))
			return
		}
	}
	sess, err := h.Auth.Login(r.Context(), in.Username, in.Password, ip, r.UserAgent())
	if err != nil {
		if errors.Is(err, errs.Unauthenticated) {
			h.loginFailuresIP.Allow(ip)
			h.loginFailuresUser.Allow(userKey)
		}
		WriteError(w, r, err)
		return
	}
	h.loginFailuresUser.Reset(userKey)
	h.setSessionCookie(w, sess)
	writeJSON(w, r, http.StatusOK, map[string]any{"user": toUserJSON(sess.User, true), "csrf_token": sess.CSRFToken})
}

// logout serves POST /api/v1/auth/logout. It always clears the cookie.
func (h *handlers) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(SessionCookie); err == nil {
		if err := h.Auth.Logout(r.Context(), c.Value); err != nil {
			WriteError(w, r, err)
			return
		}
	}
	ClearSessionCookie(w, h.SecureCookies)
	w.WriteHeader(http.StatusNoContent)
}

// register serves POST /api/v1/auth/register (only when auth.allow_registration is true)
// and signs the new user in.
func (h *handlers) register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		FullName string `json:"full_name"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &in); err != nil {
		WriteError(w, r, err)
		return
	}
	actor := auth.ActorFrom(r.Context())
	if !actor.IsAnonymous() {
		WriteError(w, r, fmt.Errorf("already signed in: %w", errs.Conflict))
		return
	}
	if _, err := h.Users.Create(r.Context(), actor, users.CreateUser{Username: in.Username, Email: in.Email, FullName: in.FullName, Password: in.Password}); err != nil {
		WriteError(w, r, err)
		return
	}
	sess, err := h.Auth.Login(r.Context(), in.Username, in.Password, h.ClientIP(r), r.UserAgent())
	if err != nil {
		WriteError(w, r, err)
		return
	}
	h.setSessionCookie(w, sess)
	writeJSON(w, r, http.StatusCreated, map[string]any{"user": toUserJSON(sess.User, true), "csrf_token": sess.CSRFToken})
}

// currentUser serves GET /api/v1/user. For session callers it also returns the CSRF token,
// so the web app can recover it after a page reload.
func (h *handlers) currentUser(w http.ResponseWriter, r *http.Request) {
	a, ok := requireUser(w, r)
	if !ok {
		return
	}
	u, err := h.Users.GetByID(r.Context(), a.UserID)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	body := map[string]any{"user": toUserJSON(u, true)}
	if a.Via == auth.ViaSession {
		body["csrf_token"] = a.CSRFToken
	}
	writeJSON(w, r, http.StatusOK, body)
}

// updateCurrentUser serves PATCH /api/v1/user: {email?, full_name?}.
func (h *handlers) updateCurrentUser(w http.ResponseWriter, r *http.Request) {
	a, ok := requireUser(w, r)
	if !ok {
		return
	}
	var in struct {
		Email    *string `json:"email"`
		FullName *string `json:"full_name"`
	}
	if err := decodeJSON(r, &in); err != nil {
		WriteError(w, r, err)
		return
	}
	u, err := h.Users.Update(r.Context(), a, a.Username, users.UpdateUser{Email: in.Email, FullName: in.FullName})
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, toUserJSON(u, true))
}

// changePassword serves PUT /api/v1/user/password: {current, new}. Other sessions end.
func (h *handlers) changePassword(w http.ResponseWriter, r *http.Request) {
	a, ok := requireUser(w, r)
	if !ok {
		return
	}
	var in struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if err := decodeJSON(r, &in); err != nil {
		WriteError(w, r, err)
		return
	}
	if err := h.Users.ChangePassword(r.Context(), a, in.Current, in.New); err != nil {
		WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// tokenJSON describes a token without its secret.
type tokenJSON struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	Prefix     string   `json:"token_prefix"`
	Scopes     []string `json:"scopes"`
	ExpiresAt  string   `json:"expires_at,omitempty"`
	LastUsedAt string   `json:"last_used_at,omitempty"`
	CreatedAt  string   `json:"created_at"`
}

func toTokenJSON(t *store.AccessToken) tokenJSON {
	return tokenJSON{ID: t.ID, Name: t.Name, Prefix: t.Prefix, Scopes: t.Scopes,
		ExpiresAt: rfc3339(t.ExpiresAt), LastUsedAt: rfc3339(t.LastUsedAt), CreatedAt: rfc3339(t.CreatedAt)}
}

// listTokens serves GET /api/v1/user/tokens.
func (h *handlers) listTokens(w http.ResponseWriter, r *http.Request) {
	a, ok := requireUser(w, r)
	if !ok {
		return
	}
	toks, err := h.Auth.ListTokens(r.Context(), a.UserID)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	out := make([]tokenJSON, len(toks))
	for i := range toks {
		out[i] = toTokenJSON(&toks[i])
	}
	writeJSON(w, r, http.StatusOK, out)
}

// createToken serves POST /api/v1/user/tokens: {name, scopes, expires_at?}. The response
// carries the plaintext token in "token"; it is never shown again (FR-AUTH-04).
// Tokens cannot create tokens: only a session may, so a leaked token cannot mint more.
func (h *handlers) createToken(w http.ResponseWriter, r *http.Request) {
	a, ok := requireUser(w, r)
	if !ok {
		return
	}
	if a.Via != auth.ViaSession {
		WriteError(w, r, fmt.Errorf("tokens can only be created from a signed-in browser session: %w", errs.Forbidden))
		return
	}
	var in struct {
		Name      string     `json:"name"`
		Scopes    []string   `json:"scopes"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := decodeJSON(r, &in); err != nil {
		WriteError(w, r, err)
		return
	}
	plain, tok, err := h.Auth.CreateToken(r.Context(), a.UserID, in.Name, in.Scopes, in.ExpiresAt)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusCreated, struct {
		tokenJSON
		Token string `json:"token"`
	}{toTokenJSON(tok), plain})
}

// revokeToken serves DELETE /api/v1/user/tokens/{id}.
func (h *handlers) revokeToken(w http.ResponseWriter, r *http.Request) {
	a, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		WriteError(w, r, fmt.Errorf("token %q: %w", chi.URLParam(r, "id"), errs.NotFound))
		return
	}
	if err := h.Auth.RevokeToken(r.Context(), a.UserID, id); err != nil {
		WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listSessions serves GET /api/v1/user/sessions. "current" marks the caller's own session.
func (h *handlers) listSessions(w http.ResponseWriter, r *http.Request) {
	a, ok := requireUser(w, r)
	if !ok {
		return
	}
	sessions, err := h.Auth.ListSessions(r.Context(), a.UserID)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	type sessionJSON struct {
		ID         string `json:"id"`
		IP         string `json:"ip"`
		UserAgent  string `json:"user_agent"`
		CreatedAt  string `json:"created_at"`
		LastSeenAt string `json:"last_seen_at"`
		Current    bool   `json:"current"`
	}
	out := make([]sessionJSON, len(sessions))
	for i, s := range sessions {
		out[i] = sessionJSON{ID: s.IDHash, IP: s.IP, UserAgent: s.UserAgent, CreatedAt: rfc3339(s.CreatedAt),
			LastSeenAt: rfc3339(s.LastSeenAt), Current: s.IDHash == a.SessionHash}
	}
	writeJSON(w, r, http.StatusOK, out)
}

// revokeSession serves DELETE /api/v1/user/sessions/{id}.
func (h *handlers) revokeSession(w http.ResponseWriter, r *http.Request) {
	a, ok := requireUser(w, r)
	if !ok {
		return
	}
	if err := h.Auth.RevokeSession(r.Context(), a.UserID, chi.URLParam(r, "id")); err != nil {
		WriteError(w, r, fmt.Errorf("session: %w", err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
