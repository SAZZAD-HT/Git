// Package version reports which build of Anvil is running.
//
// The values are set at link time by tools/devtool:
//
//	go build -ldflags "-X anvil/internal/version.Version=1.2.3 -X anvil/internal/version.Commit=abc123 ..."
//
// Unreleased builds report Version "dev". Commit and BuildDate fall back to the
// VCS information Go embeds in the binary, so `go build` alone still reports them.
// The package has no dependencies and may be imported by every other package,
// including the runner (docs/reference/14-code-structure.md §2).
package version
