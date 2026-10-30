package config

import (
	"bytes"
	"strconv"
	"strings"
	"text/template"
)

// fileTemplate mirrors docs/reference/11-configuration-and-operations.md §3.
// Keep the comments in sync with that section.
const fileTemplate = `# Anvil server configuration. Reference: docs/reference/11-configuration-and-operations.md
# Every key can be overridden with ANVIL_<SECTION>_<KEY>, e.g. ANVIL_SERVER_HTTP_ADDR.

[server]
http_addr        = {{q .Server.HTTPAddr}}     # main listener
internal_addr    = {{q .Server.InternalAddr}}   # hook API — must stay on loopback
public_url       = {{q .Server.PublicURL}}  # used in clone URLs, emails, runner payloads
tls_cert         = {{q .Server.TLSCert}}                 # set both to serve HTTPS directly
tls_key          = {{q .Server.TLSKey}}
trusted_proxies  = {{list .Server.TrustedProxies}}                 # CIDRs allowed to set X-Forwarded-*
shutdown_timeout = {{text .Server.ShutdownTimeout}}

[storage]
data_dir = {{q .Storage.DataDir}}   # repos/, logs/, blobs/, tmp/, anvil.db (relative to this file)

[database]
driver = {{q .Database.Driver}}                     # "postgres" planned
dsn    = {{q .Database.DSN}}                           # default: <data_dir>/anvil.db

[git]
binary            = {{q .Git.Binary}}             # full path if not on PATH
command_timeout   = {{text .Git.CommandTimeout}}             # UI read commands
max_push_size     = {{text .Git.MaxPushSize}}
maintenance_cron  = {{q .Git.MaintenanceCron}}       # daily git gc --auto

[auth]
allow_registration   = {{.Auth.AllowRegistration}}
require_signin_view  = {{.Auth.RequireSigninView}}          # true = no anonymous browsing
allow_password_git   = {{.Auth.AllowPasswordGit}}          # git over HTTP needs a PAT unless true
session_ttl          = {{text .Auth.SessionTTL}}

[security]
secret_key = {{q .Security.SecretKey}}  # 32-byte base64; keep this file private

[ci]
long_poll_timeout       = {{text .CI.LongPollTimeout}}
default_job_timeout     = {{text .CI.DefaultJobTimeout}}
max_log_size            = {{text .CI.MaxLogSize}}
artifact_retention_days = {{.CI.ArtifactRetentionDays}}
fork_pr_approval        = {{.CI.ForkPRApproval}}
protected_labels        = {{list .CI.ProtectedLabels}}  # only protected refs may target these runner labels

[explorer]
enabled = {{.Explorer.Enabled}}
persist = {{.Explorer.Persist}}                       # keep traces in DB, not only in memory

[log]
level  = {{q .Log.Level}}                       # debug | info | warn | error
format = {{q .Log.Format}}                       # text | json
file   = {{q .Log.File}}                           # empty = stderr

[mail]
enabled = {{.Mail.Enabled}}
smtp_addr = {{q .Mail.SMTPAddr}}
from = {{q .Mail.From}}
`

var tmpl = template.Must(template.New("anvil.toml").Funcs(template.FuncMap{
	"q": strconv.Quote, // Go quoting is valid TOML for basic strings, including Windows paths
	"text": func(v interface{ MarshalText() ([]byte, error) }) (string, error) {
		b, err := v.MarshalText()
		return strconv.Quote(string(b)), err
	},
	"list": func(items []string) string {
		quoted := make([]string, len(items))
		for i, s := range items {
			quoted[i] = strconv.Quote(s)
		}
		return "[" + strings.Join(quoted, ", ") + "]"
	},
}).Parse(fileTemplate))

// Render returns c as a commented anvil.toml. Loading the result gives back an equal
// Config (tested), so `anvil init` can write any Config it builds.
func (c *Config) Render() ([]byte, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, c); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
