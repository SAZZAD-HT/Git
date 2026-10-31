package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"anvil/internal/errs"
)

// Repository is one row of the repositories table, with the owner's username joined in.
type Repository struct {
	ID            int64
	OwnerID       int64
	OwnerName     string // owner's username (display form)
	Name          string
	Description   string
	Visibility    string // public | private
	DefaultBranch string
	IsEmpty       bool
	ForkOfID      int64 // 0 = not a fork
	SizeBytes     int64
	CreatedAt     int64
	UpdatedAt     int64
}

// IsPrivate reports whether only collaborators may see the repository.
func (r *Repository) IsPrivate() bool { return r.Visibility == "private" }

// FullName returns "owner/name".
func (r *Repository) FullName() string { return r.OwnerName + "/" + r.Name }

const repoColumns = `r.id, r.owner_id, u.username, r.name, r.description, r.visibility, r.default_branch,
	r.is_empty, COALESCE(r.fork_of_id, 0), r.size_bytes, r.created_at, r.updated_at`

const repoFrom = " FROM repositories r JOIN users u ON u.id = r.owner_id "

func scanRepo(row interface{ Scan(...any) error }) (*Repository, error) {
	var r Repository
	err := row.Scan(&r.ID, &r.OwnerID, &r.OwnerName, &r.Name, &r.Description, &r.Visibility, &r.DefaultBranch,
		&r.IsEmpty, &r.ForkOfID, &r.SizeBytes, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.NotFound
	}
	return &r, err
}

func scanRepos(rows *sql.Rows, err error) ([]Repository, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Repository
	for rows.Next() {
		r, err := scanRepo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// InsertRepo stores r and returns its id. errs.Conflict if the owner already has a
// repository with that name (case-insensitive).
func (q *Queries) InsertRepo(ctx context.Context, r *Repository) (int64, error) {
	var forkOf any
	if r.ForkOfID != 0 {
		forkOf = r.ForkOfID
	}
	res, err := q.q.ExecContext(ctx,
		`INSERT INTO repositories (owner_id, name, name_lower, description, visibility, default_branch, is_empty, fork_of_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.OwnerID, r.Name, strings.ToLower(r.Name), r.Description, r.Visibility, r.DefaultBranch, r.IsEmpty, forkOf, r.CreatedAt, r.UpdatedAt)
	if isUniqueViolation(err) {
		return 0, fmt.Errorf("repository %s already exists: %w", r.Name, errs.Conflict)
	}
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// RepoByName returns owner/name (case-insensitive), or errs.NotFound.
func (q *Queries) RepoByName(ctx context.Context, owner, name string) (*Repository, error) {
	return scanRepo(q.q.QueryRowContext(ctx, "SELECT "+repoColumns+repoFrom+"WHERE u.username_lower = ? AND r.name_lower = ?",
		strings.ToLower(owner), strings.ToLower(name)))
}

// RepoByID returns the repository with id, or errs.NotFound.
func (q *Queries) RepoByID(ctx context.Context, id int64) (*Repository, error) {
	return scanRepo(q.q.QueryRowContext(ctx, "SELECT "+repoColumns+repoFrom+"WHERE r.id = ?", id))
}

// ReposVisibleTo lists repositories owned by ownerID that viewerID may see: all public
// ones plus private ones the viewer owns or collaborates on. viewerID 0 means anonymous;
// allPrivate shows every private repository (site admins). Ordered by name.
func (q *Queries) ReposVisibleTo(ctx context.Context, ownerID, viewerID int64, allPrivate bool, limit, offset int) ([]Repository, int, error) {
	where := `WHERE r.owner_id = ? AND (r.visibility = 'public' OR ? OR r.owner_id = ?
		OR EXISTS (SELECT 1 FROM collaborators c WHERE c.repo_id = r.id AND c.user_id = ?))`
	args := []any{ownerID, allPrivate, viewerID, viewerID}
	var total int
	if err := q.q.QueryRowContext(ctx, "SELECT COUNT(*) FROM repositories r "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	list, err := scanRepos(q.q.QueryContext(ctx, "SELECT "+repoColumns+repoFrom+where+" ORDER BY r.name_lower LIMIT ? OFFSET ?",
		append(args, limit, offset)...))
	return list, total, err
}

// ReposForMember lists repositories userID owns or collaborates on, most recently updated first.
func (q *Queries) ReposForMember(ctx context.Context, userID int64, limit int) ([]Repository, error) {
	return scanRepos(q.q.QueryContext(ctx, "SELECT "+repoColumns+repoFrom+
		`WHERE r.owner_id = ? OR EXISTS (SELECT 1 FROM collaborators c WHERE c.repo_id = r.id AND c.user_id = ?)
		 ORDER BY r.updated_at DESC LIMIT ?`, userID, userID, limit))
}

// UpdateRepo writes the editable fields of r. errs.Conflict if the new name is taken.
func (q *Queries) UpdateRepo(ctx context.Context, r *Repository, now int64) error {
	res, err := q.q.ExecContext(ctx,
		`UPDATE repositories SET name = ?, name_lower = ?, description = ?, visibility = ?, default_branch = ?, updated_at = ? WHERE id = ?`,
		r.Name, strings.ToLower(r.Name), r.Description, r.Visibility, r.DefaultBranch, now, r.ID)
	if isUniqueViolation(err) {
		return fmt.Errorf("repository %s already exists: %w", r.Name, errs.Conflict)
	}
	return affectedOne(res, err)
}

// MarkPushed records a push: the repository is no longer empty, its default branch is set
// (first push only, when defaultBranch is not ""), and updated_at moves.
func (q *Queries) MarkPushed(ctx context.Context, repoID int64, defaultBranch string, now int64) error {
	_, err := q.q.ExecContext(ctx,
		`UPDATE repositories SET is_empty = 0, default_branch = CASE WHEN ? != '' THEN ? ELSE default_branch END, updated_at = ? WHERE id = ?`,
		defaultBranch, defaultBranch, now, repoID)
	return err
}

// DeleteRepo removes the repository row; collaborators and protection rules cascade.
func (q *Queries) DeleteRepo(ctx context.Context, id int64) error {
	return affectedOne(q.q.ExecContext(ctx, "DELETE FROM repositories WHERE id = ?", id))
}

// ---- collaborators ----

// Collaborator is a user with an explicit role on a repository.
type Collaborator struct {
	UserID   int64
	Username string
	Role     string // read | write | admin
}

// CollaboratorRole returns userID's role on repoID, or "" if they are not a collaborator.
func (q *Queries) CollaboratorRole(ctx context.Context, repoID, userID int64) (string, error) {
	var role string
	err := q.q.QueryRowContext(ctx, "SELECT role FROM collaborators WHERE repo_id = ? AND user_id = ?", repoID, userID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return role, err
}

// SetCollaborator adds userID to repoID with role, or changes their role.
func (q *Queries) SetCollaborator(ctx context.Context, repoID, userID int64, role string, now int64) error {
	_, err := q.q.ExecContext(ctx,
		"INSERT INTO collaborators (repo_id, user_id, role, created_at) VALUES (?, ?, ?, ?) ON CONFLICT (repo_id, user_id) DO UPDATE SET role = excluded.role",
		repoID, userID, role, now)
	return err
}

// RemoveCollaborator removes userID from repoID; errs.NotFound if they were not one.
func (q *Queries) RemoveCollaborator(ctx context.Context, repoID, userID int64) error {
	return affectedOne(q.q.ExecContext(ctx, "DELETE FROM collaborators WHERE repo_id = ? AND user_id = ?", repoID, userID))
}

// ListCollaborators returns repoID's collaborators ordered by username.
func (q *Queries) ListCollaborators(ctx context.Context, repoID int64) ([]Collaborator, error) {
	rows, err := q.q.QueryContext(ctx,
		"SELECT c.user_id, u.username, c.role FROM collaborators c JOIN users u ON u.id = c.user_id WHERE c.repo_id = ? ORDER BY u.username_lower", repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Collaborator
	for rows.Next() {
		var c Collaborator
		if err := rows.Scan(&c.UserID, &c.Username, &c.Role); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ---- protected branches ----

// ProtectedBranch is one branch protection rule.
type ProtectedBranch struct {
	ID                 int64
	RepoID             int64
	Pattern            string
	AllowForcePush     bool
	AllowDelete        bool
	RequirePullRequest bool
	RequiredChecks     []string
	CreatedAt          int64
}

// ListProtectedBranches returns repoID's protection rules ordered by pattern.
func (q *Queries) ListProtectedBranches(ctx context.Context, repoID int64) ([]ProtectedBranch, error) {
	rows, err := q.q.QueryContext(ctx,
		"SELECT id, repo_id, pattern, allow_force_push, allow_delete, require_pull_request, required_checks, created_at FROM protected_branches WHERE repo_id = ? ORDER BY pattern", repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProtectedBranch
	for rows.Next() {
		var p ProtectedBranch
		var checks string
		if err := rows.Scan(&p.ID, &p.RepoID, &p.Pattern, &p.AllowForcePush, &p.AllowDelete, &p.RequirePullRequest, &checks, &p.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(checks), &p.RequiredChecks); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SaveProtectedBranch inserts p (ID 0) or updates it, and returns its id. errs.Conflict if
// another rule in the repository has the same pattern.
func (q *Queries) SaveProtectedBranch(ctx context.Context, p *ProtectedBranch) (int64, error) {
	if p.RequiredChecks == nil {
		p.RequiredChecks = []string{}
	}
	checks, err := json.Marshal(p.RequiredChecks)
	if err != nil {
		return 0, err
	}
	if p.ID == 0 {
		res, err := q.q.ExecContext(ctx,
			"INSERT INTO protected_branches (repo_id, pattern, allow_force_push, allow_delete, require_pull_request, required_checks, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			p.RepoID, p.Pattern, p.AllowForcePush, p.AllowDelete, p.RequirePullRequest, string(checks), p.CreatedAt)
		if isUniqueViolation(err) {
			return 0, fmt.Errorf("a rule for %s already exists: %w", p.Pattern, errs.Conflict)
		}
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	}
	res, err := q.q.ExecContext(ctx,
		"UPDATE protected_branches SET pattern = ?, allow_force_push = ?, allow_delete = ?, require_pull_request = ?, required_checks = ? WHERE id = ? AND repo_id = ?",
		p.Pattern, p.AllowForcePush, p.AllowDelete, p.RequirePullRequest, string(checks), p.ID, p.RepoID)
	if isUniqueViolation(err) {
		return 0, fmt.Errorf("a rule for %s already exists: %w", p.Pattern, errs.Conflict)
	}
	return p.ID, affectedOne(res, err)
}

// DeleteProtectedBranch removes rule id from repoID; errs.NotFound if absent.
func (q *Queries) DeleteProtectedBranch(ctx context.Context, repoID, id int64) error {
	return affectedOne(q.q.ExecContext(ctx, "DELETE FROM protected_branches WHERE id = ? AND repo_id = ?", id, repoID))
}
