package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"anvil/internal/errs"
)

// Config is the complete, validated server configuration. See docs/reference/11 §3 for every key.
type Config struct {
	Server   ServerConfig   `toml:"server"`
	Storage  StorageConfig  `toml:"storage"`
	Database DatabaseConfig `toml:"database"`
	Git      GitConfig      `toml:"git"`
	Auth     AuthConfig     `toml:"auth"`
	Security SecurityConfig `toml:"security"`
	CI       CIConfig       `toml:"ci"`
	Explorer ExplorerConfig `toml:"explorer"`
	Log      LogConfig      `toml:"log"`
	Mail     MailConfig     `toml:"mail"`
}

// ServerConfig controls the HTTP listeners.
type ServerConfig struct {
	HTTPAddr        string   `toml:"http_addr"`        // main listener, host:port
	InternalAddr    string   `toml:"internal_addr"`    // hook API; must be a loopback address
	PublicURL       string   `toml:"public_url"`       // base URL used in clone URLs, emails and runner payloads
	TLSCert         string   `toml:"tls_cert"`         // PEM certificate file; set together with TLSKey
	TLSKey          string   `toml:"tls_key"`          // PEM private key file
	TrustedProxies  []string `toml:"trusted_proxies"`  // CIDRs allowed to set X-Forwarded-*
	ShutdownTimeout Duration `toml:"shutdown_timeout"` // how long to wait for in-flight requests on stop
}

// StorageConfig says where Anvil keeps its files.
type StorageConfig struct {
	// DataDir holds repos/, logs/, blobs/, tmp/ and anvil.db. A relative path is
	// resolved against the directory of the config file, not the working directory,
	// so a service started from C:\Windows\System32 still finds its data.
	DataDir string `toml:"data_dir"`
}

// DatabaseConfig selects the SQL database.
type DatabaseConfig struct {
	Driver string `toml:"driver"` // only "sqlite" in 1.0
	DSN    string `toml:"dsn"`    // empty means <data_dir>/anvil.db
}

// GitConfig configures how the git binary is used.
type GitConfig struct {
	Binary          string   `toml:"binary"`           // "git" or a full path
	CommandTimeout  Duration `toml:"command_timeout"`  // limit for git commands run by the web UI
	MaxPushSize     ByteSize `toml:"max_push_size"`    // largest pack accepted by receive-pack
	MaintenanceCron string   `toml:"maintenance_cron"` // when to run git gc --auto, cron syntax
}

// AuthConfig sets sign-in policy.
type AuthConfig struct {
	AllowRegistration bool     `toml:"allow_registration"`  // anyone may create an account
	RequireSigninView bool     `toml:"require_signin_view"` // no anonymous browsing
	AllowPasswordGit  bool     `toml:"allow_password_git"`  // accept passwords (not only PATs) for git over HTTP
	SessionTTL        Duration `toml:"session_ttl"`         // browser session lifetime
}

// SecurityConfig holds key material.
type SecurityConfig struct {
	// SecretKey is 32 random bytes, base64-encoded, used to encrypt secrets and sign
	// tokens. When empty the server reads <data_dir>/secret.key instead.
	SecretKey string `toml:"secret_key"`
}

// CIConfig tunes pipelines and runners.
type CIConfig struct {
	LongPollTimeout       Duration `toml:"long_poll_timeout"`       // how long a runner's fetch-job request may wait
	DefaultJobTimeout     Duration `toml:"default_job_timeout"`     // job timeout when the workflow sets none
	MaxLogSize            ByteSize `toml:"max_log_size"`            // log size at which a job's log is truncated
	ArtifactRetentionDays int      `toml:"artifact_retention_days"` // default artifact lifetime
	ForkPRApproval        bool     `toml:"fork_pr_approval"`        // pipelines from forks wait for approval
	ProtectedLabels       []string `toml:"protected_labels"`        // runner labels only protected refs may target
}

// ExplorerConfig controls the Architecture Explorer.
type ExplorerConfig struct {
	Enabled bool `toml:"enabled"` // record spans and serve /api/v1/explorer
	Persist bool `toml:"persist"` // also keep traces in the database
}

// LogConfig controls server logging.
type LogConfig struct {
	Level  string `toml:"level"`  // debug | info | warn | error
	Format string `toml:"format"` // text | json
	File   string `toml:"file"`   // empty means stderr
}

// MailConfig configures outgoing email.
type MailConfig struct {
	Enabled  bool   `toml:"enabled"`
	SMTPAddr string `toml:"smtp_addr"` // host:port
	From     string `toml:"from"`      // sender address
}

// Default returns a Config with every default value set. Used by `anvil init` to write a
// commented file, and as the base that Load decodes the file over.
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			HTTPAddr:        "0.0.0.0:3000",
			InternalAddr:    "127.0.0.1:3001",
			PublicURL:       "http://localhost:3000",
			TrustedProxies:  []string{},
			ShutdownTimeout: Duration{30 * time.Second},
		},
		Storage:  StorageConfig{DataDir: "./data"},
		Database: DatabaseConfig{Driver: "sqlite"},
		Git: GitConfig{
			Binary:          "git",
			CommandTimeout:  Duration{60 * time.Second},
			MaxPushSize:     2 << 30,
			MaintenanceCron: "0 3 * * *",
		},
		Auth: AuthConfig{SessionTTL: Duration{336 * time.Hour}},
		CI: CIConfig{
			LongPollTimeout:       Duration{30 * time.Second},
			DefaultJobTimeout:     Duration{time.Hour},
			MaxLogSize:            50 << 20,
			ArtifactRetentionDays: 30,
			ForkPRApproval:        true,
			ProtectedLabels:       []string{"deploy"},
		},
		Explorer: ExplorerConfig{Enabled: true},
		Log:      LogConfig{Level: "info", Format: "text"},
	}
}

// Load reads the TOML file at path, applies ANVIL_* environment overrides, fills defaults,
// resolves a relative data_dir against the file's directory, and validates the result.
//
// It returns an errs.Invalid error (errs.FieldErrors) describing every unknown or invalid
// key, not just the first. A missing file is reported as an error wrapping os.ErrNotExist.
func Load(path string) (*Config, error) {
	cfg := Default()
	md, err := toml.DecodeFile(path, cfg)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("config file %s: %w (create one with `anvil init`)", path, err)
		}
		return nil, fmt.Errorf("config file %s: %w", path, err)
	}
	problems := errs.FieldErrors{}
	for _, k := range md.Undecoded() {
		problems[k.String()] = "unknown key"
	}
	for k, v := range applyEnv(cfg, os.Environ()) {
		problems[k] = v
	}
	if !filepath.IsAbs(cfg.Storage.DataDir) {
		abs, err := filepath.Abs(filepath.Join(filepath.Dir(path), cfg.Storage.DataDir))
		if err != nil {
			return nil, fmt.Errorf("resolve storage.data_dir: %w", err)
		}
		cfg.Storage.DataDir = abs
	}
	if err := cfg.Validate(); err != nil {
		var fe errs.FieldErrors
		if errors.As(err, &fe) {
			for k, v := range fe {
				problems[k] = v
			}
		} else {
			return nil, err
		}
	}
	if err := problems.OrNil(); err != nil {
		return nil, fmt.Errorf("config file %s: %w", path, err)
	}
	return cfg, nil
}

// Validate checks single-field and cross-field rules (internal_addr must be loopback,
// TLS cert and key set together, enums, positive durations). It returns errs.FieldErrors
// keyed by "section.key", or nil.
func (c *Config) Validate() error {
	p := errs.FieldErrors{}
	if _, _, err := net.SplitHostPort(c.Server.HTTPAddr); err != nil {
		p["server.http_addr"] = "must be host:port"
	}
	if host, _, err := net.SplitHostPort(c.Server.InternalAddr); err != nil {
		p["server.internal_addr"] = "must be host:port"
	} else if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		// The hook API trusts its callers; exposing it would let anyone forge pushes (docs/reference/10-security.md).
		p["server.internal_addr"] = "must be a loopback address such as 127.0.0.1:3001"
	}
	if u, err := url.Parse(c.Server.PublicURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		p["server.public_url"] = "must be an absolute http(s) URL"
	}
	if (c.Server.TLSCert == "") != (c.Server.TLSKey == "") {
		p["server.tls_cert"] = "tls_cert and tls_key must be set together"
	}
	for _, cidr := range c.Server.TrustedProxies {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			p["server.trusted_proxies"] = fmt.Sprintf("%q is not a CIDR", cidr)
		}
	}
	if c.Storage.DataDir == "" {
		p["storage.data_dir"] = "must not be empty"
	}
	if c.Database.Driver != "sqlite" {
		p["database.driver"] = `only "sqlite" is supported`
	}
	if c.Git.Binary == "" {
		p["git.binary"] = "must not be empty"
	}
	if c.Security.SecretKey != "" {
		if b, err := base64.StdEncoding.DecodeString(c.Security.SecretKey); err != nil || len(b) != 32 {
			p["security.secret_key"] = "must be 32 bytes, base64-encoded"
		}
	}
	positive := map[string]Duration{
		"server.shutdown_timeout": c.Server.ShutdownTimeout,
		"git.command_timeout":     c.Git.CommandTimeout,
		"auth.session_ttl":        c.Auth.SessionTTL,
		"ci.long_poll_timeout":    c.CI.LongPollTimeout,
		"ci.default_job_timeout":  c.CI.DefaultJobTimeout,
	}
	for k, d := range positive {
		if d.Duration <= 0 {
			p[k] = "must be greater than zero"
		}
	}
	if c.CI.ArtifactRetentionDays < 1 {
		p["ci.artifact_retention_days"] = "must be at least 1"
	}
	switch strings.ToLower(c.Log.Level) {
	case "debug", "info", "warn", "error":
	default:
		p["log.level"] = "must be debug, info, warn or error"
	}
	if c.Log.Format != "text" && c.Log.Format != "json" {
		p["log.format"] = "must be text or json"
	}
	if c.Mail.Enabled && (c.Mail.SMTPAddr == "" || c.Mail.From == "") {
		p["mail.smtp_addr"] = "smtp_addr and from are required when mail is enabled"
	}
	return p.OrNil()
}

// DatabasePath returns the SQLite file path: database.dsn if set, otherwise <data_dir>/anvil.db.
func (c *Config) DatabasePath() string {
	if c.Database.DSN != "" {
		return c.Database.DSN
	}
	return filepath.Join(c.Storage.DataDir, "anvil.db")
}
