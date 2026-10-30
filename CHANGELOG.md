# Changelog

All notable user-visible changes. Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## Unreleased

### Added (P1 Foundation)

- `anvil init`: writes a commented `anvil.toml` with a random secret key (owner-only permissions) and creates the data directory.
- `anvil serve`: starts the server, applies database migrations automatically, serves `/healthz`, `/readyz` and `/api/v1/version`, and shuts down gracefully on Ctrl+C, SIGTERM or service stop.
- `anvil migrate [--dry-run]`, `anvil doctor`, `anvil version`.
- `anvil install-service` / `uninstall-service`: systemd unit on Linux, Windows service (no NSSM) on Windows.
- Every config key can be overridden with `ANVIL_<SECTION>_<KEY>`.
- Structured logs with `request_id` and `trace_id` on every request line.
- `go run ./tools/devtool build|test|lint|dist|dev|docs`.
