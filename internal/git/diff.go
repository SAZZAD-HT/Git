package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"anvil/internal/errs"
	"anvil/internal/git/gitcmd"
)

// Limits that keep a huge diff from overwhelming the server or the browser
// (docs/reference/09 §4: DiffView virtualises, but the payload must stay bounded).
const (
	MaxDiffFiles     = 300   // files beyond this are listed as truncated
	MaxDiffFileLines = 3000  // lines per file before that file is truncated
	MaxDiffLines     = 30000 // lines across all files
)

// Diff is a parsed unified diff.
type Diff struct {
	Files     []FileDiff `json:"files"`
	Additions int        `json:"additions"`
	Deletions int        `json:"deletions"`
	Truncated bool       `json:"truncated"` // more files or lines than the limits allow
}

// FileDiff is the change to one file.
type FileDiff struct {
	OldPath   string `json:"old_path"`
	NewPath   string `json:"new_path"`
	Status    string `json:"status"` // added | deleted | modified | renamed | copied
	Binary    bool   `json:"binary"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Hunks     []Hunk `json:"hunks"`
	Truncated bool   `json:"truncated"` // hunks were cut at MaxDiffFileLines
}

// Hunk is one "@@ -a,b +c,d @@" section.
type Hunk struct {
	Header   string     `json:"header"` // the full @@ line, including any function context
	OldStart int        `json:"old_start"`
	NewStart int        `json:"new_start"`
	Lines    []DiffLine `json:"lines"`
}

// DiffLine is one line of a hunk. Old and New are line numbers in each version (0 when the
// line does not exist on that side).
type DiffLine struct {
	Kind string `json:"kind"` // context | add | del
	Old  int    `json:"old"`
	New  int    `json:"new"`
	Text string `json:"text"`
}

// diffArgs are the options every diff command uses: machine-stable output, renames
// detected, no colour, no external diff drivers or textconv from the repository config.
var diffArgs = []string{"--no-color", "--no-ext-diff", "--no-textconv", "--find-renames", "--unified=3"}

// Diff returns the parsed diff between base and head (three-dot semantics when threeDot is
// true: changes on head since it diverged from base, as a pull request shows them).
// errs.NotFound if either revision is unknown.
func (r *Repository) Diff(ctx context.Context, base, head string, threeDot bool) (*Diff, error) {
	spec := []string{base, head}
	if threeDot {
		spec = []string{base + "..." + head}
	}
	args := append(append([]string{"diff"}, diffArgs...), "--end-of-options")
	args = append(append(args, spec...), "--")
	var out bytes.Buffer
	if err := r.cmd(args...).Stream(ctx, &out); err != nil {
		var ee *gitcmd.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("compare %s and %s: %w", base, head, errs.NotFound)
		}
		return nil, err
	}
	return ParseDiff(&out)
}

// CommitDetail is a commit with its diff against its first parent.
type CommitDetail struct {
	Commit
	Diff *Diff `json:"diff,omitempty"`
}

// Commit returns one commit with its parents and, if withDiff, the parsed diff against its
// first parent (the whole tree for a root commit). errs.NotFound if sha is unknown.
func (r *Repository) Commit(ctx context.Context, sha string, withDiff bool) (*CommitDetail, error) {
	commits, err := r.Log(ctx, sha, LogOptions{Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(commits) == 0 {
		return nil, fmt.Errorf("commit %s: %w", sha, errs.NotFound)
	}
	d := &CommitDetail{Commit: commits[0]}
	if !withDiff {
		return d, nil
	}
	// --first-parent shows merges as the change they brought into the branch.
	args := append(append([]string{"show", "--format=", "--first-parent"}, diffArgs...), "--end-of-options", commits[0].SHA, "--")
	var out bytes.Buffer
	if err := r.cmd(args...).Stream(ctx, &out); err != nil {
		return nil, err
	}
	d.Diff, err = ParseDiff(&out)
	return d, err
}

// ParseDiff parses `git diff` output (unified format, a/ b/ prefixes). Exported for tests.
func ParseDiff(rd io.Reader) (*Diff, error) {
	d := &Diff{Files: []FileDiff{}}
	sc := bufio.NewScanner(rd)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024) // minified files have very long lines
	var f *FileDiff
	var h *Hunk
	oldNo, newNo, total := 0, 0, 0
	flush := func() {
		if f != nil {
			d.Additions += f.Additions
			d.Deletions += f.Deletions
			d.Files = append(d.Files, *f)
		}
		f, h = nil, nil
	}
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flush()
			if len(d.Files) >= MaxDiffFiles {
				d.Truncated = true
				for sc.Scan() { // drain so git is not blocked on a full pipe
				}
				return d, sc.Err()
			}
			oldP, newP := splitDiffGitLine(strings.TrimPrefix(line, "diff --git "))
			f = &FileDiff{OldPath: oldP, NewPath: newP, Status: "modified", Hunks: []Hunk{}}
		case f == nil:
			continue
		case h == nil && strings.HasPrefix(line, "new file mode"):
			f.Status = "added"
		case h == nil && strings.HasPrefix(line, "deleted file mode"):
			f.Status = "deleted"
		case h == nil && strings.HasPrefix(line, "rename from "):
			f.Status, f.OldPath = "renamed", unquotePath(strings.TrimPrefix(line, "rename from "))
		case h == nil && strings.HasPrefix(line, "rename to "):
			f.NewPath = unquotePath(strings.TrimPrefix(line, "rename to "))
		case h == nil && strings.HasPrefix(line, "copy from "):
			f.Status, f.OldPath = "copied", unquotePath(strings.TrimPrefix(line, "copy from "))
		case h == nil && strings.HasPrefix(line, "copy to "):
			f.NewPath = unquotePath(strings.TrimPrefix(line, "copy to "))
		case h == nil && strings.HasPrefix(line, "Binary files "):
			f.Binary = true
		case h == nil && strings.HasPrefix(line, "--- "):
			if p := strings.TrimPrefix(line, "--- "); p != "/dev/null" {
				f.OldPath = strings.TrimPrefix(unquotePath(p), "a/")
			}
		case h == nil && strings.HasPrefix(line, "+++ "):
			if p := strings.TrimPrefix(line, "+++ "); p != "/dev/null" {
				f.NewPath = strings.TrimPrefix(unquotePath(p), "b/")
			}
		case strings.HasPrefix(line, "@@ "):
			o, n, ok := parseHunkHeader(line)
			if !ok {
				continue
			}
			oldNo, newNo = o, n
			f.Hunks = append(f.Hunks, Hunk{Header: line, OldStart: o, NewStart: n, Lines: []DiffLine{}})
			h = &f.Hunks[len(f.Hunks)-1]
		case h != nil:
			kind := ""
			switch {
			case strings.HasPrefix(line, "+"):
				kind = "add"
				f.Additions++
			case strings.HasPrefix(line, "-"):
				kind = "del"
				f.Deletions++
			case strings.HasPrefix(line, " ") || line == "":
				kind = "context"
			default:
				continue // "\ No newline at end of file" and similar markers
			}
			fileLines := 0
			for _, hk := range f.Hunks {
				fileLines += len(hk.Lines)
			}
			if fileLines >= MaxDiffFileLines || total >= MaxDiffLines {
				f.Truncated = true
				if total >= MaxDiffLines {
					d.Truncated = true
				}
				continue // keep counting additions/deletions, drop the text
			}
			dl := DiffLine{Kind: kind}
			if len(line) > 0 {
				dl.Text = line[1:]
			}
			switch kind {
			case "add":
				dl.New = newNo
				newNo++
			case "del":
				dl.Old = oldNo
				oldNo++
			default:
				dl.Old, dl.New = oldNo, newNo
				oldNo++
				newNo++
			}
			h.Lines = append(h.Lines, dl)
			total++
		}
	}
	flush()
	return d, sc.Err()
}

// parseHunkHeader reads the start lines from "@@ -a[,b] +c[,d] @@ …".
func parseHunkHeader(line string) (oldStart, newStart int, ok bool) {
	f := strings.Fields(line)
	if len(f) < 3 || !strings.HasPrefix(f[1], "-") || !strings.HasPrefix(f[2], "+") {
		return 0, 0, false
	}
	num := func(s string) int {
		s, _, _ = strings.Cut(s[1:], ",")
		n, _ := strconv.Atoi(s)
		return n
	}
	return num(f[1]), num(f[2]), true
}

// splitDiffGitLine splits "a/<old> b/<new>" from a "diff --git" header. Paths with spaces
// are ambiguous here, so the "---"/"+++" and rename lines that follow override this guess.
func splitDiffGitLine(s string) (oldPath, newPath string) {
	if strings.HasPrefix(s, `"`) { // quoted paths (unusual characters)
		if q, err := strconv.QuotedPrefix(s); err == nil {
			rest := strings.TrimSpace(s[len(q):])
			return strings.TrimPrefix(unquotePath(q), "a/"), strings.TrimPrefix(unquotePath(rest), "b/")
		}
	}
	// Unquoted: "a/X b/X" — when old and new are the same, the halves have equal length.
	if len(s)%2 == 1 {
		mid := len(s) / 2
		if s[mid] == ' ' && strings.HasPrefix(s[mid+1:], "b/") && s[2:mid] == s[mid+3:] {
			return s[2:mid], s[mid+3:]
		}
	}
	if i := strings.Index(s, " b/"); i > 0 {
		return strings.TrimPrefix(s[:i], "a/"), s[i+3:]
	}
	return s, s
}

// unquotePath undoes git's C-style quoting of unusual path names.
func unquotePath(p string) string {
	if len(p) >= 2 && p[0] == '"' && p[len(p)-1] == '"' {
		if u, err := strconv.Unquote(p); err == nil {
			return u
		}
	}
	return p
}

// Archive streams a zip or tar.gz of commit sha to w, with every path under prefix/.
// format is "zip" or "tar.gz".
func (r *Repository) Archive(ctx context.Context, sha, format, prefix string, w io.Writer) error {
	if format != "zip" && format != "tar.gz" {
		return fmt.Errorf("archive format %q: %w", format, errs.Invalid)
	}
	// No timeout: large repositories take a while; the request context still cancels it.
	return gitcmd.New("archive", "--format="+format, "--prefix="+prefix+"/", "--end-of-options", sha).
		WithDir(r.path).WithEnv(r.env...).Stream(ctx, w)
}
