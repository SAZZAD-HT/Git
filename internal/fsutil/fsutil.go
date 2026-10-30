package fsutil

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// retryFor is how long RemoveAll and ReplaceFile keep retrying. Antivirus scanners and
// the Windows search indexer hold files open for short periods (docs/reference/12 §2.1).
const retryFor = 5 * time.Second

// RemoveAll is os.RemoveAll that also clears read-only attributes (Git marks pack files
// read-only, which makes os.RemoveAll fail on Windows) and retries with backoff for up to
// five seconds while files are locked. A path that does not exist is not an error.
// It blocks while retrying.
func RemoveAll(path string) error {
	return retry(func() error {
		err := os.RemoveAll(path)
		if err == nil || errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		_ = filepath.WalkDir(path, func(p string, _ fs.DirEntry, werr error) error {
			if werr == nil {
				_ = makeWritable(p)
			}
			return nil
		})
		return err
	})
}

// ReplaceFile atomically replaces dst with src, so readers see either the old or the new
// file and never a partial one. src and dst must be on the same volume. On Windows it
// uses MoveFileEx(MOVEFILE_REPLACE_EXISTING) and retries while dst is open elsewhere.
func ReplaceFile(src, dst string) error {
	return retry(func() error { return replaceFile(src, dst) })
}

// WriteFileAtomic writes data to a temporary file next to path, then replaces path with it.
// The file gets permission perm on Unix; on Windows perm is ignored.
func WriteFileAtomic(path string, data []byte, perm fs.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-"+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp) // no-op after a successful rename
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil { // must be closed before rename on Windows
		return err
	}
	if err := os.Chmod(tmp, perm); err != nil {
		return err
	}
	return ReplaceFile(tmp, path)
}

// RestrictToOwner makes path readable and writable only by the current user:
// mode 0600 on Unix, a protected owner-only ACL on Windows. Used for secret.key and
// .runner.toml. It returns an error if the permissions cannot be changed.
func RestrictToOwner(path string) error { return restrictToOwner(path) }

func retry(op func() error) error {
	deadline := time.Now().Add(retryFor)
	wait := 20 * time.Millisecond
	for {
		err := op()
		if err == nil || !isTransient(err) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(wait)
		wait = min(wait*2, 500*time.Millisecond)
	}
}
