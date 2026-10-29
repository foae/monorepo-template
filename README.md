# monorepo-template

A monorepo template for Go backend services: one Go module, shared packages, and a fully wired example service that shows the conventions every new service follows. Copy it, rename the module, and add services by cloning the example.

The example service ships with a chi HTTP API, PostgreSQL via pgx and sqlc, embedded migrations, env-based config, slog JSON logging, Prometheus metrics, optional OpenTelemetry tracing, health and readiness endpoints, graceful shutdown, a background worker, a non-root Dockerfile, and `just` recipes for build, test, code generation, Docker, and releases.

## Start here: readers and agents

1. Run the example using [getting started](docs/getting-started.md).
2. Before changing Go, read [conventions](docs/conventions.md#read-by-task) and follow its task-specific links. Source links in [patterns](docs/patterns.md) are the implementation authority.
3. To adopt the repository, follow [using the template](docs/using-the-template.md); to add a service, follow [adding a service](docs/adding-a-service.md).
4. Verify with `just check` and a real service/container smoke run before publishing via [releasing](docs/releasing.md).

The example trusts `X-Owner-Id` from an authentication proxy; it is **not a public authentication system**. Prevent direct access, replace untrusted identity headers, and keep metrics private before deployment.

## Quick start

```bash
git clone https://github.com/foae/monorepo-template.git
cd monorepo-template
just check                                   # gofmt, vet, tidy, build, race tests, sqlc vet/diff

docker run --rm -d --name example-pg -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=example -p 127.0.0.1:5432:5432 postgres:18.6-trixie
docker exec example-pg pg_isready -h 127.0.0.1 -U postgres -d example
# Wait for TCP readiness before continuing.

cd backend/services/example
cp .env.example .env
just run-local                               # migrates and listens on :8080

curl -s -X POST localhost:8080/api/v1/items -H 'X-Owner-Id: alice' \
  -H 'Content-Type: application/json' -d '{"name":"first"}'
```

Requires Go 1.27.3, [just](https://github.com/casey/just) 1.59.0, [sqlc](https://sqlc.dev/) 1.31.1, and PostgreSQL. Full setup, API examples, and configuration reference: [docs/getting-started.md](docs/getting-started.md).

## Layout

```
backend/            Go module: pkg/ (shared packages) and services/<name>/ (cmd, core, handler, storage)
docs/               Guides and reference (index below)
infra/              Per-component infrastructure configs
justfile            Repo-level recipes: check, version, release
.github/workflows/  CI: Go/SQL checks above, plus Docker build
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
| On-demand debugging, refactoring, safety and trust-boundary checks | [docs/go-workflows.md](docs/go-workflows.md) |
| CI, versioning, and publishing releases | [docs/releasing.md](docs/releasing.md) |
| Contributor workflow summary | [CLAUDE.md](CLAUDE.md) |

## License

[MIT](LICENSE)
