# Getting started

Use this page to run the unmodified example. To change code, start with [conventions](conventions.md); to create your own service, use [adding a service](adding-a-service.md).

## Prerequisites

| Tool | Pinned version | Source of truth |
|---|---|---|
| Go | 1.27.3 | `backend/go.mod` |
| just | 1.59.0 | `.github/workflows/ci.yml` |
| sqlc | 1.31.1 | `.github/workflows/ci.yml` |
| PostgreSQL image | 18.6-trixie | Local example below |
| Docker | Current stable | Optional for native builds |

Install from [Go](https://go.dev/dl/), [just](https://github.com/casey/just/releases) and [sqlc](https://docs.sqlc.dev/en/latest/overview/install.html). The GitHub CLI is only required for publishing releases.

## 1. Verify the checkout

```bash
git clone https://github.com/foae/monorepo-template.git
cd monorepo-template
just check
```

Expected: formatting, vet, dependency tidiness, build, race tests and SQL generation checks pass. Run direct `go` commands from `backend/`, the module root.

## 2. Start a disposable database

```bash
docker run --rm -d --name example-pg \
  -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=example \
  -p 127.0.0.1:5432:5432 postgres:18.6-trixie
docker exec example-pg pg_isready -h 127.0.0.1 -U postgres -d example
```

Wait for the TCP readiness check to succeed before starting the service. During first initialization PostgreSQL briefly starts a Unix-socket-only server; a socket readiness check can succeed too early. This container has no persistent volume and is for local development only. Stop it with `docker stop example-pg` when finished.

## 3. Run the service

```bash
cd backend/services/example
cp .env.example .env
just run-local
```

Migrations finish before HTTP serving. Expect JSON startup logs and `service listening` on port 8080. The automaxprocs blank import emits one upstream pre-main plain-text diagnostic; see [logging](patterns.md#json-logging-and-trace-correlation).

## 4. Exercise the API

Use another terminal:

```bash
curl -i localhost:8080/health
curl -i localhost:8080/ready
curl -s localhost:8080/metrics
curl -i -X POST localhost:8080/api/v1/items \
  -H 'X-Owner-Id: alice' -H 'Content-Type: application/json' \
  -d '{"name":"first"}'
curl -i localhost:8080/api/v1/items -H 'X-Owner-Id: alice'
```

Use the returned ID for `GET /api/v1/items/<id>` and `DELETE /api/v1/items/<id>`. Create returns 201; delete returns 204 without a body. Repeating creation returns the same item ID and refreshes `updated_at`. Missing owner returns 401; another owner's ID returns 403; missing items return 404. Invalid JSON returns a safe 400, unknown internal failures a safe 500.

**Security boundary:** `X-Owner-Id` is not authentication. Before exposing the API, an authenticated proxy must replace untrusted owner headers and prevent direct access, or you must implement authentication. Keep metrics private. The example's readiness reports lifecycle state, not continuous database availability.

Press Ctrl-C to stop: readiness becomes unavailable, the worker is canceled, HTTP drains and the worker is joined before database closure. Expected exit code: 0. Invalid config, bind failure and shutdown deadline failures exit 1.

## 5. Build a container

From `backend/services/example/`:

```bash
just docker-build
just docker-run .env
```

Set `POSTGRES_URL` to an address reachable **from the container**, not its localhost. Docker Desktop can use `POSTGRES_URL_HOST=host.docker.internal`. On Linux, use a user-defined Docker network with the database container name, or explicitly supply a host-gateway mapping. The image runs without root or a shell; it contains only the binary and CA certificates. `.env` enters only through runtime `--env-file`, never the build context.

## Configuration

Environment variables take precedence over a working-directory `.env`. Commit only placeholder `.env.example` files; keep actual credentials in ignored local files or deployment secrets.

| Variable | Default | Contract |
|---|---|---|
| `POSTGRES_URL` | Required | Nonempty PostgreSQL URL |
| `POSTGRES_URL_HOST` | Unset | Override hostname; preserve original port |
| `SERVICE_NAME` | `example` | Service identity |
| `SERVICE_VERSION` | `v1.0.0` | Deployment version label |
| `ENV_MODE` | `prod` | `dev`: Debug+; `prod`: Info+; local `.env.example` opts into `dev` |
| `HTTP_LISTEN_ADDR` | `0.0.0.0:8080` | Use `[::]:8080` for IPv6 |
| `SERVICE_REGION` | `local` | Trace resource region |
| `WORKER_INTERVAL` | `1h` | Positive interval; first run after one minute |
| `GRACEFUL_SHUTDOWN_TIMEOUT` | `90s` | Positive HTTP/worker/drain budget |
| `HTTP_DRAIN_DELAY` | `5s` | Nonnegative endpoint deregistration delay |
| `OTEL_TRACING_ENDPOINT` | Empty | Disabled; enable with `host:4317` |
| `OTEL_TRACING_SAMPLE_RATIO` | `0.1` | Root ratio in `[0,1]` |
| `OTEL_TRACING_INSECURE` | `false` | TLS by default; true for trusted plaintext |

For tracing activation, propagation, sampling, shutdown and IPv6 override details, read [patterns](patterns.md). Plan the orchestrator termination grace for the shutdown timeout **plus five seconds** for trace flushing.
