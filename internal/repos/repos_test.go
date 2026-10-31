package repos

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"anvil/internal/auth"
	"anvil/internal/config"
	"anvil/internal/errs"
	"anvil/internal/git/gitcmd"
	"anvil/internal/store"
)

type fixture struct {
	svc          *Service
	db           *store.DB
	alice, bob   auth.Actor
	carol, admin auth.Actor
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
	f := &fixture{db: db, svc: NewService(Deps{DB: db, Authorizer: auth.NewAuthorizer(db, false), ReposDir: filepath.Join(dir, "repos")})}
	mk := func(name string, admin bool) auth.Actor {
		u := &store.User{Username: name, Email: name + "@example.com", PasswordHash: "x", IsActive: true, IsAdmin: admin}
		db.Tx(ctx, func(tx *store.Tx) error { var err error; u.ID, err = tx.InsertUser(ctx, u); return err })
		return auth.Actor{Kind: auth.UserActor, UserID: u.ID, Username: name, IsAdmin: admin}
	}
	f.alice, f.bob, f.carol, f.admin = mk("alice", false), mk("bob", false), mk("carol", false), mk("root", true)
	return f
}

// runGit runs git in dir for test setup (tests may use os/exec; production code may not).
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=T", "-c", "user.email=t@example.com", "-c", "core.autocrlf=false"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestCreateAndBrowse(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	r, err := f.svc.Create(ctx, f.alice, CreateRepo{Name: "Demo", Description: "A demo", InitReadme: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.IsEmpty || r.DefaultBranch != "main" {
		t.Fatalf("%+v", r)
	}
	if _, err := f.svc.Create(ctx, f.alice, CreateRepo{Name: "demo"}); !errors.Is(err, errs.Invalid) {
		t.Fatalf("duplicate name (different case) accepted: %v", err)
	}
	tree, err := f.svc.Tree(ctx, f.bob, "alice", "demo", "", "")
	if err != nil || len(tree.Entries) != 1 || tree.Entries[0].Name != "README.md" {
		t.Fatalf("%+v %v", tree, err)
	}
	blob, err := f.svc.Blob(ctx, f.bob, "alice", "demo", "main", "README.md")
	if err != nil || blob.Content != "# Demo\n\nA demo\n" {
		t.Fatalf("%+v %v", blob, err)
	}
	if _, err := f.svc.Blob(ctx, f.bob, "alice", "demo", "main", "../etc/passwd"); !errors.Is(err, errs.NotFound) {
		t.Fatalf("path traversal: %v", err)
	}
	commits, err := f.svc.Commits(ctx, f.bob, "alice", "demo", "", "", Page{})
	if err != nil || len(commits) != 1 || commits[0].Subject != "Initial commit" {
		t.Fatalf("%+v %v", commits, err)
	}
}

func TestPermissions(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	pub, _ := f.svc.Create(ctx, f.alice, CreateRepo{Name: "pub"})
	priv, _ := f.svc.Create(ctx, f.alice, CreateRepo{Name: "priv", Private: true})
	if err := f.svc.SetCollaborator(ctx, f.alice, "alice", "priv", "bob", auth.LevelWrite); err != nil {
		t.Fatal(err)
	}
	anon := auth.Actor{Kind: auth.Anonymous}
	readOnlyToken := f.alice
	readOnlyToken.Scopes = []string{auth.ScopeRepoRead}
	cases := []struct {
		name  string
		actor auth.Actor
		repo  *store.Repository
		want  auth.Level
	}{
		{"owner", f.alice, priv, auth.LevelAdmin},
		{"collaborator", f.bob, priv, auth.LevelWrite},
		{"stranger on private", f.carol, priv, auth.LevelNone},
		{"anonymous on private", anon, priv, auth.LevelNone},
		{"anonymous on public", anon, pub, auth.LevelRead},
		{"stranger on public", f.carol, pub, auth.LevelRead},
		{"site admin", f.admin, priv, auth.LevelAdmin},
		{"owner with read-only token", readOnlyToken, priv, auth.LevelRead},
	}
	for _, c := range cases {
		if got := f.svc.Permission(ctx, c.actor, c.repo); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
	// Private repositories are hidden (404), not forbidden.
	if _, err := f.svc.Get(ctx, f.carol, "alice", "priv"); !errors.Is(err, errs.NotFound) {
		t.Fatalf("stranger sees private repo: %v", err)
	}
	if _, err := f.svc.GetForAccess(ctx, f.carol, "alice", "pub", auth.LevelWrite); !errors.Is(err, errs.Forbidden) {
		t.Fatalf("stranger writes public repo: %v", err)
	}
	list, total, _ := f.svc.ListForUser(ctx, f.carol, "alice", Page{})
	if total != 1 || list[0].Name != "pub" {
		t.Fatalf("carol lists %d repos", total)
	}
	list, total, _ = f.svc.ListForUser(ctx, f.bob, "alice", Page{})
	if total != 2 {
		t.Fatalf("collaborator lists %d repos", total)
	}
	_ = list
}

func TestCheckPushProtection(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	r, err := f.svc.Create(ctx, f.alice, CreateRepo{Name: "prot", InitReadme: true})
	if err != nil {
		t.Fatal(err)
	}
	g, _ := f.svc.Open(r)
	base, _ := g.ResolveCommit(ctx, "main")

	// Build a fast-forward commit and a divergent one in a clone, then push them to side refs
	// so the objects exist in the bare repository.
	work := filepath.Join(t.TempDir(), "w")
	runGit(t, filepath.Dir(work), "clone", "-q", g.Path(), work)
	runGit(t, work, "commit", "-q", "--allow-empty", "-m", "ff")
	ff := runGit(t, work, "rev-parse", "HEAD")
	runGit(t, work, "reset", "-q", "--hard", base)
	runGit(t, work, "commit", "-q", "--allow-empty", "-m", "diverge")
	div := runGit(t, work, "rev-parse", "HEAD")
	runGit(t, work, "push", "-q", "origin", ff+":refs/heads/tmp-ff", div+":refs/heads/tmp-div")

	if _, err := f.svc.SaveProtection(ctx, f.alice, "alice", "prot", ProtectionRule{Pattern: "main"}); err != nil {
		t.Fatal(err)
	}
	check := func(u RefUpdate) (bool, string) {
		ok, msg, err := f.svc.CheckPush(ctx, r.ID, []RefUpdate{u}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return ok, msg
	}
	if ok, msg := check(RefUpdate{Old: base, New: ff, Ref: "refs/heads/main"}); !ok {
		t.Fatalf("fast-forward rejected: %s", msg)
	}
	if ok, _ := check(RefUpdate{Old: ff, New: div, Ref: "refs/heads/main"}); ok {
		t.Fatal("force-push to protected branch allowed")
	}
	if ok, _ := check(RefUpdate{Old: ff, New: div, Ref: "refs/heads/other"}); !ok {
		t.Fatal("force-push to unprotected branch rejected")
	}
	if ok, _ := check(RefUpdate{Old: base, New: strings.Repeat("0", 40), Ref: "refs/heads/main"}); ok {
		t.Fatal("deleting the default branch allowed")
	}
	if ok, _ := check(RefUpdate{Old: strings.Repeat("0", 40), New: ff, Ref: "refs/pull/1/head"}); ok {
		t.Fatal("writing refs/pull allowed")
	}
	f.svc.SaveProtection(ctx, f.alice, "alice", "prot", ProtectionRule{Pattern: "release/*", RequirePullRequest: true})
	if ok, _ := check(RefUpdate{Old: base, New: ff, Ref: "refs/heads/release/1.0"}); ok {
		t.Fatal("direct push to a require-PR branch allowed")
	}
}

func TestAfterPushSetsDefaultBranch(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	r, _ := f.svc.Create(ctx, f.alice, CreateRepo{Name: "empty"})
	f.svc.now = func() time.Time { return time.Unix(2000, 0) }
	sha := strings.Repeat("a", 40)
	if _, err := f.svc.AfterPush(ctx, r.ID, f.alice.UserID, []RefUpdate{{Old: strings.Repeat("0", 40), New: sha, Ref: "refs/heads/trunk"}}, ""); err != nil {
		t.Fatal(err)
	}
	got, _ := f.db.RepoByID(ctx, r.ID)
	if got.IsEmpty || got.DefaultBranch != "trunk" {
		t.Fatalf("%+v", got)
	}
}

func TestRenameAndDelete(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.svc.Create(ctx, f.alice, CreateRepo{Name: "old", InitReadme: true})
	newName := "new-name"
	r, err := f.svc.Update(ctx, f.alice, "alice", "old", UpdateRepo{Name: &newName})
	if err != nil || r.Name != "new-name" {
		t.Fatalf("%+v %v", r, err)
	}
	if _, err := f.svc.Tree(ctx, f.alice, "alice", "new-name", "", ""); err != nil {
		t.Fatalf("renamed repository not readable: %v", err)
	}
	if err := f.svc.Delete(ctx, f.alice, "alice", "new-name", "alice/wrong"); !errors.Is(err, errs.Invalid) {
		t.Fatalf("delete without confirmation: %v", err)
	}
	if err := f.svc.Delete(ctx, f.bob, "alice", "new-name", "alice/new-name"); !errors.Is(err, errs.Forbidden) {
		t.Fatalf("non-admin delete: %v", err)
	}
	if err := f.svc.Delete(ctx, f.alice, "alice", "new-name", "alice/new-name"); err != nil {
		t.Fatal(err)
	}
	p, _ := f.svc.PathFor("alice", "new-name")
	if _, err := exec.Command("git", "-C", p, "rev-parse").Output(); err == nil {
		t.Fatal("repository directory still exists")
	}
}

func TestValidateName(t *testing.T) {
	for name, ok := range map[string]bool{"demo": true, "my.repo_1-x": true, "-x": false, "a.git": false, "con": false, "x.": false, "..": false, "a/b": false} {
		if (ValidateName(name) == nil) != ok {
			t.Errorf("%q: ok=%v", name, !ok)
		}
	}
}
