# Logtailr

Concurrent multi-source log aggregator with real-time web dashboard. Tail, parse, filter, and alert on logs from files, Docker, journalctl, Kubernetes, and stdin.

## Features

- **Multi-source** — Files, Docker, journalctl, Kubernetes pods, stdin (all simultaneous)
- **Multi-format** — JSON, logfmt, plain text with auto-detection
- **Filtering** — By severity level and regex
- **Multiple outputs** — Console, JSON, file (with rotation), OpenSearch, webhooks
- **Alert engine** — Pattern, level, error rate, and health change rules with cooldown
- **Web dashboard** — Real-time log viewer (100k+ virtual scroll), sources, alerts, config management
- **PostgreSQL mode** — DB-backed config with CRUD API and hot-reload
- **Health monitoring** — Per-source status tracking (healthy/degraded/failed/stopped)
- **Prometheus metrics** — Logs processed, source health, WebSocket clients
- **Auto-discovery** — Scan system for log sources and generate config
- **Bookmarks** — Save/resume file reading position
- **Security** — API auth, SSRF prevention, rate limiting, secret masking

## Install

```bash
git clone https://github.com/Mariusrfx/logtailr.git
cd logtailr
make build-all    # Frontend + Go binary (dashboard included)
# Binary at bin/logtailr
```

Requires Go 1.25+ and Node.js 20+ (for dashboard build).

## Quick start

```bash
# Tail a single file
logtailr tail --file /var/log/syslog --level error

# Demo with dashboard (Docker + syslog)
docker compose -f dev/docker-compose.dev.yaml up -d
logtailr tail --config config.example.yaml --api --web
# Open http://localhost:8080

# With PostgreSQL (persistent config + CRUD API)
logtailr migrate up --db-url "postgres://user:pass@localhost:5432/logtailr"
logtailr tail --api --web --db-url "postgres://..."
```

## Documentation

| Document | Description |
|----------|-------------|
| [Architecture](docs/architecture.md) | Component map, data flow, interfaces, concurrency model |
| [Configuration](docs/configuration.md) | Full YAML reference, CLI flags, environment variables |
| [Deployment](docs/deployment.md) | Docker, systemd, Kubernetes, Nginx reverse proxy |
| [Contributing](CONTRIBUTING.md) | Development setup, code conventions |
| [OpenAPI Spec](api/openapi.json) | REST API specification (OpenAPI 3.1) |

## Project structure

```
logtailr/
├── cmd/                     # CLI commands (Cobra)
├── internal/
│   ├── api/                 # REST API, WebSocket, Prometheus metrics
│   ├── alert/               # Alert engine, notifiers, rate limiting
│   ├── store/               # PostgreSQL layer, migrations
│   ├── tailer/              # File, Docker, journalctl, K8s, stdin
│   ├── output/              # Console, file, OpenSearch, webhook writers
│   ├── config/              # YAML + DB config loading, validation
│   ├── configwatch/         # Hot-reload via DB polling
│   ├── health/              # Source health monitoring
│   ├── aggregator/          # Log deduplication
│   ├── discovery/           # Auto-discovery scanners
│   └── parser/              # JSON, logfmt, text parsers
├── web/                     # React dashboard (Vite + TypeScript + Tailwind)
├── pkg/logline/             # Core types
├── docs/                    # Architecture, configuration, deployment guides
├── dev/                     # Docker compose for demo
└── api/openapi.json         # OpenAPI spec
```

## License

[MIT](LICENSE)
