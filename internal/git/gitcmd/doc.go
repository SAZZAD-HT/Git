// Package gitcmd runs the git binary. It is the only package in the server allowed to
// import os/exec (enforced by internal/archtest).
//
// Init resolves git to an absolute path once at startup, so a git.exe planted in a
// working directory is never picked up (docs/reference/12-cross-platform.md §2.2), and
// checks the minimum version. Every command is an argv list, never a shell string;
// callers put user-supplied values after "--end-of-options" or "--"
// (docs/reference/10-security.md §5). Commands are killed when their context ends or
// their timeout expires.
package gitcmd
