package git

import (
	"strings"
	"testing"
)

const sample = `diff --git a/README.md b/README.md
index 9b80189..e3c1d2a 100644
--- a/README.md
+++ b/README.md
@@ -1,3 +1,4 @@ intro
 # demo
-old line
+new line
+added
 end
diff --git a/new file.txt b/new file.txt
new file mode 100644
index 0000000..ce01362
--- /dev/null
+++ b/new file.txt
@@ -0,0 +1 @@
+hello
\ No newline at end of file
diff --git a/old.go b/renamed.go
similarity index 90%
rename from old.go
rename to renamed.go
index 1111111..2222222 100644
--- a/old.go
+++ b/renamed.go
@@ -10,2 +10,2 @@ func main() {
-	a()
+	b()
 }
diff --git a/logo.png b/logo.png
deleted file mode 100644
index 3333333..0000000
Binary files a/logo.png and /dev/null differ
`

func TestParseDiff(t *testing.T) {
	d, err := ParseDiff(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Files) != 4 || d.Additions != 4 || d.Deletions != 2 {
		t.Fatalf("files=%d +%d -%d", len(d.Files), d.Additions, d.Deletions)
	}
	readme := d.Files[0]
	if readme.Status != "modified" || len(readme.Hunks) != 1 || readme.Hunks[0].NewStart != 1 {
		t.Fatalf("%+v", readme)
	}
	lines := readme.Hunks[0].Lines
	if len(lines) != 5 || lines[1].Kind != "del" || lines[1].Old != 2 || lines[3].Kind != "add" || lines[3].New != 3 || lines[4].Old != 3 || lines[4].New != 4 {
		t.Fatalf("lines %+v", lines)
	}
	if nf := d.Files[1]; nf.Status != "added" || nf.NewPath != "new file.txt" || len(nf.Hunks[0].Lines) != 1 {
		t.Fatalf("new file %+v", nf)
	}
	if rn := d.Files[2]; rn.Status != "renamed" || rn.OldPath != "old.go" || rn.NewPath != "renamed.go" || rn.Hunks[0].OldStart != 10 {
		t.Fatalf("rename %+v", rn)
	}
	if bin := d.Files[3]; bin.Status != "deleted" || !bin.Binary || bin.OldPath != "logo.png" {
		t.Fatalf("binary %+v", bin)
	}
}

func TestParseDiffTruncatesLongFiles(t *testing.T) {
	var b strings.Builder
	b.WriteString("diff --git a/big b/big\n--- a/big\n+++ b/big\n@@ -0,0 +1,5000 @@\n")
	for i := 0; i < 5000; i++ {
		b.WriteString("+x\n")
	}
	d, err := ParseDiff(strings.NewReader(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	f := d.Files[0]
	if !f.Truncated || len(f.Hunks[0].Lines) != MaxDiffFileLines || f.Additions != 5000 {
		t.Fatalf("truncated=%v lines=%d additions=%d", f.Truncated, len(f.Hunks[0].Lines), f.Additions)
	}
}

func TestSplitDiffGitLine(t *testing.T) {
	for in, want := range map[string][2]string{
		"a/x.go b/x.go":               {"x.go", "x.go"},
		"a/with space b/with space":   {"with space", "with space"},
		"a/old b/new":                 {"old", "new"},
		`"a/tab\there" "b/tab\there"`: {"tab\there", "tab\there"},
	} {
		o, n := splitDiffGitLine(in)
		if o != want[0] || n != want[1] {
			t.Errorf("%q -> %q %q", in, o, n)
		}
	}
}
