# Contributing to Logtailr

## Prerequisites

- Go 1.25+
- Node.js 20+ and npm
- Docker (for running the demo log generator)
- PostgreSQL (optional, for database features)

## Getting Started

```bash
git clone https://github.com/Mariusrfx/logtailr.git
cd logtailr

# Start the demo log generator
docker compose -f dev/docker-compose.dev.yaml up -d

# Terminal 1: Start Go backend
go run . tail --config config.example.yaml --api --api-port 8080

# Terminal 2: Start frontend dev server
cd web && npm install && npm run dev

# Open http://localhost:5173
```

## Project Structure

See [docs/architecture.md](docs/architecture.md) for a detailed component map.

## Development Workflow

### Make targets

```bash
make build        # Go binary only
make build-web    # Frontend only
make build-all    # Frontend + Go binary
make test         # Run Go tests with race detector
make vet          # Run go vet
make lint         # Run govulncheck
make clean        # Remove build artifacts
```

### Running tests

```bash
# All tests
go test -race -timeout 30s ./...

# Specific package
go test -race ./internal/alert/...

# With database (integration tests)
TEST_DATABASE_URL="postgres://..." go test -race ./internal/store/...
```

### Frontend type checking

```bash
cd web
npx tsc --noEmit       # Type check
npx vite build         # Production build
```

## Code Conventions

- **Error wrapping**: Always use `fmt.Errorf("context: %w", err)`
- **Context propagation**: Pass `context.Context` as first argument
- **Channel communication**: Prefer channels over shared memory for goroutine communication
- **Tests**: Place in `*_test.go` next to the code. Use table-driven tests where possible.
- **Logging**: Use `log/slog` with structured fields, not `fmt.Printf` or `log.Printf`
- **Goroutines**: Always wrap with `safego.Go()` for panic recovery
- **Interfaces**: Define at the consumer side, not the producer

## Branching

- `main` — stable branch
- `feature/<name>` — feature branches, one per roadmap item
- Each feature branch is merged to main via fast-forward

## Commit Messages

Use conventional format:

```
feat(F7.2): Add structured logging with slog
fix: Resolve WebSocket reconnection loop
refactor: Split cmd/tail.go into pipeline.go
docs: Add deployment guide
```
