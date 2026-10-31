package repos

import (
	"context"
	"fmt"
	"path"
	"strings"

	"anvil/internal/auth"
	"anvil/internal/errs"
	"anvil/internal/events"
	"anvil/internal/git"
	"anvil/internal/store"
)

// RefUpdate is one line of hook input: a ref moving from Old to New. A zero sha in Old
// means the ref is being created; in New, deleted.
type RefUpdate struct {
	Old string `json:"old"`
	New string `json:"new"`
	Ref string `json:"ref"`
}

// IsCreate reports whether the update creates the ref.
func (u RefUpdate) IsCreate() bool { return strings.Trim(u.Old, "0") == "" }

// IsDelete reports whether the update deletes the ref.
func (u RefUpdate) IsDelete() bool { return strings.Trim(u.New, "0") == "" }

// ProtectionFor returns the rule that applies to branch, if any. A rule whose pattern equals
// the branch wins; otherwise the longest matching glob ("release/*") wins. nil if none.
func (s *Service) ProtectionFor(ctx context.Context, repoID int64, branch string) (*store.ProtectedBranch, error) {
	rules, err := s.DB.ListProtectedBranches(ctx, repoID)
	if err != nil {
		return nil, err
	}
	var best *store.ProtectedBranch
	for i := range rules {
		p := &rules[i]
		if p.Pattern == branch {
			return p, nil
		}
		if ok, _ := path.Match(p.Pattern, branch); ok && (best == nil || len(p.Pattern) > len(best.Pattern)) {
			best = p
		}
	}
	return best, nil
}

// CheckPush applies branch protection and reserved-ref rules to a pending push
// (docs/reference/04 §3.2). quarantineEnv carries GIT_OBJECT_DIRECTORY and
// GIT_ALTERNATE_OBJECT_DIRECTORIES from the hook, because during pre-receive the pushed
// objects are not yet in the repository's object store. It returns allowed=false and a
// message for the user (shown as "remote: …") when the push must be rejected.
func (s *Service) CheckPush(ctx context.Context, repoID int64, updates []RefUpdate, quarantineEnv []string) (allowed bool, message string, err error) {
	repo, err := s.DB.RepoByID(ctx, repoID)
	if err != nil {
		return false, "", err
	}
	g, err := s.Open(repo)
	if err != nil {
		return false, "", err
	}
	g = g.WithEnv(quarantineEnv...)
	for _, u := range updates {
		if strings.HasPrefix(u.Ref, "refs/pull/") {
			return false, "refs/pull is read-only: pull request refs are managed by Anvil", nil
		}
		branch, isBranch := strings.CutPrefix(u.Ref, "refs/heads/")
		if !isBranch {
			continue
		}
		if !git.ValidBranchName(branch) {
			return false, fmt.Sprintf("%q is not a valid branch name", branch), nil
		}
		if u.IsDelete() && branch == repo.DefaultBranch && !repo.IsEmpty {
			return false, fmt.Sprintf("cannot delete the default branch %q; change the default branch first", branch), nil
		}
		rule, err := s.ProtectionFor(ctx, repoID, branch)
		if err != nil {
			return false, "", err
		}
		if rule == nil {
			continue
		}
		switch {
		case u.IsDelete():
			if !rule.AllowDelete {
				return false, fmt.Sprintf("branch %q is protected: it cannot be deleted", branch), nil
			}
		case rule.RequirePullRequest:
			return false, fmt.Sprintf("branch %q is protected: changes must be made through a pull request", branch), nil
		case !u.IsCreate() && !rule.AllowForcePush:
			ff, err := g.IsAncestor(ctx, u.Old, u.New)
			if err != nil {
				return false, "", err
			}
			if !ff {
				return false, fmt.Sprintf("branch %q is protected: force-pushes are not allowed", branch), nil
			}
		}
	}
	return true, "", nil
}

// AfterPush records the push, sets the default branch on the first push to an empty
// repository, and publishes a git.push event per ref (which CI and pull requests react to).
// It returns messages to show the pusher, such as a link to open a pull request.
func (s *Service) AfterPush(ctx context.Context, repoID, pusherID int64, updates []RefUpdate, publicURL string) ([]string, error) {
	repo, err := s.DB.RepoByID(ctx, repoID)
	if err != nil {
		return nil, err
	}
	setDefault := ""
	if repo.IsEmpty {
		// First push: keep the configured default branch if it was pushed, else use the first pushed branch.
		for _, u := range updates {
			if b, ok := strings.CutPrefix(u.Ref, "refs/heads/"); ok && !u.IsDelete() {
				if setDefault == "" || b == repo.DefaultBranch {
					setDefault = b
				}
			}
		}
		if setDefault != "" && setDefault != repo.DefaultBranch {
			if g, err := s.Open(repo); err == nil {
				_ = g.SetHead(ctx, setDefault)
			}
		} else if setDefault == repo.DefaultBranch {
			setDefault = "" // nothing to change
		}
	}
	if err := s.DB.Tx(ctx, func(tx *store.Tx) error { return tx.MarkPushed(ctx, repoID, setDefault, s.now().UnixMilli()) }); err != nil {
		return nil, err
	}
	defaultBranch := repo.DefaultBranch
	if setDefault != "" {
		defaultBranch = setDefault
	}
	var messages []string
	actorKind := "user"
	if pusherID == 0 {
		actorKind = "system"
	}
	for _, u := range updates {
		typ := "git.push"
		if strings.HasPrefix(u.Ref, "refs/tags/") {
			typ = "git.tag"
		}
		if s.Bus != nil {
			_ = s.Bus.Publish(ctx, events.Event{Type: typ, RepoID: repoID, ActorKind: actorKind, ActorID: pusherID,
				Payload: map[string]string{"ref": u.Ref, "before": u.Old, "after": u.New}})
		}
		if b, ok := strings.CutPrefix(u.Ref, "refs/heads/"); ok && u.IsCreate() && b != defaultBranch && !repo.IsEmpty && publicURL != "" {
			messages = append(messages, "", fmt.Sprintf("Create a pull request for '%s':", b),
				fmt.Sprintf("  %s/%s/compare/%s...%s", strings.TrimRight(publicURL, "/"), repo.FullName(), defaultBranch, b), "")
		}
	}
	return messages, nil
}

// ProtectionRule is the input to SaveProtection.
type ProtectionRule struct {
	ID                 int64
	Pattern            string
	AllowForcePush     bool
	AllowDelete        bool
	RequirePullRequest bool
	RequiredChecks     []string
}

// Protections lists owner/name's branch protection rules. Requires read access.
func (s *Service) Protections(ctx context.Context, actor auth.Actor, owner, name string) ([]store.ProtectedBranch, error) {
	r, err := s.Get(ctx, actor, owner, name)
	if err != nil {
		return nil, err
	}
	return s.DB.ListProtectedBranches(ctx, r.ID)
}

// SaveProtection creates (ID 0) or updates a branch protection rule. Requires admin access.
func (s *Service) SaveProtection(ctx context.Context, actor auth.Actor, owner, name string, rule ProtectionRule) (*store.ProtectedBranch, error) {
	r, err := s.GetForAccess(ctx, actor, owner, name, auth.LevelAdmin)
	if err != nil {
		return nil, err
	}
	rule.Pattern = strings.TrimSpace(rule.Pattern)
	if _, err := path.Match(rule.Pattern, "x"); err != nil || rule.Pattern == "" || len(rule.Pattern) > 255 {
		return nil, errs.FieldErrors{"pattern": "must be a branch name or glob such as release/*"}
	}
	p := &store.ProtectedBranch{ID: rule.ID, RepoID: r.ID, Pattern: rule.Pattern, AllowForcePush: rule.AllowForcePush,
		AllowDelete: rule.AllowDelete, RequirePullRequest: rule.RequirePullRequest, RequiredChecks: rule.RequiredChecks, CreatedAt: s.now().UnixMilli()}
	err = s.DB.Tx(ctx, func(tx *store.Tx) error {
		p.ID, err = tx.SaveProtectedBranch(ctx, p)
		return err
	})
	if err != nil {
		return nil, err
	}
	s.publish(ctx, actor, "repo.protection_saved", r.ID, map[string]any{"pattern": p.Pattern})
	return p, nil
}

// DeleteProtection removes rule id. Requires admin access.
func (s *Service) DeleteProtection(ctx context.Context, actor auth.Actor, owner, name string, id int64) error {
	r, err := s.GetForAccess(ctx, actor, owner, name, auth.LevelAdmin)
	if err != nil {
		return err
	}
	return s.DB.Tx(ctx, func(tx *store.Tx) error { return tx.DeleteProtectedBranch(ctx, r.ID, id) })
}
