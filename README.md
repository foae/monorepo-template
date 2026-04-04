# Go Service Template

A production-ready monorepo template for building Go microservices. Based on patterns proven in production across multiple S3-compatible storage gateways handling real traffic.

## How to Use This Template

1. Copy this directory into your project
2. Search-and-replace `go-service-template/backend` with your module path in all `.go` files and `go.mod`
3. `cd backend && go build ./services/example/cmd/example` to verify
4. See [docs/adding-a-service.md](docs/adding-a-service.md) for creating new services from the example

## Repository Structure

```
go-service-template/
├── README.md                          # YOU ARE HERE — start reading here
├── backend/                           # Go module root
│   ├── go.mod                         # Single module for all services + shared packages
│   ├── pkg/                           # Shared packages (stable utilities, not domain code)
│   │   ├── envutil/                   # DNS pinning via _HOST env var companions
│   │   ├── httputil/                  # Prometheus request monitoring middleware (Chi)
│   │   ├── postgres/                  # PGX connection pool wrapper
│   │   └── observability/             # OpenTelemetry tracing setup (optional, gRPC)
│   └── services/                      # Service implementations
│       └── example/                   # Canonical example service — copy this for new services
│           ├── cmd/example/main.go    # Entry point: config, DI, router, graceful shutdown
│           ├── core/                  # Business logic, state machines, orchestration
│           ├── handler/               # HTTP transport (request parsing, response writing)
│           ├── storage/postgres/      # Data access (sqlc, migrations, PGX)
│           ├── Dockerfile             # Multi-stage build, non-root user
│           ├── justfile               # Per-service recipes (imports shared service.just)
│           └── .env.example           # All env vars with defaults
├── infra/                             # Per-service infrastructure configs (Dockerfiles, configs)
└── docs/                              # Architecture, patterns, and guides
    ├── architecture.md                # Layer model, module strategy, dependency flow
    ├── patterns.md                    # Each pattern: WHAT / WHY / HOW / WHERE
    ├── adding-a-service.md            # Step-by-step guide to create a new service
    └── conventions.md                 # Naming, imports, errors, logging, testing rules
```

## Layer Model

Every service follows a 4-layer architecture. Dependencies flow downward only.

```
cmd/          Entry point. Config loading, dependency injection, router setup,
              middleware registration, graceful shutdown. Depends on everything.

core/         Business logic. State machines, validation, orchestration.
              Depends on storage. Never imports net/http.

handler/      HTTP transport. Extracts request params, calls core methods,
              maps errors to status codes, writes responses.
              Depends on core and storage types. No business logic here.

storage/      Data access. sqlc queries, migrations, DB client.
              Depends on pkg/postgres. No business logic here.
```

## Key Patterns (Quick Reference)

| Pattern | Where | What |
|---------|-------|------|
| Config from env | `cmd/*/main.go` | Struct tags via `caarlos0/env`, required/optional/defaults |
| Structured logging | `cmd/*/main.go` | slog JSON, dev=Debug/prod=Info, OTel trace IDs in prod |
| Graceful shutdown | `cmd/*/main.go` | 3-phase: mark unready → 5s K8s drain → shutdown with timeout |
| Health/ready/metrics | `cmd/*/main.go` | /health=200, /ready=503 during shutdown, /metrics=Prometheus |
| Error mapping | `handler/errors.go` | Sentinel errors in core → HTTP status codes in handler |
| Idempotent creates | `core/item_create.go` | INSERT ... ON CONFLICT DO UPDATE — DB-level idempotency |
| Background worker | `core/worker.go` | 1min startup delay, then fixed interval, context cancellation |
| sqlc type-safe SQL | `storage/postgres/` | Write q.sql → `sqlc generate` → use typed Go methods |
| Embedded migrations | `storage/postgres/client.go` | `embed.FS` + golang-migrate, runs on startup |
| DNS pinning | `pkg/envutil/` | `POSTGRES_URL_HOST` overrides hostname in URL env vars |
| Prometheus metrics | `pkg/httputil/` | Request count, in-flight, latency histogram per service |
| OTel tracing | `pkg/observability/` | Optional gRPC exporter, skips silently if no endpoint |

## Deep Dives

- **Architecture & layer model**: [docs/architecture.md](docs/architecture.md)
- **Pattern reference (WHAT/WHY/HOW)**: [docs/patterns.md](docs/patterns.md)
- **Adding a new service**: [docs/adding-a-service.md](docs/adding-a-service.md)
- **Coding conventions**: [docs/conventions.md](docs/conventions.md)

## Build & Verify

This template uses [just](https://github.com/casey/just) as its command runner. Each service has a `justfile` that imports shared recipes from `backend/service.just`.

```bash
# From a service directory (backend/services/example/):
just build          # Build binary to backend/bin/example
just test           # Run tests with race detection
just vet            # Run go vet
just run-local      # Build and run (godotenv loads .env)
just docker-build   # Build Docker image
just docker-run     # Build + run Docker container
just --list         # Show all available recipes

# From the module root (backend/):
just build-all      # Build all services
just test-all       # Run all tests
just vet-all        # Run go vet on everything
```

### Adding shared recipes

All common recipes live in `backend/service.just`. Each service includes them:

```just
SERVICE_NAME := "myservice"
import '../../service.just'

# Service-specific recipes below:
```

### Install just

```bash
# macOS
brew install just

# Linux
curl --proto '=https' --tlsv1.2 -sSf https://just.systems/install.sh | bash -s -- --to /usr/local/bin

# GitHub Actions
- uses: extractions/setup-just@v3
```

## Dependencies

| Dependency | Purpose |
|------------|---------|
| [chi](https://github.com/go-chi/chi) | HTTP router |
| [pgx](https://github.com/jackc/pgx) | PostgreSQL driver (pool + stdlib bridge) |
| [sqlc](https://sqlc.dev/) | Type-safe SQL code generation |
| [golang-migrate](https://github.com/golang-migrate/migrate) | Database migrations |
| [slog](https://pkg.go.dev/log/slog) | Structured logging (stdlib) |
| [caarlos0/env](https://github.com/caarlos0/env) | Config from env vars via struct tags |
| [prometheus](https://github.com/prometheus/client_golang) | Metrics exposition |
| [otel](https://opentelemetry.io/docs/languages/go/) | Distributed tracing |
| [automaxprocs](https://github.com/uber-go/automaxprocs) | Container-aware GOMAXPROCS |
| [slog-chi](https://github.com/samber/slog-chi) | Chi request logging via slog |
| [godotenv](https://github.com/joho/godotenv) | .env file loading for local dev |
