//go:build !windows

package fsutil

import "os"

func makeWritable(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	return os.Chmod(path, fi.Mode().Perm()|0o200)
}

// On Unix, rename(2) over an existing file is atomic and never blocked by open handles.
func replaceFile(src, dst string) error { return os.Rename(src, dst) }

// Unix has no transient sharing violations; failures are final.
func isTransient(error) bool { return false }

func restrictToOwner(path string) error { return os.Chmod(path, 0o600) }
