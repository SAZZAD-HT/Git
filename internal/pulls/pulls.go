package pulls

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"anvil/internal/auth"
	"anvil/internal/errs"
	"anvil/internal/events"
	"anvil/internal/git"
	"anvil/internal/issues"
	"anvil/internal/queue"
	"anvil/internal/repos"
	"anvil/internal/store"
)

// TaskCheckMergeable is the queue task kind that recomputes a pull request's merge status.
const TaskCheckMergeable = "pull.check_mergeable"

// Deps are the Service's dependencies.
type Deps struct {
	DB     *store.DB
	Repos  *repos.Service
	Issues *issues.Service
	Bus    *events.Bus  // may be nil in tests
	Queue  *queue.Queue // may be nil in tests: checks then run inline
}

// Service manages pull requests. Safe for concurrent use.
type Service struct {
	Deps
	now func() time.Time
}

// NewService returns a Service and registers its queue handler (call before Queue.Start).
func NewService(d Deps) *Service {
	s := &Service{Deps: d, now: time.Now}
	if d.Queue != nil {
		d.Queue.Handle(TaskCheckMergeable, func(ctx context.Context, t queue.Task) error {
			var p struct {
				PullID int64 `json:"pull_id"`
			}
			if err := json.Unmarshal(t.Payload, &p); err != nil {
				return err
			}
			return s.CheckMergeable(ctx, p.PullID)
		})
	}
	return s
}

// View is a pull request with its issue.
type View struct {
	Issue *store.Issue
	Pull  *store.Pull
	Repo  *store.Repository
}

// OpenPull is the input to Open.
type OpenPull struct {
	Title string
	Body  string
	Base  string // target branch
	Head  string // source branch in the same repository (forks arrive later)
}

// pullRef is where Anvil keeps a copy of each pull request's head (docs/reference/04 §5.1).
func pullRef(number int64) string { return "refs/pull/" + strconv.FormatInt(number, 10) + "/head" }

// Open creates the issue and pull_requests rows, writes refs/pull/<n>/head, queues a
// mergeability check, and publishes pull.opened. Any signed-in reader may open one.
// Errors: errs.FieldErrors (unknown branches, nothing to merge), errs.Conflict (an open pull
// request for the same branches exists).
func (s *Service) Open(ctx context.Context, actor auth.Actor, owner, name string, in OpenPull) (*View, error) {
	if !actor.IsUser() {
		return nil, errs.Unauthenticated
	}
	repo, err := s.Repos.Get(ctx, actor, owner, name)
	if err != nil {
		return nil, err
	}
	if err := issues.ValidateText(in.Title, in.Body); err != nil {
		return nil, err
	}
	if in.Base == in.Head {
		return nil, errs.FieldErrors{"head": "choose a different branch from the base"}
	}
	g, err := s.Repos.Open(repo)
	if err != nil {
		return nil, err
	}
	baseSHA, err := g.ResolveCommit(ctx, "refs/heads/"+in.Base)
	if err != nil {
		return nil, errs.FieldErrors{"base": "branch does not exist"}
	}
	headSHA, err := g.ResolveCommit(ctx, "refs/heads/"+in.Head)
	if err != nil {
		return nil, errs.FieldErrors{"head": "branch does not exist"}
	}
	if ahead, err := g.IsAncestor(ctx, headSHA, baseSHA); err != nil {
		return nil, err
	} else if ahead {
		return nil, errs.FieldErrors{"head": in.Head + " has no commits that are not already on " + in.Base}
	}
	if existing, err := s.DB.OpenPullFor(ctx, repo.ID, in.Base, in.Head); err == nil {
		n := int64(0)
		if iss, err := s.DB.IssueByID(ctx, existing.IssueID); err == nil {
			n = iss.Number
		}
		return nil, fmt.Errorf("pull request #%d already exists for these branches: %w", n, errs.Conflict)
	}
	now := s.now().UnixMilli()
	iss := &store.Issue{RepoID: repo.ID, AuthorID: actor.UserID, Title: strings.TrimSpace(in.Title), Body: in.Body, IsPull: true, CreatedAt: now, UpdatedAt: now}
	p := &store.Pull{BaseRepoID: repo.ID, BaseBranch: in.Base, HeadRepoID: repo.ID, HeadBranch: in.Head, HeadSHA: headSHA}
	err = s.DB.Tx(ctx, func(tx *store.Tx) error {
		var err error
		if iss.Number, err = tx.NextIssueNumber(ctx, repo.ID); err != nil {
			return err
		}
		if iss.ID, err = tx.InsertIssue(ctx, iss); err != nil {
			return err
		}
		p.IssueID = iss.ID
		p.ID, err = tx.InsertPull(ctx, p)
		return err
	})
	if err != nil {
		return nil, err
	}
	// Outside the transaction: never hold a DB transaction across a git call (docs/reference/05 §5).
	if err := g.UpdateRef(ctx, pullRef(iss.Number), headSHA, ""); err != nil && !errors.Is(err, errs.Conflict) {
		return nil, fmt.Errorf("write %s: %w", pullRef(iss.Number), err)
	}
	s.scheduleCheck(ctx, p.ID)
	s.publish(ctx, actor.UserID, "pull.opened", repo.ID, map[string]any{"number": iss.Number, "base": in.Base, "head": in.Head})
	return s.Get(ctx, actor, owner, name, iss.Number)
}

// Get returns pull request number of owner/name. Requires read access. errs.NotFound if
// number is an issue, not a pull request.
func (s *Service) Get(ctx context.Context, actor auth.Actor, owner, name string, number int64) (*View, error) {
	repo, iss, err := s.Issues.Get(ctx, actor, owner, name, number)
	if err != nil {
		return nil, err
	}
	if !iss.IsPull {
		return nil, fmt.Errorf("#%d is an issue, not a pull request: %w", number, errs.NotFound)
	}
	p, err := s.DB.PullByIssue(ctx, iss.ID)
	if err != nil {
		return nil, err
	}
	return &View{Issue: iss, Pull: p, Repo: repo}, nil
}

// List returns one page of pull requests of owner/name with the given state
// (open, closed or all). Requires read access.
func (s *Service) List(ctx context.Context, actor auth.Actor, owner, name, state string, page repos.Page) ([]View, int, error) {
	t := true
	list, total, err := s.Issues.List(ctx, actor, owner, name, issues.Filter{Pulls: &t, State: state}, page)
	if err != nil {
		return nil, 0, err
	}
	out := make([]View, 0, len(list))
	for i := range list {
		p, err := s.DB.PullByIssue(ctx, list[i].ID)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, View{Issue: &list[i], Pull: p})
	}
	return out, total, nil
}

// compareBase is what the pull request is compared against: the live base branch while it
// is open, and the base tip recorded at merge time afterwards (the branch has moved on).
func compareBase(v *View) string {
	if v.Pull.MergedAt != 0 && v.Pull.BaseSHA != "" {
		return v.Pull.BaseSHA
	}
	return "refs/heads/" + v.Pull.BaseBranch
}

// Compare returns the commits and per-file diff of pull request number (FR-COLLAB-06).
func (s *Service) Compare(ctx context.Context, actor auth.Actor, owner, name string, number int64) (*repos.CompareView, error) {
	v, err := s.Get(ctx, actor, owner, name, number)
	if err != nil {
		return nil, err
	}
	return s.Repos.Compare(ctx, actor, owner, name, compareBase(v), v.Pull.HeadSHA)
}

// scheduleCheck queues a mergeability check, or runs it inline when there is no queue (tests).
func (s *Service) scheduleCheck(ctx context.Context, pullID int64) {
	if s.Queue == nil {
		_ = s.CheckMergeable(ctx, pullID)
		return
	}
	_, _ = s.Queue.Enqueue(ctx, TaskCheckMergeable, map[string]int64{"pull_id": pullID})
}

// CheckMergeable recomputes merge_status with git merge-tree. Runs as a queue task. The
// result is written only if the head has not moved meanwhile.
func (s *Service) CheckMergeable(ctx context.Context, pullID int64) error {
	p, err := s.DB.PullByID(ctx, pullID)
	if err != nil {
		return err
	}
	if p.MergedAt != 0 {
		return nil
	}
	repo, err := s.DB.RepoByID(ctx, p.BaseRepoID)
	if err != nil {
		return err
	}
	g, err := s.Repos.Open(repo)
	if err != nil {
		return err
	}
	baseSHA, err := g.ResolveCommit(ctx, "refs/heads/"+p.BaseBranch)
	if err != nil {
		return s.DB.Tx(ctx, func(tx *store.Tx) error {
			return tx.SetMergeStatus(ctx, p.ID, p.HeadSHA, "", "conflict", []string{"(base branch " + p.BaseBranch + " no longer exists)"})
		})
	}
	_, conflicts, err := g.MergeTree(ctx, baseSHA, p.HeadSHA)
	if err != nil {
		return err
	}
	status := "clean"
	if len(conflicts) > 0 {
		status = "conflict"
	}
	return s.DB.Tx(ctx, func(tx *store.Tx) error { return tx.SetMergeStatus(ctx, p.ID, p.HeadSHA, baseSHA, status, conflicts) })
}

// MergeRequest is the input to Merge.
type MergeRequest struct {
	Strategy        string // merge (default) | squash
	Message         string // optional commit message; a sensible default is used when empty
	ExpectedHeadSHA string // optional: refuse if the head moved since the user reviewed it
}

// Merge merges the PR with the chosen strategy using compare-and-swap ref updates
// (docs/reference/04 §5.3). Requires write access. Errors: errs.Conflict when the pull
// request is closed or merged, has conflicts, its head moved past ExpectedHeadSHA, or the
// base branch moved during the merge (retry). Publishes pull.merged and git.push.
func (s *Service) Merge(ctx context.Context, actor auth.Actor, owner, name string, number int64, in MergeRequest) (*View, error) {
	if _, err := s.Repos.GetForAccess(ctx, actor, owner, name, auth.LevelWrite); err != nil {
		return nil, err
	}
	v, err := s.Get(ctx, actor, owner, name, number)
	if err != nil {
		return nil, err
	}
	switch {
	case v.Pull.MergedAt != 0:
		return nil, fmt.Errorf("pull request #%d is already merged: %w", number, errs.Conflict)
	case v.Issue.State != "open":
		return nil, fmt.Errorf("pull request #%d is closed: %w", number, errs.Conflict)
	case in.ExpectedHeadSHA != "" && in.ExpectedHeadSHA != v.Pull.HeadSHA:
		return nil, fmt.Errorf("the branch received new commits since you loaded the page; review them and try again: %w", errs.Conflict)
	}
	if in.Strategy == "" {
		in.Strategy = "merge"
	}
	if in.Strategy != "merge" && in.Strategy != "squash" {
		return nil, errs.FieldErrors{"strategy": "must be merge or squash"}
	}
	// Required checks (FR-COLLAB-10) are enforced here once pipelines exist (P5).
	g, err := s.Repos.Open(v.Repo)
	if err != nil {
		return nil, err
	}
	baseRef := "refs/heads/" + v.Pull.BaseBranch
	baseSHA, err := g.ResolveCommit(ctx, baseRef)
	if err != nil {
		return nil, fmt.Errorf("base branch %s no longer exists: %w", v.Pull.BaseBranch, errs.Conflict)
	}
	tree, conflicts, err := g.MergeTree(ctx, baseSHA, v.Pull.HeadSHA)
	if err != nil {
		return nil, err
	}
	if len(conflicts) > 0 {
		_ = s.DB.Tx(ctx, func(tx *store.Tx) error {
			return tx.SetMergeStatus(ctx, v.Pull.ID, v.Pull.HeadSHA, baseSHA, "conflict", conflicts)
		})
		return nil, fmt.Errorf("this pull request has conflicts in %s: %w", strings.Join(conflicts, ", "), errs.Conflict)
	}
	u, err := s.DB.UserByID(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	sig := git.Signature{Name: u.FullName, Email: u.Email, When: s.now()}
	if sig.Name == "" {
		sig.Name = u.Username
	}
	msg, parents := in.Message, []string{baseSHA, v.Pull.HeadSHA}
	if in.Strategy == "squash" {
		parents = []string{baseSHA}
		if msg == "" {
			msg = s.squashMessage(ctx, g, v, baseSHA)
		}
	} else if msg == "" {
		msg = fmt.Sprintf("Merge pull request #%d from %s\n\n%s\n", v.Issue.Number, v.Pull.HeadBranch, v.Issue.Title)
	}
	commit, err := g.CommitTree(ctx, tree, parents, msg, sig, sig)
	if err != nil {
		return nil, err
	}
	if err := g.UpdateRef(ctx, baseRef, commit, baseSHA); err != nil {
		if errors.Is(err, errs.Conflict) {
			return nil, fmt.Errorf("%s changed while merging; reload and try again: %w", v.Pull.BaseBranch, errs.Conflict)
		}
		return nil, err
	}
	now := s.now().UnixMilli()
	err = s.DB.Tx(ctx, func(tx *store.Tx) error {
		if err := tx.SetMergeStatus(ctx, v.Pull.ID, v.Pull.HeadSHA, baseSHA, "clean", nil); err != nil {
			return err
		}
		if err := tx.MarkPullMerged(ctx, v.Pull.ID, actor.UserID, commit, now); err != nil {
			return err
		}
		if err := tx.UpdateIssue(ctx, v.Issue.ID, v.Issue.Title, v.Issue.Body, "closed", now); err != nil {
			return err
		}
		_, err := tx.InsertComment(ctx, &store.Comment{IssueID: v.Issue.ID, AuthorID: actor.UserID, Kind: "system",
			Body: "merged commit " + commit[:12] + " into " + v.Pull.BaseBranch, CreatedAt: now, UpdatedAt: now})
		return err
	})
	if err != nil {
		return nil, err
	}
	s.publish(ctx, actor.UserID, "pull.merged", v.Repo.ID, map[string]any{"number": number, "strategy": in.Strategy, "merge_commit": commit})
	// The ref moved without receive-pack, so no hook ran: publish the push ourselves so CI
	// and other pull requests targeting this branch react (docs/reference/04 §5.3).
	s.publish(ctx, actor.UserID, "git.push", v.Repo.ID, map[string]string{"ref": baseRef, "before": baseSHA, "after": commit})
	return s.Get(ctx, actor, owner, name, number)
}

// squashMessage builds "<title> (#n)" followed by the subjects of the squashed commits.
func (s *Service) squashMessage(ctx context.Context, g *git.Repository, v *View, baseSHA string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (#%d)\n", v.Issue.Title, v.Issue.Number)
	if commits, err := g.Log(ctx, baseSHA+".."+v.Pull.HeadSHA, git.LogOptions{Limit: 100}); err == nil && len(commits) > 0 {
		b.WriteString("\n")
		for i := len(commits) - 1; i >= 0; i-- {
			fmt.Fprintf(&b, "* %s\n", commits[i].Subject)
		}
	}
	return b.String()
}

// ReviewComment is an inline comment on one line of the diff.
type ReviewComment struct {
	Path string
	Line int
	Side string // old | new
	Body string
}

// Review is the input to SubmitReview.
type Review struct {
	Event    string // comment | approve | request_changes
	Body     string
	Comments []ReviewComment
}

// SubmitReview records a review with optional inline comments on the current head commit
// (FR-COLLAB-08). Any signed-in reader may comment; authors cannot approve or request
// changes on their own pull request. Publishes pull.reviewed.
func (s *Service) SubmitReview(ctx context.Context, actor auth.Actor, owner, name string, number int64, in Review) (*store.Comment, error) {
	if !actor.IsUser() {
		return nil, errs.Unauthenticated
	}
	v, err := s.Get(ctx, actor, owner, name, number)
	if err != nil {
		return nil, err
	}
	switch in.Event {
	case "comment":
		if strings.TrimSpace(in.Body) == "" && len(in.Comments) == 0 {
			return nil, errs.FieldErrors{"body": "write a comment"}
		}
	case "approve", "request_changes":
		if actor.UserID == v.Issue.AuthorID {
			return nil, fmt.Errorf("you cannot approve or request changes on your own pull request: %w", errs.Forbidden)
		}
	default:
		return nil, errs.FieldErrors{"event": "must be comment, approve or request_changes"}
	}
	for i, c := range in.Comments {
		if c.Path == "" || c.Line < 1 || (c.Side != "old" && c.Side != "new") || strings.TrimSpace(c.Body) == "" {
			return nil, errs.FieldErrors{fmt.Sprintf("comments[%d]", i): "needs path, line ≥ 1, side old|new and a body"}
		}
	}
	now := s.now().UnixMilli()
	review := &store.Comment{IssueID: v.Issue.ID, AuthorID: actor.UserID, Body: in.Body, Kind: "review", ReviewState: in.Event,
		CommitSHA: v.Pull.HeadSHA, CreatedAt: now, UpdatedAt: now}
	err = s.DB.Tx(ctx, func(tx *store.Tx) error {
		var err error
		if review.ID, err = tx.InsertComment(ctx, review); err != nil {
			return err
		}
		for _, c := range in.Comments {
			if _, err := tx.InsertComment(ctx, &store.Comment{IssueID: v.Issue.ID, AuthorID: actor.UserID, Body: c.Body, Kind: "comment",
				ReviewID: review.ID, CommitSHA: v.Pull.HeadSHA, Path: c.Path, Line: c.Line, Side: c.Side, CreatedAt: now, UpdatedAt: now}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.publish(ctx, actor.UserID, "pull.reviewed", v.Repo.ID, map[string]any{"number": number, "event": in.Event})
	return s.DB.CommentByID(ctx, v.Repo.ID, review.ID)
}

// OnPush updates refs/pull/<n>/head for open PRs whose head branch moved, and re-checks
// mergeability of PRs whose head or base moved (FR-COLLAB-11). Subscribed to git.push.
func (s *Service) OnPush(ctx context.Context, ev events.Event) error {
	var p struct {
		Ref   string `json:"ref"`
		After string `json:"after"`
	}
	b, err := json.Marshal(ev.Payload) // the payload is a map in-process and raw JSON when replayed
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	branch, ok := strings.CutPrefix(p.Ref, "refs/heads/")
	if !ok || strings.Trim(p.After, "0") == "" {
		return nil // tags and branch deletions do not change pull requests
	}
	pulls, err := s.DB.OpenPullsTouching(ctx, ev.RepoID, branch)
	if err != nil {
		return err
	}
	for i := range pulls {
		pr := &pulls[i]
		if pr.HeadRepoID == ev.RepoID && pr.HeadBranch == branch && pr.HeadSHA != p.After {
			if err := s.moveHead(ctx, pr, p.After); err != nil {
				return err
			}
		}
		s.scheduleCheck(ctx, pr.ID)
	}
	return nil
}

func (s *Service) moveHead(ctx context.Context, pr *store.Pull, newSHA string) error {
	repo, err := s.DB.RepoByID(ctx, pr.BaseRepoID)
	if err != nil {
		return err
	}
	g, err := s.Repos.Open(repo)
	if err != nil {
		return err
	}
	var number int64
	if iss, err := s.DB.IssueByID(ctx, pr.IssueID); err == nil {
		number = iss.Number
	}
	if number > 0 {
		// Compare-and-swap from the recorded head; fall back to a plain update if the ref drifted.
		if err := g.UpdateRef(ctx, pullRef(number), newSHA, pr.HeadSHA); errors.Is(err, errs.Conflict) {
			cur, _ := g.ResolveCommit(ctx, pullRef(number))
			_ = g.UpdateRef(ctx, pullRef(number), newSHA, cur)
		}
	}
	err = s.DB.Tx(ctx, func(tx *store.Tx) error { return tx.SetPullHead(ctx, pr.ID, newSHA) })
	if err == nil {
		s.publish(ctx, 0, "pull.synchronized", repo.ID, map[string]any{"number": number, "head": newSHA})
	}
	return err
}

func (s *Service) publish(ctx context.Context, actorID int64, typ string, repoID int64, payload any) {
	if s.Bus == nil {
		return
	}
	kind := "user"
	if actorID == 0 {
		kind = "system"
	}
	_ = s.Bus.Publish(ctx, events.Event{Type: typ, RepoID: repoID, ActorKind: kind, ActorID: actorID, Payload: payload})
}
