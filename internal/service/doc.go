// Package service installs Anvil as a native operating-system service and runs it
// under the service manager (FR-ADM-05, docs/reference/12-cross-platform.md §2.4).
//
// Linux: a systemd unit in /etc/systemd/system, enabled with systemctl.
// Windows: a Service Control Manager entry through golang.org/x/sys/windows/svc, so no
// wrapper such as NSSM is needed; when the SCM starts anvil.exe, RunAsServiceIfNeeded
// translates service stop requests into context cancellation.
//
// Installing needs administrator (Windows) or root (Linux) rights. This package and
// internal/runner may run external programs (systemctl); see docs/reference/14 §2.
package service
