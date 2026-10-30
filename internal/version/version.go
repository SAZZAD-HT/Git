package version

import (
	"runtime"
	"runtime/debug"
)

// Set by -ldflags at release time. Left empty or "dev" in development builds.
var (
	// Version is the semantic version of this build, or "dev".
	Version = "dev"
	// Commit is the full Git commit SHA the binary was built from.
	Commit = ""
	// BuildDate is the UTC build time in RFC 3339 format.
	BuildDate = ""
)

// Info describes the running build. It is the payload of GET /api/v1/version
// (docs/reference/06-api-spec.md) without the git_version field, which the server adds.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
}

// Get returns the build information, filling Commit and BuildDate from the
// VCS stamp embedded by the Go toolchain when they were not set with -ldflags.
// It is cheap and safe for concurrent use.
func Get() Info {
	info := Info{Version: Version, Commit: Commit, BuildDate: BuildDate, GoVersion: runtime.Version()}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			switch {
			case s.Key == "vcs.revision" && info.Commit == "":
				info.Commit = s.Value
			case s.Key == "vcs.time" && info.BuildDate == "":
				info.BuildDate = s.Value
			}
		}
	}
	return info
}

// String formats the build as one line, e.g. "anvil dev (abc1234, 2026-11-02T10:00:00Z, go1.27.3)".
func (i Info) String() string {
	commit := i.Commit
	if len(commit) > 7 {
		commit = commit[:7]
	}
	if commit == "" {
		commit = "unknown commit"
	}
	date := i.BuildDate
	if date == "" {
		date = "unknown date"
	}
	return "anvil " + i.Version + " (" + commit + ", " + date + ", " + i.GoVersion + ")"
}
