package repos

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"anvil/internal/auth"
	"anvil/internal/errs"
	"anvil/internal/git"
	"anvil/internal/store"
)

// MaxBlobDisplay is the largest file returned inline to the web UI (docs/reference/04 §4).
const MaxBlobDisplay = 1 << 20

// resolve checks read access and resolves ref ("" = default branch) to a commit sha.
// An empty repository returns errs.NotFound.
func (s *Service) resolve(ctx context.Context, actor auth.Actor, owner, name, ref string) (*store.Repository, *git.Repository, string, error) {
	r, err := s.Get(ctx, actor, owner, name)
	if err != nil {
		return nil, nil, "", err
	}
	if r.IsEmpty {
		return r, nil, "", fmt.Errorf("repository %s is empty: %w", r.FullName(), errs.NotFound)
	}
	g, err := s.Open(r)
	if err != nil {
		return nil, nil, "", err
	}
	if ref == "" {
		ref = r.DefaultBranch
	}
	sha, err := g.ResolveCommit(ctx, ref)
	if err != nil {
		return nil, nil, "", err
	}
	return r, g, sha, nil
}

// cleanPath normalises a repository path and rejects "..", which git would refuse anyway,
// so errors are clear (docs/reference/10 §5).
func cleanPath(p string) (string, error) {
	p = strings.Trim(p, "/")
	for _, part := range strings.Split(p, "/") {
		if part == ".." || part == "." {
			return "", fmt.Errorf("invalid path %q: %w", p, errs.NotFound)
		}
	}
	return p, nil
}

// TreeView is a directory listing at a commit.
type TreeView struct {
	Commit  string          `json:"commit"`
	Path    string          `json:"path"`
	Entries []git.TreeEntry `json:"entries"`
}

// Tree lists directory path of owner/name at ref. Requires read access.
func (s *Service) Tree(ctx context.Context, actor auth.Actor, owner, name, ref, path string) (*TreeView, error) {
	path, err := cleanPath(path)
	if err != nil {
		return nil, err
	}
	_, g, sha, err := s.resolve(ctx, actor, owner, name, ref)
	if err != nil {
		return nil, err
	}
	entries, err := g.ListTree(ctx, sha, path)
	if err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []git.TreeEntry{}
	}
	return &TreeView{Commit: sha, Path: path, Entries: entries}, nil
}

// BlobView is one file at a commit. Content holds the text when the file is text and at
// most MaxBlobDisplay bytes; Binary or TooLarge explain why it is empty otherwise.
type BlobView struct {
	Commit   string `json:"commit"`
	Path     string `json:"path"`
	SHA      string `json:"sha"`
	Size     int64  `json:"size"`
	Binary   bool   `json:"binary"`
	TooLarge bool   `json:"too_large"`
	Content  string `json:"content"`
}

// Blob returns file path of owner/name at ref. Requires read access. A file is binary
// when its first 8 KiB contain a NUL byte or are not valid UTF-8.
func (s *Service) Blob(ctx context.Context, actor auth.Actor, owner, name, ref, path string) (*BlobView, error) {
	path, err := cleanPath(path)
	if err != nil || path == "" {
		return nil, fmt.Errorf("file %q: %w", path, errs.NotFound)
	}
	_, g, sha, err := s.resolve(ctx, actor, owner, name, ref)
	if err != nil {
		return nil, err
	}
	data, size, err := g.ReadBlob(ctx, sha, path, MaxBlobDisplay+1)
	if err != nil {
		return nil, err
	}
	info, _ := g.Stat(ctx, sha, path)
	v := &BlobView{Commit: sha, Path: path, Size: size}
	if info != nil {
		v.SHA = info.SHA
	}
	head := data[:min(len(data), 8000)]
	switch {
	case bytes.IndexByte(head, 0) >= 0 || (!utf8.Valid(head) && len(data) <= MaxBlobDisplay):
		v.Binary = true
	case size > MaxBlobDisplay:
		v.TooLarge = true
	default:
		v.Content = string(data)
	}
	return v, nil
}

// RawBlob returns up to limit bytes of a file for download. Requires read access.
func (s *Service) RawBlob(ctx context.Context, actor auth.Actor, owner, name, ref, path string, limit int64) ([]byte, error) {
	path, err := cleanPath(path)
	if err != nil {
		return nil, err
	}
	_, g, sha, err := s.resolve(ctx, actor, owner, name, ref)
	if err != nil {
		return nil, err
	}
	data, _, err := g.ReadBlob(ctx, sha, path, limit)
	return data, err
}

// Commits returns a page of history for ref (and optionally path). Requires read access.
func (s *Service) Commits(ctx context.Context, actor auth.Actor, owner, name, ref, path string, page Page) ([]git.Commit, error) {
	path, err := cleanPath(path)
	if err != nil {
		return nil, err
	}
	_, g, sha, err := s.resolve(ctx, actor, owner, name, ref)
	if err != nil {
		return nil, err
	}
	page = page.Normalize()
	return g.Log(ctx, sha, git.LogOptions{Path: path, Limit: page.PerPage, Skip: (page.Page - 1) * page.PerPage})
}

// Branch is a branch with its head commit and whether a protection rule applies.
type Branch struct {
	Name      string `json:"name"`
	Commit    string `json:"commit"`
	Protected bool   `json:"protected"`
	IsDefault bool   `json:"is_default"`
}

// Branches lists owner/name's branches. Requires read access. Empty repositories have none.
func (s *Service) Branches(ctx context.Context, actor auth.Actor, owner, name string) ([]Branch, error) {
	r, err := s.Get(ctx, actor, owner, name)
	if err != nil {
		return nil, err
	}
	g, err := s.Open(r)
	if err != nil {
		return nil, err
	}
	refs, err := g.Refs(ctx, "refs/heads")
	if err != nil {
		return nil, err
	}
	out := make([]Branch, 0, len(refs))
	for _, ref := range refs {
		rule, err := s.ProtectionFor(ctx, r.ID, ref.Short)
		if err != nil {
			return nil, err
		}
		out = append(out, Branch{Name: ref.Short, Commit: ref.Target, Protected: rule != nil, IsDefault: ref.Short == r.DefaultBranch})
	}
	return out, nil
}

// Tags lists owner/name's tags with their peeled commits. Requires read access.
func (s *Service) Tags(ctx context.Context, actor auth.Actor, owner, name string) ([]git.Ref, error) {
	r, err := s.Get(ctx, actor, owner, name)
	if err != nil {
		return nil, err
	}
	g, err := s.Open(r)
	if err != nil {
		return nil, err
	}
	return g.Refs(ctx, "refs/tags")
}

// SplitRefPath splits "<ref>/<path>" as used in /tree/ and /blob/ URLs, where the ref may
// itself contain slashes (feature/login). The longest branch or tag name that is a prefix
// wins; otherwise the first segment is the ref (a commit sha). Requires read access.
func (s *Service) SplitRefPath(ctx context.Context, actor auth.Actor, owner, name, refAndPath string) (ref, path string, err error) {
	refAndPath = strings.Trim(refAndPath, "/")
	r, err := s.Get(ctx, actor, owner, name)
	if err != nil {
		return "", "", err
	}
	best := ""
	if !r.IsEmpty {
		if g, err := s.Open(r); err == nil {
			if refs, err := g.Refs(ctx); err == nil {
				for _, rf := range refs {
					if (refAndPath == rf.Short || strings.HasPrefix(refAndPath, rf.Short+"/")) && len(rf.Short) > len(best) {
						best = rf.Short
					}
				}
			}
		}
	}
	if best != "" {
		return best, strings.TrimPrefix(strings.TrimPrefix(refAndPath, best), "/"), nil
	}
	ref, path, _ = strings.Cut(refAndPath, "/")
	return ref, path, nil
}
