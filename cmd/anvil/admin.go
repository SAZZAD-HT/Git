package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"anvil/internal/config"
	"anvil/internal/git/gitcmd"
	"anvil/internal/service"
	"anvil/internal/store"
	"anvil/internal/version"
)

// runMigrate implements `anvil migrate [--dry-run]`.
func runMigrate(ctx context.Context, args []string) error {
	fs, cfgPath := newFlags("migrate")
	dry := fs.Bool("dry-run", false, "only list the migrations that would run")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	db, err := store.Open(ctx, cfg.Database, cfg.Storage.DataDir)
	if err != nil {
		return err
	}
	defer db.Close()
	names, err := db.Migrate(ctx, *dry)
	verb := "Applied"
	if *dry {
		verb = "Would apply"
	}
	if len(names) == 0 && err == nil {
		fmt.Println("Database is up to date.")
	}
	for _, n := range names {
		fmt.Printf("%s %s\n", verb, n)
	}
	return err
}

// runDoctor implements `anvil doctor`: it runs every check, prints one line per check,
// and fails if any check fails.
func runDoctor(ctx context.Context, args []string) error {
	fs, cfgPath := newFlags("doctor")
	if err := fs.Parse(args); err != nil {
		return err
	}
	failed := 0
	report := func(name string, err error, detail string) {
		if err != nil {
			failed++
			fmt.Printf("FAIL  %-14s %v\n", name, err)
			return
		}
		fmt.Printf("ok    %-14s %s\n", name, detail)
	}

	cfg, err := config.Load(*cfgPath)
	report("config", err, *cfgPath)
	if err != nil {
		return fmt.Errorf("%d check(s) failed", failed)
	}
	v, err := gitcmd.Init(cfg.Git.Binary)
	path, _, _ := gitcmd.Installed()
	report("git", err, fmt.Sprintf("%s at %s", v, path))
	report("data dir", checkWritable(cfg.Storage.DataDir), cfg.Storage.DataDir)
	_, err = loadSecretKey(cfg)
	report("secret key", err, "present")

	db, err := store.Open(ctx, cfg.Database, cfg.Storage.DataDir)
	report("database", err, cfg.DatabasePath())
	if err == nil {
		defer db.Close()
		pending, err := db.PendingMigrations(ctx)
		if err == nil && len(pending) > 0 {
			err = fmt.Errorf("%d pending: %s (run `anvil migrate`)", len(pending), strings.Join(pending, ", "))
		}
		report("migrations", err, "up to date")
		report("integrity", db.IntegrityCheck(ctx), "quick_check ok")
	}
	if failed > 0 {
		return fmt.Errorf("%d check(s) failed", failed)
	}
	return nil
}

// checkWritable creates and removes a file in dir.
func checkWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".doctor-*")
	if err != nil {
		return fmt.Errorf("not writable: %w", err)
	}
	name := f.Name()
	f.Close()
	return os.Remove(name)
}

// runInstallService implements `anvil install-service`.
func runInstallService(ctx context.Context, args []string) error {
	fs, cfgPath := newFlags("install-service")
	user := fs.String("user", "anvil", "Linux: account the service runs as")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	absCfg, err := filepath.Abs(*cfgPath)
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	name := serviceName
	if runtime.GOOS != "windows" {
		name = strings.ToLower(serviceName)
	}
	err = service.Install(service.Spec{
		Name: name, DisplayName: "Anvil forge", Description: "Anvil: Git hosting, pull requests and CI/CD",
		Exe: exe, Args: []string{"serve", "--config", absCfg}, User: *user, WorkDir: filepath.Dir(absCfg),
	})
	if err != nil {
		return err
	}
	fmt.Printf("Installed service %q.\n", name)
	if runtime.GOOS == "windows" {
		fmt.Printf("Grant the service account access to its files, then start it:\n"+
			"  icacls \"%s\" /grant \"NT SERVICE\\%s:(OI)(CI)M\"\n  icacls \"%s\" /grant \"NT SERVICE\\%s:R\"\n  Start-Service %s\n"+
			"Firewall (if clients connect from other machines):\n  New-NetFirewallRule -DisplayName Anvil -Direction Inbound -Protocol TCP -LocalPort 3000 -Action Allow\n",
			cfg.Storage.DataDir, name, absCfg, name, name)
	} else {
		fmt.Printf("Start it with: sudo systemctl start %s\n", name)
	}
	return nil
}

// runUninstallService implements `anvil uninstall-service`.
func runUninstallService(ctx context.Context, args []string) error {
	fs, _ := newFlags("uninstall-service")
	if err := fs.Parse(args); err != nil {
		return err
	}
	name := serviceName
	if runtime.GOOS != "windows" {
		name = strings.ToLower(serviceName)
	}
	if err := service.Uninstall(name); err != nil {
		return err
	}
	fmt.Printf("Removed service %q. Data and config were left in place.\n", name)
	return nil
}

// runVersion implements `anvil version`.
func runVersion(ctx context.Context, args []string) error {
	fmt.Println(version.Get().String())
	return nil
}
