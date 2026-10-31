// Package smarthttp serves the Git smart HTTP protocol (clone, fetch, push over HTTP).
//
// It authorizes the caller (the actor was authenticated by httpserver.Authenticate from
// HTTP Basic or Bearer credentials), then streams the request body into
// `git upload-pack` or `git receive-pack --stateless-rpc` and the output back. Anvil does
// not parse pack files; git does all of that (docs/reference/04-git-hosting.md §2).
//
// Anonymous callers who may not read a repository get 401 with a Basic challenge so the
// git client asks for credentials; signed-in callers get 404 (private repositories are
// never revealed) or 403 (read but not write).
package smarthttp
