package main

import (
	"context"
	"errors"
	"os"

	"anvil/internal/git/hooks"
)

// runHook implements `anvil hook <name>`, run by the scripts that hooks.Installer writes
// into every repository. It exits with the hook's status directly, because git decides
// whether to accept the push from the exit code alone.
func runHook(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: anvil hook <pre-receive|post-receive>")
	}
	os.Exit(hooks.RunClient(ctx, args[0], os.Stdin, os.Stderr, os.Environ()))
	return nil
}
