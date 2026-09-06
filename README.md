# monorepo-template

A monorepo template for Go backend services: one Go module, shared packages, and a fully wired example service that shows the conventions every new service follows. Copy it, rename the module, and add services by cloning the example.

The example service ships with a chi HTTP API, PostgreSQL via pgx and sqlc, embedded migrations, env-based config, slog JSON logging, Prometheus metrics, optional OpenTelemetry tracing, health and readiness endpoints, graceful shutdown, a background worker, a non-root Dockerfile, and `just` recipes for build, test, code generation, Docker, and releases.

## Quick start

```bash
git clone https://github.com/foae/monorepo-template.git
cd monorepo-template
just check                                   # gofmt, vet, tidy, build, test, sqlc diff

docker run --rm -d --name example-pg -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=example -p 5432:5432 postgres:17

cd backend/services/example
cp .env.example .env
just run-local                               # migrates and listens on :8080

curl -s -X POST localhost:8080/api/v1/items -H 'X-Owner-Id: alice' \
  -H 'Content-Type: application/json' -d '{"name":"first"}'
```

Requires Go 1.27+, [just](https://github.com/casey/just), [sqlc](https://sqlc.dev/) 1.31.x, and a PostgreSQL. Full setup, API examples, and configuration reference: [docs/getting-started.md](docs/getting-started.md).

## Layout

```
backend/            Go module: pkg/ (shared packages) and services/<name>/ (cmd, core, handler, storage)
docs/               Guides and reference (index below)
infra/              Per-component infrastructure configs
justfile            Repo-level recipes: check, version, release
.github/workflows/  CI: gofmt, vet, build, test, tidy, sqlc diff, Docker build
```

## Documentation

| Topic | Document |
|-------|----------|
| Setup, running the example, API, configuration | [docs/getting-started.md](docs/getting-started.md) |
| Adopting the template, everyday commands, dependencies | [docs/using-the-template.md](docs/using-the-template.md) |
| Layer model, module strategy, dependency flow | [docs/architecture.md](docs/architecture.md) |
| Pattern reference (config, logging, shutdown, sqlc, idempotency, ...) | [docs/patterns.md](docs/patterns.md) |
| Creating a new service step by step | [docs/adding-a-service.md](docs/adding-a-service.md) |
| Naming, imports, errors, logging, testing rules | [docs/conventions.md](docs/conventions.md) |
| CI, versioning, and publishing releases | [docs/releasing.md](docs/releasing.md) |
| Contributor workflow summary | [CLAUDE.md](CLAUDE.md) |

## License

[MIT](LICENSE)
