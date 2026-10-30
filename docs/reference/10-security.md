# 10 — Security Model

A forge stores a team's most valuable asset (source code) and runs code on its machines (CI). This document defines how Anvil authenticates, authorizes, protects secrets, and which threats it does and does not defend against.

## 1. Credentials

| Credential | Format | Stored as | Lifetime | Used for |
|---|---|---|---|---|
| Password | user-chosen, min 10 chars, checked against a built-in list of the 10,000 most common passwords | **argon2id** hash (m=64 MiB, t=3, p=2, 16-byte salt) | until changed | Web login |
| Session | 32 random bytes, base64url, in cookie `anvil_session` | SHA-256 | 14 days sliding; 30 days absolute | Browser |
| CSRF token | 32 random bytes | plain, in `sessions` row | with session | Header `X-CSRF-Token` on unsafe methods |
| Personal access token | `anv_pat_` + 40 base62 chars | SHA-256 | optional expiry | API, Git over HTTP |
| Runner registration token | `anv_reg_` + 40 chars | SHA-256 | limited uses / expiry | Registering a runner, once |
| Runner token | `anv_run_` + 40 chars | SHA-256 | until the runner is deleted | Polling for jobs |
| Job token | `anv_job_` + 40 chars | SHA-256 | **only while the job runs** | Clone, artifacts, releases, packages for that one repo |
| Internal token | 32 random bytes, per server process | memory only | process lifetime | Hook → internal API |

**Why prefixes?** A token found in a log or a leaked file can be recognised by its prefix, and secret scanners can match `anv_pat_` and similar patterns. **Why SHA-256 and not argon2 for tokens?** Tokens are long random strings, so they can't be brute-forced, and a fast hash lets every API call be checked cheaply. Passwords are short and human-chosen, so they need a slow hash.

All random values come from `crypto/rand`. Token comparisons use `crypto/subtle.ConstantTimeCompare` on the hashes.

## 2. Authorization

One function decides every repository permission:

```go
// Permission returns the highest access level actor has on repo.
// It is the ONLY place repository access is computed; handlers and services call it
// instead of checking roles themselves.
func (a *Authorizer) Permission(ctx context.Context, actor Actor, repo *Repository) Level
```

| Actor | Level |
|---|---|
| Site admin | `admin` on every repo |
| Repo owner | `admin` |
| Collaborator | their `role` (`read` / `write` / `admin`) |
| Any signed-in user, public repo | `read` |
| Anonymous, public repo, `auth.require_signin_view = false` | `read` |
| Job token | `read` on its own repo (`write` for `contents`/`packages` when granted, §4) |
| Anyone else | `none` → **404**, never 403, so private repo names are not leaked |

PAT **scopes** further limit what a token can do, whatever the user's role: `repo:read` cannot push even if the user is an admin of the repo.

## 3. Web security

| Threat | Defence |
|---|---|
| CSRF | `SameSite=Lax` cookie **and** `X-CSRF-Token` header required on POST/PUT/PATCH/DELETE with cookie auth. Bearer-token requests don't need CSRF (browsers don't attach those automatically). |
| XSS through Markdown, file names, commit messages | React escapes by default; Markdown goes through DOMPurify; `Content-Security-Policy: default-src 'self'; script-src 'self'; object-src 'none'; frame-ancestors 'none'` |
| XSS through raw files (an uploaded `.html` or `.svg`) | Raw and download endpoints send `Content-Disposition: attachment`, `X-Content-Type-Options: nosniff`, and a sandboxed CSP. Planned: serve them from a separate origin. |
| Clickjacking | `frame-ancestors 'none'` |
| Session theft | Cookie flags `HttpOnly; Secure` (when TLS); session ID rotated at login; password change revokes other sessions |
| Brute force | Per-IP and per-account rate limits on login and Git auth; growing delay after failures |
| Open redirect after login | `next=` must be a relative path starting with `/` but not `//` |
| User enumeration | Login error is always "Invalid username or password" |

## 4. CI security

CI is where a forge runs untrusted code, so it is the largest risk area.

| Threat | Defence |
|---|---|
| Pull request from a fork steals secrets | Fork-PR runs get **no secrets** and a **read-only** job token. Running them at all can require approval (`ci.fork_pr_approval = true`, the default). |
| Job token reused after the job | Token invalidated at finish, timeout or cancel; it is only valid for one repo |
| Job token leaked to disk | Checkout passes it per-command (`-c http.extraHeader`), never through `.git/config` |
| Secret printed in logs | Runner-side masking before upload, including across chunk boundaries ([07 §8.5](07-ci-cd-design.md)) |
| Job reads another job's files | Each job has its own workspace, deleted afterwards. **The OS user is the boundary**: run each runner as a dedicated unprivileged account. No container isolation in 1.0 — documented clearly in the UI and install guide. |
| Job affects the runner machine | Same: dedicated low-privilege user. Production deploy runners should be separate machines with labels that only protected branches/tags can target (`ci.protected_labels`). |
| Malicious `uses:` pulling remote code | Only built-in actions are supported; nothing is downloaded at run time |
| Runner impersonation | Runner token required; registration tokens limited in uses and time |
| Secrets at rest | AES-256-GCM with a random 12-byte nonce per value. The master key comes from `security.secret_key` or `data/secret.key` (created on first start with 0600 permissions on Linux and an owner-only ACL on Windows). **Back up this key**: without it, secrets cannot be decrypted. |

## 5. Git and process security

| Threat | Defence |
|---|---|
| **Argument injection** (a ref or path starting with `-` turned into a git option, a real CVE class in forges) | `gitcmd` builds argument lists only (never a shell string). User input is validated (`git check-ref-format`) and always placed after `--end-of-options` or `--`. A lint test greps for `exec.Command("sh"`. |
| Path traversal in repo names or file paths | Names validated by regex ([04 §1](04-git-hosting.md)); repo paths built only by `repos.PathFor(id)`, then checked to be inside `data_dir/repos`; tree paths cleaned and `..` rejected |
| Malicious repository content (huge files, zip bombs in uploads) | Upload size limits; artifacts unzipped by the runner only into the workspace, with entry paths checked (zip-slip) |
| Git config injection through hooks or cloned repos | The server never runs hooks from repository content; hooks are written by Anvil and reinstalled on startup. `safe.directory` is set only for the data directory |
| Server-side request forgery | Anvil makes no outgoing requests based on user input in 1.0 (no webhooks, no remote includes) |

## 6. Transport

- Anvil can terminate TLS itself (`server.tls_cert` / `server.tls_key`) or run behind a reverse proxy (nginx, Caddy, IIS). With a proxy, set `server.trusted_proxies` so client IPs from `X-Forwarded-For` are believed only from that proxy.
- Without TLS, Anvil logs a warning at startup unless it is listening only on localhost.
- The internal API listens only on `127.0.0.1` and also requires the internal token.

## 7. Auditing

Security-relevant actions are recorded as events with actor and IP: login success and failure, token created or revoked, collaborator changes, protection rule changes, secret changes (name only, never value), runner registration, admin actions. They are visible to site admins under **Admin → Audit log**.

## 8. Threat model summary

**In scope:** malicious anonymous users, malicious signed-in users (other tenants), malicious pull requests from forks, leaked tokens, stolen database file (tokens hashed, secrets encrypted).

**Out of scope for 1.0:** a malicious *site admin*; an attacker with OS-level access to the server; malicious code in a trusted branch run on a shared runner (mitigated only by OS user separation); denial of service from authenticated users beyond basic rate limits.

## 9. Reporting vulnerabilities

`SECURITY.md` in the repository root explains how to report privately. Fixes for security issues get a patch release and an advisory in the release notes.
