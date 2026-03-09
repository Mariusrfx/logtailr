# Logtailr

> **Status: In active development** Not yet recommended for production use.

Concurrent multi-source log aggregator. Tail, parse, and filter logs from files, Docker containers, journalctl, and stdin simultaneously. Send to console, files, OpenSearch, or webhooks.

## Features

- **Multi-source tailing** — Files, Docker containers (with auto-reconnect), journalctl units, stdin pipes
- **Multi-format parser** — JSON, logfmt, and plain text with auto-detection
- **Level filtering** — Filter by severity: `debug < info < warn < error < fatal`
- **Regex filtering** — Match log messages with regular expressions
- **Real-time file tailing** — Follow files with fsnotify, handles log rotation
- **Multiple outputs** — Console (colored), JSON (NDJSON), file (with size rotation, compression, and age cleanup), OpenSearch/Elasticsearch, webhooks
- **Health monitoring** — Track source status (healthy/degraded/failed/stopped)
- **REST API** — Health endpoints, config inspection, Prometheus metrics
- **WebSocket streaming** — Real-time log streaming with level/source filters
- **Prometheus metrics** — `logtailr_logs_total`, source health gauges, WebSocket client count
- **YAML config** — Define multiple sources and outputs with a single config file
- **Alert engine** — Rule-based alerts on patterns, log levels, error rates, and health changes with per-rule cooldown and webhook/console notifications
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
| `docker` | `container` | Docker container logs via `docker logs`, auto-reconnects on restart |
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

With rotation via config file:

```yaml
outputs:
  file:
    path: "/var/log/logtailr/output.log"
    max_size: "50MB"     # Rotate when file exceeds this size
    max_age: "168h"      # Delete rotated files older than 7 days
    compress: true        # Gzip rotated files
```

Rotated files are named with a timestamp: `output.log.2026-03-09T10-30-00.gz`.

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

## API & Monitoring

Enable the API server with `--api`:

```bash
logtailr tail --config config.yaml --api --api-port 8080
```

### REST endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /health` | Overall health status (healthy/degraded/unhealthy) |
| `GET /health/sources` | All sources with status, error count, uptime |
| `GET /health/sources/:name` | Single source detail |
| `GET /config` | Current config (secrets redacted) |
| `GET /alerts` | Recent alert events with severity and source |
| `GET /alerts/rules` | Configured alert rules with fire count and last fired |
| `GET /metrics` | Prometheus metrics |

### WebSocket

Stream logs in real-time:

```javascript
const ws = new WebSocket('ws://localhost:8080/ws/logs');
ws.onmessage = (event) => {
  const log = JSON.parse(event.data);
  console.log(`[${log.level}] ${log.message}`);
};

// Filter by level and source
const ws = new WebSocket('ws://localhost:8080/ws/logs?level=error&source=app.log');
```

### Alerts

Define rules to trigger alerts on specific conditions. Alerts support per-rule cooldown to prevent spam and can notify via console (stderr) or webhook:

```yaml
alerts:
  enabled: true
  default_cooldown: "5m"
  notify:
    console: true
    webhook:
      url: "https://hooks.slack.com/services/XXX"
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

| Rule type | Description | Required fields |
|-----------|-------------|-----------------|
| `pattern` | Regex match on log message | `pattern` |
| `level` | Fires when log level >= threshold | `level` |
| `error_rate` | Fires when errors exceed count in sliding window | `threshold`, `window` |
| `health_change` | Fires when a source changes health status | — |

### Prometheus metrics

```
logtailr_logs_total{source="app.log",level="error"} 150
logtailr_source_healthy{source="app.log",status="healthy"} 1
logtailr_source_errors_total{source="app.log"} 2
logtailr_alerts_total{rule="fatal-errors",severity="critical"} 3
logtailr_active_sources 4
logtailr_websocket_clients 2
```

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

## OpenAPI spec

The API is documented in [api/openapi.json](api/openapi.json) (OpenAPI 3.1). Use it to generate typed clients for the frontend or other consumers.

## Project structure

```
logtailr/
├── api/
│   └── openapi.json        # OpenAPI 3.1 spec
├── cmd/                    # CLI commands (cobra)
│   ├── alerts.go
│   ├── root.go
│   └── tail.go
├── internal/
│   ├── alert/              # Alert engine, rules, notifiers, rate limiting
│   ├── api/                # REST API, Prometheus metrics, WebSocket hub
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