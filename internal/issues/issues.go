package issues

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"anvil/internal/auth"
	"anvil/internal/errs"
	"anvil/internal/events"
	"anvil/internal/repos"
	"anvil/internal/store"
)

// Size limits for user input.
const (
	MaxTitle = 255
	MaxBody  = 65536
)

// Service manages issues, comments and labels. Safe for concurrent use.
type Service struct {
	db    *store.DB
	repos *repos.Service
	bus   *events.Bus
	now   func() time.Time
}

// NewService returns a Service. bus may be nil in tests.
func NewService(db *store.DB, r *repos.Service, bus *events.Bus) *Service {
	return &Service{db: db, repos: r, bus: bus, now: time.Now}
}

// CreateIssue is the input to Create.
type CreateIssue struct {
	Title    string
	Body     string
	LabelIDs []int64 // applied only when the actor has write access
}

// ValidateText checks a title and body. Exported for internal/pulls.
func ValidateText(title, body string) error {
	p := errs.FieldErrors{}
	if t := strings.TrimSpace(title); t == "" || utf8.RuneCountInString(t) > MaxTitle {
		p["title"] = fmt.Sprintf("must be 1 to %d characters", MaxTitle)
	}
	if len(body) > MaxBody {
		p["body"] = "is too long"
	}
	return p.OrNil()
}

// Create opens an issue. Any signed-in user who can read the repository may (FR-COLLAB-01).
// Errors: errs.Unauthenticated, errs.NotFound, errs.FieldErrors. Publishes issue.opened.
func (s *Service) Create(ctx context.Context, actor auth.Actor, owner, name string, in CreateIssue) (*store.Issue, error) {
	if !actor.IsUser() {
		return nil, errs.Unauthenticated
	}
	repo, err := s.repos.Get(ctx, actor, owner, name)
	if err != nil {
		return nil, err
	}
	if err := ValidateText(in.Title, in.Body); err != nil {
		return nil, err
	}
	canLabel := s.repos.Permission(ctx, actor, repo) >= auth.LevelWrite
	now := s.now().UnixMilli()
	iss := &store.Issue{RepoID: repo.ID, AuthorID: actor.UserID, Title: strings.TrimSpace(in.Title), Body: in.Body, CreatedAt: now, UpdatedAt: now}
	err = s.db.Tx(ctx, func(tx *store.Tx) error {
		var err error
		if iss.Number, err = tx.NextIssueNumber(ctx, repo.ID); err != nil {
			return err
		}
		if iss.ID, err = tx.InsertIssue(ctx, iss); err != nil {
			return err
		}
		if canLabel && len(in.LabelIDs) > 0 {
			return tx.SetIssueLabels(ctx, repo.ID, iss.ID, in.LabelIDs)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.publish(ctx, actor, "issue.opened", repo.ID, map[string]any{"number": iss.Number, "title": iss.Title})
	return s.db.IssueByNumber(ctx, repo.ID, iss.Number)
}

// Get returns issue or pull request number of owner/name. Requires read access.
func (s *Service) Get(ctx context.Context, actor auth.Actor, owner, name string, number int64) (*store.Repository, *store.Issue, error) {
	repo, err := s.repos.Get(ctx, actor, owner, name)
	if err != nil {
		return nil, nil, err
	}
	iss, err := s.db.IssueByNumber(ctx, repo.ID, number)
	if err != nil {
		return nil, nil, fmt.Errorf("issue #%d: %w", number, err)
	}
	return repo, iss, nil
}

// Filter selects issues for List.
type Filter struct {
	Pulls  *bool  // nil = issues and pull requests
	State  string // open (default) | closed | all
	Label  string // label name
	Author string // username
	Query  string // text in title or body
}

// List returns one page of issues of owner/name matching f, newest first. Requires read access.
// An unknown label or author matches nothing rather than failing.
func (s *Service) List(ctx context.Context, actor auth.Actor, owner, name string, f Filter, page repos.Page) ([]store.Issue, int, error) {
	repo, err := s.repos.Get(ctx, actor, owner, name)
	if err != nil {
		return nil, 0, err
	}
	sf := store.IssueFilter{Pulls: f.Pulls, Query: strings.TrimSpace(f.Query)}
	switch f.State {
	case "", "open":
		sf.State = "open"
	case "closed":
		sf.State = "closed"
	}
	if f.Label != "" {
		labels, err := s.db.ListLabels(ctx, repo.ID)
		if err != nil {
			return nil, 0, err
		}
		sf.LabelID = -1
		for _, l := range labels {
			if strings.EqualFold(l.Name, f.Label) {
				sf.LabelID = l.ID
			}
		}
	}
	if f.Author != "" {
		sf.AuthorID = -1
		if u, err := s.db.UserByUsername(ctx, f.Author); err == nil {
			sf.AuthorID = u.ID
		}
	}
	page = page.Normalize()
	list, total, err := s.db.ListIssues(ctx, repo.ID, sf, page.PerPage, (page.Page-1)*page.PerPage)
	if list == nil {
		list = []store.Issue{}
	}
	return list, total, err
}

// UpdateIssue is the input to Update. Nil fields are unchanged.
type UpdateIssue struct {
	Title    *string
	Body     *string
	State    *string // open | closed
	LabelIDs *[]int64
}

// Update edits an issue or pull request. The author may change title, body and state;
// writers may also change labels. Closing or reopening adds a system comment. A merged pull
// request cannot be reopened (errs.Conflict). Publishes issue.updated / issue.closed / issue.reopened.
func (s *Service) Update(ctx context.Context, actor auth.Actor, owner, name string, number int64, in UpdateIssue) (*store.Issue, error) {
	repo, iss, err := s.Get(ctx, actor, owner, name, number)
	if err != nil {
		return nil, err
	}
	writer := s.repos.Permission(ctx, actor, repo) >= auth.LevelWrite
	if !actor.IsUser() || (!writer && actor.UserID != iss.AuthorID) {
		return nil, fmt.Errorf("only the author or someone with write access can edit this: %w", errs.Forbidden)
	}
	if in.LabelIDs != nil && !writer {
		return nil, fmt.Errorf("you need write access to change labels: %w", errs.Forbidden)
	}
	title, body, state := iss.Title, iss.Body, iss.State
	if in.Title != nil {
		title = strings.TrimSpace(*in.Title)
	}
	if in.Body != nil {
		body = *in.Body
	}
	if in.State != nil {
		if *in.State != "open" && *in.State != "closed" {
			return nil, errs.FieldErrors{"state": "must be open or closed"}
		}
		state = *in.State
	}
	if err := ValidateText(title, body); err != nil {
		return nil, err
	}
	if iss.IsPull && state == "open" && iss.State == "closed" {
		if p, err := s.db.PullByIssue(ctx, iss.ID); err == nil && p.MergedAt != 0 {
			return nil, fmt.Errorf("a merged pull request cannot be reopened: %w", errs.Conflict)
		}
	}
	now := s.now().UnixMilli()
	err = s.db.Tx(ctx, func(tx *store.Tx) error {
		if err := tx.UpdateIssue(ctx, iss.ID, title, body, state, now); err != nil {
			return err
		}
		if in.LabelIDs != nil {
			if err := tx.SetIssueLabels(ctx, repo.ID, iss.ID, *in.LabelIDs); err != nil {
				return err
			}
		}
		if state != iss.State {
			verb := map[string]string{"closed": "closed", "open": "reopened"}[state]
			_, err := tx.InsertComment(ctx, &store.Comment{IssueID: iss.ID, AuthorID: actor.UserID, Kind: "system", Body: verb, CreatedAt: now, UpdatedAt: now})
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	typ := "issue.updated"
	if state != iss.State {
		typ = map[string]string{"closed": "issue.closed", "open": "issue.reopened"}[state]
	}
	s.publish(ctx, actor, typ, repo.ID, map[string]any{"number": number})
	return s.db.IssueByNumber(ctx, repo.ID, number)
}

// Comments returns the comments of issue number, oldest first. Requires read access.
func (s *Service) Comments(ctx context.Context, actor auth.Actor, owner, name string, number int64) ([]store.Comment, error) {
	_, iss, err := s.Get(ctx, actor, owner, name, number)
	if err != nil {
		return nil, err
	}
	return s.db.ListComments(ctx, iss.ID)
}

// AddComment adds a comment to issue number (FR-COLLAB-03). Any signed-in reader may comment.
// Publishes issue.commented.
func (s *Service) AddComment(ctx context.Context, actor auth.Actor, owner, name string, number int64, body string) (*store.Comment, error) {
	if !actor.IsUser() {
		return nil, errs.Unauthenticated
	}
	repo, iss, err := s.Get(ctx, actor, owner, name, number)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(body) == "" || len(body) > MaxBody {
		return nil, errs.FieldErrors{"body": "must not be empty or too long"}
	}
	now := s.now().UnixMilli()
	c := &store.Comment{IssueID: iss.ID, AuthorID: actor.UserID, Body: body, Kind: "comment", CreatedAt: now, UpdatedAt: now}
	err = s.db.Tx(ctx, func(tx *store.Tx) error {
		c.ID, err = tx.InsertComment(ctx, c)
		return err
	})
	if err != nil {
		return nil, err
	}
	s.publish(ctx, actor, "issue.commented", repo.ID, map[string]any{"number": number, "comment_id": c.ID})
	return s.db.CommentByID(ctx, repo.ID, c.ID)
}

// commentForEdit loads comment id and checks the actor may change it: its author, or a
// repository admin. System comments cannot be edited.
func (s *Service) commentForEdit(ctx context.Context, actor auth.Actor, owner, name string, id int64) (*store.Repository, *store.Comment, error) {
	repo, err := s.repos.Get(ctx, actor, owner, name)
	if err != nil {
		return nil, nil, err
	}
	c, err := s.db.CommentByID(ctx, repo.ID, id)
	if err != nil {
		return nil, nil, fmt.Errorf("comment %d: %w", id, err)
	}
	if c.Kind == "system" {
		return nil, nil, fmt.Errorf("system comments cannot be changed: %w", errs.Forbidden)
	}
	if !actor.IsUser() || (actor.UserID != c.AuthorID && s.repos.Permission(ctx, actor, repo) < auth.LevelAdmin) {
		return nil, nil, fmt.Errorf("only the author can change this comment: %w", errs.Forbidden)
	}
	return repo, c, nil
}

// EditComment replaces the body of comment id. Allowed for its author and repository admins.
func (s *Service) EditComment(ctx context.Context, actor auth.Actor, owner, name string, id int64, body string) (*store.Comment, error) {
	repo, c, err := s.commentForEdit(ctx, actor, owner, name, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(body) == "" || len(body) > MaxBody {
		return nil, errs.FieldErrors{"body": "must not be empty or too long"}
	}
	if err := s.db.Tx(ctx, func(tx *store.Tx) error { return tx.UpdateComment(ctx, c.ID, body, s.now().UnixMilli()) }); err != nil {
		return nil, err
	}
	return s.db.CommentByID(ctx, repo.ID, c.ID)
}

// DeleteComment removes comment id. Allowed for its author and repository admins.
func (s *Service) DeleteComment(ctx context.Context, actor auth.Actor, owner, name string, id int64) error {
	_, c, err := s.commentForEdit(ctx, actor, owner, name, id)
	if err != nil {
		return err
	}
	return s.db.Tx(ctx, func(tx *store.Tx) error { return tx.DeleteComment(ctx, c.ID) })
}

// Labels returns the labels of owner/name. Requires read access.
func (s *Service) Labels(ctx context.Context, actor auth.Actor, owner, name string) ([]store.Label, error) {
	repo, err := s.repos.Get(ctx, actor, owner, name)
	if err != nil {
		return nil, err
	}
	return s.db.ListLabels(ctx, repo.ID)
}

var colorRE = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// SaveLabel creates (ID 0) or updates a label. Requires write access.
func (s *Service) SaveLabel(ctx context.Context, actor auth.Actor, owner, name string, l store.Label) (*store.Label, error) {
	repo, err := s.repos.GetForAccess(ctx, actor, owner, name, auth.LevelWrite)
	if err != nil {
		return nil, err
	}
	l.Name = strings.TrimSpace(l.Name)
	p := errs.FieldErrors{}
	if l.Name == "" || utf8.RuneCountInString(l.Name) > 50 {
		p["name"] = "must be 1 to 50 characters"
	}
	if l.Color == "" {
		l.Color = "#6b7280"
	}
	if !colorRE.MatchString(l.Color) {
		p["color"] = "must be a colour like #1d76db"
	}
	if err := p.OrNil(); err != nil {
		return nil, err
	}
	l.RepoID = repo.ID
	err = s.db.Tx(ctx, func(tx *store.Tx) error {
		l.ID, err = tx.SaveLabel(ctx, &l)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// DeleteLabel removes label id. Requires write access.
func (s *Service) DeleteLabel(ctx context.Context, actor auth.Actor, owner, name string, id int64) error {
	repo, err := s.repos.GetForAccess(ctx, actor, owner, name, auth.LevelWrite)
	if err != nil {
		return err
	}
	err = s.db.Tx(ctx, func(tx *store.Tx) error { return tx.DeleteLabel(ctx, repo.ID, id) })
	if errors.Is(err, errs.NotFound) {
		return fmt.Errorf("label %d: %w", id, err)
	}
	return err
}

func (s *Service) publish(ctx context.Context, actor auth.Actor, typ string, repoID int64, payload any) {
	if s.bus != nil {
		_ = s.bus.Publish(ctx, events.Event{Type: typ, RepoID: repoID, ActorKind: "user", ActorID: actor.UserID, Payload: payload})
	}
}
