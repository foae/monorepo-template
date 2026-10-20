# Adding a service

**Goal:** produce a runnable service with the example's configuration, safety and lifecycle contracts intact. First read [conventions](conventions.md); consult [patterns](patterns.md) only for the concern you are changing.

## 1. Copy a clean example

From the repository root:

```bash
cp -r backend/services/example backend/services/myservice
```

Do not copy a developer's `.env`, binaries or other local artifacts into the new service. `backend/` remains the only Go module; do not run `go mod init` in a service.

## 2. Rename the wiring

Within `backend/services/myservice/`:

- Rename `cmd/example/` to `cmd/myservice/`.
- Replace `services/example` in imports, including tests, with `services/myservice`.
- Set `SERVICE_NAME := "myservice"` in the service justfile; retain the shared `service.just` import.
- Update the Dockerfile's build target, output binary, copy and entrypoint names. Keep its build context at `backend/`, not the service directory.
- Update `SERVICE_NAME`, database name and any service-specific defaults in `main.go` and `.env.example`.

Check for remaining `example` references and decide each one; do not blindly rename dependency/module names.

## 3. Define schema and queries

For a new, never-deployed service, replace the copied initial schema and matching down migration with your entity. Once deployed, migrations are append-only.

Edit `storage/postgres/q.sql`, then run from the new service directory:

```bash
just sqlc-generate
```

Commit the generated files. Check `sqlc.yaml` package/output paths and all `go:embed` inputs. SQL migrations are already admitted by `backend/.dockerignore`; other embedded assets require explicit allowlist entries.

## 4. Implement core and transport

- Core owns validation, ownership rules, domain errors and database operations. It accepts contexts and domain inputs, not HTTP types.
- Handlers parse bounded inputs, invoke core, and map errors to safe JSON responses. Retain fixed messages for unknown errors; never expose `err.Error()`.
- Update response DTOs, routes, error mappings and behavior tests. Remove example-only endpoints and obsolete tests rather than retaining dead compatibility paths.
- Keep `X-Owner-Id` only behind an authentication proxy that strips untrusted values and prevents direct access. Otherwise implement the project's actual authentication boundary before deployment.

Do not claim the example supplies pagination, user authentication, distributed locks or a state machine: those are application-specific contracts.

## 5. Preserve lifecycle ownership

Keep the top-level automaxprocs blank import. Its pre-main plain-text log is the documented upstream exception; all service-owned startup/runtime logs remain JSON. Libraries return errors; only `main` selects the process exit status.

Keep synchronous listener binding, bounded HTTP timeouts, safe recovery, readiness drain and completion joins. Retain the middleware order in the example. If the service needs no background work, remove the worker implementation and its launch/join coherently; do not leave an unowned goroutine. Every new goroutine needs cancellation and a completion owner; task panic recovery belongs inside that goroutine.

Tracing stays disabled until the service-specific endpoint is set. Pass request contexts through core/database calls so tracing, cancellation and log correlation remain effective.

## 6. Verify the runnable service

From the new service directory:

```bash
cp .env.example .env
# Set a reachable PostgreSQL URL in .env.
just run-local
```

In another terminal, exercise health/readiness, real CRUD, ownership denial, malformed input, database errors and SIGTERM shutdown. Verify dev Debug output, prod Info filtering, JSON error responses, and nonzero exit on an occupied listener. Do not add production panic endpoints merely for smoke testing.

From the repository root, run `just check`. Go checks discover all services; `sqlc vet` and `sqlc diff` discover every `sqlc.yaml` beneath `backend/services/`. CI uses the same gate.

Run `just docker-build` from the new service and start the image against a real database with runtime configuration. Add a CI image-build job for this service if it is independently deployed; the existing image job builds only `example`.

## 7. Update the project documentation

Document new configuration in `.env.example` and the service's operator guide, including authentication/network assumptions, migrations and shutdown budgets. Update the root entry point to route readers to the service. Keep implementation details in source and link them rather than duplicating long code blocks. Follow [releasing](releasing.md) when publishing consumed template changes.
