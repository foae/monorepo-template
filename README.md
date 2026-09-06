# monorepo-template

A monorepo template for Go backend services: one Go module, shared packages, and a fully wired example service that shows the conventions every new service follows. Copy it, rename the module, and add services by cloning the example.

What the example service already does:

- HTTP API with [chi](https://github.com/go-chi/chi), JSON request/response types, and sentinel-error to HTTP-status mapping
- PostgreSQL access through [pgx](https://github.com/jackc/pgx) with [sqlc](https://sqlc.dev/)-generated, type-safe queries and embedded [golang-migrate](https://github.com/golang-migrate/migrate) migrations that run on startup
- Configuration from environment variables via struct tags, with `.env` support for local development
- Structured JSON logging with `log/slog`, Prometheus metrics on `/metrics`, optional OpenTelemetry tracing
- `/health` and `/ready` endpoints and a three-phase graceful shutdown suited to rolling deployments
- A background worker with context cancellation
- A single-stage Dockerfile producing a static binary that runs as a non-root user
- [just](https://github.com/casey/just) recipes for build, test, lint, code generation, Docker, and releases

## Repository structure

```
monorepo-template/
├── README.md
├── CLAUDE.md                          # Contributor workflow: checks, versioning, releases
├── justfile                           # Repo-level recipes: check, version, release
├── .github/workflows/ci.yml           # CI: gofmt, vet, build, test, tidy, sqlc diff, docker build
├── backend/                           # Go module root (module "monorepo-template/backend")
│   ├── go.mod
│   ├── justfile                       # Module-wide recipes (build-all, test-all, ...)
│   ├── service.just                   # Shared per-service recipes, imported by each service
│   ├── pkg/                           # Shared packages (utilities, not domain code)
│   │   ├── envutil/                   # Hostname override for URL env vars (*_HOST companions)
│   │   ├── httputil/                  # Prometheus request middleware for chi
│   │   ├── postgres/                  # pgx connection pool wrapper
│   │   └── observability/             # OpenTelemetry tracing setup (optional, gRPC exporter)
│   └── services/
│       └── example/                   # Canonical service; copy this to create new ones
│           ├── cmd/example/main.go    # Config, dependency wiring, router, graceful shutdown
│           ├── core/                  # Business logic; never imports net/http
│           ├── handler/               # HTTP transport: parse requests, map errors, write responses
│           ├── storage/postgres/      # sqlc queries, migrations, DB client
│           ├── Dockerfile
│           ├── justfile
│           └── .env.example           # Every env var the service reads, with defaults
├── infra/                             # Per-component infrastructure configs
└── docs/
    ├── architecture.md                # Layer model, module strategy, dependency flow
    ├── patterns.md                    # Each pattern: what, why, how, where
    ├── adding-a-service.md            # Step-by-step guide for a new service
    └── conventions.md                 # Naming, imports, errors, logging, testing rules
```

## Layer model

Every service has four layers. Dependencies point downward only.

| Layer | Responsibility | May import |
|-------|----------------|------------|
| `cmd/` | Config loading, dependency injection, router and middleware, graceful shutdown | everything |
| `core/` | Business logic, validation, orchestration, background work | `storage/`, `pkg/` |
| `handler/` | HTTP transport: request parsing, calling `core`, error to status mapping | `core/`, storage types |
| `storage/` | Data access: sqlc queries, migrations, DB client | `pkg/postgres` |

See [docs/architecture.md](docs/architecture.md) for the rationale.

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| [Go](https://go.dev/dl/) | 1.27 or newer (see `backend/go.mod`) | build and test |
| [just](https://github.com/casey/just) | any recent | command runner |
| [sqlc](https://docs.sqlc.dev/en/latest/overview/install.html) | 1.31.x | regenerate query code (CI pins 1.31.1) |
| PostgreSQL | 14 or newer | runtime dependency of the example service |
| Docker | optional | container builds, or running PostgreSQL locally |
| [GitHub CLI](https://cli.github.com/) | optional | needed only by the `release` recipes |

Install `just`:

```bash
# macOS
brew install just

# Linux
curl --proto '=https' --tlsv1.2 -sSf https://just.systems/install.sh | bash -s -- --to /usr/local/bin
```

## Getting started

### 1. Clone and build

```bash
git clone https://github.com/foae/monorepo-template.git
cd monorepo-template
just check                 # gofmt, vet, tidy, build, test, sqlc diff (same as CI)
```

### 2. Start PostgreSQL

Any reachable PostgreSQL works. For a throwaway local instance:

```bash
docker run --rm -d --name example-pg \
  -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=example \
  -p 5432:5432 postgres:17
```

### 3. Configure and run the example service

```bash
cd backend/services/example
cp .env.example .env       # edit POSTGRES_URL if your database differs
just run-local             # builds backend/bin/example and runs it from this directory
```

Migrations run automatically on startup. The service listens on `0.0.0.0:8080` by default.

### 4. Try the API

Every item belongs to an owner identified by the `X-Owner-Id` header, which is expected to be set by an upstream authentication proxy.

```bash
# health, readiness, metrics
curl -i localhost:8080/health
curl -i localhost:8080/ready
curl -s localhost:8080/metrics | head

# create (idempotent per owner + name), list, get, delete
curl -s -X POST localhost:8080/api/v1/items \
  -H 'X-Owner-Id: alice' -H 'Content-Type: application/json' \
  -d '{"name":"first"}'
curl -s localhost:8080/api/v1/items -H 'X-Owner-Id: alice'
curl -s localhost:8080/api/v1/items/<id> -H 'X-Owner-Id: alice'
curl -s -X DELETE localhost:8080/api/v1/items/<id> -H 'X-Owner-Id: alice'
```

### 5. Docker

```bash
cd backend/services/example
just docker-build          # image tagged "example"
just docker-run            # runs with .env.example; pass a file to override: just docker-run .env
```

Inside a container `localhost` in `POSTGRES_URL` refers to the container itself. Point it at your database host, or set `POSTGRES_URL_HOST=host.docker.internal` (Docker Desktop) to override only the hostname.

## Configuration

The example service reads these environment variables (see `backend/services/example/.env.example`). A `.env` file in the service directory is loaded automatically when present; it is git-ignored.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `POSTGRES_URL` | yes | | PostgreSQL connection URL |
| `POSTGRES_URL_HOST` | no | | Overrides only the hostname in `POSTGRES_URL` (useful for per-cluster DNS without changing the secret) |
| `SERVICE_NAME` | no | `example` | Name used in logs and metrics |
| `SERVICE_VERSION` | no | `v1.0.0` | Version reported in logs and traces |
| `ENV_MODE` | no | `dev` | `dev` enables debug logging; `prod` enables info level and trace IDs in logs |
| `HTTP_LISTEN_ADDR` | no | `0.0.0.0:8080` | Listen address |
| `SERVICE_REGION` | no | `local` | Region label for logs and traces |
| `WORKER_INTERVAL` | no | `1h` | Background worker interval |
| `GRACEFUL_SHUTDOWN_TIMEOUT` | no | `90s` | Upper bound for draining in-flight requests |
| `OTEL_TRACING_ENDPOINT` | no | | OTLP gRPC endpoint; tracing is disabled when empty |

Keep real credentials out of the repository: use `.env` files or a `.private/` directory, both of which are git-ignored. Commit only `.env.example` files with placeholder values.

## Using this template for your own project

1. Copy the repository (or use it as a GitHub template).
2. Replace the module path `monorepo-template/backend` with your own in `backend/go.mod` and every import:
   ```bash
   grep -rl 'monorepo-template/backend' backend --include='*.go' --include='go.mod' \
     | xargs sed -i 's#monorepo-template/backend#github.com/you/yourrepo/backend#g'
   ```
3. Run `just check` to confirm everything still builds.
4. Follow [docs/adding-a-service.md](docs/adding-a-service.md) to create your first real service from `services/example`.

## Everyday commands

```bash
# repo root
just check                 # all CI checks
just version               # latest release tag
just release-next patch    # tag, push, and publish the next patch release (see CLAUDE.md)

# backend/ (module root)
just build-all | test-all | vet-all | fmt-all | tidy | check-all

# backend/services/<name>/
just build | test | vet | fmt | run-local
just sqlc-vet              # lint and compile SQL queries
just sqlc-generate         # regenerate storage/postgres/sqlc/*_gen.go after editing q.sql
just docker-build | docker-run
just --list                # everything available
```

## Key patterns

| Pattern | Where | Summary |
|---------|-------|---------|
| Config from env | `cmd/*/main.go` | Struct tags via `caarlos0/env`; required vars fail fast |
| Structured logging | `cmd/*/main.go` | slog JSON; trace and span IDs injected in `prod` mode |
| Graceful shutdown | `cmd/*/main.go` | mark unready, wait for endpoint deregistration, drain with timeout |
| Health, ready, metrics | `cmd/*/main.go` | `/health` always 200; `/ready` 503 while shutting down; `/metrics` Prometheus |
| Error mapping | `handler/errors.go` | Sentinel errors in `core` become HTTP status codes in one place |
| Idempotent creates | `core/item_create.go` | `INSERT ... ON CONFLICT DO UPDATE` gives database-level idempotency |
| Background worker | `core/worker.go` | Startup delay, fixed interval, stops on context cancellation |
| Type-safe SQL | `storage/postgres/` | Edit `q.sql`, run `just sqlc-generate`, call generated methods |
| Embedded migrations | `storage/postgres/client.go` | `embed.FS` plus golang-migrate, applied on startup |
| Hostname override | `pkg/envutil/` | `FOO_URL_HOST` replaces the host in `FOO_URL` |
| Request metrics | `pkg/httputil/` | Request count, in-flight gauge, latency histogram |
| Tracing | `pkg/observability/` | OTLP gRPC exporter, silently disabled without an endpoint |

Details for each pattern are in [docs/patterns.md](docs/patterns.md); coding rules are in [docs/conventions.md](docs/conventions.md).

## Continuous integration and releases

`.github/workflows/ci.yml` runs on every push to `main`, every pull request, and every `v*` tag: gofmt, `go vet`, `go build`, `go test -race`, a `go mod tidy` cleanliness check, `sqlc diff`, and a Docker build of the example service.

Releases are SemVer tags on `main` published as GitHub releases. `just release-next patch|minor|major` (or `just release vX.Y.Z`) verifies the tree, runs the checks, creates an annotated tag, pushes it, and publishes the release with generated notes. Published tags are never moved. See [CLAUDE.md](CLAUDE.md) for when to bump which component.

## Dependencies

| Dependency | Purpose |
|------------|---------|
| [chi](https://github.com/go-chi/chi) | HTTP router |
| [pgx](https://github.com/jackc/pgx) | PostgreSQL driver and connection pool |
| [sqlc](https://sqlc.dev/) | Type-safe SQL code generation |
| [golang-migrate](https://github.com/golang-migrate/migrate) | Database migrations |
| [caarlos0/env](https://github.com/caarlos0/env) | Config from env vars via struct tags |
| [godotenv](https://github.com/joho/godotenv) | `.env` loading for local development |
| [prometheus/client_golang](https://github.com/prometheus/client_golang) | Metrics |
| [OpenTelemetry Go](https://opentelemetry.io/docs/languages/go/) | Distributed tracing |
| [slog-chi](https://github.com/samber/slog-chi) | Request logging through slog |
| [automaxprocs](https://github.com/uber-go/automaxprocs) | Container-aware `GOMAXPROCS` |

## License

[MIT](LICENSE)
