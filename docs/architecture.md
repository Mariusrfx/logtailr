# Architecture

## Overview

Logtailr follows a pipeline architecture where log lines flow from sources through processing stages to outputs. Each stage is decoupled via Go interfaces and channels.

```
                     +-------------------+
                     |   Configuration   |
                     | YAML / PostgreSQL |
                     +--------+----------+
                              |
              +---------------+---------------+
              |               |               |
              v               v               v
        +-----------+   +-----------+   +-----------+
        |  Sources  |   |  Outputs  |   |   Alerts  |
        |  (config) |   |  (config) |   |  (rules)  |
        +-----------+   +-----------+   +-----------+
              |               |               |
              v               |               |
  +-----+-----+-----+-----+  |               |
  |     |     |     |     |  |               |
  v     v     v     v     v  |               |
+---+ +---+ +---+ +---+ +---+               |
|Fil| |Doc| |Jou| |K8s| |Std|               |
|e  | |ker| |rna| |   | |in |               |
+---+ +---+ +---+ +---+ +---+               |
  |     |     |     |     |                  |
  +-----+-----+-----+-----+                 |
              |                              |
              v                              |
       +------+------+                       |
       |   logChan   | (buffered channel)    |
       +------+------+                       |
              |                              |
              v                              |
       +------+------+                       |
       |   Pipeline  |                       |
       |             |                       |
       | 1. Parse    |                       |
       | 2. Metrics  +-----> Prometheus      |
       | 3. Alert    +-----> Alert Engine <--+
       | 4. Filter   |       (async eval)
       | 5. Aggregate|
       | 6. Write    +-----> Writers (console, file, OpenSearch, webhook)
       | 7. Broadcast+-----> WebSocket Hub ---> React Dashboard
       +-------------+
```

## Component Map

```
logtailr/
├── cmd/                        # CLI commands (Cobra)
│   ├── root.go                 # Root command, flags, config init
│   ├── tail.go                 # Main command: wires all components
│   ├── pipeline.go             # Processing loop (parse→filter→write)
│   ├── tailer_manager.go       # Hot-reload: add/remove/restart tailers
│   ├── output_manager.go       # Hot-reload: swap output writers
│   ├── alerts.go               # Alert config wiring
│   ├── discover.go             # Auto-discovery command
│   ├── import.go               # YAML→DB import command
│   └── migrate.go              # Database migration commands
│
├── internal/
│   ├── tailer/                 # Log source implementations
│   │   ├── tailer.go           # Tailer interface + BaseTailer
│   │   ├── file_tailer.go      # File tailing with fsnotify
│   │   ├── docker_tailer.go    # Docker container logs
│   │   ├── journalctl_tailer.go# Systemd journal
│   │   ├── kubernetes_tailer.go# Kubernetes pod logs
│   │   └── stdin_tailer.go     # Stdin pipe
│   │
│   ├── parser/                 # Log format parsers
│   │   └── parser.go           # JSON, logfmt, text + auto-detect
│   │
│   ├── filter/                 # Log filtering
│   │   └── filter.go           # Level + regex filters
│   │
│   ├── output/                 # Output writers
│   │   ├── output.go           # Writer interface + ConsoleWriter
│   │   ├── file_writer.go      # File with rotation + compression
│   │   ├── opensearch_writer.go# OpenSearch bulk insert
│   │   └── webhook_writer.go   # HTTP webhook batching
│   │
│   ├── alert/                  # Alert engine
│   │   ├── engine.go           # Rule evaluation + event firing
│   │   ├── notifier.go         # Console + webhook + email notifiers
│   │   ├── email_notifier.go   # SMTP email notifications
│   │   └── ratelimit.go        # Per-rule cooldown
│   │
│   ├── api/                    # HTTP server
│   │   ├── server.go           # Server setup, route registration
│   │   ├── middleware.go       # Auth, CORS, rate limit, headers
│   │   ├── hub.go              # WebSocket hub (broadcast)
│   │   ├── websocket.go        # WS connection handling
│   │   ├── metrics.go          # Prometheus metrics
│   │   ├── handlers_*.go       # CRUD handlers for each entity
│   │   └── helpers.go          # Validation, JSON writing
│   │
│   ├── store/                  # PostgreSQL data layer
│   │   ├── store.go            # Connection pool + migrations
│   │   ├── sources.go          # Source CRUD
│   │   ├── outputs.go          # Output CRUD
│   │   ├── alert_rules.go      # Alert rule CRUD
│   │   ├── alert_events.go     # Alert event queries
│   │   ├── settings.go         # Key-value settings
│   │   ├── saved_searches.go   # Saved filter presets
│   │   ├── bookmarks.go        # File position bookmarks
│   │   └── migrations/         # Embedded SQL migrations
│   │
│   ├── config/                 # Configuration
│   │   ├── config.go           # YAML loading + validation
│   │   └── loader.go           # DB config loading + import
│   │
│   ├── configwatch/            # Hot-reload
│   │   └── configwatch.go      # DB polling + change notifications
│   │
│   ├── health/                 # Health monitoring
│   │   └── health.go           # Status tracking per source
│   │
│   ├── aggregator/             # Log deduplication
│   │   └── aggregator.go       # Sliding window counter
│   │
│   ├── discovery/              # Auto-discovery
│   │   └── discovery.go        # File, Docker, journalctl scanners
│   │
│   ├── bookmark/               # Position bookmarks
│   │   └── bookmark.go         # Save/load file offsets
│   │
│   ├── logger/                 # Structured logging
│   │   └── logger.go           # slog setup (text/json)
│   │
│   ├── safego/                 # Goroutine safety
│   │   └── safego.go           # Panic recovery wrapper
│   │
│   └── web/                    # Embedded frontend
│       └── embed.go            # //go:embed for static assets
│
├── web/                        # React frontend
│   ├── src/
│   │   ├── components/
│   │   │   ├── layout/         # Sidebar, Header, CommandPalette
│   │   │   ├── dashboard/      # Overview, StatsCard, SourceHealth
│   │   │   ├── logs/           # LogViewer, LogRow, LogDetail, Filters
│   │   │   ├── sources/        # SourceList, SourceCard, SourceDetail
│   │   │   ├── alerts/         # AlertsPage
│   │   │   ├── config/         # ConfigPage, CRUD tabs, modals
│   │   │   └── ui/             # Skeleton components
│   │   ├── hooks/              # useWebSocket, useLogs, useToast, etc.
│   │   ├── lib/                # API client, utils
│   │   └── types/              # TypeScript type definitions
│   └── vite.config.ts
│
├── pkg/logline/                # Shared types (LogLine, SourceConfig)
├── api/openapi.json            # OpenAPI 3.1 spec
├── dev/                        # Development tools
│   └── docker-compose.dev.yaml # Demo log generator
└── docs/                       # Documentation
```

## Key Interfaces

### Tailer (log source)
```go
type Tailer interface {
    Start(ctx context.Context, out chan<- *logline.LogLine, errChan chan<- error)
    Stop() error
    GetSourceName() string
}
```

### Writer (output sink)
```go
type Writer interface {
    Write(line *logline.LogLine) error
    Close() error
}
```

### Notifier (alert delivery)
```go
type Notifier interface {
    Notify(event *Event) error
}
```

### EventStore (alert persistence)
```go
type EventStore interface {
    CreateAlertEvent(ctx context.Context, event AlertEventRow) error
    DeleteAlertEventsOlderThan(ctx context.Context, before time.Time) (int, error)
}
```

## Data Flow

### Log Processing Pipeline
1. Tailers run in separate goroutines, push to shared `logChan`
2. Single pipeline goroutine consumes from `logChan`
3. Each line goes through: parse -> metrics -> alert (async) -> filter -> aggregate -> write + broadcast
4. Non-blocking at every stage: full channels use `select` with `default`

### Configuration Flow
```
CLI flags (--file, --level, --api)
    ↓
Viper merges: env vars + YAML file + CLI flags
    ↓
config.LoadConfig() validates everything
    ↓
If --db-url: config.LoadFromStore() overrides with DB values
    ↓
ConfigWatcher polls DB every 5s for changes
    ↓
On change: TailerManager/OutputManager/AlertEngine hot-reload
```

### WebSocket Flow
```
Client connects → Hub.Register(client)
Pipeline produces log → Hub.Broadcast(line)
Hub.Run() → for each client: filter by level/source → send to client.Send channel
wsWritePump → reads from client.Send → writes to WebSocket connection
Client disconnects → Hub.Unregister(client) → close(client.Send)
```

## Concurrency Model

| Component | Goroutines | Synchronization |
|-----------|-----------|-----------------|
| Tailers | 1 per source | Channel (logChan) |
| Pipeline | 1 | select loop |
| Alert Engine | 1 (processLoop) + 1 (cleanupLoop) | Channel + RWMutex |
| WebSocket Hub | 1 (event loop) | Channels (register/unregister/broadcast) |
| WS per client | 2 (read pump + write pump) | Channel (client.Send) |
| Config Watcher | 1 (poll loop) | Callbacks |
| Metrics Updater | 1 | Prometheus atomic counters |

All goroutines are wrapped with `safego.Go()` for panic recovery.

## Database Schema

7 tables in PostgreSQL:

| Table | Purpose | Key fields |
|-------|---------|------------|
| `sources` | Log source configs | name, type, path/container/unit/pod |
| `outputs` | Output destinations | name, type, config (JSONB) |
| `alert_rules` | Alert rule definitions | name, type, severity, pattern/level/threshold |
| `alert_events` | Fired alert history | rule_name, severity, message, fired_at |
| `settings` | Global key-value config | key, value (JSONB) |
| `saved_searches` | Saved filter presets | name, filters (JSONB) |
| `bookmarks` | File reading positions | name, file, offset, inode |

All tables have `updated_at` triggers for change detection by ConfigWatcher.
