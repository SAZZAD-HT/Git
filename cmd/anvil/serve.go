package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"anvil/internal/api"
	"anvil/internal/auth"
	"anvil/internal/config"
	"anvil/internal/events"
	"anvil/internal/git/gitcmd"
	"anvil/internal/git/hooks"
	"anvil/internal/git/smarthttp"
	"anvil/internal/httpserver"
	"anvil/internal/logging"
	"anvil/internal/queue"
	"anvil/internal/repos"
	"anvil/internal/service"
	"anvil/internal/store"
	"anvil/internal/users"
	"anvil/internal/version"
	"anvil/web"
)

// serviceName is the Windows service name and the systemd unit name.
const serviceName = "Anvil"

// dataSubdirs are created under storage.data_dir at startup.
var dataSubdirs = []string{"repos", "logs", "blobs", "tmp"}

func runServe(ctx context.Context, args []string) error {
	fs, cfgPath := newFlags("serve")
	if err := fs.Parse(args); err != nil {
		return err
	}
	main := func(ctx context.Context) error { return serve(ctx, *cfgPath) }
	if handled, err := service.RunAsServiceIfNeeded(serviceName, main); handled {
		return err
	}
	return main(ctx)
}

// serve is the composition root. It follows the startup sequence in
// docs/reference/03-architecture-overview.md §4.3 and returns when ctx is cancelled.
func serve(ctx context.Context, cfgPath string) error {
	start := time.Now()

	// 1. Configuration and logging.
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	logger, logCloser, err := logging.New(cfg.Log)
	if err != nil {
		return err
	}
	defer logCloser.Close()
	ctx = logging.WithContext(ctx, logger)
	logger.Info("starting", "version", version.Get().String(), "config", cfgPath, "data_dir", cfg.Storage.DataDir)

	// 2. Git.
	gitVersion, err := gitcmd.Init(cfg.Git.Binary)
	if err != nil {
		return err
	}

	// 3. Database and migrations (FR-ADM-08).
	if err := ensureDataDirs(cfg.Storage.DataDir); err != nil {
		return err
	}
	db, err := store.Open(ctx, cfg.Database, cfg.Storage.DataDir)
	if err != nil {
		return err
	}
	defer db.Close()
	applied, err := db.Migrate(ctx, false)
	if err != nil {
		return err
	}
	if len(applied) > 0 {
		logger.Info("database migrated", "applied", applied)
	}
	if _, err := loadSecretKey(cfg); err != nil {
		return err
	}

	// 4. (P3) Hook scripts are checked and reinstalled here.

	// 5. Background machinery.
	bus := events.NewBus(db)
	authSvc, usersSvc := newIdentityServices(cfg, db, bus)
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	reposSvc := repos.NewService(repos.Deps{
		DB: db, Authorizer: auth.NewAuthorizer(db, cfg.Auth.RequireSigninView), Bus: bus,
		Hooks: hooks.Installer{AnvilBinary: exe}, ReposDir: filepath.Join(cfg.Storage.DataDir, "repos"),
	})
	// 4. Keep every repository's hook scripts pointing at this binary (it may have moved or been upgraded).
	if n, err := reposSvc.EnsureHooks(ctx, exe+" "+version.Get().Version, false); err != nil {
		logger.Warn("some repository hooks could not be reinstalled; run `anvil admin repair-hooks`", "err", err)
	} else if n > 0 {
		logger.Info("reinstalled repository hooks", "repositories", n)
	}
	// The hook API trusts only callers that know this per-process secret (docs/reference/04 §3.1).
	internalToken, err := randomHex(32)
	if err != nil {
		return err
	}
	go pruneSessions(ctx, authSvc)
	q := queue.New(db, 10*time.Minute)
	workCtx, stopWork := context.WithCancel(ctx)
	waitWorkers := q.Start(workCtx, 4)
	defer func() { stopWork(); waitWorkers() }()

	// 6. HTTP.
	srv := httpserver.New(httpserver.Deps{
		Config: cfg,
		Logger: logger,
		Checkers: []httpserver.ReadyChecker{
			dbCheck{db},
			gitCheck{},
		},
		API: api.Deps{
			GitVersion:        gitVersion.String(),
			Auth:              authSvc,
			Users:             usersSvc,
			Repos:             reposSvc,
			PublicURL:         cfg.Server.PublicURL,
			AllowRegistration: cfg.Auth.AllowRegistration,
			SecureCookies:     strings.HasPrefix(cfg.Server.PublicURL, "https://"),
			SessionTTL:        cfg.Auth.SessionTTL.Duration,
			ClientIP:          httpserver.ClientIP(cfg.Server.TrustedProxies),
		},
		Auth: authSvc,
		Git: smarthttp.New(smarthttp.Deps{
			Repos: reposSvc, InternalURL: "http://" + cfg.Server.InternalAddr, InternalToken: internalToken,
			MaxPushSize: int64(cfg.Git.MaxPushSize), WriteError: api.WriteError,
		}),
		Hooks: hooks.InternalAPI(hooks.Deps{Repos: reposSvc, Token: internalToken, PublicURL: cfg.Server.PublicURL}),
		SPA:   web.Dist(),
	})
	mainLn, internalLn, err := srv.Listen()
	if err != nil {
		return err
	}
	logger.Info("listening", "addr", mainLn.Addr().String(), "internal_addr", internalLn.Addr().String(),
		"public_url", cfg.Server.PublicURL, "git", gitVersion.String(), "startup_ms", time.Since(start).Milliseconds())

	// 7. Run until Ctrl+C, SIGTERM or service stop, then shut down gracefully.
	err = srv.Serve(ctx, mainLn, internalLn)
	logger.Info("stopped")
	return err
}

// newIdentityServices builds the auth and users services. Shared by serve and the admin CLI.
// randomHex returns n random bytes as hex.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func newIdentityServices(cfg *config.Config, db *store.DB, bus *events.Bus) (*auth.Service, *users.Service) {
	a := auth.NewService(db, bus, auth.Config{SessionTTL: cfg.Auth.SessionTTL.Duration, AllowPasswordGit: cfg.Auth.AllowPasswordGit})
	return a, users.NewService(db, a, bus, users.Config{AllowRegistration: cfg.Auth.AllowRegistration})
}

// pruneSessions deletes expired sessions every hour until ctx ends.
func pruneSessions(ctx context.Context, a *auth.Service) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		if n, err := a.PruneSessions(ctx); err != nil {
			logging.FromContext(ctx).Warn("prune sessions", "err", err)
		} else if n > 0 {
			logging.FromContext(ctx).Info("pruned expired sessions", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func ensureDataDirs(dataDir string) error {
	for _, d := range append([]string{""}, dataSubdirs...) {
		if err := os.MkdirAll(filepath.Join(dataDir, d), 0o750); err != nil {
			return fmt.Errorf("create data directory: %w", err)
		}
	}
	return nil
}

// loadSecretKey returns security.secret_key, falling back to <data_dir>/secret.key.
func loadSecretKey(cfg *config.Config) (string, error) {
	if cfg.Security.SecretKey != "" {
		return cfg.Security.SecretKey, nil
	}
	b, err := os.ReadFile(filepath.Join(cfg.Storage.DataDir, "secret.key"))
	if errors.Is(err, os.ErrNotExist) {
		return "", errors.New("no secret key: set security.secret_key or run `anvil init` to create <data_dir>/secret.key")
	}
	return string(b), err
}

// dbCheck reports ready when the database answers and no migration is pending.
type dbCheck struct{ db *store.DB }

func (dbCheck) Name() string { return "database" }

func (c dbCheck) Ready(ctx context.Context) error {
	if err := c.db.Ping(ctx); err != nil {
		return err
	}
	pending, err := c.db.PendingMigrations(ctx)
	if err != nil {
		return err
	}
	if len(pending) > 0 {
		return fmt.Errorf("pending migrations: %v", pending)
	}
	return nil
}

// gitCheck reports ready when gitcmd.Init has found a usable git.
type gitCheck struct{}

func (gitCheck) Name() string { return "git" }

func (gitCheck) Ready(context.Context) error {
	if _, _, ok := gitcmd.Installed(); !ok {
		return errors.New("git not initialised")
	}
	return nil
}
