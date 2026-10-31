package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"anvil/internal/errs"
)

// User is one row of the users table.
type User struct {
	ID           int64
	Username     string
	Email        string
	FullName     string
	PasswordHash string
	IsAdmin      bool
	IsActive     bool
	CreatedAt    int64
	UpdatedAt    int64
}

const userColumns = "id, username, email, full_name, password_hash, is_admin, is_active, created_at, updated_at"

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.FullName, &u.PasswordHash, &u.IsAdmin, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.NotFound
	}
	return &u, err
}

// isUniqueViolation reports whether err is a SQLite UNIQUE constraint failure.
func isUniqueViolation(err error) bool {
	var se *sqlite.Error
	return errors.As(err, &se) && se.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
}

// InsertUser stores u and returns its new id. It returns errs.Conflict (with the
// field name in the message) if the username or email is taken.
func (q *Queries) InsertUser(ctx context.Context, u *User) (int64, error) {
	res, err := q.q.ExecContext(ctx,
		`INSERT INTO users (username, username_lower, email, full_name, password_hash, is_admin, is_active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.Username, strings.ToLower(u.Username), strings.ToLower(u.Email), u.FullName, u.PasswordHash, u.IsAdmin, u.IsActive, u.CreatedAt, u.UpdatedAt)
	if isUniqueViolation(err) {
		field := "username"
		if strings.Contains(err.Error(), "users.email") {
			field = "email"
		}
		return 0, fmt.Errorf("%s is already taken: %w", field, errs.Conflict)
	}
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UserByUsername returns the user whose username matches case-insensitively, or errs.NotFound.
func (q *Queries) UserByUsername(ctx context.Context, username string) (*User, error) {
	return scanUser(q.q.QueryRowContext(ctx, "SELECT "+userColumns+" FROM users WHERE username_lower = ?", strings.ToLower(username)))
}

// UserByID returns the user with id, or errs.NotFound.
func (q *Queries) UserByID(ctx context.Context, id int64) (*User, error) {
	return scanUser(q.q.QueryRowContext(ctx, "SELECT "+userColumns+" FROM users WHERE id = ?", id))
}

// UpdateUserProfile sets email and full name. It returns errs.Conflict if the email is taken
// and errs.NotFound if the user does not exist.
func (q *Queries) UpdateUserProfile(ctx context.Context, id int64, email, fullName string, now int64) error {
	res, err := q.q.ExecContext(ctx, "UPDATE users SET email = ?, full_name = ?, updated_at = ? WHERE id = ?", strings.ToLower(email), fullName, now, id)
	if isUniqueViolation(err) {
		return fmt.Errorf("email is already taken: %w", errs.Conflict)
	}
	return affectedOne(res, err)
}

// SetUserPassword replaces the password hash of user id.
func (q *Queries) SetUserPassword(ctx context.Context, id int64, hash string, now int64) error {
	return affectedOne(q.q.ExecContext(ctx, "UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?", hash, now, id))
}

// SetUserFlags sets is_admin and is_active of user id.
func (q *Queries) SetUserFlags(ctx context.Context, id int64, isAdmin, isActive bool, now int64) error {
	return affectedOne(q.q.ExecContext(ctx, "UPDATE users SET is_admin = ?, is_active = ?, updated_at = ? WHERE id = ?", isAdmin, isActive, now, id))
}

// ListUsers returns users ordered by username, limit at a time starting at offset, and the total count.
func (q *Queries) ListUsers(ctx context.Context, limit, offset int) ([]User, int, error) {
	var total int
	if err := q.q.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := q.q.QueryContext(ctx, "SELECT "+userColumns+" FROM users ORDER BY username_lower LIMIT ? OFFSET ?", limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *u)
	}
	return out, total, rows.Err()
}

// CountAdmins returns the number of active site admins. Used to refuse removing the last one.
func (q *Queries) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := q.q.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE is_admin = 1 AND is_active = 1").Scan(&n)
	return n, err
}

// ---- sessions ----

// Session is one row of the sessions table.
type Session struct {
	IDHash     string
	UserID     int64
	CSRFToken  string
	IP         string
	UserAgent  string
	CreatedAt  int64
	LastSeenAt int64
	ExpiresAt  int64
}

const sessionColumns = "id_hash, user_id, csrf_token, ip, user_agent, created_at, last_seen_at, expires_at"

func scanSession(row interface{ Scan(...any) error }) (*Session, error) {
	var s Session
	err := row.Scan(&s.IDHash, &s.UserID, &s.CSRFToken, &s.IP, &s.UserAgent, &s.CreatedAt, &s.LastSeenAt, &s.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.NotFound
	}
	return &s, err
}

// InsertSession stores s.
func (q *Queries) InsertSession(ctx context.Context, s *Session) error {
	_, err := q.q.ExecContext(ctx, "INSERT INTO sessions ("+sessionColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		s.IDHash, s.UserID, s.CSRFToken, s.IP, s.UserAgent, s.CreatedAt, s.LastSeenAt, s.ExpiresAt)
	return err
}

// SessionByHash returns the session whose id hash is idHash and that has not expired at now,
// or errs.NotFound.
func (q *Queries) SessionByHash(ctx context.Context, idHash string, now int64) (*Session, error) {
	return scanSession(q.q.QueryRowContext(ctx, "SELECT "+sessionColumns+" FROM sessions WHERE id_hash = ? AND expires_at > ?", idHash, now))
}

// TouchSession records activity and moves the sliding expiry.
func (q *Queries) TouchSession(ctx context.Context, idHash string, lastSeen, expiresAt int64) error {
	_, err := q.q.ExecContext(ctx, "UPDATE sessions SET last_seen_at = ?, expires_at = ? WHERE id_hash = ?", lastSeen, expiresAt, idHash)
	return err
}

// DeleteSession removes one session of userID; errs.NotFound if there is none.
// A userID of 0 matches any owner (used by logout, which knows only the cookie).
func (q *Queries) DeleteSession(ctx context.Context, idHash string, userID int64) error {
	return affectedOne(q.q.ExecContext(ctx, "DELETE FROM sessions WHERE id_hash = ? AND (? = 0 OR user_id = ?)", idHash, userID, userID))
}

// DeleteUserSessions removes every session of userID except keepHash (pass "" to remove all)
// and returns how many were removed.
func (q *Queries) DeleteUserSessions(ctx context.Context, userID int64, keepHash string) (int64, error) {
	res, err := q.q.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ? AND id_hash != ?", userID, keepHash)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ListSessions returns the unexpired sessions of userID, most recently used first.
func (q *Queries) ListSessions(ctx context.Context, userID, now int64) ([]Session, error) {
	rows, err := q.q.QueryContext(ctx, "SELECT "+sessionColumns+" FROM sessions WHERE user_id = ? AND expires_at > ? ORDER BY last_seen_at DESC", userID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

// DeleteExpiredSessions removes sessions that expired before now. Run periodically.
func (q *Queries) DeleteExpiredSessions(ctx context.Context, now int64) (int64, error) {
	res, err := q.q.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at <= ?", now)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ---- access tokens ----

// AccessToken is one row of the access_tokens table.
type AccessToken struct {
	ID         int64
	UserID     int64
	Name       string
	Prefix     string
	Hash       string
	Scopes     []string
	ExpiresAt  int64 // 0 = never
	LastUsedAt int64 // 0 = never used
	CreatedAt  int64
}

const tokenColumns = "id, user_id, name, token_prefix, token_hash, scopes, COALESCE(expires_at, 0), COALESCE(last_used_at, 0), created_at"

func scanToken(row interface{ Scan(...any) error }) (*AccessToken, error) {
	var t AccessToken
	var scopes string
	err := row.Scan(&t.ID, &t.UserID, &t.Name, &t.Prefix, &t.Hash, &scopes, &t.ExpiresAt, &t.LastUsedAt, &t.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.NotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, json.Unmarshal([]byte(scopes), &t.Scopes)
}

// InsertToken stores t and returns its new id.
func (q *Queries) InsertToken(ctx context.Context, t *AccessToken) (int64, error) {
	scopes, err := json.Marshal(t.Scopes)
	if err != nil {
		return 0, err
	}
	res, err := q.q.ExecContext(ctx,
		"INSERT INTO access_tokens (user_id, name, token_prefix, token_hash, scopes, expires_at, created_at) VALUES (?, ?, ?, ?, ?, NULLIF(?, 0), ?)",
		t.UserID, t.Name, t.Prefix, t.Hash, string(scopes), t.ExpiresAt, t.CreatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// TokenByHash returns the token with hash, or errs.NotFound. Expiry is checked by the caller.
func (q *Queries) TokenByHash(ctx context.Context, hash string) (*AccessToken, error) {
	return scanToken(q.q.QueryRowContext(ctx, "SELECT "+tokenColumns+" FROM access_tokens WHERE token_hash = ?", hash))
}

// ListTokens returns userID's tokens, newest first.
func (q *Queries) ListTokens(ctx context.Context, userID int64) ([]AccessToken, error) {
	rows, err := q.q.QueryContext(ctx, "SELECT "+tokenColumns+" FROM access_tokens WHERE user_id = ? ORDER BY id DESC", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AccessToken
	for rows.Next() {
		t, err := scanToken(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// DeleteToken removes token id if userID owns it; errs.NotFound otherwise.
func (q *Queries) DeleteToken(ctx context.Context, userID, id int64) error {
	return affectedOne(q.q.ExecContext(ctx, "DELETE FROM access_tokens WHERE id = ? AND user_id = ?", id, userID))
}

// TouchToken records that token id was used at now.
func (q *Queries) TouchToken(ctx context.Context, id, now int64) error {
	_, err := q.q.ExecContext(ctx, "UPDATE access_tokens SET last_used_at = ? WHERE id = ?", now, id)
	return err
}

// affectedOne turns "no row changed" into errs.NotFound.
func affectedOne(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errs.NotFound
	}
	return nil
}
