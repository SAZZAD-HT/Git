package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestShQuoteRoundTripsThroughSh(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not available:", err)
	}
	for _, s := range []string{
		"/opt/anvil/anvil",
		"/home/o'brien/bin/anvil",
		"/srv/my $HOME `id` \\ \"dir\"/anvil",
		"C:/Program Files/Anvil/anvil.exe",
	} {
		out, err := exec.Command(sh, "-c", "printf %s "+shQuote(s)).Output()
		if err != nil {
			t.Fatalf("sh failed for %q: %v", s, err)
		}
		if string(out) != s {
			t.Errorf("shQuote(%q) through sh = %q", s, out)
		}
	}
}

func TestInstallWritesExecutableScripts(t *testing.T) {
	repo := t.TempDir()
	if err := (Installer{AnvilBinary: "/opt/my $dir/anvil"}).Install(repo); err != nil {
		t.Fatal(err)
	}
	for _, name := range Names {
		p := filepath.Join(repo, "hooks", name)
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if want := "exec '/opt/my $dir/anvil' hook " + name + "\n"; !strings.HasSuffix(string(b), want) {
			t.Errorf("%s = %q, want suffix %q", name, b, want)
		}
		if fi, err := os.Stat(p); err == nil && fi.Mode().Perm()&0o111 == 0 && filepath.Separator == '/' {
			t.Errorf("%s is not executable: %v", name, fi.Mode())
		}
	}
}
