package auth

import "slices"

// ActorKind says what kind of caller an Actor is.
type ActorKind string

// Actor kinds.
const (
	Anonymous ActorKind = "anonymous" // no credentials
	UserActor ActorKind = "user"      // a signed-in user (session or token)
	Runner    ActorKind = "runner"    // a CI runner (P5)
	Job       ActorKind = "job"       // a running CI job's token (P5)
	System    ActorKind = "system"    // the server itself or an operator using the CLI
)

// Scopes a personal access token can carry (docs/reference/05 §3.1).
const (
	ScopeRepoRead     = "repo:read"
	ScopeRepoWrite    = "repo:write"
	ScopeAdmin        = "admin"
	ScopePackageWrite = "package:write"
)

// AllScopes lists every valid scope, in display order.
var AllScopes = []string{ScopeRepoRead, ScopeRepoWrite, ScopePackageWrite, ScopeAdmin}

// Via says how a user actor authenticated.
type Via string

// Authentication methods.
const (
	ViaNone     Via = ""
	ViaSession  Via = "session"
	ViaToken    Via = "token"
	ViaPassword Via = "password" // HTTP Basic with a password (git only, when allowed)
)

// Actor is whoever is making a request: a user, a runner, a job, or anonymous.
// The zero value is the anonymous actor.
type Actor struct {
	Kind     ActorKind
	UserID   int64
	Username string
	RunnerID int64
	JobID    int64
	RepoID   int64 // for Job actors: the repository the job runs in
	IsAdmin  bool  // site admin (and, for tokens, only when the token has the admin scope)
	Via      Via
	// Scopes limits a token's power whatever the user's role. nil means unrestricted
	// (session, password and system actors).
	Scopes []string
	// SessionHash and CSRFToken are set when Via is ViaSession, for the CSRF check and
	// for "revoke all other sessions".
	SessionHash string
	CSRFToken   string
}

// SystemActor is used by the CLI and background work; it may do anything.
var SystemActor = Actor{Kind: System, IsAdmin: true}

// IsAnonymous reports whether no one is signed in.
func (a Actor) IsAnonymous() bool { return a.Kind == "" || a.Kind == Anonymous }

// IsUser reports whether the actor is a signed-in user.
func (a Actor) IsUser() bool { return a.Kind == UserActor }

// HasScope reports whether the actor's credentials allow scope. Unrestricted actors
// (Scopes == nil) have every scope; repo:write implies repo:read.
func (a Actor) HasScope(scope string) bool {
	if a.Scopes == nil {
		return true
	}
	if slices.Contains(a.Scopes, scope) {
		return true
	}
	return scope == ScopeRepoRead && slices.Contains(a.Scopes, ScopeRepoWrite)
}

// IsSiteAdmin reports whether the actor may use site administration.
func (a Actor) IsSiteAdmin() bool {
	return a.Kind == System || (a.IsUser() && a.IsAdmin && a.HasScope(ScopeAdmin))
}
