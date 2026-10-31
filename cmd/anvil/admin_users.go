package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"anvil/internal/auth"
	"anvil/internal/config"
	"anvil/internal/events"
	"anvil/internal/git/hooks"
	"anvil/internal/repos"
	"anvil/internal/store"
	"anvil/internal/users"
	"anvil/internal/version"
)

// runAdmin implements `anvil admin <subcommand>`: user administration without the web UI
// (docs/reference/11 §4). It works while the server is running (SQLite WAL allows it).
func runAdmin(ctx context.Context, args []string) error {
	subs := map[string]func(context.Context, *users.Service, []string) error{
		"create-user":    adminCreateUser,
		"reset-password": adminResetPassword,
		"promote":        func(ctx context.Context, s *users.Service, a []string) error { return adminFlags(ctx, s, "promote", a) },
		"demote":         func(ctx context.Context, s *users.Service, a []string) error { return adminFlags(ctx, s, "demote", a) },
		"activate": func(ctx context.Context, s *users.Service, a []string) error {
			return adminFlags(ctx, s, "activate", a)
		},
		"deactivate": func(ctx context.Context, s *users.Service, a []string) error {
			return adminFlags(ctx, s, "deactivate", a)
		},
	}
	if len(args) == 0 || (subs[args[0]] == nil && args[0] != "repair-hooks") {
		fmt.Fprintln(os.Stderr, "Usage: anvil admin <create-user|reset-password|promote|demote|activate|deactivate|repair-hooks> [flags]")
		return flag.ErrHelp
	}
	fs, cfgPath := newFlags("admin " + args[0])
	// Parse only --config here; each subcommand parses its own flags from the rest.
	fs.Usage = func() {}
	rest := extractConfigFlag(args[1:], cfgPath)
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	db, err := store.Open(ctx, cfg.Database, cfg.Storage.DataDir)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.Migrate(ctx, false); err != nil {
		return err
	}
	if args[0] == "repair-hooks" {
		return adminRepairHooks(ctx, cfg, db)
	}
	_, usersSvc := newIdentityServices(cfg, db, events.NewBus(db))
	return subs[args[0]](ctx, usersSvc, rest)
}

// adminRepairHooks implements `anvil admin repair-hooks`: rewrite the hook scripts in every
// repository so they call this anvil binary (docs/reference/11 §4).
func adminRepairHooks(ctx context.Context, cfg *config.Config, db *store.DB) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	svc := repos.NewService(repos.Deps{DB: db, Hooks: hooks.Installer{AnvilBinary: exe}, ReposDir: filepath.Join(cfg.Storage.DataDir, "repos")})
	n, err := svc.EnsureHooks(ctx, exe+" "+version.Get().Version, true)
	fmt.Printf("Reinstalled hooks in %d repositories.\n", n)
	return err
}

// extractConfigFlag removes --config/-config (with its value) from args, storing it in dst.
func extractConfigFlag(args []string, dst *string) []string {
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--config" || a == "-config":
			if i+1 < len(args) {
				*dst = args[i+1]
				i++
			}
		case strings.HasPrefix(a, "--config=") || strings.HasPrefix(a, "-config="):
			*dst = a[strings.Index(a, "=")+1:]
		default:
			rest = append(rest, a)
		}
	}
	return rest
}

// readPassword returns the password from stdin (--password-stdin) or generates one.
// Passwords are never taken as a flag value, which would leak into shell history.
func readPassword(fromStdin bool) (pw string, generated bool, err error) {
	if fromStdin {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return "", false, fmt.Errorf("read password from stdin: %w", err)
		}
		return strings.TrimRight(line, "\r\n"), false, nil
	}
	const alphabet = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 20)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", false, err
		}
		b[i] = alphabet[n.Int64()]
	}
	return string(b), true, nil
}

func adminCreateUser(ctx context.Context, s *users.Service, args []string) error {
	fs := flag.NewFlagSet("anvil admin create-user", flag.ContinueOnError)
	username := fs.String("username", "", "username (required)")
	email := fs.String("email", "", "email address (required)")
	fullName := fs.String("full-name", "", "display name")
	admin := fs.Bool("admin", false, "make the user a site admin")
	stdin := fs.Bool("password-stdin", false, "read the password from standard input (default: generate one)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *username == "" || *email == "" {
		return errors.New("--username and --email are required")
	}
	pw, generated, err := readPassword(*stdin)
	if err != nil {
		return err
	}
	u, err := s.Create(ctx, auth.SystemActor, users.CreateUser{Username: *username, Email: *email, FullName: *fullName, Password: pw, IsAdmin: *admin})
	if err != nil {
		return err
	}
	role := "user"
	if u.IsAdmin {
		role = "site admin"
	}
	fmt.Printf("Created %s %s.\n", role, u.Username)
	if generated {
		fmt.Printf("Password: %s\n(Shown once. Change it after signing in.)\n", pw)
	}
	return nil
}

func adminResetPassword(ctx context.Context, s *users.Service, args []string) error {
	fs := flag.NewFlagSet("anvil admin reset-password", flag.ContinueOnError)
	username := fs.String("username", "", "username (required)")
	stdin := fs.Bool("password-stdin", false, "read the new password from standard input (default: generate one)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *username == "" {
		return errors.New("--username is required")
	}
	pw, generated, err := readPassword(*stdin)
	if err != nil {
		return err
	}
	if err := s.ResetPassword(ctx, auth.SystemActor, *username, pw); err != nil {
		return err
	}
	fmt.Printf("Password for %s reset; their sessions were ended.\n", *username)
	if generated {
		fmt.Printf("New password: %s\n", pw)
	}
	return nil
}

func adminFlags(ctx context.Context, s *users.Service, action string, args []string) error {
	fs := flag.NewFlagSet("anvil admin "+action, flag.ContinueOnError)
	username := fs.String("username", "", "username (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *username == "" {
		return errors.New("--username is required")
	}
	t, f := true, false
	var in users.AdminUpdate
	switch action {
	case "promote":
		in.IsAdmin = &t
	case "demote":
		in.IsAdmin = &f
	case "activate":
		in.IsActive = &t
	case "deactivate":
		in.IsActive = &f
	}
	if _, err := s.SetFlags(ctx, auth.SystemActor, *username, in); err != nil {
		return err
	}
	fmt.Printf("%s: %sd.\n", *username, action)
	return nil
}
