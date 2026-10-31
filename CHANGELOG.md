# Changelog

All notable user-visible changes. Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## Unreleased

### Added (P2 Identity)

- Sign in and out with username and password; browser sessions with CSRF protection (14-day sliding, 30-day absolute expiry).
- Self-registration, off by default (`auth.allow_registration`).
- Personal access tokens with scopes (`repo:read`, `repo:write`, `package:write`, `admin`) and optional expiry, usable as Bearer tokens or as the password for git over HTTP.
- Change password (ends your other sessions), list and revoke sessions.
- Site admin: list, create, promote, demote, activate and deactivate users, from the API or with `anvil admin`.
- Rate limits: 60 requests a minute per IP when anonymous, 5000 an hour per user, 10 failed sign-ins a minute per IP and per username.
- Passwords: at least 10 characters, not among the 10,000 most common, stored as argon2id.

### Added (P1 Foundation)

- `anvil init`: writes a commented `anvil.toml` with a random secret key (owner-only permissions) and creates the data directory.
- `anvil serve`: starts the server, applies database migrations automatically, serves `/healthz`, `/readyz` and `/api/v1/version`, and shuts down gracefully on Ctrl+C, SIGTERM or service stop.
- `anvil migrate [--dry-run]`, `anvil doctor`, `anvil version`.
- `anvil install-service` / `uninstall-service`: systemd unit on Linux, Windows service (no NSSM) on Windows.
- Every config key can be overridden with `ANVIL_<SECTION>_<KEY>`.
- Structured logs with `request_id` and `trace_id` on every request line.
- `go run ./tools/devtool build|test|lint|dist|dev|docs`.
