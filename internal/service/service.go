package service

import "context"

// Spec describes a service to install.
type Spec struct {
	Name        string   // service / unit name, e.g. "anvil"
	DisplayName string   // human-readable name shown by the service manager
	Description string   // one line
	Exe         string   // absolute path of the binary
	Args        []string // arguments, e.g. ["serve", "--config", "/etc/anvil/anvil.toml"]
	User        string   // Linux: account to run as (empty = root). Windows: ignored; a virtual account NT SERVICE\<Name> is used.
	WorkDir     string   // working directory
}

// Install registers spec with the OS service manager and enables it to start at boot.
// It does not start the service. It returns an error if a service with that name exists
// or the caller lacks the rights to install services.
func Install(spec Spec) error { return install(spec) }

// Uninstall stops (if running) and removes the named service.
func Uninstall(name string) error { return uninstall(name) }

// RunAsServiceIfNeeded detects being started by the Windows SCM and runs main under its
// control, cancelling main's context when the service is asked to stop. It returns
// handled=false (and does nothing) when not started by the SCM, including always on Linux,
// where systemd just runs the process and sends SIGTERM.
func RunAsServiceIfNeeded(name string, main func(ctx context.Context) error) (handled bool, err error) {
	return runAsService(name, main)
}
