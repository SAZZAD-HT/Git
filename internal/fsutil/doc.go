// Package fsutil holds the file-system operations that behave differently on
// Windows and Unix: deleting trees that contain read-only or briefly locked files,
// atomically replacing a file, and restricting a file to its owner.
//
// Each difference is listed in docs/reference/12-cross-platform.md §2.1. OS-specific
// code lives in *_windows.go and *_unix.go; callers never branch on runtime.GOOS.
package fsutil
