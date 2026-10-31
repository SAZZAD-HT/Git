// Package pulls implements pull requests (FR-COLLAB-05..11): opening them, keeping their
// head up to date when the source branch receives pushes, checking mergeability in the
// background with `git merge-tree`, reviews, and merging.
//
// Merging never uses a working tree: the merged tree comes from MergeTree, the commit from
// CommitTree, and the base branch moves with a compare-and-swap UpdateRef, so two merges
// at the same moment can never overwrite each other (docs/reference/04 §5.3). Because no
// git hook runs for that ref update, Merge publishes git.push itself.
//
// Pull requests are issues with extra state; title, body, comments, labels and
// closing/reopening are handled by internal/issues.
package pulls
