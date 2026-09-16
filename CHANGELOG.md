# Changelog

Todos los cambios notables por release, reconstruido desde el checklist de
releases de [ROADMAP.md](ROADMAP.md).

## v0.15.0 — Hardening & Reliability

**Panic recovery** (F7.1)
- `safego.Go()` helper con `recover()` + stack trace logging.
- Goroutines protegidas: alert engine (`processLoop`, `cleanupLoop`), los 5 tailers,
  API server (hub, metrics updater, HTTP server, WS pumps, rate limit cleanup).

**Structured logging** (F7.2)
- Paquete `internal/logger` con `Setup(format, level)`.
- Flags `--log-format` (text/json) y `--log-level` (debug/info/warn/error).
- Migración de `log.Print*` y `fmt.Fprintf(os.Stderr, ...)` a `slog` con campos
  contextuales (source, error, count, table, path, addr).

**Resiliencia de conexión DB** (F7.3)
- Retry con backoff exponencial en `store.New()` (5 intentos, 1s→30s),
  context-aware con logging estructurado por intento.
- _Pendiente: health check periódico de la conexión y métricas de pool._

**Transacciones en store** (F7.4)
- `Store.WithTx()` con rollback en error/panic.
- Interface `Querier` para abstraer pool vs tx; `s.q()` en todos los sub-stores
  (31 queries migradas).
- `ImportToStore` ejecuta dentro de transacción.

**Error boundaries en React** (F7.5)
- Componente `ErrorBoundary` (fallback UI con mensaje + "Try again").
- Boundary global (app) y por página (Outlet).

**Pendiente de este release** (F7.6)
- Dividir `cmd/tail.go` y `internal/config/config.go`.

## v0.14.0 — Dashboard Complete

- F6.7 Alerts page: listado de eventos con filtros por severidad/regla,
  paginación y acknowledge.
- F6.7 Logs inline en el detalle de fuente con virtual scroll.
- F6.15 Config management UI: tabs de Sources, Outputs, Alert Rules y Settings
  con CRUD completo.
- F6.15 Modal de import YAML (subida de archivo + pegar).
- F6.15 Modal de confirmación de borrado.
- F6.15 Banner "Database not configured" sin `--db-url`.
- Entorno de demo: `dev/docker-compose.dev.yaml` (generador de logs) y
  `config.example.yaml` con fuentes reales.
- README con instrucciones de demo.

## v0.13.0 — Web Dashboard

- F6.1 Scaffold: React 19 + Vite + TypeScript + Tailwind CSS 4 + shadcn/ui.
- F6.2 Layout: sidebar colapsable, header con estado de health, toggle
  dark/light, responsive.
- F6.3 Dashboard: stats cards, grid de health por fuente, alertas recientes,
  contadores en tiempo real.
- F6.4 Log Viewer: virtual scroll (100k+), filtros por nivel/regex/fuente,
  panel de detalle, pause/resume.
- F6.5 Sources Panel: grid de cards, filtro por estado, vista de detalle.
- F6.6 Polish: favicon, título dinámico, atajos de teclado, focus rings,
  transiciones de página.
- F6.14 Build: `make build-web`, `make build-all`, `//go:embed`, flag `--web`,
  proxy de Vite.

## v0.12.0 — Backend Complete

- Middleware de autenticación de API (Bearer token, flag `--api-token`).
- Prevención de SSRF en outputs creados por API.
- Masking de secrets en respuestas de la API de outputs.
- Prevención de path traversal en fuentes creadas por API.
- Hot-reload: `TailerManager` + `OutputManager` con `sync.RWMutex`.
- Config watcher cableado a los managers de hot-reload (sources, outputs).
- Goroutine de limpieza de alert events (retención 30 días).
- Endpoint `POST /api/v1/import/yaml`.
- Notifier de email (SMTP con soporte TLS).
- Tests de integración del store (se saltan sin `TEST_DATABASE_URL`).

## v0.10.0 — Discovery & Aggregation

- F5.3 Log Aggregator: deduplicación por fuente+nivel+mensaje con ventana
  configurable.
- Flags `--aggregate` y `--aggregate-window`; campos `global.aggregate` y
  `global.aggregate_window` en config.
- Ticker de flush para entradas de agregación expiradas.
- F5.2 Comando `logtailr discover`.
- FileScanner (escaneo recursivo de `/var/log/`), DockerScanner (`docker ps`)
  y JournalctlScanner (`systemctl list-units`).
- Generación de config YAML con `--save`; salidas en tabla o YAML.
- Selección de scanners con `--scan` (all/file/docker/journalctl).

## v0.9.0 — Kubernetes

- F2.3 KubernetesTailer (por nombre de pod + label selector).
- Namespace, contenedor específico y kubeconfig configurables.
- Auto-reconexión con backoff exponencial.
- Validación de config para fuentes kubernetes.
- Prevención de command injection en todos los inputs de K8s.

## v0.8.0 — Completions

- F3.2 Index template automático de OpenSearch en el arranque.
- F3.2 Index pattern automático de OpenSearch Dashboards.
- F2.2 Filtro de prioridad (`-p`) en JournalctlTailer.
- F2.2 Salida JSON (`-o json`) de journalctl con parseo de campos.
- Validación de config para priority y output_format.
- Flag `--allow-local` para desactivar la prevención de SSRF en desarrollo.
- Parseo de JSON embebido (detección de prefijo de Docker/Fluentd).
- Limpieza de líneas de Docker: extracción de timestamp + eliminación de
  secuencias ANSI.
- OpenAPI spec actualizada a v0.8.0 (endpoints y schemas de alertas).

## v0.7.0 — Hardening

- Reconexión automática de Docker con backoff exponencial.
- Rotación de file output por tamaño.
- Compresión gzip de archivos rotados.
- Limpieza por edad de archivos rotados.
- Sección de config YAML para file output (`outputs.file`).
- Helper `ParseByteSize` para tamaños legibles (KB/MB/GB).

## v0.6.0 — Alerts

- Alert engine con 4 tipos de regla: pattern, level, error_rate, health_change.
- Notifiers de console y webhook.
- Rate limiting / cooldown por regla.
- Endpoints REST (`/alerts`, `/alerts/rules`).
- Métrica Prometheus `logtailr_alerts_total`.
- Validación de config para reglas de alerta.

## v0.5.0 — API & Monitoring

- F4.1 API REST.
- F4.2 Métricas Prometheus.
- F4.4 Stream WebSocket.

## v0.4.0 — Integraciones

- F3.1 JSON Output (ya en v0.2.0).
- F3.2 OpenSearch Output.
- F3.3 File Output (ya en v0.2.0).
- F3.4 Webhook Output.
- `dev/docker-compose.dev.yaml` con stack de desarrollo.

## v0.3.0 — Multi-source

- F2.1 DockerTailer.
- F2.2 JournalctlTailer.
- F2.4 StdinTailer.
- Múltiples fuentes simultáneas.

## v0.2.0 — Core Funcional

- F1.1 Parser (JSON, logfmt, text + auto-detección).
- F1.2 Filter (nivel + regex).
- F1.3 FileTailer (con fsnotify).
- F1.4 Output Console.
- F1.5 Config Loader (Viper: env + YAML + flags).
- README con ejemplos; `make build` operativo.
