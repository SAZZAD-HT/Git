package users

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"anvil/internal/auth"
	"anvil/internal/errs"
	"anvil/internal/events"
	"anvil/internal/store"
)

// usernameRE is the username rule from docs/reference/05 §3.1. It also keeps names safe
// as directory names on Windows and Linux.
var usernameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)

// reserved names cannot be usernames: Windows device names (so a backup made on Linux
// restores on Windows, docs/reference/12 §2.1) and top-level routes of the web UI and API.
var reserved = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true, "com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true, "lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
	"admin": true, "api": true, "assets": true, "explore": true, "explorer": true, "healthz": true, "readyz": true,
	"login": true, "logout": true, "register": true, "new": true, "settings": true, "user": true, "users": true,
	"help": true, "static": true, "favicon.svg": true, "metrics": true, "anvil": true, "ghost": true,
}

// Config is the part of the server configuration users needs.
type Config struct {
	AllowRegistration bool
}

// Service manages user accounts. Safe for concurrent use.
type Service struct {
	db   *store.DB
	auth *auth.Service
	bus  *events.Bus
	cfg  Config
	now  func() time.Time
}

// NewService returns a Service. bus may be nil in tests.
func NewService(db *store.DB, a *auth.Service, bus *events.Bus, cfg Config) *Service {
	return &Service{db: db, auth: a, bus: bus, cfg: cfg, now: time.Now}
}

// CreateUser is the input to Create.
type CreateUser struct {
	Username string
	Email    string
	FullName string
	Password string
	IsAdmin  bool
}

// Create adds an account. Allowed for site admins and the system actor; anonymous callers
// may create a non-admin account only when registration is enabled (FR-AUTH-03).
// It returns errs.Forbidden when not allowed, errs.FieldErrors for invalid input, and
// errs.Conflict when the username or email is taken. Publishes user.created.
func (s *Service) Create(ctx context.Context, actor auth.Actor, in CreateUser) (*store.User, error) {
	switch {
	case actor.IsSiteAdmin():
	case actor.IsAnonymous() && s.cfg.AllowRegistration && !in.IsAdmin:
	case actor.IsAnonymous():
		return nil, fmt.Errorf("registration is disabled on this server: %w", errs.Forbidden)
	default:
		return nil, fmt.Errorf("only site admins can create users: %w", errs.Forbidden)
	}
	in.Username = strings.TrimSpace(in.Username)
	in.Email = strings.TrimSpace(in.Email)
	in.FullName = strings.TrimSpace(in.FullName)
	problems := errs.FieldErrors{}
	if err := ValidateUsername(in.Username); err != nil {
		problems["username"] = err.Error()
	}
	if err := validateEmail(in.Email); err != nil {
		problems["email"] = err.Error()
	}
	if len(in.FullName) > 100 {
		problems["full_name"] = "must be at most 100 characters"
	}
	if err := auth.CheckPasswordPolicy(in.Password, in.Username); err != nil {
		var fe errs.FieldErrors
		errors.As(err, &fe)
		problems["password"] = fe["password"]
	}
	if err := problems.OrNil(); err != nil {
		return nil, err
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return nil, err
	}
	now := s.now().UnixMilli()
	u := &store.User{Username: in.Username, Email: strings.ToLower(in.Email), FullName: in.FullName,
		PasswordHash: hash, IsAdmin: in.IsAdmin, IsActive: true, CreatedAt: now, UpdatedAt: now}
	err = s.db.Tx(ctx, func(tx *store.Tx) error {
		u.ID, err = tx.InsertUser(ctx, u)
		return err
	})
	if err != nil {
		return nil, err
	}
	s.publish(ctx, actor, "user.created", map[string]any{"user_id": u.ID, "username": u.Username, "is_admin": u.IsAdmin})
	return u, nil
}

// ValidateUsername checks the username rules: 1–39 letters, digits or hyphens, starting
// with a letter or digit, and not a reserved name.
func ValidateUsername(name string) error {
	if !usernameRE.MatchString(name) {
		return errors.New("use 1-39 letters, digits or hyphens, starting with a letter or digit")
	}
	if reserved[strings.ToLower(name)] {
		return errors.New("this name is reserved")
	}
	return nil
}

func validateEmail(email string) error {
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || len(email) > 254 {
		return errors.New("must be a valid email address")
	}
	return nil
}

// Get returns the user with username (case-insensitive), or errs.NotFound.
func (s *Service) Get(ctx context.Context, username string) (*store.User, error) {
	u, err := s.db.UserByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("user %s: %w", username, err)
	}
	return u, nil
}

// GetByID returns the user with id, or errs.NotFound.
func (s *Service) GetByID(ctx context.Context, id int64) (*store.User, error) {
	return s.db.UserByID(ctx, id)
}

// UpdateUser is the input to Update. Nil fields are left unchanged.
type UpdateUser struct {
	Email    *string
	FullName *string
}

// Update changes the profile of username. Allowed for the user themself and site admins;
// others get errs.NotFound or errs.Forbidden. Publishes user.updated.
func (s *Service) Update(ctx context.Context, actor auth.Actor, username string, in UpdateUser) (*store.User, error) {
	u, err := s.Get(ctx, username)
	if err != nil {
		return nil, err
	}
	if actor.UserID != u.ID && !actor.IsSiteAdmin() {
		return nil, errs.Forbidden
	}
	email, fullName := u.Email, u.FullName
	problems := errs.FieldErrors{}
	if in.Email != nil {
		email = strings.TrimSpace(*in.Email)
		if err := validateEmail(email); err != nil {
			problems["email"] = err.Error()
		}
	}
	if in.FullName != nil {
		fullName = strings.TrimSpace(*in.FullName)
		if len(fullName) > 100 {
			problems["full_name"] = "must be at most 100 characters"
		}
	}
	if err := problems.OrNil(); err != nil {
		return nil, err
	}
	if err := s.db.Tx(ctx, func(tx *store.Tx) error {
		return tx.UpdateUserProfile(ctx, u.ID, email, fullName, s.now().UnixMilli())
	}); err != nil {
		return nil, err
	}
	s.publish(ctx, actor, "user.updated", map[string]any{"user_id": u.ID})
	return s.db.UserByID(ctx, u.ID)
}

// ChangePassword verifies current, sets next, and revokes the user's other sessions
// (FR-AUTH-09). The actor must be a signed-in user. A wrong current password returns
// errs.FieldErrors for "current". Publishes user.password_changed.
func (s *Service) ChangePassword(ctx context.Context, actor auth.Actor, current, next string) error {
	if !actor.IsUser() {
		return errs.Unauthenticated
	}
	u, err := s.db.UserByID(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if ok, err := auth.VerifyPassword(u.PasswordHash, current); err != nil {
		return err
	} else if !ok {
		return errs.FieldErrors{"current": "is not your current password"}
	}
	if err := auth.CheckPasswordPolicy(next, u.Username); err != nil {
		return err
	}
	if err := s.setPassword(ctx, u.ID, next); err != nil {
		return err
	}
	if _, err := s.auth.RevokeOtherSessions(ctx, u.ID, actor.SessionHash); err != nil {
		return err
	}
	s.publish(ctx, actor, "user.password_changed", map[string]any{"user_id": u.ID})
	return nil
}

// ResetPassword sets a new password for username without knowing the old one and ends
// all of their sessions. Site admins and the CLI only. Publishes user.password_reset.
func (s *Service) ResetPassword(ctx context.Context, actor auth.Actor, username, next string) error {
	if !actor.IsSiteAdmin() {
		return errs.Forbidden
	}
	u, err := s.Get(ctx, username)
	if err != nil {
		return err
	}
	if err := auth.CheckPasswordPolicy(next, u.Username); err != nil {
		return err
	}
	if err := s.setPassword(ctx, u.ID, next); err != nil {
		return err
	}
	if _, err := s.auth.RevokeOtherSessions(ctx, u.ID, ""); err != nil {
		return err
	}
	s.publish(ctx, actor, "user.password_reset", map[string]any{"user_id": u.ID})
	return nil
}

func (s *Service) setPassword(ctx context.Context, userID int64, password string) error {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	return s.db.Tx(ctx, func(tx *store.Tx) error { return tx.SetUserPassword(ctx, userID, hash, s.now().UnixMilli()) })
}

// AdminUpdate is the input to SetFlags. Nil fields are left unchanged.
type AdminUpdate struct {
	IsAdmin  *bool
	IsActive *bool
}

// SetFlags promotes, demotes, activates or deactivates username (FR-ADM-01). Site admins
// only. It refuses (errs.Conflict) to leave the server without an active admin.
// Deactivating ends the user's sessions; their tokens stop working at once because
// authentication checks is_active. Publishes user.flags_changed.
func (s *Service) SetFlags(ctx context.Context, actor auth.Actor, username string, in AdminUpdate) (*store.User, error) {
	if !actor.IsSiteAdmin() {
		return nil, errs.Forbidden
	}
	u, err := s.Get(ctx, username)
	if err != nil {
		return nil, err
	}
	isAdmin, isActive := u.IsAdmin, u.IsActive
	if in.IsAdmin != nil {
		isAdmin = *in.IsAdmin
	}
	if in.IsActive != nil {
		isActive = *in.IsActive
	}
	err = s.db.Tx(ctx, func(tx *store.Tx) error {
		if err := tx.SetUserFlags(ctx, u.ID, isAdmin, isActive, s.now().UnixMilli()); err != nil {
			return err
		}
		n, err := tx.CountAdmins(ctx)
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("the server must keep at least one active admin: %w", errs.Conflict)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !isActive {
		if _, err := s.auth.RevokeOtherSessions(ctx, u.ID, ""); err != nil {
			return nil, err
		}
	}
	s.publish(ctx, actor, "user.flags_changed", map[string]any{"user_id": u.ID, "is_admin": isAdmin, "is_active": isActive})
	return s.db.UserByID(ctx, u.ID)
}

// Page selects one page of a list. Page numbers start at 1.
type Page struct {
	Page    int
	PerPage int
}

// Normalize clamps p to page ≥ 1 and 1 ≤ per_page ≤ 100 (default 30), per docs/reference/06 §1.
func (p Page) Normalize() Page {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PerPage < 1 {
		p.PerPage = 30
	}
	if p.PerPage > 100 {
		p.PerPage = 100
	}
	return p
}

// List returns one page of all users and the total count. Site admins only.
func (s *Service) List(ctx context.Context, actor auth.Actor, page Page) ([]store.User, int, error) {
	if !actor.IsSiteAdmin() {
		return nil, 0, errs.Forbidden
	}
	page = page.Normalize()
	return s.db.ListUsers(ctx, page.PerPage, (page.Page-1)*page.PerPage)
}

func (s *Service) publish(ctx context.Context, actor auth.Actor, typ string, payload any) {
	if s.bus == nil {
		return
	}
	kind := "user"
	if actor.Kind != auth.UserActor {
		kind = "system"
	}
	_ = s.bus.Publish(ctx, events.Event{Type: typ, ActorKind: kind, ActorID: actor.UserID, Payload: payload})
}
