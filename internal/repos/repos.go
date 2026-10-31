package repos

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"anvil/internal/auth"
	"anvil/internal/errs"
	"anvil/internal/events"
	"anvil/internal/fsutil"
	"anvil/internal/git"
	"anvil/internal/store"
)

// nameRE is the repository name rule from docs/reference/04 §1 (case-insensitive; stored
// lower-cased on disk).
var nameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)

// HookInstaller writes the server hooks into a new repository. Implemented by
// internal/git/hooks; an interface so this package does not depend on the transport layer.
type HookInstaller interface {
	Install(repoPath string) error
}

// Deps are the Service's dependencies.
type Deps struct {
	DB         *store.DB
	Authorizer *auth.Authorizer
	Bus        *events.Bus // may be nil in tests
	Hooks      HookInstaller
	// ReposDir is <data_dir>/repos; every repository lives below it.
	ReposDir string
}

// Service manages repositories. Safe for concurrent use.
type Service struct {
	Deps
	now func() time.Time
}

// NewService returns a Service.
func NewService(d Deps) *Service { return &Service{Deps: d, now: time.Now} }

// ValidateName checks a repository name: 1–100 letters, digits, '.', '_' or '-', starting
// with a letter or digit, not ending in ".git", and not a reserved name.
func ValidateName(name string) error {
	lower := strings.ToLower(name)
	switch {
	case !nameRE.MatchString(name):
		return errors.New("use 1-100 letters, digits, '.', '_' or '-', starting with a letter or digit")
	case strings.HasSuffix(lower, ".git"):
		return errors.New(`must not end in ".git"`)
	case lower == "api" || lower == "con" || lower == "prn" || lower == "aux" || lower == "nul" ||
		regexp.MustCompile(`^(com|lpt)[1-9]$`).MatchString(lower):
		return errors.New("this name is reserved")
	case strings.HasSuffix(name, "."):
		return errors.New(`must not end in "."`) // Windows strips trailing dots
	}
	return nil
}

// PathFor returns the on-disk path of a repository and verifies it is inside data_dir/repos.
// It returns errs.Invalid for names that fail validation.
func (s *Service) PathFor(owner, name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", fmt.Errorf("repository name %q: %v: %w", name, err, errs.Invalid)
	}
	if !nameRE.MatchString(owner) {
		return "", fmt.Errorf("owner name %q: %w", owner, errs.Invalid)
	}
	p := filepath.Join(s.ReposDir, strings.ToLower(owner), strings.ToLower(name)+".git")
	rel, err := filepath.Rel(s.ReposDir, p)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("repository path escapes the data directory: %w", errs.Invalid)
	}
	return p, nil
}

// Open returns the git handle for a repository row.
func (s *Service) Open(r *store.Repository) (*git.Repository, error) {
	p, err := s.PathFor(r.OwnerName, r.Name)
	if err != nil {
		return nil, err
	}
	return git.Open(p)
}

// CreateRepo is the input to Create.
type CreateRepo struct {
	Name          string
	Description   string
	Private       bool
	DefaultBranch string // "" means main
	InitReadme    bool   // commit a README.md so the repository is not empty
}

// Create makes the bare repository on disk and the DB row, installs hooks, optionally
// commits an initial README, and publishes repo.created. The actor must be a signed-in user
// (with repo:write when using a token); the repository is created in their namespace.
//
// Ordering (docs/reference/05 §5): the directory is created first, then the row; if the
// row cannot be inserted the directory is removed, so no orphan rows point at nothing.
// Errors: errs.Unauthenticated, errs.Forbidden, errs.FieldErrors, errs.Conflict (name taken).
func (s *Service) Create(ctx context.Context, actor auth.Actor, in CreateRepo) (*store.Repository, error) {
	if !actor.IsUser() {
		return nil, errs.Unauthenticated
	}
	if !actor.HasScope(auth.ScopeRepoWrite) {
		return nil, fmt.Errorf("token lacks the repo:write scope: %w", errs.Forbidden)
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.DefaultBranch == "" {
		in.DefaultBranch = "main"
	}
	problems := errs.FieldErrors{}
	if err := ValidateName(in.Name); err != nil {
		problems["name"] = err.Error()
	}
	if !git.ValidBranchName(in.DefaultBranch) {
		problems["default_branch"] = "is not a valid branch name"
	}
	if len(in.Description) > 500 {
		problems["description"] = "must be at most 500 characters"
	}
	if err := problems.OrNil(); err != nil {
		return nil, err
	}
	if _, err := s.DB.RepoByName(ctx, actor.Username, in.Name); err == nil {
		return nil, errs.FieldErrors{"name": "you already have a repository with this name"}
	}
	path, err := s.PathFor(actor.Username, in.Name)
	if err != nil {
		return nil, err
	}
	if err := git.InitBare(ctx, path, in.DefaultBranch); err != nil {
		if errors.Is(err, errs.Conflict) {
			return nil, errs.FieldErrors{"name": "a repository directory with this name already exists on disk"}
		}
		return nil, err
	}
	cleanup := func() { _ = fsutil.RemoveAll(path) }
	if s.Hooks != nil {
		if err := s.Hooks.Install(path); err != nil {
			cleanup()
			return nil, fmt.Errorf("install hooks: %w", err)
		}
	}
	now := s.now()
	visibility := "public"
	if in.Private {
		visibility = "private"
	}
	repo := &store.Repository{OwnerID: actor.UserID, OwnerName: actor.Username, Name: in.Name, Description: strings.TrimSpace(in.Description),
		Visibility: visibility, DefaultBranch: in.DefaultBranch, IsEmpty: true, CreatedAt: now.UnixMilli(), UpdatedAt: now.UnixMilli()}
	if in.InitReadme {
		u, err := s.DB.UserByID(ctx, actor.UserID)
		if err != nil {
			cleanup()
			return nil, err
		}
		g, err := git.Open(path)
		if err != nil {
			cleanup()
			return nil, err
		}
		readme := "# " + in.Name + "\n"
		if repo.Description != "" {
			readme += "\n" + repo.Description + "\n"
		}
		name := u.FullName
		if name == "" {
			name = u.Username
		}
		if _, err := g.CreateInitialCommit(ctx, in.DefaultBranch, map[string][]byte{"README.md": []byte(readme)}, "Initial commit",
			git.Signature{Name: name, Email: u.Email, When: now}); err != nil {
			cleanup()
			return nil, fmt.Errorf("initial commit: %w", err)
		}
		repo.IsEmpty = false
	}
	err = s.DB.Tx(ctx, func(tx *store.Tx) error {
		repo.ID, err = tx.InsertRepo(ctx, repo)
		return err
	})
	if err != nil {
		cleanup()
		return nil, err
	}
	s.publish(ctx, actor, "repo.created", repo.ID, map[string]any{"full_name": repo.FullName(), "visibility": visibility})
	return repo, nil
}

// Get returns owner/name if actor can read it, otherwise errs.NotFound.
func (s *Service) Get(ctx context.Context, actor auth.Actor, owner, name string) (*store.Repository, error) {
	r, err := s.DB.RepoByName(ctx, owner, name)
	if err != nil {
		return nil, fmt.Errorf("repository %s/%s: %w", owner, name, err)
	}
	if err := s.Authorizer.Require(ctx, actor, r, auth.LevelRead); err != nil {
		return nil, err
	}
	return r, nil
}

// GetForAccess returns owner/name and checks actor has at least level. Used by the git
// endpoint and by every write operation.
func (s *Service) GetForAccess(ctx context.Context, actor auth.Actor, owner, name string, level auth.Level) (*store.Repository, error) {
	r, err := s.DB.RepoByName(ctx, owner, name)
	if err != nil {
		return nil, fmt.Errorf("repository %s/%s: %w", owner, name, err)
	}
	if err := s.Authorizer.Require(ctx, actor, r, level); err != nil {
		return nil, err
	}
	return r, nil
}

// Page selects one page of a list. Page numbers start at 1.
type Page struct{ Page, PerPage int }

// Normalize clamps p to page ≥ 1 and 1 ≤ per_page ≤ 100 (default 30).
func (p Page) Normalize() Page {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PerPage < 1 {
		p.PerPage = 30
	}
	return Page{p.Page, min(p.PerPage, 100)}
}

// ListForUser returns the repositories owned by owner that actor may see, and the total.
func (s *Service) ListForUser(ctx context.Context, actor auth.Actor, owner string, page Page) ([]store.Repository, int, error) {
	u, err := s.DB.UserByUsername(ctx, owner)
	if err != nil {
		return nil, 0, fmt.Errorf("user %s: %w", owner, err)
	}
	page = page.Normalize()
	viewer := int64(0)
	if actor.IsUser() && actor.HasScope(auth.ScopeRepoRead) {
		viewer = actor.UserID
	}
	return s.DB.ReposVisibleTo(ctx, u.ID, viewer, actor.IsSiteAdmin(), page.PerPage, (page.Page-1)*page.PerPage)
}

// ListMine returns repositories the actor owns or collaborates on, most recent first.
func (s *Service) ListMine(ctx context.Context, actor auth.Actor, limit int) ([]store.Repository, error) {
	if !actor.IsUser() {
		return nil, errs.Unauthenticated
	}
	return s.DB.ReposForMember(ctx, actor.UserID, limit)
}

// Permission returns actor's level on r (for showing what the UI may offer).
func (s *Service) Permission(ctx context.Context, actor auth.Actor, r *store.Repository) auth.Level {
	return s.Authorizer.Permission(ctx, actor, r)
}

// UpdateRepo is the input to Update. Nil fields are unchanged.
type UpdateRepo struct {
	Name          *string
	Description   *string
	Private       *bool
	DefaultBranch *string
}

// Update renames a repository or changes its description, visibility or default branch.
// Requires admin access. A rename moves the directory inside the DB transaction and moves
// it back if the transaction fails (docs/reference/04 §1). Publishes repo.updated.
func (s *Service) Update(ctx context.Context, actor auth.Actor, owner, name string, in UpdateRepo) (*store.Repository, error) {
	r, err := s.GetForAccess(ctx, actor, owner, name, auth.LevelAdmin)
	if err != nil {
		return nil, err
	}
	oldPath, err := s.PathFor(r.OwnerName, r.Name)
	if err != nil {
		return nil, err
	}
	problems := errs.FieldErrors{}
	upd := *r
	if in.Name != nil {
		upd.Name = strings.TrimSpace(*in.Name)
		if err := ValidateName(upd.Name); err != nil {
			problems["name"] = err.Error()
		}
	}
	if in.Description != nil {
		upd.Description = strings.TrimSpace(*in.Description)
		if len(upd.Description) > 500 {
			problems["description"] = "must be at most 500 characters"
		}
	}
	if in.Private != nil {
		upd.Visibility = map[bool]string{true: "private", false: "public"}[*in.Private]
	}
	g, err := git.Open(oldPath)
	if err != nil {
		return nil, err
	}
	if in.DefaultBranch != nil && *in.DefaultBranch != r.DefaultBranch {
		upd.DefaultBranch = *in.DefaultBranch
		if !git.ValidBranchName(upd.DefaultBranch) {
			problems["default_branch"] = "is not a valid branch name"
		} else if !r.IsEmpty {
			if _, err := g.ResolveCommit(ctx, "refs/heads/"+upd.DefaultBranch); err != nil {
				problems["default_branch"] = "branch does not exist"
			}
		}
	}
	if err := problems.OrNil(); err != nil {
		return nil, err
	}
	newPath, err := s.PathFor(r.OwnerName, upd.Name)
	if err != nil {
		return nil, err
	}
	renamed := !strings.EqualFold(newPath, oldPath)
	err = s.DB.Tx(ctx, func(tx *store.Tx) error {
		if err := tx.UpdateRepo(ctx, &upd, s.now().UnixMilli()); err != nil {
			return err
		}
		if renamed {
			if _, err := os.Stat(newPath); err == nil {
				return errs.FieldErrors{"name": "a repository directory with this name already exists"}
			}
			if err := os.Rename(oldPath, newPath); err != nil {
				return fmt.Errorf("rename repository directory: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		if renamed {
			if _, statErr := os.Stat(newPath); statErr == nil {
				_ = os.Rename(newPath, oldPath) // the commit failed after the move: undo it
			}
		}
		return nil, err
	}
	if upd.DefaultBranch != r.DefaultBranch {
		if g2, err := git.Open(newPath); err == nil {
			_ = g2.SetHead(ctx, upd.DefaultBranch)
		}
	}
	s.publish(ctx, actor, "repo.updated", r.ID, map[string]any{"full_name": upd.FullName()})
	return s.DB.RepoByID(ctx, r.ID)
}

// Delete removes the repository directory and all rows, after checking that confirm equals
// "owner/name" (case-insensitive). Requires admin access. Publishes repo.deleted.
func (s *Service) Delete(ctx context.Context, actor auth.Actor, owner, name, confirm string) error {
	r, err := s.GetForAccess(ctx, actor, owner, name, auth.LevelAdmin)
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(confirm), r.FullName()) {
		return errs.FieldErrors{"confirm": "type " + r.FullName() + " to confirm"}
	}
	path, err := s.PathFor(r.OwnerName, r.Name)
	if err != nil {
		return err
	}
	if err := s.DB.Tx(ctx, func(tx *store.Tx) error { return tx.DeleteRepo(ctx, r.ID) }); err != nil {
		return err
	}
	if err := fsutil.RemoveAll(path); err != nil {
		return fmt.Errorf("repository row deleted but directory %s remains: %w", path, err)
	}
	s.publish(ctx, actor, "repo.deleted", r.ID, map[string]any{"full_name": r.FullName()})
	return nil
}

// Collaborators returns the collaborators of owner/name. Requires read access.
func (s *Service) Collaborators(ctx context.Context, actor auth.Actor, owner, name string) ([]store.Collaborator, error) {
	r, err := s.Get(ctx, actor, owner, name)
	if err != nil {
		return nil, err
	}
	return s.DB.ListCollaborators(ctx, r.ID)
}

// SetCollaborator gives username the role on owner/name. Requires admin access. The owner
// cannot be added (they already have admin). Publishes repo.collaborator_set.
func (s *Service) SetCollaborator(ctx context.Context, actor auth.Actor, owner, name, username string, role auth.Level) error {
	r, err := s.GetForAccess(ctx, actor, owner, name, auth.LevelAdmin)
	if err != nil {
		return err
	}
	if role < auth.LevelRead {
		return errs.FieldErrors{"role": "must be read, write or admin"}
	}
	u, err := s.DB.UserByUsername(ctx, username)
	if err != nil {
		return fmt.Errorf("user %s: %w", username, err)
	}
	if u.ID == r.OwnerID {
		return errs.FieldErrors{"username": "the owner already has full access"}
	}
	if err := s.DB.Tx(ctx, func(tx *store.Tx) error {
		return tx.SetCollaborator(ctx, r.ID, u.ID, role.String(), s.now().UnixMilli())
	}); err != nil {
		return err
	}
	s.publish(ctx, actor, "repo.collaborator_set", r.ID, map[string]any{"user_id": u.ID, "role": role.String()})
	return nil
}

// RemoveCollaborator removes username from owner/name. Requires admin access, except that
// collaborators may remove themselves. errs.NotFound if they were not a collaborator.
func (s *Service) RemoveCollaborator(ctx context.Context, actor auth.Actor, owner, name, username string) error {
	u, err := s.DB.UserByUsername(ctx, username)
	if err != nil {
		return fmt.Errorf("user %s: %w", username, err)
	}
	level := auth.LevelAdmin
	if actor.UserID == u.ID {
		level = auth.LevelRead
	}
	r, err := s.GetForAccess(ctx, actor, owner, name, level)
	if err != nil {
		return err
	}
	if err := s.DB.Tx(ctx, func(tx *store.Tx) error { return tx.RemoveCollaborator(ctx, r.ID, u.ID) }); err != nil {
		return fmt.Errorf("collaborator %s: %w", username, err)
	}
	s.publish(ctx, actor, "repo.collaborator_removed", r.ID, map[string]any{"user_id": u.ID})
	return nil
}

func (s *Service) publish(ctx context.Context, actor auth.Actor, typ string, repoID int64, payload any) {
	if s.Bus == nil {
		return
	}
	kind := "user"
	if !actor.IsUser() {
		kind = "system"
	}
	_ = s.Bus.Publish(ctx, events.Event{Type: typ, RepoID: repoID, ActorKind: kind, ActorID: actor.UserID, Payload: payload})
}
