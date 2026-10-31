package pulls

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"anvil/internal/auth"
	"anvil/internal/config"
	"anvil/internal/errs"
	"anvil/internal/events"
	"anvil/internal/git/gitcmd"
	"anvil/internal/issues"
	"anvil/internal/repos"
	"anvil/internal/store"
)

type fixture struct {
	t     *testing.T
	db    *store.DB
	repos *repos.Service
	iss   *issues.Service
	svc   *Service
	alice auth.Actor // owner
	bob   auth.Actor // writer
	carol auth.Actor // reader (public repo)
	work  string     // a clone of alice/demo
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	if _, err := gitcmd.Init("git"); err != nil {
		t.Skip("git not available:", err)
	}
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(ctx, config.DatabaseConfig{Driver: "sqlite"}, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Migrate(ctx, false); err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, db: db}
	f.repos = repos.NewService(repos.Deps{DB: db, Authorizer: auth.NewAuthorizer(db, false), ReposDir: filepath.Join(dir, "repos")})
	f.iss = issues.NewService(db, f.repos, nil)
	f.svc = NewService(Deps{DB: db, Repos: f.repos, Issues: f.iss})
	mk := func(name string) auth.Actor {
		u := &store.User{Username: name, Email: name + "@example.com", PasswordHash: "x", IsActive: true}
		db.Tx(ctx, func(tx *store.Tx) error { var err error; u.ID, err = tx.InsertUser(ctx, u); return err })
		return auth.Actor{Kind: auth.UserActor, UserID: u.ID, Username: name}
	}
	f.alice, f.bob, f.carol = mk("alice"), mk("bob"), mk("carol")
	r, err := f.repos.Create(ctx, f.alice, repos.CreateRepo{Name: "demo", InitReadme: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.repos.SetCollaborator(ctx, f.alice, "alice", "demo", "bob", auth.LevelWrite); err != nil {
		t.Fatal(err)
	}
	g, _ := f.repos.Open(r)
	f.work = filepath.Join(t.TempDir(), "w")
	f.git(filepath.Dir(f.work), "clone", "-q", g.Path(), f.work)
	return f
}

func (f *fixture) git(dir string, args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=T", "-c", "user.email=t@example.com", "-c", "core.autocrlf=false"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		f.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// commitOn creates branch (from its current state or from main) with a file change and pushes it.
func (f *fixture) commitOn(branch, file, content string) string {
	f.t.Helper()
	f.git(f.work, "checkout", "-q", "-B", branch, "origin/"+branch)
	if err := os.WriteFile(filepath.Join(f.work, file), []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
	f.git(f.work, "add", ".")
	f.git(f.work, "commit", "-q", "-m", "change "+file+" on "+branch)
	f.git(f.work, "push", "-q", "origin", branch)
	return f.git(f.work, "rev-parse", "HEAD")
}

// branchFromMain creates a new branch at origin/main.
func (f *fixture) branchFromMain(branch string) {
	f.git(f.work, "fetch", "-q", "origin")
	f.git(f.work, "push", "-q", "origin", "origin/main:refs/heads/"+branch)
}

func TestOpenCheckMerge(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.branchFromMain("feature")
	head := f.commitOn("feature", "a.txt", "hello\n")

	if _, err := f.svc.Open(ctx, f.carol, "alice", "demo", OpenPull{Title: "x", Base: "main", Head: "nope"}); !errors.Is(err, errs.Invalid) {
		t.Fatalf("unknown head branch: %v", err)
	}
	v, err := f.svc.Open(ctx, f.carol, "alice", "demo", OpenPull{Title: "Add a.txt", Base: "main", Head: "feature"})
	if err != nil {
		t.Fatal(err)
	}
	if v.Issue.Number != 1 || v.Pull.HeadSHA != head || v.Pull.MergeStatus != "clean" {
		t.Fatalf("%+v %+v", v.Issue, v.Pull)
	}
	if _, err := f.svc.Open(ctx, f.carol, "alice", "demo", OpenPull{Title: "dup", Base: "main", Head: "feature"}); !errors.Is(err, errs.Conflict) {
		t.Fatalf("duplicate PR: %v", err)
	}
	// Issues and pull requests share one number sequence.
	iss, err := f.iss.Create(ctx, f.carol, "alice", "demo", issues.CreateIssue{Title: "A bug"})
	if err != nil || iss.Number != 2 {
		t.Fatalf("issue number %v %v", iss, err)
	}
	if _, err := f.svc.Merge(ctx, f.carol, "alice", "demo", 1, MergeRequest{}); !errors.Is(err, errs.Forbidden) {
		t.Fatalf("reader merged: %v", err)
	}
	if _, err := f.svc.Merge(ctx, f.bob, "alice", "demo", 1, MergeRequest{ExpectedHeadSHA: strings.Repeat("a", 40)}); !errors.Is(err, errs.Conflict) {
		t.Fatalf("stale head merged: %v", err)
	}
	merged, err := f.svc.Merge(ctx, f.bob, "alice", "demo", 1, MergeRequest{ExpectedHeadSHA: head})
	if err != nil {
		t.Fatal(err)
	}
	if merged.Pull.MergedAt == 0 || merged.Issue.State != "closed" {
		t.Fatalf("%+v", merged.Pull)
	}
	g, _ := f.repos.Open(merged.Repo)
	mainSHA, _ := g.ResolveCommit(ctx, "main")
	c, _ := g.Commit(ctx, mainSHA, false)
	if mainSHA != merged.Pull.MergeCommitSHA || len(c.Parents) != 2 || c.Parents[1] != head {
		t.Fatalf("merge commit %+v", c)
	}
	if _, err := f.svc.Merge(ctx, f.bob, "alice", "demo", 1, MergeRequest{}); !errors.Is(err, errs.Conflict) {
		t.Fatalf("merged twice: %v", err)
	}
	// The diff of a merged PR is still the PR's own change.
	cmp, err := f.svc.Compare(ctx, f.carol, "alice", "demo", 1)
	if err != nil || len(cmp.Diff.Files) != 1 || cmp.Diff.Files[0].NewPath != "a.txt" {
		t.Fatalf("compare after merge %+v %v", cmp, err)
	}
}

func TestSquash(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.branchFromMain("sq")
	f.commitOn("sq", "one.txt", "1\n")
	f.commitOn("sq", "two.txt", "2\n")
	if _, err := f.svc.Open(ctx, f.bob, "alice", "demo", OpenPull{Title: "Two files", Base: "main", Head: "sq"}); err != nil {
		t.Fatal(err)
	}
	v, err := f.svc.Merge(ctx, f.alice, "alice", "demo", 1, MergeRequest{Strategy: "squash"})
	if err != nil {
		t.Fatal(err)
	}
	g, _ := f.repos.Open(v.Repo)
	c, _ := g.Commit(ctx, v.Pull.MergeCommitSHA, true)
	if len(c.Parents) != 1 || c.Subject != "Two files (#1)" || len(c.Diff.Files) != 2 || !strings.Contains(c.Body, "change one.txt") {
		t.Fatalf("squash commit %+v", c)
	}
}

func TestConflict(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.branchFromMain("left")
	f.branchFromMain("right")
	f.commitOn("left", "README.md", "left\n")
	f.commitOn("right", "README.md", "right\n")
	if _, err := f.svc.Open(ctx, f.bob, "alice", "demo", OpenPull{Title: "L", Base: "main", Head: "left"}); err != nil {
		t.Fatal(err)
	}
	v, err := f.svc.Open(ctx, f.bob, "alice", "demo", OpenPull{Title: "R", Base: "main", Head: "right"})
	if err != nil || v.Pull.MergeStatus != "clean" {
		t.Fatalf("%v %+v", err, v)
	}
	if _, err := f.svc.Merge(ctx, f.bob, "alice", "demo", 1, MergeRequest{}); err != nil {
		t.Fatal(err)
	}
	// Merging #1 moved main: #2 now conflicts in README.md.
	_, err = f.svc.Merge(ctx, f.bob, "alice", "demo", 2, MergeRequest{})
	if !errors.Is(err, errs.Conflict) || !strings.Contains(err.Error(), "README.md") {
		t.Fatalf("conflicting merge: %v", err)
	}
	v, _ = f.svc.Get(ctx, f.bob, "alice", "demo", 2)
	if v.Pull.MergeStatus != "conflict" || len(v.Pull.ConflictFiles) != 1 {
		t.Fatalf("%+v", v.Pull)
	}
}

func TestOnPushUpdatesHead(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.branchFromMain("topic")
	f.commitOn("topic", "t.txt", "1\n")
	v, err := f.svc.Open(ctx, f.bob, "alice", "demo", OpenPull{Title: "T", Base: "main", Head: "topic"})
	if err != nil {
		t.Fatal(err)
	}
	newHead := f.commitOn("topic", "t.txt", "2\n")
	ev := events.Event{Type: "git.push", RepoID: v.Repo.ID, Payload: map[string]string{"ref": "refs/heads/topic", "after": newHead}}
	if err := f.svc.OnPush(ctx, ev); err != nil {
		t.Fatal(err)
	}
	v, _ = f.svc.Get(ctx, f.bob, "alice", "demo", 1)
	g, _ := f.repos.Open(v.Repo)
	ref, _ := g.ResolveCommit(ctx, "refs/pull/1/head")
	if v.Pull.HeadSHA != newHead || ref != newHead || v.Pull.MergeStatus != "clean" {
		t.Fatalf("head %s ref %s status %s, want %s", v.Pull.HeadSHA, ref, v.Pull.MergeStatus, newHead)
	}
}

func TestReviewRules(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.branchFromMain("rv")
	f.commitOn("rv", "r.txt", "1\n")
	if _, err := f.svc.Open(ctx, f.bob, "alice", "demo", OpenPull{Title: "R", Base: "main", Head: "rv"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.SubmitReview(ctx, f.bob, "alice", "demo", 1, Review{Event: "approve"}); !errors.Is(err, errs.Forbidden) {
		t.Fatalf("author approved own PR: %v", err)
	}
	rv, err := f.svc.SubmitReview(ctx, f.carol, "alice", "demo", 1, Review{Event: "request_changes", Body: "Please fix",
		Comments: []ReviewComment{{Path: "r.txt", Line: 1, Side: "new", Body: "typo"}}})
	if err != nil || rv.ReviewState != "request_changes" {
		t.Fatalf("%+v %v", rv, err)
	}
	cs, _ := f.iss.Comments(ctx, f.carol, "alice", "demo", 1)
	if len(cs) != 2 || cs[1].ReviewID != rv.ID || cs[1].Path != "r.txt" {
		t.Fatalf("%+v", cs)
	}
}

func TestIssueEditing(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	iss, err := f.iss.Create(ctx, f.carol, "alice", "demo", issues.CreateIssue{Title: "Bug", Body: "It breaks"})
	if err != nil {
		t.Fatal(err)
	}
	closed := "closed"
	if _, err := f.iss.Update(ctx, f.bob, "alice", "demo", iss.Number, issues.UpdateIssue{State: &closed}); err != nil {
		t.Fatalf("writer could not close: %v", err)
	}
	other := auth.Actor{Kind: auth.UserActor, UserID: 9999}
	title := "hijack"
	if _, err := f.iss.Update(ctx, other, "alice", "demo", iss.Number, issues.UpdateIssue{Title: &title}); !errors.Is(err, errs.Forbidden) {
		t.Fatalf("stranger edited: %v", err)
	}
	c, err := f.iss.AddComment(ctx, f.bob, "alice", "demo", iss.Number, "On it")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.iss.DeleteComment(ctx, f.carol, "alice", "demo", c.ID); !errors.Is(err, errs.Forbidden) {
		t.Fatalf("non-author deleted a comment: %v", err)
	}
	open := "open"
	list, total, _ := f.iss.List(ctx, f.carol, "alice", "demo", issues.Filter{State: open}, repos.Page{})
	if total != 0 || len(list) != 0 {
		t.Fatalf("closed issue listed as open")
	}
	got, _ := f.db.IssueByNumber(ctx, iss.RepoID, iss.Number)
	if got.Comments != 1 {
		t.Fatalf("comment count %d", got.Comments)
	}
}
