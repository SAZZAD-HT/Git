// Package issues manages issues, comments and labels (FR-COLLAB-01..04).
//
// Issues and pull requests share one number sequence per repository and one table; this
// package handles everything they have in common (title, body, state, labels, comments),
// and internal/pulls adds what is specific to pull requests. Every method checks the
// actor's repository access through internal/repos and publishes issue.* events.
package issues
