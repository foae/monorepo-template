# Getting Started

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



## 1. Clone and build

```bash
git clone https://github.com/foae/monorepo-template.git
cd monorepo-template
just check                 # gofmt, vet, tidy, build, test, sqlc diff (same as CI)
```

## 2. Start PostgreSQL

Any reachable PostgreSQL works. For a throwaway local instance:

```bash
docker run --rm -d --name example-pg \
  -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=example \
  -p 5432:5432 postgres:17
```

## 3. Configure and run the example service

```bash
cd backend/services/example
cp .env.example .env       # edit POSTGRES_URL if your database differs
just run-local             # builds backend/bin/example and runs it from this directory
```

Migrations run automatically on startup. The service listens on `0.0.0.0:8080` by default.

## 4. Try the API

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

## 5. Docker

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

