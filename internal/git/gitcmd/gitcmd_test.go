package gitcmd

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestParseVersion(t *testing.T) {
	cases := map[string]Version{
		"git version 2.53.0.windows.1\n": {Major: 2, Minor: 53},
		"git version 2.38.1":             {Major: 2, Minor: 38, Patch: 1},
		"git version 2.39 (Apple Git-1)": {Major: 2, Minor: 39},
	}
	for in, want := range cases {
		got, err := ParseVersion(in)
		if err != nil || got.Major != want.Major || got.Minor != want.Minor || got.Patch != want.Patch {
			t.Errorf("%q -> %v, %v", in, got, err)
		}
	}
	if (Version{Major: 2, Minor: 37, Patch: 9}).AtLeast(MinVersion) {
		t.Error("2.37.9 accepted")
	}
}

func TestRunGit(t *testing.T) {
	if _, err := Init("git"); err != nil {
		t.Skip("git not available:", err)
	}
	out, err := New("version").Output(context.Background())
	if err != nil || !strings.HasPrefix(string(out), "git version") {
		t.Fatalf("%q, %v", out, err)
	}
	_, err = New("rev-parse", "--end-of-options", "HEAD").WithDir(t.TempDir()).Output(context.Background())
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Stderr == "" {
		t.Fatalf("want ExitError with stderr outside a repository, got %v", err)
	}
}
