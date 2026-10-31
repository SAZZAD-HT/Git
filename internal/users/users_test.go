package users

import (
	"context"
	"errors"
	"testing"
	"time"

	"anvil/internal/auth"
	"anvil/internal/config"
	"anvil/internal/errs"
	"anvil/internal/store"
)

func newService(t *testing.T, allowRegistration bool) (*Service, *auth.Service) {
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
	return NewService(db, a, nil, Config{AllowRegistration: allowRegistration}), a
}

const pw = "tangerine-orbit-42"

func TestCreatePermissions(t *testing.T) {
	ctx := context.Background()
	s, _ := newService(t, false)
	anon := auth.Actor{Kind: auth.Anonymous}
	if _, err := s.Create(ctx, anon, CreateUser{Username: "bob", Email: "bob@example.com", Password: pw}); !errors.Is(err, errs.Forbidden) {
		t.Fatalf("registration disabled but allowed: %v", err)
	}
	admin, err := s.Create(ctx, auth.SystemActor, CreateUser{Username: "root-admin", Email: "a@example.com", Password: pw, IsAdmin: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, auth.SystemActor, CreateUser{Username: "ROOT-admin", Email: "b@example.com", Password: pw}); !errors.Is(err, errs.Conflict) {
		t.Fatalf("case-insensitive duplicate allowed: %v", err)
	}
	user, _ := s.Create(ctx, auth.SystemActor, CreateUser{Username: "carol", Email: "c@example.com", Password: pw})
	carol := auth.Actor{Kind: auth.UserActor, UserID: user.ID}
	if _, err := s.Create(ctx, carol, CreateUser{Username: "dave", Email: "d@example.com", Password: pw}); !errors.Is(err, errs.Forbidden) {
		t.Fatalf("non-admin created a user: %v", err)
	}
	_ = admin
}

func TestRegistrationCannotCreateAdmin(t *testing.T) {
	s, _ := newService(t, true)
	anon := auth.Actor{Kind: auth.Anonymous}
	if _, err := s.Create(context.Background(), anon, CreateUser{Username: "eve", Email: "e@example.com", Password: pw, IsAdmin: true}); !errors.Is(err, errs.Forbidden) {
		t.Fatalf("self-registered admin: %v", err)
	}
	if _, err := s.Create(context.Background(), anon, CreateUser{Username: "eve", Email: "e@example.com", Password: pw}); err != nil {
		t.Fatal(err)
	}
}

func TestValidation(t *testing.T) {
	s, _ := newService(t, false)
	_, err := s.Create(context.Background(), auth.SystemActor, CreateUser{Username: "-bad", Email: "nope", Password: "short"})
	var fe errs.FieldErrors
	if !errors.As(err, &fe) || fe["username"] == "" || fe["email"] == "" || fe["password"] == "" {
		t.Fatalf("%v", err)
	}
	for _, name := range []string{"CON", "admin", "lpt1"} {
		if ValidateUsername(name) == nil {
			t.Errorf("reserved name %s accepted", name)
		}
	}
}

func TestChangePasswordRevokesOtherSessions(t *testing.T) {
	ctx := context.Background()
	s, a := newService(t, false)
	u, _ := s.Create(ctx, auth.SystemActor, CreateUser{Username: "frank", Email: "f@example.com", Password: pw})
	s1, _ := a.Login(ctx, "frank", pw, "", "")
	s2, _ := a.Login(ctx, "frank", pw, "", "")
	actor, _, _ := a.Authenticate(ctx, auth.Credentials{SessionCookie: s1.Cookie})
	if err := s.ChangePassword(ctx, actor, "wrong-current-pw", "new-password-xyz-9"); !errors.Is(err, errs.Invalid) {
		t.Fatalf("wrong current password accepted: %v", err)
	}
	if err := s.ChangePassword(ctx, actor, pw, "new-password-xyz-9"); err != nil {
		t.Fatal(err)
	}
	if a1, _, _ := a.Authenticate(ctx, auth.Credentials{SessionCookie: s1.Cookie}); a1.UserID != u.ID {
		t.Fatal("current session was revoked")
	}
	if a2, _, _ := a.Authenticate(ctx, auth.Credentials{SessionCookie: s2.Cookie}); !a2.IsAnonymous() {
		t.Fatal("other session survived a password change")
	}
}

func TestCannotRemoveLastAdmin(t *testing.T) {
	ctx := context.Background()
	s, _ := newService(t, false)
	s.Create(ctx, auth.SystemActor, CreateUser{Username: "only-admin", Email: "o@example.com", Password: pw, IsAdmin: true})
	f := false
	if _, err := s.SetFlags(ctx, auth.SystemActor, "only-admin", AdminUpdate{IsAdmin: &f}); !errors.Is(err, errs.Conflict) {
		t.Fatalf("last admin demoted: %v", err)
	}
	if _, err := s.SetFlags(ctx, auth.SystemActor, "only-admin", AdminUpdate{IsActive: &f}); !errors.Is(err, errs.Conflict) {
		t.Fatalf("last admin deactivated: %v", err)
	}
}
