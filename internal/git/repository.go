package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"anvil/internal/errs"
	"anvil/internal/git/gitcmd"
)

// ZeroSHA is the all-zero object id git uses for "no commit" in ref updates (SHA-1 repositories).
const ZeroSHA = "0000000000000000000000000000000000000000"

// DefaultTimeout limits commands run for the web UI (git.command_timeout).
var DefaultTimeout = 60 * time.Second

var shaRE = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

// IsSHA reports whether s is a full SHA-1 or SHA-256 object id in lower-case hex.
func IsSHA(s string) bool { return shaRE.MatchString(s) }

// Repository is a handle to one bare repository on disk. Safe for concurrent use.
type Repository struct {
	path string
	env  []string // extra environment for every command, e.g. quarantine object directories
}

// InitBare creates a new bare repository at path with the given default branch, and sets
// the server-side options from docs/reference/04 §1. It returns errs.Conflict if path exists.
func InitBare(ctx context.Context, path, defaultBranch string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("repository directory %s already exists: %w", path, errs.Conflict)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	if _, err := gitcmd.New("init", "--bare", "--quiet", "--initial-branch="+defaultBranch, "--", path).WithTimeout(DefaultTimeout).Output(ctx); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	for _, kv := range [][2]string{
		{"receive.denyNonFastForwards", "false"}, // protection is enforced by Anvil's pre-receive hook instead
		{"core.quotePath", "false"},              // keep non-ASCII file names readable in output
		{"receive.advertisePushOptions", "true"},
		{"uploadpack.allowFilter", "true"}, // partial clones
	} {
		if _, err := gitcmd.New("config", kv[0], kv[1]).WithDir(path).Output(ctx); err != nil {
			return fmt.Errorf("git config %s: %w", kv[0], err)
		}
	}
	return nil
}

// Open returns a handle for the bare repository at path, or errs.NotFound if it does not exist.
func Open(path string) (*Repository, error) {
	if fi, err := os.Stat(filepath.Join(path, "HEAD")); err != nil || fi.IsDir() {
		return nil, fmt.Errorf("repository at %s: %w", path, errs.NotFound)
	}
	return &Repository{path: path}, nil
}

// Path returns the repository's directory.
func (r *Repository) Path() string { return r.path }

// WithEnv returns a copy of r whose commands also get the given KEY=VALUE variables.
// Hooks use it to see objects that are still in the push quarantine
// (GIT_OBJECT_DIRECTORY, GIT_ALTERNATE_OBJECT_DIRECTORIES).
func (r *Repository) WithEnv(kv ...string) *Repository {
	return &Repository{path: r.path, env: append(append([]string{}, r.env...), kv...)}
}

func (r *Repository) cmd(args ...string) *gitcmd.Cmd {
	return gitcmd.New(args...).WithDir(r.path).WithEnv(r.env...).WithTimeout(DefaultTimeout)
}

// ResolveCommit turns a branch, tag or sha into a full commit sha, or errs.NotFound.
func (r *Repository) ResolveCommit(ctx context.Context, rev string) (string, error) {
	out, err := r.cmd("rev-parse", "--verify", "--quiet", "--end-of-options", rev+"^{commit}").Output(ctx)
	if err != nil {
		var ee *gitcmd.ExitError
		if errors.As(err, &ee) {
			return "", fmt.Errorf("revision %q: %w", rev, errs.NotFound)
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// TreeEntry is one entry of a directory listing.
type TreeEntry struct {
	Name string `json:"name"` // base name
	Path string `json:"path"` // full path from the repository root, '/'-separated
	Type string `json:"type"` // blob | tree | commit (submodule)
	Mode string `json:"mode"` // e.g. 100644, 100755, 120000 (symlink), 040000
	SHA  string `json:"sha"`
	Size int64  `json:"size"` // blobs only; -1 otherwise
}

// ListTree lists the entries of directory path at commit sha (not recursive), directories
// first, then files, each sorted by name. path "" is the root. errs.NotFound if path is not
// a directory at sha.
func (r *Repository) ListTree(ctx context.Context, sha, path string) ([]TreeEntry, error) {
	path = strings.Trim(path, "/")
	treeish := sha
	if path != "" {
		treeish = sha + ":" + path
	}
	out, err := r.cmd("ls-tree", "-z", "--long", "--end-of-options", treeish).Output(ctx)
	if err != nil {
		var ee *gitcmd.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("directory %q: %w", path, errs.NotFound)
		}
		return nil, err
	}
	var dirs, files []TreeEntry
	for _, rec := range bytes.Split(out, []byte{0}) {
		if len(rec) == 0 {
			continue
		}
		// "<mode> SP <type> SP <sha> SP+ <size>\t<name>"
		meta, name, ok := strings.Cut(string(rec), "\t")
		if !ok {
			return nil, fmt.Errorf("unexpected ls-tree output %q", rec)
		}
		f := strings.Fields(meta)
		if len(f) != 4 {
			return nil, fmt.Errorf("unexpected ls-tree output %q", rec)
		}
		e := TreeEntry{Name: name, Mode: f[0], Type: f[1], SHA: f[2], Size: -1}
		if path == "" {
			e.Path = name
		} else {
			e.Path = path + "/" + name
		}
		if n, err := strconv.ParseInt(f[3], 10, 64); err == nil {
			e.Size = n
		}
		if e.Type == "tree" {
			dirs = append(dirs, e)
		} else {
			files = append(files, e)
		}
	}
	return append(dirs, files...), nil
}

// ObjectInfo describes the object at a path in a commit.
type ObjectInfo struct {
	Type string // blob | tree | commit
	SHA  string
	Size int64
}

// Stat returns the type, id and size of the object at path in commit sha, or errs.NotFound.
func (r *Repository) Stat(ctx context.Context, sha, path string) (*ObjectInfo, error) {
	path = strings.Trim(path, "/")
	obj := sha + ":" + path
	out, err := r.cmd("cat-file", "--batch-check=%(objecttype) %(objectname) %(objectsize)").
		WithStdin(strings.NewReader(obj + "\n")).Output(ctx)
	if err != nil {
		return nil, err
	}
	f := strings.Fields(string(out))
	if len(f) != 3 || f[1] == "missing" || (len(f) >= 2 && f[len(f)-1] == "missing") {
		return nil, fmt.Errorf("path %q: %w", path, errs.NotFound)
	}
	size, _ := strconv.ParseInt(f[2], 10, 64)
	return &ObjectInfo{Type: f[0], SHA: f[1], Size: size}, nil
}

// ReadBlob returns up to maxBytes of the file at path in commit sha, and its full size.
// errs.NotFound if there is no file at path. (A pooled `cat-file --batch` process replaces
// the per-call process in a later phase; the signature stays.)
func (r *Repository) ReadBlob(ctx context.Context, sha, path string, maxBytes int64) ([]byte, int64, error) {
	info, err := r.Stat(ctx, sha, path)
	if err != nil {
		return nil, 0, err
	}
	if info.Type != "blob" {
		return nil, 0, fmt.Errorf("path %q is a %s: %w", path, info.Type, errs.NotFound)
	}
	var buf limitedWriter
	buf.max = maxBytes
	if err := r.cmd("cat-file", "blob", info.SHA).Stream(ctx, &buf); err != nil {
		return nil, 0, err
	}
	return buf.Bytes(), info.Size, nil
}

type limitedWriter struct {
	bytes.Buffer
	max int64
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if room := w.max - int64(w.Len()); room > 0 {
		w.Buffer.Write(p[:min(int64(len(p)), room)])
	}
	return len(p), nil
}

// Signature is an author or committer identity.
type Signature struct {
	Name  string    `json:"name"`
	Email string    `json:"email"`
	When  time.Time `json:"when"`
}

// Commit is one commit in a log.
type Commit struct {
	SHA       string    `json:"sha"`
	Parents   []string  `json:"parents"`
	Author    Signature `json:"author"`
	Committer Signature `json:"committer"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
}

// LogOptions limits a log.
type LogOptions struct {
	Path  string // only commits touching this path ("" = all)
	Limit int    // at most this many (default 30)
	Skip  int    // skip this many first
}

const (
	fieldSep  = "\x1f"
	recordSep = "\x1e"
	logFormat = "%H%x1f%P%x1f%an%x1f%ae%x1f%aI%x1f%cn%x1f%ce%x1f%cI%x1f%s%x1f%b%x1e"
)

// Log returns up to opts.Limit commits reachable from rev, newest first, optionally
// limited to a path. errs.NotFound if rev does not exist.
func (r *Repository) Log(ctx context.Context, rev string, opts LogOptions) ([]Commit, error) {
	if opts.Limit <= 0 {
		opts.Limit = 30
	}
	args := []string{"log", "--format=" + logFormat, "--max-count=" + strconv.Itoa(opts.Limit), "--skip=" + strconv.Itoa(opts.Skip), "--end-of-options", rev, "--"}
	if p := strings.Trim(opts.Path, "/"); p != "" {
		args = append(args, p)
	}
	out, err := r.cmd(args...).Output(ctx)
	if err != nil {
		var ee *gitcmd.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("revision %q: %w", rev, errs.NotFound)
		}
		return nil, err
	}
	var commits []Commit
	for _, rec := range strings.Split(string(out), recordSep) {
		rec = strings.TrimLeft(rec, "\n")
		if rec == "" {
			continue
		}
		f := strings.Split(rec, fieldSep)
		if len(f) != 10 {
			return nil, fmt.Errorf("unexpected log record with %d fields", len(f))
		}
		at, _ := time.Parse(time.RFC3339, f[4])
		ct, _ := time.Parse(time.RFC3339, f[7])
		c := Commit{SHA: f[0], Parents: strings.Fields(f[1]),
			Author: Signature{Name: f[2], Email: f[3], When: at}, Committer: Signature{Name: f[5], Email: f[6], When: ct},
			Subject: f[8], Body: strings.TrimSpace(f[9])}
		if c.Parents == nil {
			c.Parents = []string{}
		}
		commits = append(commits, c)
	}
	return commits, nil
}

// Ref is a branch or tag.
type Ref struct {
	Name   string `json:"name"`   // full name, e.g. refs/heads/main
	Short  string `json:"short"`  // e.g. main
	Target string `json:"target"` // commit sha (annotated tags are peeled)
}

// Refs lists refs under the given prefixes (default refs/heads and refs/tags), sorted by name.
func (r *Repository) Refs(ctx context.Context, prefixes ...string) ([]Ref, error) {
	if len(prefixes) == 0 {
		prefixes = []string{"refs/heads", "refs/tags"}
	}
	args := append([]string{"for-each-ref", "--format=%(refname)%00%(objectname)%00%(*objectname)", "--sort=refname"}, prefixes...)
	out, err := r.cmd(args...).Output(ctx)
	if err != nil {
		return nil, err
	}
	refs := []Ref{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Split(line, "\x00")
		if len(f) != 3 {
			continue
		}
		target := f[1]
		if f[2] != "" {
			target = f[2]
		}
		short := strings.TrimPrefix(strings.TrimPrefix(f[0], "refs/heads/"), "refs/tags/")
		refs = append(refs, Ref{Name: f[0], Short: short, Target: target})
	}
	return refs, nil
}

// IsAncestor reports whether ancestor is reachable from descendant (fast-forward check).
func (r *Repository) IsAncestor(ctx context.Context, ancestor, descendant string) (bool, error) {
	_, err := r.cmd("merge-base", "--is-ancestor", "--end-of-options", ancestor, descendant).Output(ctx)
	var ee *gitcmd.ExitError
	if errors.As(err, &ee) && ee.ExitCode == 1 {
		return false, nil
	}
	return err == nil, err
}

// MergeBase returns the best common ancestor of a and b. errs.NotFound if they share none
// (unrelated histories).
func (r *Repository) MergeBase(ctx context.Context, a, b string) (string, error) {
	out, err := r.cmd("merge-base", "--end-of-options", a, b).Output(ctx)
	var ee *gitcmd.ExitError
	if errors.As(err, &ee) && ee.ExitCode == 1 {
		return "", fmt.Errorf("%s and %s have no common history: %w", a, b, errs.NotFound)
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// UpdateRef moves ref to newSHA only if it currently points to oldSHA (compare-and-swap).
// This makes concurrent merges safe: the second one fails instead of silently overwriting
// the first (docs/reference/04 §5.3). An empty oldSHA means the ref must not exist yet.
// It returns errs.Conflict if the ref moved. UpdateRef does not run server hooks; callers
// must publish git.push themselves.
func (r *Repository) UpdateRef(ctx context.Context, ref, newSHA, oldSHA string) error {
	if oldSHA == "" {
		oldSHA = ZeroSHA
	}
	_, err := r.cmd("update-ref", "--end-of-options", ref, newSHA, oldSHA).Output(ctx)
	var ee *gitcmd.ExitError
	if errors.As(err, &ee) {
		return fmt.Errorf("%s moved: %w", ref, errs.Conflict)
	}
	return err
}

// CreateInitialCommit writes files as the first commit on branch of an empty repository
// and returns its sha. Used to add a README when creating a repository. errs.Conflict if
// the branch already exists.
func (r *Repository) CreateInitialCommit(ctx context.Context, branch string, files map[string][]byte, msg string, author Signature) (string, error) {
	var tree strings.Builder
	for name, content := range files {
		if strings.ContainsAny(name, "/\t\n\x00") {
			return "", fmt.Errorf("initial files must be top-level: %q", name)
		}
		out, err := r.cmd("hash-object", "-w", "--stdin").WithStdin(bytes.NewReader(content)).Output(ctx)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&tree, "100644 blob %s\t%s\n", strings.TrimSpace(string(out)), name)
	}
	treeSHA, err := r.cmd("mktree").WithStdin(strings.NewReader(tree.String())).Output(ctx)
	if err != nil {
		return "", err
	}
	when := strconv.FormatInt(author.When.Unix(), 10) + " +0000"
	commit, err := r.cmd("commit-tree", strings.TrimSpace(string(treeSHA)), "-m", msg).WithEnv(
		"GIT_AUTHOR_NAME="+author.Name, "GIT_AUTHOR_EMAIL="+author.Email, "GIT_AUTHOR_DATE="+when,
		"GIT_COMMITTER_NAME="+author.Name, "GIT_COMMITTER_EMAIL="+author.Email, "GIT_COMMITTER_DATE="+when,
	).Output(ctx)
	if err != nil {
		return "", err
	}
	sha := strings.TrimSpace(string(commit))
	return sha, r.UpdateRef(ctx, "refs/heads/"+branch, sha, "")
}

// SetHead points HEAD at branch (the repository's default branch).
func (r *Repository) SetHead(ctx context.Context, branch string) error {
	_, err := r.cmd("symbolic-ref", "HEAD", "refs/heads/"+branch).Output(ctx)
	return err
}

// ValidBranchName reports whether name is a valid branch name according to
// `git check-ref-format --branch` rules, checked without running git for the common cases.
func ValidBranchName(name string) bool {
	if name == "" || len(name) > 255 || strings.HasPrefix(name, "-") || strings.HasPrefix(name, "/") ||
		strings.HasSuffix(name, "/") || strings.HasSuffix(name, ".") || strings.HasSuffix(name, ".lock") ||
		strings.Contains(name, "..") || strings.Contains(name, "//") || strings.Contains(name, "@{") || name == "@" {
		return false
	}
	for _, c := range name {
		if c < 0x20 || c == 0x7f || strings.ContainsRune(" ~^:?*[\\", c) {
			return false
		}
	}
	for _, part := range strings.Split(name, "/") {
		if strings.HasPrefix(part, ".") {
			return false
		}
	}
	return true
}
