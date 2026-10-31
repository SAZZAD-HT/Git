//go:build !windows

package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const unitDir = "/etc/systemd/system"

// unitFile renders a systemd unit for spec. Arguments are quoted for systemd's own
// command-line parser, not a shell.
func unitFile(spec Spec) string {
	quoted := []string{systemdQuote(spec.Exe)}
	for _, a := range spec.Args {
		quoted = append(quoted, systemdQuote(a))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[Unit]\nDescription=%s\nAfter=network-online.target\nWants=network-online.target\n\n", spec.Description)
	b.WriteString("[Service]\nType=simple\n")
	fmt.Fprintf(&b, "ExecStart=%s\n", strings.Join(quoted, " "))
	if spec.User != "" {
		fmt.Fprintf(&b, "User=%s\n", spec.User)
	}
	if spec.WorkDir != "" {
		fmt.Fprintf(&b, "WorkingDirectory=%s\n", spec.WorkDir)
	}
	b.WriteString("Restart=on-failure\nRestartSec=5\nTimeoutStopSec=45\nLimitNOFILE=65536\n\n[Install]\nWantedBy=multi-user.target\n")
	return b.String()
}

func systemdQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\"'\\$%") {
		return s
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, `$`, `$$`, `%`, `%%`)
	return `"` + r.Replace(s) + `"`
}

func install(spec Spec) error {
	path := filepath.Join(unitDir, spec.Name+".service")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists; run uninstall-service first", path)
	}
	if err := os.WriteFile(path, []byte(unitFile(spec)), 0o644); err != nil {
		return fmt.Errorf("write unit file (are you root?): %w", err)
	}
	if err := systemctl("daemon-reload"); err != nil {
		return err
	}
	return systemctl("enable", spec.Name+".service")
}

func uninstall(name string) error {
	unit := name + ".service"
	_ = systemctl("disable", "--now", unit) // may already be stopped or disabled
	if err := os.Remove(filepath.Join(unitDir, unit)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return systemctl("daemon-reload")
}

func systemctl(args ...string) error {
	out, err := exec.Command("systemctl", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func runAsService(string, func(context.Context) error) (bool, error) { return false, nil }
