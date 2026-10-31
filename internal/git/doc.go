// Package git reads and writes bare repositories on disk by running the git binary
// through internal/git/gitcmd (docs/reference/04-git-hosting.md §4).
//
// Every command uses machine-readable output (NUL or control-character separators,
// never human-readable text), has a timeout, and places caller-supplied revisions and
// paths after "--end-of-options" or "--". Callers pass resolved commit SHAs where they
// can, so results stay consistent while branches move.
//
// The package knows nothing about users or permissions; internal/repos checks those
// before calling it. Subpackages: gitcmd (process execution), smarthttp (clone/push over
// HTTP) and hooks (server-side push hooks).
package git
