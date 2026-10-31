package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveAllReadOnly(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "repo.git")
	pack := filepath.Join(dir, "objects", "pack", "pack-1.pack")
	if err := os.MkdirAll(filepath.Dir(pack), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pack, []byte("x"), 0o444); err != nil { // Git writes packs read-only
		t.Fatal(err)
	}
	if err := RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("directory still exists: %v", err)
	}
	if err := RemoveAll(dir); err != nil {
		t.Fatalf("missing path should not be an error: %v", err)
	}
}

func TestWriteFileAtomicReplaces(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f.txt")
	for _, s := range []string{"one", "two"} {
		if err := WriteFileAtomic(p, []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(p)
	if string(b) != "two" {
		t.Fatalf("content = %q", b)
	}
	entries, _ := os.ReadDir(filepath.Dir(p))
	if len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v", entries)
	}
}

func TestRestrictToOwner(t *testing.T) {
	p := filepath.Join(t.TempDir(), "secret.key")
	if err := os.WriteFile(p, []byte("k"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RestrictToOwner(p); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(p); err != nil || string(b) != "k" {
		t.Fatalf("owner can no longer read the file: %v", err)
	}
}
