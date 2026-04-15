# Configuration Reference

Logtailr supports three configuration methods with the following priority (highest first):

1. **CLI flags** (`--file`, `--level`, `--api`, etc.)
2. **Environment variables** (`LOGTAILR_DB_URL`, `LOGTAILR_API_TOKEN`)
3. **YAML config file** (`--config config.yaml`)
4. **PostgreSQL database** (`--db-url postgres://...`)

When `--db-url` is set, database configuration takes precedence over YAML. Without it, YAML is the source of truth.

---

## YAML Configuration

### Full example

```yaml
sources:
  # File source
  - name: "app-logs"
    type: "file"
    path: "/var/log/app/app.log"
    follow: true
    parser: "json"

  # Docker container
  - name: "nginx"
    type: "docker"
    container: "nginx"
    follow: true
    parser: "text"

  # Systemd journal
  - name: "ssh"
    type: "journalctl"
    unit: "ssh.service"
    priority: "err"           # emerg, alert, crit, err, warning, notice, info, debug
    output_format: "json"     # Structured fields from journald
    follow: true

  # Kubernetes pod
  - name: "k8s-api"
    type: "kubernetes"
    namespace: "production"
    pod: "api-server"
    container: "app"                    # Optional: specific container
    kubeconfig: "~/.kube/config"        # Optional: defaults to kubectl default
    follow: true
    parser: "json"

  # Kubernetes by label selector
  - name: "k8s-workers"
    type: "kubernetes"
    namespace: "production"
    label_selector: "app=worker,version=v2"
    follow: true

global:
  level: "info"                # Minimum log level: debug, info, warn, error, fatal
  regex: ""                    # Regex filter on message (empty = no filter)
  output: "console"            # Default output: console, json, file
  output_path: ""              # File path when output=file
  show_health: true            # Show health status in console
  aggregate: false             # Enable log deduplication
  aggregate_window: "5s"       # Time window for dedup

outputs:
  opensearch:
    enabled: true
    hosts:
      - "https://opensearch.example.com:9200"
    index: "logtailr-logs-%{+YYYY.MM.dd}"
    username: "admin"
    password: "${OPENSEARCH_PASSWORD}"      # Env var substitution
    bulk_size: 500
    flush_interval: "5s"
    max_retries: 3
    template_name: "logtailr"              # Auto-created index template
    dashboards_url: "http://localhost:5601" # Auto-create index pattern

  webhook:
    enabled: true
    url: "https://hooks.slack.com/services/XXX"
    min_level: "error"
    batch_size: 10
    batch_timeout: "30s"

  file:
    path: "/var/log/logtailr/output.log"
    max_size: "50MB"           # Rotate when file exceeds this
    max_age: "168h"            # Delete rotated files older than 7 days
    compress: true             # Gzip rotated files

alerts:
  enabled: true
  default_cooldown: "5m"
  notify:
    console: true
    webhook:
      url: "https://hooks.slack.com/services/XXX"
    email:
      host: "smtp.example.com"
      port: 587
      from: "alerts@example.com"
      to: ["oncall@example.com"]
      username: "alerts@example.com"
      password: "${SMTP_PASSWORD}"
      tls: true
  rules:
    - name: "fatal-errors"
      type: "level"
      severity: "critical"
      level: "fatal"
      cooldown: "10m"

    - name: "oom-pattern"
      type: "pattern"
      severity: "critical"
      pattern: "OutOfMemory|OOM"

    - name: "high-error-rate"
      type: "error_rate"
      severity: "warning"
      threshold: 100
      window: "5m"

    - name: "source-down"
      type: "health_change"
      severity: "critical"
```

---

## Source Types

| Type | Required fields | Optional fields |
|------|----------------|-----------------|
| `file` | `path` | `follow`, `parser` |
| `docker` | `container` | `follow`, `parser` |
| `journalctl` | `unit` | `follow`, `priority`, `output_format`, `parser` |
| `kubernetes` | `pod` or `label_selector` | `namespace`, `container`, `kubeconfig`, `follow`, `parser` |
| `stdin` | (none) | `parser` |

### Parser options
- `json` — Parse JSON logs (`{"timestamp":"...","level":"...","message":"..."}`)
- `logfmt` — Parse logfmt (`time=... level=... msg=...`)
- `text` — Parse plain text with timestamp/level extraction
- (empty) — Auto-detect format per line

---

## Alert Rule Types

| Type | Fields | Description |
|------|--------|-------------|
| `pattern` | `pattern` (regex) | Fires when message matches regex |
| `level` | `level` | Fires when log level >= specified level |
| `error_rate` | `threshold`, `window` | Fires when error count exceeds threshold in time window |
| `health_change` | (none) | Fires when any source changes health status |

All rules accept optional `source` (limit to specific source), `severity` (warning/critical), and `cooldown` (minimum time between fires).

---

## CLI Flags

### `logtailr tail`

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `./config.yaml` | Path to YAML config file |
| `--file` | | Single file to tail (shortcut, no config needed) |
| `--level` | | Minimum log level filter |
| `--regex` | | Regex filter on message |
| `--output` | `console` | Output format: console, json, file |
| `--output-path` | | File path for file output |
| `--follow` | `true` | Follow file for new lines |
| `--parser` | | Force parser: json, logfmt, text |
| `--aggregate` | `false` | Enable log deduplication |
| `--aggregate-window` | `5s` | Dedup time window |
| `--bookmark` | | Save reading position with this name on exit |
| `--resume` | | Resume from saved bookmark |
| `--api` | `false` | Enable REST API + WebSocket server |
| `--api-addr` | `127.0.0.1` | API bind address |
| `--api-port` | `8080` | API port (1024-65535) |
| `--api-token` | | Bearer token for API auth (env: `LOGTAILR_API_TOKEN`) |
| `--web` | `false` | Serve embedded web dashboard |
| `--db-url` | | PostgreSQL URL (env: `LOGTAILR_DB_URL`) |
| `--allow-local` | `false` | Disable SSRF prevention for local dev |
| `--log-format` | `text` | Log output format: text, json |
| `--log-level` | `info` | Internal log level: debug, info, warn, error |

### `logtailr discover`

| Flag | Default | Description |
|------|---------|-------------|
| `--scan` | `all` | Scanners to run: all, file, docker, journalctl |
| `--output` | `table` | Output format: table, yaml |
| `--save` | | Save generated config to file |

### `logtailr migrate`

| Subcommand | Description |
|------------|-------------|
| `up` | Apply pending migrations |
| `down` | Rollback all migrations |
| `version` | Show current schema version |

### `logtailr import`

| Flag | Description |
|------|-------------|
| `--config-file` | YAML file to import into database |

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `LOGTAILR_DB_URL` | PostgreSQL connection URL |
| `LOGTAILR_API_TOKEN` | API authentication token |

---

## PostgreSQL Mode

When `--db-url` is set:

1. Config is loaded from database first, YAML as fallback
2. CRUD API endpoints (`/api/v1/*`) become available
3. ConfigWatcher polls for changes every 5 seconds
4. Sources, outputs, and alert rules hot-reload without restart
5. Alert events are persisted with 30-day retention
6. Web dashboard Config page becomes functional

Import existing YAML config:
```bash
logtailr import --config-file config.yaml --db-url "postgres://..."
```

Valid setting keys for `GET/PUT /api/v1/settings/{key}`:
- `global.level`
- `global.regex`
- `global.output`
- `global.output_path`
- `global.show_health`
- `global.aggregate`
- `global.aggregate_window`
- `alerts.default_cooldown`
- `alerts.notify.console`
- `alerts.notify.webhook.url`
