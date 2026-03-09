# Logtailr

> **Status: In active development** Not yet recommended for production use.

Concurrent multi-source log aggregator. Tail, parse, and filter logs from files, Docker containers, journalctl, and stdin simultaneously. Send to console, files, OpenSearch, or webhooks.

## Features

- **Multi-source tailing** — Files, Docker containers, journalctl units, stdin pipes
- **Multi-format parser** — JSON, logfmt, and plain text with auto-detection
- **Level filtering** — Filter by severity: `debug < info < warn < error < fatal`
- **Regex filtering** — Match log messages with regular expressions
- **Real-time file tailing** — Follow files with fsnotify, handles log rotation
- **Multiple outputs** — Console (colored), JSON (NDJSON), file, OpenSearch/Elasticsearch, webhooks
- **Health monitoring** — Track source status (healthy/degraded/failed/stopped)
- **YAML config** — Define multiple sources and outputs with a single config file
- **Security hardened** — Input validation, SSRF prevention, command injection protection, TLS 1.2+, path traversal prevention

## Install

```bash
git clone https://github.com/Mariusrfx/logtailr.git
cd logtailr
make build
```

The binary will be at `bin/logtailr`.

## Quick start

### Tail a single file

```bash
logtailr tail --file /var/log/app.log
logtailr tail --file /var/log/app.log --level error
logtailr tail --file /var/log/app.log --regex "timeout|connection refused"
logtailr tail --file /var/log/app.log --show-health --health-every 10
```

### Pipe from stdin

```bash
cat /var/log/app.log | logtailr tail --level error
kubectl logs -f my-pod | logtailr tail --regex "ERROR|WARN"
docker logs -f my-container | logtailr tail --output json
```

### Multi-source with config file

```yaml
# config.yaml
sources:
  - name: "app-logs"
    type: "file"
    path: "/var/log/app/app.log"
    follow: true
    parser: "json"

  - name: "nginx-container"
    type: "docker"
    container: "nginx"
    follow: true

  - name: "ssh-service"
    type: "journalctl"
    unit: "ssh.service"
    follow: true

global:
  level: "info"
  output: "console"
  show_health: true
```

```bash
logtailr tail --config config.yaml
```

## Source types

| Type | Config field | Description |
|------|-------------|-------------|
| `file` | `path` | Local log file, supports follow and log rotation |
| `docker` | `container` | Docker container logs via `docker logs` |
| `journalctl` | `unit` | Systemd journal via `journalctl -u` |
| `stdin` | — | Read from pipe (auto-detected or via config) |

## Output formats

### Console (default)

Colored output by severity level:

```
[2024-01-15 10:30:00] [app.log] ERROR: Connection failed to database
[2024-01-15 10:30:01] [app.log] INFO: Retrying connection...
```

Colors: debug=dim, info=default, warn=yellow, error=red, fatal=red+bold.

### JSON

```bash
logtailr tail --file app.log --output json
```

Outputs one JSON object per line (NDJSON), suitable for piping to `jq`.

### File

```bash
logtailr tail --file app.log --output file --output-path errors.log
```

### OpenSearch / Elasticsearch

Send logs directly to OpenSearch with bulk insert, retry, and date-based indices:

```yaml
outputs:
  opensearch:
    enabled: true
    hosts:
      - "https://opensearch.example.com:9200"
    index: "logtailr-logs-%{+YYYY.MM.dd}"
    username: "admin"
    password: "${OPENSEARCH_PASSWORD}"
    bulk_size: 500
    flush_interval: "5s"
    max_retries: 3
```

### Webhook

Send batched log alerts to Slack, Discord, or any HTTP endpoint:

```yaml
outputs:
  webhook:
    enabled: true
    url: "https://hooks.slack.com/services/XXX"
    min_level: "error"
    batch_size: 10
    batch_timeout: "30s"
```

Multiple outputs can be active simultaneously (e.g. console + OpenSearch + webhook).

## Supported log formats

### JSON

```json
{"timestamp":"2024-01-15T10:30:00Z","level":"error","message":"Connection failed"}
```

### Logfmt

```
time=2024-01-15T10:30:00Z level=error msg="Connection failed"
```

### Plain text

```
[2024-01-15 10:30:00] ERROR: Connection failed
```

All formats are auto-detected if no parser is specified.

## Development

```bash
make build    # Compile binary
make test     # Run tests with race detector
make vet      # Run go vet
make lint     # Run govulncheck
make clean    # Remove build artifacts
make help     # Show all targets
```

## Project structure

```
logtailr/
├── cmd/                    # CLI commands (cobra)
│   ├── root.go
│   └── tail.go
├── internal/
│   ├── config/             # YAML config loader + validation
│   ├── filter/             # Level and regex filtering
│   ├── health/             # Source health monitoring
│   ├── output/             # Console, JSON, file, OpenSearch, webhook writers
│   ├── parser/             # JSON, logfmt, text parsers
│   └── tailer/             # File, Docker, journalctl, stdin tailers
├── pkg/logline/            # Core types (LogLine, SourceConfig)
├── Makefile
└── config.yaml             # Example config
```

## License

[MIT](LICENSE)