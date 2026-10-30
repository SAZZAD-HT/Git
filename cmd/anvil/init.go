package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"anvil/internal/config"
	"anvil/internal/fsutil"
	"anvil/internal/git/gitcmd"
)

// runInit implements `anvil init`: it writes a commented config with a random secret key,
// creates the data directory, and checks the Git version (docs/reference/11 §2).
// It refuses to overwrite an existing config unless --force is given.
func runInit(ctx context.Context, args []string) error {
	fs, cfgPath := newFlags("init")
	dataDir := fs.String("data", "", "data directory (default: ./data next to the config file)")
	force := fs.Bool("force", false, "overwrite an existing config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if _, err := os.Stat(*cfgPath); err == nil && !*force {
		return fmt.Errorf("%s already exists (use --force to overwrite)", *cfgPath)
	}

	cfg := config.Default()
	if *dataDir != "" {
		abs, err := filepath.Abs(*dataDir)
		if err != nil {
			return err
		}
		cfg.Storage.DataDir = abs
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return err
	}
	cfg.Security.SecretKey = base64.StdEncoding.EncodeToString(key)
	body, err := cfg.Render()
	if err != nil {
		return err
	}
	if dir := filepath.Dir(*cfgPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if err := fsutil.WriteFileAtomic(*cfgPath, body, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	// The file holds the secret key: only the owner may read it.
	if err := fsutil.RestrictToOwner(*cfgPath); err != nil {
		return fmt.Errorf("restrict config permissions: %w", err)
	}

	// Load what we wrote, so relative paths resolve exactly as `serve` will resolve them.
	loaded, err := config.Load(*cfgPath)
	if err != nil {
		return fmt.Errorf("written config does not load (bug): %w", err)
	}
	if err := ensureDataDirs(loaded.Storage.DataDir); err != nil {
		return err
	}
	fmt.Printf("Wrote %s\nData directory: %s\n", *cfgPath, loaded.Storage.DataDir)

	if v, err := gitcmd.Init(loaded.Git.Binary); err != nil {
		fmt.Printf("WARNING: %v\n", err)
	} else {
		fmt.Printf("Git %s found.\n", v)
	}
	fmt.Printf("\nNext: anvil serve --config %s\n", *cfgPath)
	return nil
}
