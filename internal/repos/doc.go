// Package repos owns repositories: creating, renaming and deleting them (disk and
// database together), collaborators, branch protection, read access to their content
// for the web UI, and the push checks that git hooks call (docs/reference/04, FR-REPO-*).
//
// Every method that acts for a user takes an auth.Actor and asks auth.Authorizer for
// permission first; private repositories the actor cannot read are reported as
// errs.NotFound. Repository directories are only ever built by PathFor, which validates
// names and checks the result stays inside <data_dir>/repos (docs/reference/10 §5).
package repos
