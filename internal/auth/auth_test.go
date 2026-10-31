package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"anvil/internal/config"
	"anvil/internal/errs"
	"anvil/internal/store"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	h, err := HashPassword("correct horse battery")
	if err != nil || !strings.HasPrefix(h, "$argon2id$v=19$m=65536,t=3,p=2$") {
		t.Fatalf("%q %v", h, err)
	}
	if ok, err := VerifyPassword(h, "correct horse battery"); !ok || err != nil {
		t.Fatal("correct password rejected", err)
	}
	if ok, _ := VerifyPassword(h, "correct horse batterY"); ok {
		t.Fatal("wrong password accepted")
	}
	h2, _ := HashPassword("correct horse battery")
	if h == h2 {
		t.Fatal("salt not random")
	}
}

func TestPasswordPolicy(t *testing.T) {
	for pw, ok := range map[string]bool{
		"short":              false, // too short
		"1234567890":         false, // in the common-passwords list
		"alice-is-great-99":  false, // contains the username
		"tangerine-orbit-42": true,
	} {
		err := CheckPasswordPolicy(pw, "alice")
		if (err == nil) != ok {
			t.Errorf("%q: err=%v, want ok=%v", pw, err, ok)
		}
		if err != nil && !errors.Is(err, errs.Invalid) {
			t.Errorf("%q: not errs.Invalid: %v", pw, err)
		}
	}
}

func TestNewToken(t *testing.T) {
	p, h, err := NewToken(PrefixPAT)
	if err != nil || len(p) != len(PrefixPAT)+40 || h != HashSecret(p) {
		t.Fatalf("%q %q %v", p, h, err)
	}
}

func TestScopes(t *testing.T) {
	a := Actor{Kind: UserActor, IsAdmin: true, Scopes: []string{ScopeRepoWrite}}
	if !a.HasScope(ScopeRepoRead) || a.HasScope(ScopePackageWrite) || a.IsSiteAdmin() {
		t.Fatal("scope rules wrong")
	}
	if !(Actor{Kind: UserActor, IsAdmin: true}).IsSiteAdmin() {
		t.Fatal("session admin should be site admin")
	}
}

func newService(t *testing.T) (*Service, *store.DB, *store.User) {
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
	hash, _ := HashPassword("tangerine-orbit-42")
	u := &store.User{Username: "Alice", Email: "alice@example.com", PasswordHash: hash, IsActive: true}
	if err := db.Tx(ctx, func(tx *store.Tx) error { var err error; u.ID, err = tx.InsertUser(ctx, u); return err }); err != nil {
		t.Fatal(err)
	}
	return NewService(db, nil, Config{SessionTTL: 14 * 24 * time.Hour}), db, u
}

func TestLoginAndSession(t *testing.T) {
	ctx := context.Background()
	s, _, u := newService(t)
	sess, err := s.Login(ctx, "alice", "tangerine-orbit-42", "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	a, stale, err := s.Authenticate(ctx, Credentials{SessionCookie: sess.Cookie})
	if err != nil || stale || a.UserID != u.ID || a.Via != ViaSession || a.CSRFToken != sess.CSRFToken {
		t.Fatalf("%+v %v %v", a, stale, err)
	}
	// A second login gets a different session id (no fixation).
	sess2, _ := s.Login(ctx, "alice", "tangerine-orbit-42", "", "")
	if sess2.Cookie == sess.Cookie {
		t.Fatal("session id reused")
	}
	if err := s.Logout(ctx, sess.Cookie); err != nil {
		t.Fatal(err)
	}
	a, stale, _ = s.Authenticate(ctx, Credentials{SessionCookie: sess.Cookie})
	if !a.IsAnonymous() || !stale {
		t.Fatalf("logged-out cookie still works: %+v", a)
	}
}

// Enumeration: unknown user and wrong password give the same error.
func TestLoginErrorsAreIdentical(t *testing.T) {
	ctx := context.Background()
	s, _, _ := newService(t)
	_, e1 := s.Login(ctx, "nobody", "tangerine-orbit-42", "", "")
	_, e2 := s.Login(ctx, "alice", "wrong-password-123", "", "")
	if e1 == nil || e2 == nil || e1.Error() != e2.Error() {
		t.Fatalf("%v / %v", e1, e2)
	}
}

func TestSessionExpiresAbsolutely(t *testing.T) {
	ctx := context.Background()
	s, _, _ := newService(t)
	clock := time.Now()
	s.now = func() time.Time { return clock }
	sess, _ := s.Login(ctx, "alice", "tangerine-orbit-42", "", "")
	for i := 0; i < 40; i++ { // active every day for 40 days
		clock = clock.Add(24 * time.Hour)
		a, _, _ := s.Authenticate(ctx, Credentials{SessionCookie: sess.Cookie})
		if i >= 30 && !a.IsAnonymous() {
			t.Fatalf("session alive after %d days", i+1)
		}
	}
}

func TestTokens(t *testing.T) {
	ctx := context.Background()
	s, db, u := newService(t)
	if _, _, err := s.CreateToken(ctx, u.ID, "", nil, nil); !errors.Is(err, errs.Invalid) {
		t.Fatalf("empty token accepted: %v", err)
	}
	plain, tok, err := s.CreateToken(ctx, u.ID, "ci", []string{ScopeRepoRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(tok.Hash, plain) || tok.Prefix != plain[:12] {
		t.Fatal("token stored badly")
	}
	for _, c := range []Credentials{{Bearer: plain}, {HasBasic: true, BasicUser: "x", BasicPassword: plain}} {
		a, _, err := s.Authenticate(ctx, c)
		if err != nil || a.UserID != u.ID || a.Via != ViaToken || a.HasScope(ScopeRepoWrite) {
			t.Fatalf("%+v %v", a, err)
		}
	}
	// Password in Basic auth is refused unless allowed.
	if _, _, err := s.Authenticate(ctx, Credentials{HasBasic: true, BasicUser: "alice", BasicPassword: "tangerine-orbit-42"}); !errors.Is(err, errs.Unauthenticated) {
		t.Fatalf("password git auth allowed: %v", err)
	}
	// Deactivated users' tokens stop working.
	db.Tx(ctx, func(tx *store.Tx) error { return tx.SetUserFlags(ctx, u.ID, false, false, 0) })
	if _, _, err := s.Authenticate(ctx, Credentials{Bearer: plain}); !errors.Is(err, errs.Unauthenticated) {
		t.Fatalf("inactive user's token works: %v", err)
	}
	db.Tx(ctx, func(tx *store.Tx) error { return tx.SetUserFlags(ctx, u.ID, false, true, 0) })
	if err := s.RevokeToken(ctx, u.ID, tok.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Authenticate(ctx, Credentials{Bearer: plain}); !errors.Is(err, errs.Unauthenticated) {
		t.Fatalf("revoked token works: %v", err)
	}
	if err := s.RevokeToken(ctx, u.ID+1, tok.ID); !errors.Is(err, errs.NotFound) {
		t.Fatalf("revoking another user's token: %v", err)
	}
}

func TestExpiredToken(t *testing.T) {
	ctx := context.Background()
	s, _, u := newService(t)
	exp := time.Now().Add(time.Hour)
	plain, _, err := s.CreateToken(ctx, u.ID, "short", []string{ScopeRepoRead}, &exp)
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return exp.Add(time.Second) }
	if _, _, err := s.Authenticate(ctx, Credentials{Bearer: plain}); !errors.Is(err, errs.Unauthenticated) {
		t.Fatalf("expired token works: %v", err)
	}
}
