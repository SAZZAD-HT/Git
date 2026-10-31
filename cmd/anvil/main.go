// Command anvil is the Anvil server: a self-hosted forge with Git hosting, pull requests
// and CI/CD that runs natively on Windows and Linux.
//
// main.go only parses the subcommand and calls run<Subcommand>(ctx, args). Each
// subcommand lives in its own file. serve.go is the composition root: the one place that
// constructs every service and wires them together. Read it first.
//
// Commands are documented in docs/reference/11-configuration-and-operations.md §4.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// command is one `anvil <name>` subcommand.
type command struct {
	name    string
	summary string
	run     func(ctx context.Context, args []string) error
}

var commands = []command{
	{"serve", "Run the server", runServe},
	{"init", "Create a config file and data directory", runInit},
	{"migrate", "Apply database migrations without starting", runMigrate},
	{"admin", "Manage users: create-user, reset-password, promote, deactivate, ...", runAdmin},
	{"doctor", "Check config, Git, permissions and database", runDoctor},
	{"install-service", "Install as a systemd unit or Windows service", runInstallService},
	{"uninstall-service", "Remove the service", runUninstallService},
	{"version", "Print version, commit and build date", runVersion},
	{"hook", "Internal: run by git hook scripts", runHook},
}

func main() {
	// Windows: Ctrl+C arrives as os.Interrupt. Linux: systemd sends SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := dispatch(ctx, os.Args[1:]); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(os.Stderr, "anvil:", err)
		}
		os.Exit(1)
	}
}

func dispatch(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		usage()
		if len(args) == 0 {
			return flag.ErrHelp
		}
		return nil
	}
	for _, c := range commands {
		if c.name == args[0] {
			return c.run(ctx, args[1:])
		}
	}
	usage()
	return fmt.Errorf("unknown command %q", args[0])
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: anvil <command> [flags]\n\nCommands:")
	for _, c := range commands {
		fmt.Fprintf(os.Stderr, "  %-18s %s\n", c.name, c.summary)
	}
	fmt.Fprintln(os.Stderr, "\nRun `anvil <command> -h` for a command's flags.")
}

// newFlags returns a FlagSet for a subcommand with the shared --config flag.
// The default config path is $ANVIL_CONFIG, or anvil.toml in the working directory.
func newFlags(name string) (*flag.FlagSet, *string) {
	fs := flag.NewFlagSet("anvil "+name, flag.ContinueOnError)
	def := os.Getenv("ANVIL_CONFIG")
	if def == "" {
		def = "anvil.toml"
	}
	return fs, fs.String("config", def, "path to anvil.toml (env ANVIL_CONFIG)")
}
