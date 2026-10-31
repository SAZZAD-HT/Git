package auth

import (
	"context"
	"fmt"

	"anvil/internal/errs"
	"anvil/internal/store"
)

// Level is an access level on a repository, ordered: LevelNone < LevelRead < LevelWrite < LevelAdmin.
type Level int

// Access levels.
const (
	LevelNone Level = iota
	LevelRead
	LevelWrite
	LevelAdmin
)

// String returns the level's name as used in the API and the collaborators table.
func (l Level) String() string {
	return [...]string{"none", "read", "write", "admin"}[l]
}

// ParseLevel parses "read", "write" or "admin". Anything else returns errs.Invalid.
func ParseLevel(s string) (Level, error) {
	switch s {
	case "read":
		return LevelRead, nil
	case "write":
		return LevelWrite, nil
	case "admin":
		return LevelAdmin, nil
	}
	return LevelNone, fmt.Errorf("role must be read, write or admin: %w", errs.Invalid)
}

// Authorizer computes repository permissions. Safe for concurrent use.
type Authorizer struct {
	db                *store.DB
	requireSigninView bool
}

// NewAuthorizer returns an Authorizer. requireSigninView is auth.require_signin_view:
// when true, anonymous visitors cannot read public repositories.
func NewAuthorizer(db *store.DB, requireSigninView bool) *Authorizer {
	return &Authorizer{db: db, requireSigninView: requireSigninView}
}

// Permission returns the highest access level actor has on repo. It is the ONLY place
// repository access is computed; handlers and services call it instead of checking roles
// themselves (docs/reference/10 §2). Token scopes then cap the result: a token without
// repo:write gets at most read, and one without repo:read gets only what anonymous
// visitors get. A database error yields LevelNone (fail closed) and is not returned.
func (a *Authorizer) Permission(ctx context.Context, actor Actor, repo *store.Repository) Level {
	public := LevelNone
	if !repo.IsPrivate() && (!a.requireSigninView || !actor.IsAnonymous()) {
		public = LevelRead
	}
	var level Level
	switch {
	case actor.Kind == System:
		return LevelAdmin
	case !actor.IsUser():
		level = public // anonymous; runners and jobs are added in P5
	case actor.IsAdmin:
		level = LevelAdmin // site admin
	case actor.UserID == repo.OwnerID:
		level = LevelAdmin
	default:
		level = public
		if role, err := a.db.CollaboratorRole(ctx, repo.ID, actor.UserID); err == nil && role != "" {
			if l, err := ParseLevel(role); err == nil && l > level {
				level = l
			}
		}
	}
	if !actor.HasScope(ScopeRepoRead) {
		level = min(level, public)
	} else if !actor.HasScope(ScopeRepoWrite) {
		level = min(level, LevelRead)
	}
	return level
}

// Require returns errs.NotFound if actor cannot read repo (so private repositories are
// never revealed), or errs.Forbidden if they can read but lack level. Anonymous callers
// who could not read get errs.NotFound too; the git endpoint turns that into a 401
// challenge so git prompts for credentials.
func (a *Authorizer) Require(ctx context.Context, actor Actor, repo *store.Repository, level Level) error {
	have := a.Permission(ctx, actor, repo)
	switch {
	case have >= level:
		return nil
	case have == LevelNone:
		return fmt.Errorf("repository %s: %w", repo.FullName(), errs.NotFound)
	default:
		return fmt.Errorf("you need %s access to %s: %w", level, repo.FullName(), errs.Forbidden)
	}
}
