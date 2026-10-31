package auth

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"anvil/internal/errs"
	"anvil/internal/events"
	"anvil/internal/store"
)

// Session lifetimes from docs/reference/10 §1.
const (
	// SessionAbsoluteTTL is the longest a session can live, however active it is.
	SessionAbsoluteTTL = 30 * 24 * time.Hour
	// touchEvery limits how often an active session's expiry is written back.
	touchEvery = time.Minute
)

// ErrInvalidLogin is returned for every failed sign-in, whatever the reason, so that the
// response does not reveal whether the username exists (docs/reference/10 §3).
var ErrInvalidLogin = fmt.Errorf("invalid username or password: %w", errs.Unauthenticated)

// Config is the part of the server configuration auth needs.
type Config struct {
	SessionTTL       time.Duration // sliding session lifetime
	AllowPasswordGit bool          // accept passwords in HTTP Basic auth
}

// Service authenticates requests and manages sessions and tokens. Safe for concurrent use.
type Service struct {
	db  *store.DB
	bus *events.Bus
	cfg Config
	now func() time.Time
}

// NewService returns a Service. bus may be nil in tests.
func NewService(db *store.DB, bus *events.Bus, cfg Config) *Service {
	return &Service{db: db, bus: bus, cfg: cfg, now: time.Now}
}

// Session is a newly created browser session. Cookie is shown to the browser once and
// never stored; only its hash is.
type Session struct {
	Cookie    string
	CSRFToken string
	ExpiresAt time.Time
	User      *store.User
}

// Login checks credentials and creates a session. It returns the cookie value and CSRF token.
// Every failure (unknown user, wrong password, inactive account) returns ErrInvalidLogin.
// It publishes auth.login on success. Callers must rate-limit failures (docs/reference/10 §3).
func (s *Service) Login(ctx context.Context, username, password, ip, ua string) (*Session, error) {
	u, err := s.checkPassword(ctx, username, password)
	if err != nil {
		return nil, err
	}
	cookie, err := randomString(32)
	if err != nil {
		return nil, err
	}
	csrf, err := randomString(32)
	if err != nil {
		return nil, err
	}
	now := s.now()
	expires := now.Add(s.cfg.SessionTTL)
	row := &store.Session{
		IDHash: HashSecret(cookie), UserID: u.ID, CSRFToken: csrf, IP: ip, UserAgent: truncate(ua, 256),
		CreatedAt: now.UnixMilli(), LastSeenAt: now.UnixMilli(), ExpiresAt: expires.UnixMilli(),
	}
	if err := s.db.Tx(ctx, func(tx *store.Tx) error { return tx.InsertSession(ctx, row) }); err != nil {
		return nil, err
	}
	s.publish(ctx, "auth.login", u.ID, map[string]string{"ip": ip})
	return &Session{Cookie: cookie, CSRFToken: csrf, ExpiresAt: expires, User: u}, nil
}

// checkPassword returns the active user whose password matches, or ErrInvalidLogin.
func (s *Service) checkPassword(ctx context.Context, username, password string) (*store.User, error) {
	u, err := s.db.UserByUsername(ctx, username)
	if errors.Is(err, errs.NotFound) {
		_, _ = VerifyPassword(dummyHash(), password) // equalise timing with the real path
		return nil, ErrInvalidLogin
	}
	if err != nil {
		return nil, err
	}
	ok, err := VerifyPassword(u.PasswordHash, password)
	if err != nil {
		return nil, err
	}
	if !ok || !u.IsActive {
		return nil, ErrInvalidLogin
	}
	return u, nil
}

// Logout deletes the session identified by cookie value. An unknown cookie is not an error.
func (s *Service) Logout(ctx context.Context, cookie string) error {
	err := s.db.Tx(ctx, func(tx *store.Tx) error { return tx.DeleteSession(ctx, HashSecret(cookie), 0) })
	if errors.Is(err, errs.NotFound) {
		return nil
	}
	return err
}

// Credentials are the raw credentials found on a request. The HTTP layer fills them in.
type Credentials struct {
	SessionCookie string // value of the anvil_session cookie
	Bearer        string // token from "Authorization: Bearer …"
	BasicUser     string // from "Authorization: Basic …"
	BasicPassword string
	HasBasic      bool
}

// Authenticate resolves credentials to an Actor (FR-AUTH-05/06). With no credentials it
// returns the anonymous Actor, not an error. A Bearer or Basic credential that is wrong
// returns errs.Unauthenticated. A stale or unknown session cookie is treated as no
// credentials (it returns anonymous with staleCookie=true so the caller can clear it),
// because browsers keep sending old cookies and public pages must still work.
func (s *Service) Authenticate(ctx context.Context, c Credentials) (actor Actor, staleCookie bool, err error) {
	switch {
	case c.Bearer != "":
		a, err := s.actorFromToken(ctx, c.Bearer)
		return a, false, err
	case c.HasBasic:
		if strings.HasPrefix(c.BasicPassword, PrefixPAT) {
			a, err := s.actorFromToken(ctx, c.BasicPassword) // git clients put the token in the password
			return a, false, err
		}
		if !s.cfg.AllowPasswordGit {
			return Actor{}, false, fmt.Errorf("use a personal access token as the password: %w", errs.Unauthenticated)
		}
		u, err := s.checkPassword(ctx, c.BasicUser, c.BasicPassword)
		if err != nil {
			return Actor{}, false, err
		}
		return userActor(u, ViaPassword, nil), false, nil
	case c.SessionCookie != "":
		a, err := s.actorFromSession(ctx, c.SessionCookie)
		if errors.Is(err, errs.Unauthenticated) {
			return Actor{Kind: Anonymous}, true, nil
		}
		return a, false, err
	}
	return Actor{Kind: Anonymous}, false, nil
}

func userActor(u *store.User, via Via, scopes []string) Actor {
	return Actor{Kind: UserActor, UserID: u.ID, Username: u.Username, IsAdmin: u.IsAdmin, Via: via, Scopes: scopes}
}

func (s *Service) actorFromSession(ctx context.Context, cookie string) (Actor, error) {
	now := s.now()
	hash := HashSecret(cookie)
	sess, err := s.db.SessionByHash(ctx, hash, now.UnixMilli())
	if errors.Is(err, errs.NotFound) {
		return Actor{}, errs.Unauthenticated
	}
	if err != nil {
		return Actor{}, err
	}
	u, err := s.db.UserByID(ctx, sess.UserID)
	if errors.Is(err, errs.NotFound) || (err == nil && !u.IsActive) {
		return Actor{}, errs.Unauthenticated
	}
	if err != nil {
		return Actor{}, err
	}
	// Slide the expiry, at most once a minute, never past the absolute limit.
	if now.UnixMilli()-sess.LastSeenAt > touchEvery.Milliseconds() {
		expires := min(now.Add(s.cfg.SessionTTL).UnixMilli(), sess.CreatedAt+SessionAbsoluteTTL.Milliseconds())
		_ = s.db.Tx(ctx, func(tx *store.Tx) error { return tx.TouchSession(ctx, hash, now.UnixMilli(), expires) })
	}
	a := userActor(u, ViaSession, nil)
	a.SessionHash, a.CSRFToken = hash, sess.CSRFToken
	return a, nil
}

func (s *Service) actorFromToken(ctx context.Context, plain string) (Actor, error) {
	invalid := fmt.Errorf("invalid or expired token: %w", errs.Unauthenticated)
	if !strings.HasPrefix(plain, PrefixPAT) {
		return Actor{}, invalid
	}
	tok, err := s.db.TokenByHash(ctx, HashSecret(plain))
	if errors.Is(err, errs.NotFound) {
		return Actor{}, invalid
	}
	if err != nil {
		return Actor{}, err
	}
	now := s.now().UnixMilli()
	if tok.ExpiresAt != 0 && tok.ExpiresAt <= now {
		return Actor{}, invalid
	}
	u, err := s.db.UserByID(ctx, tok.UserID)
	if err != nil || !u.IsActive {
		return Actor{}, invalid
	}
	if now-tok.LastUsedAt > touchEvery.Milliseconds() {
		_ = s.db.Tx(ctx, func(tx *store.Tx) error { return tx.TouchToken(ctx, tok.ID, now) })
	}
	return userActor(u, ViaToken, tok.Scopes), nil
}

// CreateToken creates a personal access token for userID and returns the plaintext once.
// scopes must be non-empty and valid; expiresAt nil means it never expires. Returns
// errs.FieldErrors for bad input. Publishes auth.token_created.
func (s *Service) CreateToken(ctx context.Context, userID int64, name string, scopes []string, expiresAt *time.Time) (plain string, tok *store.AccessToken, err error) {
	problems := errs.FieldErrors{}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 100 {
		problems["name"] = "must be 1 to 100 characters"
	}
	if len(scopes) == 0 {
		problems["scopes"] = "choose at least one scope"
	}
	for _, sc := range scopes {
		if !slices.Contains(AllScopes, sc) {
			problems["scopes"] = fmt.Sprintf("unknown scope %q", sc)
		}
	}
	now := s.now()
	if expiresAt != nil && !expiresAt.After(now) {
		problems["expires_at"] = "must be in the future"
	}
	if err := problems.OrNil(); err != nil {
		return "", nil, err
	}
	plain, hash, err := NewToken(PrefixPAT)
	if err != nil {
		return "", nil, err
	}
	tok = &store.AccessToken{UserID: userID, Name: name, Prefix: plain[:12], Hash: hash, Scopes: slices.Clone(scopes), CreatedAt: now.UnixMilli()}
	if expiresAt != nil {
		tok.ExpiresAt = expiresAt.UnixMilli()
	}
	err = s.db.Tx(ctx, func(tx *store.Tx) error {
		tok.ID, err = tx.InsertToken(ctx, tok)
		return err
	})
	if err != nil {
		return "", nil, err
	}
	s.publish(ctx, "auth.token_created", userID, map[string]any{"token_id": tok.ID, "name": name, "scopes": scopes})
	return plain, tok, nil
}

// ListTokens returns userID's tokens without their secrets, newest first.
func (s *Service) ListTokens(ctx context.Context, userID int64) ([]store.AccessToken, error) {
	return s.db.ListTokens(ctx, userID)
}

// RevokeToken deletes a token owned by userID. Returns errs.NotFound if userID has no such
// token. Publishes auth.token_revoked.
func (s *Service) RevokeToken(ctx context.Context, userID, tokenID int64) error {
	if err := s.db.Tx(ctx, func(tx *store.Tx) error { return tx.DeleteToken(ctx, userID, tokenID) }); err != nil {
		return fmt.Errorf("token %d: %w", tokenID, err)
	}
	s.publish(ctx, "auth.token_revoked", userID, map[string]any{"token_id": tokenID})
	return nil
}

// ListSessions returns userID's active sessions, most recently used first (FR-AUTH-10).
func (s *Service) ListSessions(ctx context.Context, userID int64) ([]store.Session, error) {
	return s.db.ListSessions(ctx, userID, s.now().UnixMilli())
}

// RevokeSession ends one of userID's sessions, identified by its hash. errs.NotFound if
// userID has no such session.
func (s *Service) RevokeSession(ctx context.Context, userID int64, idHash string) error {
	return s.db.Tx(ctx, func(tx *store.Tx) error { return tx.DeleteSession(ctx, idHash, userID) })
}

// RevokeOtherSessions ends every session of userID except keepHash ("" ends all).
// Used after a password change (FR-AUTH-09) and when an account is deactivated.
func (s *Service) RevokeOtherSessions(ctx context.Context, userID int64, keepHash string) (int64, error) {
	var n int64
	err := s.db.Tx(ctx, func(tx *store.Tx) error {
		var err error
		n, err = tx.DeleteUserSessions(ctx, userID, keepHash)
		return err
	})
	return n, err
}

// PruneSessions deletes expired sessions. Run it periodically.
func (s *Service) PruneSessions(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.Tx(ctx, func(tx *store.Tx) error {
		var err error
		n, err = tx.DeleteExpiredSessions(ctx, s.now().UnixMilli())
		return err
	})
	return n, err
}

func (s *Service) publish(ctx context.Context, typ string, userID int64, payload any) {
	if s.bus == nil {
		return
	}
	_ = s.bus.Publish(ctx, events.Event{Type: typ, ActorKind: "user", ActorID: userID, Payload: payload})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
