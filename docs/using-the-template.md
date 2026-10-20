# Using This Template for Your Own Project

Read [conventions](conventions.md) before editing Go. Use [architecture](architecture.md) for boundaries and [patterns](patterns.md) for a specific runtime concern; avoid loading every guide for a small change.

1. Copy the repository (or use it as a GitHub template).
2. Replace the module path `monorepo-template/backend` with your own in `backend/go.mod` and every import:
   ```bash
   grep -rl 'monorepo-template/backend' backend --include='*.go' --include='go.mod' \
     | xargs sed -i 's#monorepo-template/backend#github.com/you/yourrepo/backend#g'
   ```
3. Rename the module path in documentation examples as well. Configure `goimports -local` for the new module prefix. Run `just check` to confirm everything still builds.
4. Follow [adding-a-service.md](adding-a-service.md) to create your first real service from `services/example`.


Before deployment, replace the example's trusted owner-header assumption with your actual authentication boundary, restrict metrics access, and configure database credentials/TLS. Tracing stays off unless explicitly enabled. Go checks cover every service; add each new SQL config to the root check recipe and each deployed image to CI.

## Everyday commands

```bash
# repo root
just check                 # all CI checks
just version               # latest release tag
just release-next patch    # tag, push, and publish the next patch release (see releasing.md)

# backend/ (module root)
just build-all | test-all | vet-all | fmt-all | tidy | check-all

# backend/services/<name>/
just build | test | vet | fmt | run-local
just sqlc-vet              # lint and compile SQL queries
just sqlc-generate         # regenerate storage/postgres/sqlc/*_gen.go after editing q.sql
just docker-build | docker-run
just --list                # everything available
```


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

