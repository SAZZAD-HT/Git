# 12 — Cross-Platform Design (Windows and Linux)

Running natively on both Windows and Linux is a core goal (G4). This document lists **every known difference** between the two and how Anvil handles it. When you find a new one, add it here.

## 1. Strategy

1. **Write portable Go first.** Use `path/filepath` for OS paths, `os.UserConfigDir`, `os.TempDir`, `exec.Command` with argument lists. Most code needs nothing special.
2. **Isolate the rest behind small interfaces**, with build-tagged files: `foo_windows.go` and `foo_unix.go`. Today this is only `procgroup`, `service`, `fsutil` (permissions, rename) and `shell`.
3. **Test on both, always.** The project's own CI runs every test on Windows and Linux ([16](16-testing-strategy.md)). A change that breaks one OS cannot merge.

## 2. Known differences and decisions

### 2.1 Filesystem

| Issue | Windows | Linux | Anvil's approach |
|---|---|---|---|
| Path separator | `\` (also accepts `/`) | `/` | `filepath` for disk, `path` (always `/`) for Git paths and URLs. **Never mix them**: Git tree paths are always `/`. |
| Case sensitivity | case-insensitive (usually) | case-sensitive | Owner/repo names are lower-cased on disk ([04 §1](04-git-hosting.md)); file content is served from Git objects, not the filesystem, so repos with `README` and `readme` still work |
| Reserved names | `CON`, `PRN`, `AUX`, `NUL`, `COM1-9`, `LPT1-9`, names ending in `.` or space | none | Rejected as owner/repo names on **every** OS, so a backup from Linux always restores on Windows |
| Max path length | 260 unless long paths are enabled | 4096 | Data paths are short (`repos/<owner>/<name>.git`). The runner enables `core.longpaths=true` for checkouts. The installer recommends enabling Windows long paths. |
| Rename over an existing file | fails if the target is open | atomic | `fsutil.ReplaceFile` uses `MoveFileEx(MOVEFILE_REPLACE_EXISTING)` with retries on Windows |
| Deleting files in use | fails (sharing violation) | works | Close handles before deleting; `fsutil.RemoveAll` retries with backoff for up to 5 s (antivirus and search indexers briefly lock files) |
| Read-only files | Git marks pack files read-only, so `os.RemoveAll` fails | not an issue | `fsutil.RemoveAll` clears the read-only attribute first |
| Permissions | ACLs | mode bits | `fsutil.RestrictToOwner(path)`: `chmod 0600` on Linux, owner-only ACL on Windows. Used for `secret.key` and `.runner.toml`. |
| Line endings | CRLF tooling | LF | The server never converts content. Pipeline YAML is parsed with CRLF tolerated. Generated `.sh` scripts are written with LF, `.ps1`/`.cmd` with CRLF. |
| Text encoding | PowerShell 5.1 reads BOM-less `.ps1` files as ANSI | UTF-8 | Generated `.ps1` files are UTF-8 **with BOM** ([07 §8.3](07-ci-cd-design.md)) |

### 2.2 Processes

| Issue | Windows | Linux | Approach |
|---|---|---|---|
| Kill a process tree | no process groups; children survive killing the parent | process groups | Job Objects vs `Setpgid` + `kill(-pgid)` — `internal/runner/procgroup` |
| Graceful stop signal | no SIGTERM; console Ctrl+C events, or service stop | SIGTERM | Server: `os/signal` for `os.Interrupt` + service control handler. Runner: on Windows, step processes are terminated (no graceful phase) — documented |
| Executable lookup | `PATHEXT` (`.exe`, `.cmd`, `.bat`) | `PATH` + exec bit | `exec.LookPath` handles both. We resolve `git` once at startup and use the full path afterwards (protects against a `git.exe` planted in the working directory). |
| Default shell for CI | `pwsh` if installed, otherwise Windows PowerShell 5.1 | `bash`, otherwise `sh` | Runner detects it at startup and reports it in its labels (`shell:pwsh` / `shell:powershell` / `shell:bash`) |
| Environment variable names | case-insensitive (`Path` = `PATH`) | case-sensitive | Runner env merge uses case-insensitive keys on Windows |
| Command-line quoting | each program parses its own command line | argv array | Go's `exec` quotes for the MSVC convention. `cmd /C` needs special handling: we always run a script **file**, never inline commands |

### 2.3 Git

| Issue | Approach |
|---|---|
| Hooks are `sh` scripts. Windows has no `sh`… | Git for Windows ships `sh.exe` and uses it to run hooks automatically. Our hook script is a 2-line `sh` script calling `anvil.exe`, which works unchanged. The hook path is written with forward slashes (`C:/Anvil/anvil.exe`), which `sh.exe` understands. |
| `core.autocrlf` on the server | Bare repos have no working tree, so it does not apply. The runner sets `core.autocrlf=false` for checkouts so files match exactly what was committed; workflows can override it. |
| `safe.directory` (Git refuses repos owned by another user) | Server: runs as the owner of `data_dir`, and `anvil doctor` checks this. Runner: checkout dirs are owned by the runner user. |
| `core.fsmonitor`, credential helpers on the runner machine | Disabled per command with `-c core.fsmonitor=false -c credential.helper=` so a developer's global Git settings can't interfere with jobs |

### 2.4 Services and installation

| | Windows | Linux |
|---|---|---|
| Service manager | Service Control Manager (`golang.org/x/sys/windows/svc`) | systemd unit file |
| Service account | `NT SERVICE\Anvil` virtual account (default) or a named user | `anvil` system user |
| Default paths | `C:\Anvil\` (or `%ProgramData%\Anvil`) | `/usr/local/bin`, `/etc/anvil`, `/var/lib/anvil` |
| Logs | file + Event Log | journald |
| Firewall | `install-service` prints the `New-NetFirewallRule` command (does not change the firewall itself) | prints the `ufw`/`firewalld` hint |

### 2.5 Networking

- Binding to ports below 1024 needs root on Linux (or `CAP_NET_BIND_SERVICE`). The default port is 3000 so no privileges are needed.
- Windows may show a firewall prompt the first time `anvil.exe` listens on a non-loopback address when not run as a service.

## 3. Build matrix

| Target | Built by | Tested in CI |
|---|---|---|
| `windows/amd64` | `GOOS=windows GOARCH=amd64 go build` | ✅ unit + integration + E2E |
| `windows/arm64` | cross-compile | build only |
| `linux/amd64` | native | ✅ unit + integration + E2E |
| `linux/arm64` | cross-compile | ✅ unit (on ARM runner when available) |
| `darwin/*` | cross-compile | build only — not officially supported in 1.0 |

`CGO_ENABLED=0` for all targets. This is possible because SQLite is the pure-Go `modernc.org/sqlite` ([02 ADR-005](02-tech-stack-decisions.md)).

## 4. Developer checklist for portable code

- [ ] Paths built with `filepath.Join`, never string concatenation with `/` or `\`.
- [ ] Git paths and URL paths use `path`, never `filepath`.
- [ ] No `exec.Command("sh", "-c", …)` or `cmd /c` with user input.
- [ ] Files closed before rename or delete (matters on Windows).
- [ ] Tests use `t.TempDir()` and don't assume `/tmp`.
- [ ] Tests don't compare paths as strings without `filepath.ToSlash`.
- [ ] New OS-specific behaviour goes in a `_windows.go` / `_unix.go` pair **and** a row in this document.
