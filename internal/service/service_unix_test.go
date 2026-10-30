//go:build !windows

package service

import (
	"strings"
	"testing"
)

func TestUnitFile(t *testing.T) {
	u := unitFile(Spec{Name: "anvil", Description: "Anvil", Exe: "/usr/local/bin/anvil",
		Args: []string{"serve", "--config", "/etc/anvil/my config.toml"}, User: "anvil"})
	for _, want := range []string{
		`ExecStart=/usr/local/bin/anvil serve --config "/etc/anvil/my config.toml"`,
		"User=anvil", "Restart=on-failure", "WantedBy=multi-user.target",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("unit file missing %q:\n%s", want, u)
		}
	}
}
