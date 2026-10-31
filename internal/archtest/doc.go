// Package archtest contains only tests. They fail the build when code breaks the
// architecture rules in docs/reference/14-code-structure.md §2 or the documentation
// rules in docs/reference/15-documentation-standards.md §1 (TC-NFR-09):
//
//   - a lower layer imports a higher one;
//   - a package other than internal/store imports database/sql;
//   - a package not on the allow-list imports os/exec;
//   - a service package imports net/http;
//   - internal/runner imports a server package;
//   - a package has no doc.go with a package comment;
//   - an exported identifier has no doc comment.
//
// When a rule is wrong, change the rule here and in docs/reference/14 in the same commit.
package archtest
