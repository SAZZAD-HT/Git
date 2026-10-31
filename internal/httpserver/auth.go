package httpserver

import (
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"anvil/internal/api"
	"anvil/internal/auth"
	"anvil/internal/errs"
	"anvil/internal/logging"
	"anvil/internal/ratelimit"
)

// Authenticate resolves the request's credentials (session cookie, Bearer token or HTTP
// Basic) to an auth.Actor and stores it in the request context. Requests without
// credentials continue as anonymous; wrong Bearer/Basic credentials get 401 (with a
// WWW-Authenticate challenge so git prompts for a token). A stale session cookie is
// cleared and the request continues as anonymous.
func Authenticate(a *auth.Service, secureCookies bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var c auth.Credentials
			if h := r.Header.Get("Authorization"); h != "" {
				if tok, ok := strings.CutPrefix(h, "Bearer "); ok {
					c.Bearer = strings.TrimSpace(tok)
				} else if u, p, ok := r.BasicAuth(); ok {
					c.BasicUser, c.BasicPassword, c.HasBasic = u, p, true
				}
			}
			if ck, err := r.Cookie(api.SessionCookie); err == nil {
				c.SessionCookie = ck.Value
			}
			actor, stale, err := a.Authenticate(r.Context(), c)
			if err != nil {
				if c.HasBasic {
					w.Header().Set("WWW-Authenticate", `Basic realm="Anvil"`)
				}
				api.WriteError(w, r, err)
				return
			}
			if stale {
				api.ClearSessionCookie(w, secureCookies)
			}
			ctx := auth.WithActor(r.Context(), actor)
			if actor.IsUser() {
				ctx = logging.WithContext(ctx, logging.FromContext(ctx).With("user_id", actor.UserID))
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CSRF requires the X-CSRF-Token header to match the session's token on unsafe methods
// when the request is authenticated by session cookie (docs/reference/10 §3). Token and
// anonymous requests are not checked: browsers do not attach those credentials by themselves.
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			a := auth.ActorFrom(r.Context())
			if a.Via == auth.ViaSession {
				got := r.Header.Get("X-CSRF-Token")
				if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(a.CSRFToken)) != 1 {
					api.WriteError(w, r, fmt.Errorf("missing or wrong X-CSRF-Token header: %w", errs.Forbidden))
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// RateConfig sets the general API limits from docs/reference/06 §1.
type RateConfig struct {
	AnonymousPerMinute int // per client IP
	UserPerHour        int // per signed-in user
}

// DefaultRateConfig is 60 requests a minute per IP when anonymous, 5000 an hour per user.
var DefaultRateConfig = RateConfig{AnonymousPerMinute: 60, UserPerHour: 5000}

// RateLimit rejects requests over the limit with 429 and a Retry-After header. Signed-in
// users are counted by user id, everyone else by client IP. It must run after Authenticate.
func RateLimit(cfg RateConfig, clientIP func(*http.Request) string) func(http.Handler) http.Handler {
	anon := ratelimit.New(cfg.AnonymousPerMinute, time.Minute)
	users := ratelimit.New(cfg.UserPerHour, time.Hour)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			a := auth.ActorFrom(r.Context())
			var ok bool
			var retry time.Duration
			if a.IsUser() {
				ok, retry = users.Allow("u" + strconv.FormatInt(a.UserID, 10))
			} else {
				ok, retry = anon.Allow(clientIP(r))
			}
			if !ok {
				w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
				api.WriteError(w, r, fmt.Errorf("too many requests; try again later: %w", errs.RateLimited))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIP returns a function that finds the client's IP address. X-Forwarded-For is used
// only when the direct peer is one of the trusted proxy CIDRs (server.trusted_proxies);
// then the right-most address that is not itself a trusted proxy is the client.
func ClientIP(trusted []string) func(*http.Request) string {
	var nets []netip.Prefix
	for _, c := range trusted {
		if p, err := netip.ParsePrefix(c); err == nil {
			nets = append(nets, p)
		}
	}
	isTrusted := func(s string) bool {
		a, err := netip.ParseAddr(strings.TrimSpace(s))
		if err != nil {
			return false
		}
		for _, n := range nets {
			if n.Contains(a.Unmap()) {
				return true
			}
		}
		return false
	}
	return func(r *http.Request) string {
		peer, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			peer = r.RemoteAddr
		}
		if len(nets) == 0 || !isTrusted(peer) {
			return peer
		}
		hops := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
		for i := len(hops) - 1; i >= 0; i-- {
			h := strings.TrimSpace(hops[i])
			if h != "" && !isTrusted(h) {
				return h
			}
		}
		return peer
	}
}
