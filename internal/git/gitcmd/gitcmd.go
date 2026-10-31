package gitcmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"sync/atomic"
	"time"
)

// MinVersion is the oldest git Anvil supports. 2.38 added `git merge-tree --write-tree`,
// which pull requests use to test mergeability without a working tree.
var MinVersion = Version{Major: 2, Minor: 38}

// Version is a parsed `git version` result.
type Version struct {
	Major, Minor, Patch int
	Raw                 string // full output, e.g. "git version 2.53.0.windows.1"
}

// AtLeast reports whether v is the same as or newer than o.
func (v Version) AtLeast(o Version) bool {
	if v.Major != o.Major {
		return v.Major > o.Major
	}
	if v.Minor != o.Minor {
		return v.Minor > o.Minor
	}
	return v.Patch >= o.Patch
}

// String returns "major.minor.patch".
func (v Version) String() string { return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) }

var versionRE = regexp.MustCompile(`git version (\d+)\.(\d+)(?:\.(\d+))?`)

// ParseVersion parses the output of `git version`.
func ParseVersion(out string) (Version, error) {
	m := versionRE.FindStringSubmatch(out)
	if m == nil {
		return Version{}, fmt.Errorf("unrecognised git version output %q", out)
	}
	v := Version{Raw: string(bytes.TrimSpace([]byte(out)))}
	v.Major, _ = strconv.Atoi(m[1])
	v.Minor, _ = strconv.Atoi(m[2])
	v.Patch, _ = strconv.Atoi(m[3]) // empty patch parses as 0
	return v, nil
}

type state struct {
	path    string
	version Version
}

var current atomic.Pointer[state]

// Init records the absolute path of the git binary and checks the minimum version (2.38).
// Must be called once at startup; every later Cmd uses this path. binary may be a bare
// name looked up on PATH or a full path. It returns an error if git is missing, cannot
// run, or is older than MinVersion.
func Init(binary string) (Version, error) {
	path, err := exec.LookPath(binary)
	if err != nil {
		return Version{}, fmt.Errorf("git not found (%q): %w; install Git %s or newer, or set git.binary", binary, err, MinVersion)
	}
	if !filepath.IsAbs(path) {
		return Version{}, fmt.Errorf("git resolved to relative path %q; set git.binary to a full path", path)
	}
	out, err := exec.Command(path, "version").Output()
	if err != nil {
		return Version{}, fmt.Errorf("run %s version: %w", path, err)
	}
	v, err := ParseVersion(string(out))
	if err != nil {
		return Version{}, err
	}
	if !v.AtLeast(MinVersion) {
		return v, fmt.Errorf("git %s is too old; Anvil needs %s or newer", v, MinVersion)
	}
	current.Store(&state{path: path, version: v})
	return v, nil
}

// Installed returns the path and version recorded by Init, and false if Init has not succeeded.
func Installed() (path string, v Version, ok bool) {
	s := current.Load()
	if s == nil {
		return "", Version{}, false
	}
	return s.path, s.version, true
}

// Cmd describes one invocation of the git binary. Build it with New and the With* methods.
// A Cmd may be run once.
type Cmd struct {
	args    []string
	dir     string
	env     []string
	timeout time.Duration
	stdin   io.Reader
}

// New starts building a git command. args are passed as an argv list, never through a shell.
// Callers must place user-supplied values after "--end-of-options" or "--".
func New(args ...string) *Cmd { return &Cmd{args: args} }

// WithDir sets the working directory (a bare repository path for most server commands).
func (c *Cmd) WithDir(dir string) *Cmd { c.dir = dir; return c }

// WithEnv adds KEY=VALUE entries to the inherited environment.
func (c *Cmd) WithEnv(kv ...string) *Cmd { c.env = append(c.env, kv...); return c }

// WithTimeout kills the command if it runs longer than d. Zero means only ctx limits it.
func (c *Cmd) WithTimeout(d time.Duration) *Cmd { c.timeout = d; return c }

// WithStdin connects r to the command's standard input.
func (c *Cmd) WithStdin(r io.Reader) *Cmd { c.stdin = r; return c }

// ExitError is returned when git exits with a non-zero status. Stderr holds its error output
// (truncated to 64 KiB).
type ExitError struct {
	Args     []string
	ExitCode int
	Stderr   string
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("git %v: exit status %d: %s", e.Args, e.ExitCode, e.Stderr)
}

// ErrNotInitialized is returned when a command runs before Init succeeded.
var ErrNotInitialized = errors.New("gitcmd: Init has not been called")

// Output runs the command and returns stdout. On a non-zero exit it returns *ExitError
// carrying stderr. It blocks until git exits, ctx is cancelled, or the timeout expires.
func (c *Cmd) Output(ctx context.Context) ([]byte, error) {
	var out bytes.Buffer
	err := c.Stream(ctx, &out)
	return out.Bytes(), err
}

// Stream runs the command, copying stdout to w as it is produced. Used for packs and archives.
// Errors are the same as Output's.
func (c *Cmd) Stream(ctx context.Context, w io.Writer) error {
	s := current.Load()
	if s == nil {
		return ErrNotInitialized
	}
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, s.path, c.args...)
	cmd.Dir = c.dir
	// GIT_TERMINAL_PROMPT=0: never block waiting for credentials on a server.
	cmd.Env = append(append(os.Environ(), "GIT_TERMINAL_PROMPT=0"), c.env...)
	cmd.Stdin = c.stdin
	cmd.Stdout = w
	stderr := &limitedBuffer{max: 64 << 10}
	cmd.Stderr = stderr
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if ctx.Err() != nil {
			return fmt.Errorf("git %v: %w", c.args, ctx.Err())
		}
		return &ExitError{Args: c.args, ExitCode: ee.ExitCode(), Stderr: stderr.String()}
	}
	return err
}

type limitedBuffer struct {
	bytes.Buffer
	max int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if room := b.max - b.Len(); room > 0 {
		b.Buffer.Write(p[:min(len(p), room)])
	}
	return len(p), nil // keep draining so git never blocks on a full pipe
}
