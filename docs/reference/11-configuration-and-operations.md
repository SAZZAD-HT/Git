# 11 — Configuration and Operations

How to install, configure, run, back up and upgrade Anvil on Windows and Linux.

## 1. Requirements

| | Server (`anvil`) | Runner (`anvil-runner`) |
|---|---|---|
| OS | Windows 10/11, Windows Server 2019+, Linux (glibc or musl) x86-64/ARM64 | same |
| Git | ≥ 2.38 (Windows: Git for Windows) | ≥ 2.38 |
| RAM | 256 MB minimum, 1 GB for ~50 users | depends on jobs |
| Disk | repos + logs + artifacts | workspace for running jobs |
| Other | nothing: no database server, no Docker | the build tools your jobs need (Go, Node, .NET…) |

## 2. Install

### Linux

```bash
sudo useradd --system --create-home --home-dir /var/lib/anvil anvil
sudo install -m 0755 anvil /usr/local/bin/anvil
sudo mkdir -p /etc/anvil && sudo anvil init --config /etc/anvil/anvil.toml --data /var/lib/anvil
sudo chown -R anvil:anvil /var/lib/anvil
sudo anvil install-service --config /etc/anvil/anvil.toml   # writes a systemd unit
sudo systemctl enable --now anvil
sudo -u anvil anvil admin create-user --config /etc/anvil/anvil.toml --admin --username admin --email admin@example.com
```

### Windows (PowerShell as Administrator)

```powershell
New-Item -ItemType Directory C:\Anvil, C:\Anvil\data
Copy-Item .\anvil.exe C:\Anvil\
C:\Anvil\anvil.exe init --config C:\Anvil\anvil.toml --data C:\Anvil\data
C:\Anvil\anvil.exe install-service --config C:\Anvil\anvil.toml   # registers Windows service "Anvil"
Start-Service Anvil
C:\Anvil\anvil.exe admin create-user --config C:\Anvil\anvil.toml --admin --username admin --email admin@example.com
```

`anvil init` writes a commented config with a random `security.secret_key`, creates the data directory, and checks the Git version. `install-service` uses the native service manager: a systemd unit on Linux, the Windows Service Control Manager on Windows (through `golang.org/x/sys/windows/svc`, so no NSSM is needed for Anvil itself).

### Runner

```bash
anvil-runner register --url https://anvil.local --token anv_reg_… --name build-linux-1 --labels linux,x64
anvil-runner install-service      # systemd unit / Windows service; runs `anvil-runner run`
```

## 3. Configuration reference (`anvil.toml`)

Every key can also be set with an environment variable: `ANVIL_<SECTION>_<KEY>` in upper case (for example `ANVIL_SERVER_HTTP_ADDR`). Environment wins over the file.

```toml
[server]
http_addr        = "0.0.0.0:3000"     # main listener
internal_addr    = "127.0.0.1:3001"   # hook API — must stay on loopback
public_url       = "http://localhost:3000"  # used in clone URLs, emails, runner payloads
tls_cert         = ""                 # set both to serve HTTPS directly
tls_key          = ""
trusted_proxies  = []                 # CIDRs allowed to set X-Forwarded-*
shutdown_timeout = "30s"

[storage]
data_dir = "./data"                   # repos/, logs/, blobs/, tmp/, anvil.db

[database]
driver = "sqlite"                     # "postgres" planned
dsn    = ""                           # default: <data_dir>/anvil.db

[git]
binary            = "git"             # full path if not on PATH
command_timeout   = "60s"             # UI read commands
max_push_size     = "2GiB"
maintenance_cron  = "0 3 * * *"       # daily git gc --auto

[auth]
allow_registration   = false
require_signin_view  = false          # true = no anonymous browsing
allow_password_git   = false          # git over HTTP needs a PAT unless true
session_ttl          = "336h"         # 14 days

[security]
secret_key = ""                       # 32-byte base64; falls back to <data_dir>/secret.key

[ci]
long_poll_timeout       = "30s"
default_job_timeout     = "1h"
max_log_size            = "50MiB"
artifact_retention_days = 30
fork_pr_approval        = true
protected_labels        = ["deploy"]  # only protected refs may target these runner labels

[explorer]
enabled = true
persist = false                       # keep traces in DB, not only in memory

[log]
level  = "info"                       # debug | info | warn | error
format = "text"                       # text | json
file   = ""                           # empty = stderr (journald / Windows Event Log via service wrapper)

[mail]
enabled = false
smtp_addr = ""
from = ""
```

Runner config (`.runner.toml`, written by `register`):

```toml
url         = "https://anvil.local"
token       = "anv_run_…"              # file permissions restricted to the runner user
name        = "build-linux-1"
labels      = ["linux", "x64"]
capacity    = 1
work_dir    = "/var/lib/anvil-runner/work"
keep_workspace = false
shell_windows  = "auto"                 # auto | pwsh | powershell
```

## 4. Command-line reference

| Command | Purpose |
|---|---|
| `anvil serve` | Run the server (what the service runs) |
| `anvil init` | Create config + data directory |
| `anvil migrate [--dry-run]` | Apply DB migrations without starting |
| `anvil admin create-user / reset-password / promote` | User admin without the web UI |
| `anvil admin repair-hooks` | Reinstall hook scripts in all repositories |
| `anvil backup --out <file>` / `anvil restore --in <file>` | See §6 |
| `anvil doctor` | Check config, Git version, permissions, DB integrity, orphaned repos |
| `anvil hook <name>` | Internal: called by Git hook scripts |
| `anvil install-service` / `uninstall-service` | Native service management |
| `anvil version` | Version, commit, build date |
| `anvil-runner register / run / status / install-service` | Runner management |

## 5. Logging and monitoring

- **Logs:** structured `log/slog`, one line per event, always including `request_id` and `trace_id` where relevant. Under systemd they go to journald; as a Windows service they go to the configured file and to the Windows Event Log for errors.
- **Health:** `/healthz` (process alive) and `/readyz` (DB, Git, migrations OK) for load balancers or uptime checks.
- **Metrics:** `/metrics` in Prometheus text format (planned for 1.0): HTTP latency, Git operations, queue depth, jobs by status, runners online.

## 6. Backup and restore

`anvil backup` creates one `.tar.gz` containing:

1. A consistent copy of the SQLite DB (`VACUUM INTO`, which is safe while the server runs).
2. All bare repositories, each copied while holding the per-repo lock so no push is half-written.
3. Blobs (artifacts can be excluded with `--skip-artifacts`), release assets and packages.
4. `secret.key`, **if** `--include-secret-key` is passed. Otherwise you must back it up separately.

`anvil restore` refuses to overwrite a non-empty data directory unless `--force` is given.

**Recommended:** daily backup via cron or Windows Task Scheduler, keeping 7 daily and 4 weekly copies, stored on another machine.

## 7. Upgrades

1. Read the release notes (breaking changes are listed first).
2. `anvil backup`.
3. Stop the service, replace the binary, start the service. Migrations run automatically at startup. Hook scripts are reinstalled if the version changed.
4. Runners may be one minor version behind the server. The server tells older runners in the heartbeat response when they must upgrade.

Downgrades are not supported after migrations have run. Restore the backup instead.

## 8. Reverse proxy examples

```nginx
# nginx
location / {
  proxy_pass http://127.0.0.1:3000;
  proxy_set_header Host $host;
  proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
  proxy_set_header X-Forwarded-Proto $scheme;
  proxy_buffering off;              # live logs (SSE) and large git pushes
  client_max_body_size 2g;
  proxy_read_timeout 300s;          # long-polls and big clones
}
```

```
# Caddy
anvil.example.com {
  reverse_proxy 127.0.0.1:3000 { flush_interval -1 }
}
```

On Windows, IIS with URL Rewrite + ARR works. Set response buffering off and increase `maxAllowedContentLength`.
