// Package auth answers "who is making this request?" and owns every credential:
// passwords (argon2id), browser sessions with CSRF tokens, and personal access tokens
// (docs/reference/10-security.md §1, FR-AUTH-01..10).
//
// The HTTP layer extracts raw Credentials from a request (cookie, Bearer token, Basic
// auth) and calls Service.Authenticate, which returns an Actor. Services receive that
// Actor and decide permissions themselves; this package never imports net/http, so it
// can be tested without a server (docs/reference/14 §2).
//
// Secrets never reach the database in plain text: session ids and tokens are stored as
// SHA-256 hashes, passwords as argon2id hashes. Failed sign-ins always produce the same
// message, so usernames cannot be enumerated.
package auth
