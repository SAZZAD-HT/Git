// Package users manages accounts: creating them (by an admin, the CLI, or
// self-registration when enabled), profiles, password changes, and the site-admin
// operations to promote, demote, activate and deactivate users (FR-AUTH-03/09, FR-ADM-01).
//
// Every method takes the acting auth.Actor and checks permission itself; handlers never
// decide. Changes are published on the event bus as audit events (user.*).
package users
