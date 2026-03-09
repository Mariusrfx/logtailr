# Changelog

## [Unreleased]

### Added
- **Health Monitoring System**: Thread-safe monitor with 5 states, error tracking, and CLI visualization (`--show-health`, `--health-every`)
- **Core Types**: `LogLine` and `SourceConfig` structs with parser/source constants
- **Tailer Interface**: Base interface with health integration and reporting methods
- **F1.1 Parser**: Multi-format log parser (JSON, logfmt, text) with auto-detection, timestamp parsing, and level normalization

## [0.1.0] - 2026-03-03

### Added
- Initial project structure with Cobra CLI and Viper config

