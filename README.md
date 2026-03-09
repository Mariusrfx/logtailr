# Logtailr

> **Status: In active development** — Core functionality (Phase 1) is complete. Not yet recommended for production use.

Concurrent multi-source log aggregator. Tail, parse, and filter logs from files, Docker, and journalctl simultaneously.

## Features

- **Multi-format parser** — JSON, logfmt, and plain text with auto-detection
- **Level filtering** — Filter by severity: `debug < info < warn < error < fatal`
- **Regex filtering** — Match log messages with regular expressions
- **Real-time file tailing** — Follow files with fsnotify, handles log rotation
- **Multiple outputs** — Console (colored), JSON (NDJSON), file
- **Health monitoring** — Track source status (healthy/degraded/failed/stopped)
- **YAML config** — Define multiple sources with a single config file

## Install

```bash
git clone https://github.com/Mariusrfx/logtailr.git
cd logtailr
make build
```

The binary will be at `bin/logtailr`.

## Quick start

### Tail a file

```bash
# Basic tail
logtailr tail --file /var/log/app.log

# Filter by level (only errors and fatal)
logtailr tail --file /var/log/app.log --level error

# Filter by regex
logtailr tail --file /var/log/app.log --regex "timeout|connection refused"

# Combine level + regex
logtailr tail --file /var/log/app.log --level warn --regex "database"

# Show health status
logtailr tail --file /var/log/app.log --show-health --health-every 10
```

### Config file

Create a `config.yaml`:

```yaml
sources:
  - name: "app-logs"
    type: "file"
    path: "/var/log/app/app.log"
    follow: true
    parser: "json"

  - name: "nginx"
    type: "file"
    path: "/var/log/nginx/access.log"
    parser: "text"

global:
  level: "info"
  output: "console"
  show_health: true
```

```bash
logtailr tail --config config.yaml
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
│   ├── output/             # Console, JSON, file writers
│   ├── parser/             # JSON, logfmt, text parsers
│   └── tailer/             # File tailer with fsnotify
├── pkg/logline/            # Core types (LogLine, SourceConfig)
├── Makefile
└── config.yaml             # Example config
```

## License

[MIT](LICENSE)