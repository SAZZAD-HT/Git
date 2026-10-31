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

// Issue is one row of the issues table (issues and pull requests), with the author's
// username and labels joined in.
type Issue struct {
	ID         int64
	RepoID     int64
	Number     int64
	AuthorID   int64
	AuthorName string
	Title      string
	Body       string
	State      string // open | closed
	IsPull     bool
	Comments   int
	ClosedAt   int64
	CreatedAt  int64
	UpdatedAt  int64
	Labels     []Label
}

// Label is a repository label.
type Label struct {
	ID     int64
	RepoID int64
	Name   string
	Color  string
}

const issueColumns = `i.id, i.repo_id, i.number, i.author_id, u.username, i.title, i.body, i.state, i.is_pull,
	i.comments, COALESCE(i.closed_at, 0), i.created_at, i.updated_at`

const issueFrom = " FROM issues i JOIN users u ON u.id = i.author_id "

func scanIssue(row interface{ Scan(...any) error }) (*Issue, error) {
	var i Issue
	err := row.Scan(&i.ID, &i.RepoID, &i.Number, &i.AuthorID, &i.AuthorName, &i.Title, &i.Body, &i.State, &i.IsPull,
		&i.Comments, &i.ClosedAt, &i.CreatedAt, &i.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.NotFound
	}
	return &i, err
}

// NextIssueNumber increments and returns repositories.next_index for repoID, so issues and
// pull requests share one sequence (FR-COLLAB-02). Must run inside Tx.
func (q *Queries) NextIssueNumber(ctx context.Context, repoID int64) (int64, error) {
	var n int64
	err := q.q.QueryRowContext(ctx, "UPDATE repositories SET next_index = next_index + 1 WHERE id = ? RETURNING next_index - 1", repoID).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errs.NotFound
	}
	return n, err
}

// InsertIssue stores i (its Number must already be allocated) and returns its id.
func (q *Queries) InsertIssue(ctx context.Context, i *Issue) (int64, error) {
	res, err := q.q.ExecContext(ctx,
		"INSERT INTO issues (repo_id, number, author_id, title, body, state, is_pull, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 'open', ?, ?, ?)",
		i.RepoID, i.Number, i.AuthorID, i.Title, i.Body, i.IsPull, i.CreatedAt, i.UpdatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// IssueByNumber returns issue or pull request number of repoID with its labels, or errs.NotFound.
func (q *Queries) IssueByNumber(ctx context.Context, repoID, number int64) (*Issue, error) {
	i, err := scanIssue(q.q.QueryRowContext(ctx, "SELECT "+issueColumns+issueFrom+"WHERE i.repo_id = ? AND i.number = ?", repoID, number))
	if err != nil {
		return nil, err
	}
	labels, err := q.labelsFor(ctx, []int64{i.ID})
	i.Labels = labels[i.ID]
	return i, err
}

// IssueByID returns issue id (without labels), or errs.NotFound.
func (q *Queries) IssueByID(ctx context.Context, id int64) (*Issue, error) {
	return scanIssue(q.q.QueryRowContext(ctx, "SELECT "+issueColumns+issueFrom+"WHERE i.id = ?", id))
}

// IssueFilter selects issues for ListIssues. Zero fields match everything.
type IssueFilter struct {
	Pulls    *bool  // true: only pull requests; false: only issues; nil: both
	State    string // open | closed | "" (all)
	LabelID  int64
	AuthorID int64
	Query    string // substring of title or body, case-insensitive
}

// ListIssues returns one page of repoID's issues matching f, newest first, and the total count.
func (q *Queries) ListIssues(ctx context.Context, repoID int64, f IssueFilter, limit, offset int) ([]Issue, int, error) {
	where := []string{"i.repo_id = ?"}
	args := []any{repoID}
	if f.Pulls != nil {
		where, args = append(where, "i.is_pull = ?"), append(args, *f.Pulls)
	}
	if f.State != "" {
		where, args = append(where, "i.state = ?"), append(args, f.State)
	}
	if f.AuthorID != 0 {
		where, args = append(where, "i.author_id = ?"), append(args, f.AuthorID)
	}
	if f.LabelID != 0 {
		where, args = append(where, "EXISTS (SELECT 1 FROM issue_labels il WHERE il.issue_id = i.id AND il.label_id = ?)"), append(args, f.LabelID)
	}
	if f.Query != "" {
		like := "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(f.Query) + "%"
		where, args = append(where, `(i.title LIKE ? ESCAPE '\' OR i.body LIKE ? ESCAPE '\')`), append(args, like, like)
	}
	cond := " WHERE " + strings.Join(where, " AND ")
	var total int
	if err := q.q.QueryRowContext(ctx, "SELECT COUNT(*) FROM issues i"+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := q.q.QueryContext(ctx, "SELECT "+issueColumns+issueFrom+cond+" ORDER BY i.number DESC LIMIT ? OFFSET ?", append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Issue
	var ids []int64
	for rows.Next() {
		i, err := scanIssue(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *i)
		ids = append(ids, i.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	labels, err := q.labelsFor(ctx, ids)
	for k := range out {
		out[k].Labels = labels[out[k].ID]
	}
	return out, total, err
}

// UpdateIssue writes title, body and state of issue id, setting or clearing closed_at.
func (q *Queries) UpdateIssue(ctx context.Context, id int64, title, body, state string, now int64) error {
	return affectedOne(q.q.ExecContext(ctx,
		`UPDATE issues SET title = ?, body = ?, state = ?, updated_at = ?,
		 closed_at = CASE WHEN ? = 'closed' THEN COALESCE(closed_at, ?) ELSE NULL END WHERE id = ?`,
		title, body, state, now, state, now, id))
}

// labelsFor returns the labels of each issue id.
func (q *Queries) labelsFor(ctx context.Context, issueIDs []int64) (map[int64][]Label, error) {
	out := map[int64][]Label{}
	if len(issueIDs) == 0 {
		return out, nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(issueIDs)), ",")
	args := make([]any, len(issueIDs))
	for i, id := range issueIDs {
		args[i] = id
	}
	rows, err := q.q.QueryContext(ctx,
		"SELECT il.issue_id, l.id, l.repo_id, l.name, l.color FROM issue_labels il JOIN labels l ON l.id = il.label_id WHERE il.issue_id IN ("+ph+") ORDER BY l.name", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var issueID int64
		var l Label
		if err := rows.Scan(&issueID, &l.ID, &l.RepoID, &l.Name, &l.Color); err != nil {
			return nil, err
		}
		out[issueID] = append(out[issueID], l)
	}
	return out, rows.Err()
}

// SetIssueLabels replaces the labels of issueID with labelIDs, which must belong to repoID
// (others are ignored).
func (q *Queries) SetIssueLabels(ctx context.Context, repoID, issueID int64, labelIDs []int64) error {
	if _, err := q.q.ExecContext(ctx, "DELETE FROM issue_labels WHERE issue_id = ?", issueID); err != nil {
		return err
	}
	for _, id := range labelIDs {
		if _, err := q.q.ExecContext(ctx,
			"INSERT OR IGNORE INTO issue_labels (issue_id, label_id) SELECT ?, id FROM labels WHERE id = ? AND repo_id = ?", issueID, id, repoID); err != nil {
			return err
		}
	}
	return nil
}

// ListLabels returns repoID's labels ordered by name.
func (q *Queries) ListLabels(ctx context.Context, repoID int64) ([]Label, error) {
	rows, err := q.q.QueryContext(ctx, "SELECT id, repo_id, name, color FROM labels WHERE repo_id = ? ORDER BY name", repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Label{}
	for rows.Next() {
		var l Label
		if err := rows.Scan(&l.ID, &l.RepoID, &l.Name, &l.Color); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// SaveLabel inserts l (ID 0) or updates it and returns its id. errs.Conflict if the name is taken.
func (q *Queries) SaveLabel(ctx context.Context, l *Label) (int64, error) {
	if l.ID == 0 {
		res, err := q.q.ExecContext(ctx, "INSERT INTO labels (repo_id, name, color) VALUES (?, ?, ?)", l.RepoID, l.Name, l.Color)
		if isUniqueViolation(err) {
			return 0, fmt.Errorf("label %q already exists: %w", l.Name, errs.Conflict)
		}
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	}
	res, err := q.q.ExecContext(ctx, "UPDATE labels SET name = ?, color = ? WHERE id = ? AND repo_id = ?", l.Name, l.Color, l.ID, l.RepoID)
	if isUniqueViolation(err) {
		return 0, fmt.Errorf("label %q already exists: %w", l.Name, errs.Conflict)
	}
	return l.ID, affectedOne(res, err)
}

// DeleteLabel removes label id from repoID; errs.NotFound if absent.
func (q *Queries) DeleteLabel(ctx context.Context, repoID, id int64) error {
	return affectedOne(q.q.ExecContext(ctx, "DELETE FROM labels WHERE id = ? AND repo_id = ?", id, repoID))
}

// ---- comments ----

// Comment is one row of the comments table, with the author's username joined in.
type Comment struct {
	ID          int64
	IssueID     int64
	AuthorID    int64 // 0 for system comments
	AuthorName  string
	Body        string
	Kind        string // comment | review | system
	ReviewState string // for kind=review: comment | approve | request_changes
	ReviewID    int64  // inline comments: the review they belong to
	CommitSHA   string
	Path        string
	Line        int
	Side        string // old | new
	CreatedAt   int64
	UpdatedAt   int64
}

const commentColumns = `c.id, c.issue_id, COALESCE(c.author_id, 0), COALESCE(u.username, ''), c.body, c.kind,
	COALESCE(c.review_state, ''), COALESCE(c.review_id, 0), COALESCE(c.commit_sha, ''), COALESCE(c.path, ''),
	COALESCE(c.line, 0), COALESCE(c.side, ''), c.created_at, c.updated_at`

const commentFrom = " FROM comments c LEFT JOIN users u ON u.id = c.author_id "

func scanComment(row interface{ Scan(...any) error }) (*Comment, error) {
	var c Comment
	err := row.Scan(&c.ID, &c.IssueID, &c.AuthorID, &c.AuthorName, &c.Body, &c.Kind, &c.ReviewState, &c.ReviewID,
		&c.CommitSHA, &c.Path, &c.Line, &c.Side, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.NotFound
	}
	return &c, err
}

func nullIfZero[T comparable](v T) any {
	var zero T
	if v == zero {
		return nil
	}
	return v
}

// InsertComment stores c, increments the issue's comment count (system comments and inline
// review comments excluded) and returns the new id.
func (q *Queries) InsertComment(ctx context.Context, c *Comment) (int64, error) {
	res, err := q.q.ExecContext(ctx,
		`INSERT INTO comments (issue_id, author_id, body, kind, review_state, review_id, commit_sha, path, line, side, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.IssueID, nullIfZero(c.AuthorID), c.Body, c.Kind, nullIfZero(c.ReviewState), nullIfZero(c.ReviewID), nullIfZero(c.CommitSHA),
		nullIfZero(c.Path), nullIfZero(c.Line), nullIfZero(c.Side), c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return 0, err
	}
	if c.Kind == "comment" && c.ReviewID == 0 {
		if _, err := q.q.ExecContext(ctx, "UPDATE issues SET comments = comments + 1, updated_at = ? WHERE id = ?", c.CreatedAt, c.IssueID); err != nil {
			return 0, err
		}
	}
	return res.LastInsertId()
}

// ListComments returns every comment of issueID, oldest first.
func (q *Queries) ListComments(ctx context.Context, issueID int64) ([]Comment, error) {
	rows, err := q.q.QueryContext(ctx, "SELECT "+commentColumns+commentFrom+"WHERE c.issue_id = ? ORDER BY c.id", issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Comment{}
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// CommentByID returns comment id if it belongs to an issue of repoID, or errs.NotFound.
func (q *Queries) CommentByID(ctx context.Context, repoID, id int64) (*Comment, error) {
	return scanComment(q.q.QueryRowContext(ctx, "SELECT "+commentColumns+commentFrom+
		"JOIN issues i ON i.id = c.issue_id WHERE c.id = ? AND i.repo_id = ?", id, repoID))
}

// UpdateComment replaces the body of comment id.
func (q *Queries) UpdateComment(ctx context.Context, id int64, body string, now int64) error {
	return affectedOne(q.q.ExecContext(ctx, "UPDATE comments SET body = ?, updated_at = ? WHERE id = ?", body, now, id))
}

// DeleteComment removes comment id and decrements its issue's comment count when it counted.
func (q *Queries) DeleteComment(ctx context.Context, id int64) error {
	if _, err := q.q.ExecContext(ctx,
		"UPDATE issues SET comments = comments - 1 WHERE id = (SELECT issue_id FROM comments WHERE id = ? AND kind = 'comment' AND review_id IS NULL)", id); err != nil {
		return err
	}
	return affectedOne(q.q.ExecContext(ctx, "DELETE FROM comments WHERE id = ?", id))
}

// ---- pull requests ----

// Pull is one row of the pull_requests table.
type Pull struct {
	ID             int64
	IssueID        int64
	BaseRepoID     int64
	BaseBranch     string
	HeadRepoID     int64
	HeadBranch     string
	HeadSHA        string
	BaseSHA        string
	MergeStatus    string // unknown | checking | clean | conflict
	ConflictFiles  []string
	MergedAt       int64
	MergedBy       int64
	MergeCommitSHA string
}

const pullColumns = `id, issue_id, base_repo_id, base_branch, head_repo_id, head_branch, head_sha, base_sha,
	merge_status, conflict_files, COALESCE(merged_at, 0), COALESCE(merged_by, 0), COALESCE(merge_commit_sha, '')`

func scanPull(row interface{ Scan(...any) error }) (*Pull, error) {
	var p Pull
	var files string
	err := row.Scan(&p.ID, &p.IssueID, &p.BaseRepoID, &p.BaseBranch, &p.HeadRepoID, &p.HeadBranch, &p.HeadSHA, &p.BaseSHA,
		&p.MergeStatus, &files, &p.MergedAt, &p.MergedBy, &p.MergeCommitSHA)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.NotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, json.Unmarshal([]byte(files), &p.ConflictFiles)
}

// InsertPull stores p and returns its id.
func (q *Queries) InsertPull(ctx context.Context, p *Pull) (int64, error) {
	res, err := q.q.ExecContext(ctx,
		"INSERT INTO pull_requests (issue_id, base_repo_id, base_branch, head_repo_id, head_branch, head_sha) VALUES (?, ?, ?, ?, ?, ?)",
		p.IssueID, p.BaseRepoID, p.BaseBranch, p.HeadRepoID, p.HeadBranch, p.HeadSHA)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// PullByIssue returns the pull request row of issueID, or errs.NotFound.
func (q *Queries) PullByIssue(ctx context.Context, issueID int64) (*Pull, error) {
	return scanPull(q.q.QueryRowContext(ctx, "SELECT "+pullColumns+" FROM pull_requests WHERE issue_id = ?", issueID))
}

// PullByID returns pull request id, or errs.NotFound.
func (q *Queries) PullByID(ctx context.Context, id int64) (*Pull, error) {
	return scanPull(q.q.QueryRowContext(ctx, "SELECT "+pullColumns+" FROM pull_requests WHERE id = ?", id))
}

// OpenPullsTouching returns open, unmerged pull requests whose head or base is branch of repoID.
func (q *Queries) OpenPullsTouching(ctx context.Context, repoID int64, branch string) ([]Pull, error) {
	rows, err := q.q.QueryContext(ctx, "SELECT "+pullColumns+` FROM pull_requests p WHERE merged_at IS NULL
		AND EXISTS (SELECT 1 FROM issues i WHERE i.id = p.issue_id AND i.state = 'open')
		AND ((head_repo_id = ? AND head_branch = ?) OR (base_repo_id = ? AND base_branch = ?))`, repoID, branch, repoID, branch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Pull
	for rows.Next() {
		p, err := scanPull(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// OpenPullFor returns the open pull request from headBranch into baseBranch of repoID, or errs.NotFound.
func (q *Queries) OpenPullFor(ctx context.Context, repoID int64, baseBranch, headBranch string) (*Pull, error) {
	return scanPull(q.q.QueryRowContext(ctx, "SELECT "+pullColumns+` FROM pull_requests p WHERE base_repo_id = ? AND head_repo_id = ?
		AND base_branch = ? AND head_branch = ? AND merged_at IS NULL
		AND EXISTS (SELECT 1 FROM issues i WHERE i.id = p.issue_id AND i.state = 'open')`, repoID, repoID, baseBranch, headBranch))
}

// SetPullHead records a new head commit and resets the merge status to unknown.
func (q *Queries) SetPullHead(ctx context.Context, id int64, headSHA string) error {
	_, err := q.q.ExecContext(ctx, "UPDATE pull_requests SET head_sha = ?, merge_status = 'unknown', conflict_files = '[]' WHERE id = ?", headSHA, id)
	return err
}

// SetMergeStatus records the result of a mergeability check for the given head and base
// commits. It only writes if the head has not moved since the check started, so a slow
// check never overwrites a newer state.
func (q *Queries) SetMergeStatus(ctx context.Context, id int64, headSHA, baseSHA, status string, conflicts []string) error {
	if conflicts == nil {
		conflicts = []string{}
	}
	files, err := json.Marshal(conflicts)
	if err != nil {
		return err
	}
	_, err = q.q.ExecContext(ctx, "UPDATE pull_requests SET merge_status = ?, conflict_files = ?, base_sha = ? WHERE id = ? AND head_sha = ?",
		status, string(files), baseSHA, id, headSHA)
	return err
}

// MarkPullMerged records the merge of pull request id.
func (q *Queries) MarkPullMerged(ctx context.Context, id, mergedBy int64, mergeSHA string, now int64) error {
	return affectedOne(q.q.ExecContext(ctx,
		"UPDATE pull_requests SET merged_at = ?, merged_by = ?, merge_commit_sha = ? WHERE id = ? AND merged_at IS NULL", now, mergedBy, mergeSHA, id))
}
