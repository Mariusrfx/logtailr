# Changelog

## [Unreleased]

## [0.12.0] - 2026-03-30

### Added
- **OpenAPI spec v0.12.0**: Full documentation of all 18 CRUD endpoints under `/api/v1/` — sources, outputs, alert-rules, alert-events (with filtering and acknowledgement), saved-searches, and settings. Includes input/response schemas, validation enums, pagination parameters, and reusable `ResourceId` parameter
- **Alert rules hot-reload**: `Engine.ReloadRules()` atomically replaces evaluators at runtime, preserving fire counts and last-fired timestamps for rules that keep the same name. Evaluator access protected with RWMutex for concurrent safety
- **Config watcher alert reload**: When alert rules change in the database, the config watcher now reloads them into the running alert engine automatically (no restart required). Sources and outputs still require restart
- **README: PostgreSQL mode**: New documentation section covering `--db-url` flag, `logtailr migrate` commands, `logtailr import`, CRUD API table, and hot-reload behavior

### Changed
- **`config.LoadSettingString` exported**: Renamed from `loadSettingString` to allow reuse from `cmd` package during alert rule reload

## [0.11.0] - 2026-03-12

### Changed
- **F5.5 Search Histórico — Omitido**: Búsqueda en archivos locales descartada por redundante. OpenSearch ya indexa todos los logs con metadata completa. La búsqueda histórica se implementará como endpoint API (`GET /api/search`) sobre OpenSearch en Fase 6.

### Added
- **F5.4 Bookmarks**: Save and resume file reading position with `--bookmark` and `--resume` CLI flags
- **Bookmark persistence**: Stores file path, byte offset, inode, and timestamp in `~/.logtailr/bookmarks.json`
- **Inode verification**: Detects file rotation between sessions — warns and reads from start if inode changed
- **Atomic writes**: Bookmark file uses write-to-tmp + rename pattern to prevent corruption
- **Bookmark manager**: `internal/bookmark` package with Save, Load, List, Delete operations and name validation
- **Offset tracking**: FileTailer tracks byte offset during reading for accurate bookmark save on shutdown

## [0.10.0] - 2026-03-12

### Added
- **F5.3 Log Aggregation**: Detect repeated log messages (same source+level+message) and display a count instead of duplicates. Format: `message (x3 in last 2s)`. Configurable time window.
- **Aggregation CLI flags**: `--aggregate` to enable, `--aggregate-window` to set the dedup window (default 5s)
- **Aggregation config**: `global.aggregate: true` and `global.aggregate_window: "5s"` in YAML config
- **Aggregation ticker**: Background goroutine flushes expired aggregation entries automatically, even when no new lines arrive
- **F5.2 Auto-discovery command**: `logtailr discover` scans the system for log sources and generates configuration
- **FileScanner**: Recursively scans `/var/log/` for `.log` files, skips empty and oversized (>1GB) files
- **DockerScanner**: Lists running Docker containers via `docker ps`, each becomes a source config entry
- **JournalctlScanner**: Lists active systemd services via `systemctl list-units`, filters `.service` units
- **Discovery output**: `--output table` (default) shows tabular results, `--output yaml` prints YAML config to stdout
- **Discovery save**: `--save config.yaml` writes generated config to file (refuses to overwrite existing files)
- **Discovery scanner selection**: `--scan all|file|docker|journalctl` to choose which scanners to run

## [0.9.0] - 2026-03-12

### Added
- **F2.3 KubernetesTailer**: Read logs from Kubernetes pods via `kubectl logs` with follow mode, namespace selection, and health monitoring
- **Kubernetes pod selection**: Support for logs by pod name (`pod: "api-server"`) or by label selector (`label_selector: "app=worker,version=v2"`)
- **Kubernetes container selection**: Optional `container` field for multi-container pods (`-c` flag)
- **Kubernetes kubeconfig support**: Optional `kubeconfig` field to specify a custom kubeconfig path
- **Kubernetes auto-reconnect**: Exponential backoff reconnection (1s–30s) when pod log stream ends or pod restarts
- **Config validation**: Kubernetes sources require either `pod` or `label_selector` (not both), label selectors validated against injection, `namespace`/`label_selector`/`kubeconfig` fields restricted to kubernetes source type

## [0.8.0] - 2026-03-09

### Added
- **F3.2 OpenSearch index template**: Automatically creates an index template on startup with proper mappings (timestamp as `date`, level as `keyword`, message as `text`, source as `keyword`, fields as `object`). Configurable via `template_name` (default: `logtailr`)
- **F3.2 OpenSearch Dashboards index pattern**: Automatically creates an index pattern in Dashboards on startup when `dashboards_url` is configured. Handles 409 Conflict gracefully (pattern already exists)
- **F2.2 JournalctlTailer priority filter**: New `priority` field in journalctl source config to filter by syslog priority (`-p err`, `-p warning`, etc.)
- **F2.2 JournalctlTailer JSON output**: New `output_format: "json"` field in journalctl source config to use structured JSON output (`-o json`). Parses `MESSAGE`, `PRIORITY`, `__REALTIME_TIMESTAMP`, and systemd fields (`_HOSTNAME`, `_SYSTEMD_UNIT`, `_PID`, etc.) into LogLine fields
- **Config validation**: Priority validated against syslog levels (emerg–debug), `output_format` restricted to journalctl sources, only `json` supported
- **`--allow-local` flag**: Disables SSRF prevention for localhost/private IPs, allowing local OpenSearch, webhook, and alert URLs during development
- **Embedded JSON parsing**: JSON parser now detects and extracts JSON embedded in lines with prefixes (Docker timestamps, Fluentd tags). Enables parsing of `docker logs` output from Fluentd containers
- **Docker line cleanup**: DockerTailer now extracts the Docker timestamp as the real log timestamp (instead of `time.Now()`) and strips ANSI escape codes from messages
- **OpenAPI spec v0.8.0**: Added `GET /alerts` and `GET /alerts/rules` endpoints with `AlertEvent`, `AlertRule`, `AlertEventList`, and `AlertRuleList` schemas

## [0.7.0] - 2026-03-09

### Added
- **Docker auto-reconnect**: DockerTailer now automatically reconnects when a container stops or restarts, using exponential backoff (1s–30s). Reports degraded status during reconnection and healthy on success. Reconnection continues until the context is cancelled.
- **File output rotation**: FileWriter supports size-based rotation via `WithMaxSize()`. Rotated files are renamed with a timestamp suffix (e.g. `app.log.2026-03-09T10-30-00`). Optional gzip compression of rotated files with `WithCompress()`. Automatic cleanup of rotated files older than `WithMaxAge()`.
- **File output YAML config**: New `outputs.file` config section with `path`, `max_size` (human-readable: `10MB`, `500KB`, `1GB`), `max_age` (duration), and `compress` (bool)
- **Config validation**: File output path required, `max_size` parsed and validated, `max_age` validated as duration

## [0.6.0] - 2026-03-09

### Added
- **F4.3 Alert Engine**: Rule-based alert system with 4 rule types — `pattern` (regex match on log messages), `level` (severity threshold), `error_rate` (sliding window count), `health_change` (source status transitions)
- **Alert Notifiers**: Console notifier (colored output to stderr) and webhook notifier (HTTP POST with JSON payload, 10s timeout, 1MB response limit)
- **Rate Limiting**: Per-rule cooldown enforcement to prevent alert spam, configurable per rule or via `default_cooldown`
- **API endpoints**: `GET /alerts` (recent alert events with severity, source, timestamp), `GET /alerts/rules` (configured rules with fire count and last fired time)
- **Prometheus metric**: `logtailr_alerts_total{rule,severity}` counter for fired alerts
- **CLI integration**: Alert engine wired into the tail pipeline, health monitor change callback triggers health_change rules
- **Config validation**: Full validation of alert rules — type/severity enums, regex compilation, level whitelist, duration parsing, duplicate name detection, SSRF prevention on webhook URLs

## [0.5.0] - 2026-03-09

### Added
- **F4.1 REST API**: HTTP API server with health endpoints (`/health`, `/health/sources`, `/health/sources/:name`), config endpoint (`/config` with secret sanitization), CORS support
- **F4.2 Prometheus Metrics**: `/metrics` endpoint with `logtailr_logs_total` (by source/level), `logtailr_source_healthy` (by source/status), `logtailr_source_errors_total`, `logtailr_processing_duration_seconds`, `logtailr_active_sources`, `logtailr_websocket_clients`
- **F4.4 WebSocket Stream**: Real-time log streaming via `ws://host:port/ws/logs` with optional query filters (`?level=error&source=app.log`), ping/pong keepalive, per-client buffering
- **CLI flags**: `--api` to enable API server, `--api-port` to set port (default 8080)
- **Log Hub**: Broadcast hub for fan-out to WebSocket clients with level and source filtering
- **OpenAPI 3.1 spec**: Full API specification at `api/openapi.json` for typed client generation

### Security
- API config endpoint sanitizes passwords and webhook URLs
- WebSocket message size limited to 512 bytes (read), write deadlines enforced
- HTTP server hardened: ReadHeaderTimeout, WriteTimeout, IdleTimeout

## [0.4.0] - 2026-03-09

### Security
- **CRITICAL**: Command injection prevention — Docker container names and journalctl unit names are now validated against strict allowlist (`^[a-zA-Z0-9][a-zA-Z0-9._:@-]*$`) before passing to `os/exec`
- **HIGH**: Path traversal fix in `FileWriter` — output paths are resolved with `filepath.Abs` and directory existence is verified
- **HIGH**: Channel buffer cap — shared log/error channels are now capped at 10,000 to prevent unbounded memory growth
- **HIGH**: Credential sanitization — OpenSearch HTTP error responses no longer include server response body (may contain tokens)
- **HIGH**: TLS minimum version enforced (TLS 1.2) on OpenSearch HTTP transport
- **MEDIUM**: SSRF prevention — webhook and OpenSearch URLs are validated against internal/private IP ranges (loopback, RFC1918, link-local, cloud metadata endpoints)
- **MEDIUM**: Flush errors in OpenSearch and webhook background loops are now reported to stderr instead of silently discarded
- **MEDIUM**: HTTP response body reads are limited to 1MB in webhook writer to prevent memory exhaustion from malicious servers
- **MEDIUM**: Webhook error messages no longer include the full endpoint URL
- **MEDIUM**: Retry backoff capped at 30s to prevent indefinite blocking
- **MEDIUM**: JSON parser uses streaming decoder instead of full `json.Unmarshal`
- **LOW**: Regex pattern length limited to 1024 characters to prevent excessive memory usage

### Added
- **F3.2 OpenSearch/Elasticsearch Writer**: Bulk insert with configurable batch size and flush interval, basic auth, TLS support, exponential backoff retry, date-based index patterns (`%{+YYYY.MM.dd}`), round-robin host selection
- **F3.4 Webhook Writer**: HTTP POST with batched payloads, configurable min_level filter, batch size/timeout, JSON payload with log summary text
- **Multi-output support**: Config-driven output destinations via `outputs` section in YAML — OpenSearch, webhook, and primary output (console/json/file) run simultaneously through MultiWriter
- **Config validation**: OpenSearch host URL scheme validation, webhook URL validation, min_level validation for webhook

## [0.3.0] - 2026-03-09

### Added
- **F2.1 DockerTailer**: Read logs from Docker containers via `docker logs` with follow mode, stdout/stderr merge, and health monitoring
- **F2.2 JournalctlTailer**: Read systemd journal logs via `journalctl -u <unit>` with follow mode, short-iso format, and health monitoring
- **F2.4 StdinTailer**: Read logs from stdin for pipe usage (`cat app.log | logtailr tail`), clean EOF handling
- **Multi-source pipeline**: Full orchestration in `cmd/tail.go` — config-driven or single-file mode, shared channels with backpressure, concurrent tailer startup, startup banner, graceful shutdown with summary

## [0.2.0] - 2026-03-06

### Added
- **Health Monitoring System**: Thread-safe monitor with 5 states, error tracking, and CLI visualization (`--show-health`, `--health-every`)
- **Core Types**: `LogLine` and `SourceConfig` structs with parser/source constants
- **Tailer Interface**: Base interface with health integration and reporting methods
- **F1.1 Parser**: Multi-format log parser (JSON, logfmt, text) with auto-detection, timestamp parsing, and level normalization
- **F1.2 Filter**: Log filtering by severity level (`debug < info < warn < error < fatal`) and regex pattern matching with combined filter support
- **F1.3 FileTailer**: Real file tailing with fsnotify-based follow mode, logrotate detection with auto-reopen, permission/missing file error handling, and full health monitor integration
- **F1.4 Output**: Writer interface with ConsoleWriter (ANSI colors by level), JSONWriter (NDJSON), FileWriter (append with 0600 perms), and MultiWriter fan-out
- **F1.5 Config Loader**: YAML config loading with Viper, environment variable support (`LOGTAILR_` prefix), defaults, and comprehensive validation (source types, parsers, levels, outputs, duplicates)

### Security
- Path traversal protection on `--file` and `--config` flags (absolute path resolution, symlink evaluation, regular file check)
- Graceful shutdown with signal handling (`SIGINT`/`SIGTERM`) and context cancellation
- Goroutine leak fix in health updater (context-aware select loop)
- Input validation: log level whitelist, regex pattern early compilation
- Resource limits: max log line size (256KB), max JSON/logfmt fields (100)
- Sanitized error messages (no internal paths leaked)
- File descriptor leak fix on log rotation (checked `Close()` errors)

## [0.1.0] - 2026-03-03

### Added
- Initial project structure with Cobra CLI and Viper config