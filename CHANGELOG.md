# Changelog

## [Unreleased]

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

