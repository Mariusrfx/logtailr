# Changelog

## [Unreleased]

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