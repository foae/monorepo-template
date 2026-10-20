# Conventions

Rules and standards for code in this repository. These are not suggestions.

Read this before editing Go. Next, open only the relevant [pattern](patterns.md) and linked implementation. Keep documentation declarative: one source of truth per contract, explicit prerequisites and observable verification results.

---

## Naming

- **Exported:** `CamelCase` -- `CreateItem`, `Service`, `ErrItemNotFound`.
- **Unexported:** `camelCase` -- `extractOwnerID`, `mapItemError`, `shuttingDown`.
- **No stuttering:** The package name is already context. `core.Service`, not `core.CoreService`. `handler.Handler`, not `handler.HTTPHandler`.
- **Files:** `snake_case.go`. One file per major concept: `item_create.go`, `item_delete.go`, `worker.go`. Not one massive file per package.
- **Sentinel errors:** `Err` prefix + domain noun + condition. `ErrItemNotFound`, `ErrItemAccessDenied`. Defined in `core/service.go`.

---

## Import Grouping

Three groups separated by blank lines: stdlib, third-party, internal.

```go
import (
    "context"
    "fmt"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/jackc/pgx/v5/pgtype"

    "monorepo-template/backend/services/example/core"
    "monorepo-template/backend/services/example/storage/postgres/sqlc"
)
```

Use `gofmt`; configure `goimports` with `-local monorepo-template/backend` when using it so internal imports remain separate. Keep the required automaxprocs blank import visibly unconditional.

---

## Error Wrapping

Always wrap with short, lowercase context that identifies the call site:

```go
return fmt.Errorf("create item: %w", err)
return fmt.Errorf("mark item deleting: %w", err)
return fmt.Errorf("pg: unable to run migrations: %w", err)
```

- Use `%w` (not `%v`) so `errors.Is()` and `errors.As()` work through the chain.
- Context should be a terse function/operation name, not a sentence.
- To add detail to a sentinel error: `fmt.Errorf("%w: current status is %s", ErrItemNotActive, item.Status)`.

---

## Logging

- **Always use JSON `slog`.** Install the logger before configuration parsing. Only `main` calls `os.Exit`; libraries return errors. The automaxprocs blank import's upstream pre-main diagnostic is the sole plain-text exception.
- **Structured fields, not interpolation:**
  ```go
  // correct
  slog.InfoContext(ctx, "item created", "item_id", item.ID)

  // wrong
  slog.Info(fmt.Sprintf("item %s created for owner %s", item.ID, ownerID))
  ```
- **Use context-bearing logging** whenever a `context.Context` is available. Valid trace/span IDs are correlated in both dev and prod, including grouped loggers.
- **Never log sensitive data:** No credentials, tokens, connection strings, or PII in log messages or structured fields.
- **Log levels:**
  - `Debug` -- verbose operational detail, only active in dev.
  - `Info` -- normal operations: startup, shutdown, successful actions, worker runs.
  - `Warn` -- recoverable problems: config defaults applied, retries, degraded behavior.
  - `Error` -- failures that need attention: DB errors, failed HTTP calls, rollbacks.

---

## Testing

- **Table-driven tests.** Use `[]struct{ name string; ... }` with `t.Run(tc.name, ...)`.
- **Parallel-safe.** Use `t.Parallel()` at the top of each test and subtest unless there is a shared resource that prevents it.
- **Test meaningful failure paths and invariants.** Cover caller-visible errors, cancellation, resource ownership, precedence and boundaries; do not test incidental wording, wiring or copies.
- **No DB mocking unless justified.** For storage-layer tests, prefer a real test database (via `testcontainers` or a local Postgres). If mocking is unavoidable, document why.
- **Deterministic.** No `time.Sleep` in tests, no reliance on wall-clock ordering, no flaky assertions on timing.
- **Test file location:** Same package, `_test.go` suffix. `core/item_create_test.go` tests `core/item_create.go`.

---

## API Responses

- **API responses are JSON** with `Content-Type: application/json`; health/readiness and Prometheus endpoints intentionally use their own formats. The `writeJSON` helper handles API encoding.
- **Success responses:** Entity-specific response DTOs defined in `handler/types.go`. Never return raw sqlc models -- always convert via a helper (e.g., `itemToResponse`).
- **Error responses:** Always use the `ErrorResponse` struct:
  ```json
  {"error": "error_type_snake_case", "message": "Human-readable description"}
  ```
- Never expose arbitrary `err.Error()`, panic values, database errors or connection strings. Use fixed public messages and sanitized internal logging.
- **Empty lists:** Return `[]` (empty JSON array), not `null`. Core methods return `[]T{}` (not nil) for empty results.
- **Delete:** Return `204 No Content` with no body.

---

## Database

- **sqlc for all queries.** No raw SQL strings in Go code. Write queries in `q.sql`, run `sqlc generate`, use the typed functions.
- **Migrations are append-only.** Never edit an existing migration file. To alter a table, add a new numbered migration (e.g., `002_add_column.up.sql`). This is critical -- deployed services track migration versions. Editing a past migration will not re-run it.
- **Down migrations:** Always provide a corresponding `.down.sql` for rollback. Keep them simple and idempotent (`DROP TABLE IF EXISTS`, `ALTER TABLE ... DROP COLUMN IF EXISTS`).
- **UUIDs as primary keys.** Generated server-side via `gen_random_uuid()`.
- **Timestamps:** `TIMESTAMPTZ NOT NULL DEFAULT NOW()` for `created_at` and `updated_at`. Update `updated_at` explicitly in UPDATE queries (`SET updated_at = NOW()`).
- **Unique constraints:** Name them explicitly (`CONSTRAINT myentities_name_owner_unique UNIQUE (name, owner_id)`) to keep schema intent and migration references clear.
- **Indexes:** Create indexes for columns used in WHERE and JOIN clauses. Name them `idx_<table>_<column>`.

---

## HTTP Handlers

Handlers return closures with the signature `func() http.HandlerFunc`:

```go
func (h *Handler) HandleCreateItem() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // ...
    }
}
```

This pattern is used over implementing `http.Handler` directly because:
- It allows per-handler setup code (run once at route registration, not per request) in the outer function if needed.
- It reads cleanly when registering routes: `api.Post("/items", h.HandleCreateItem())`.

**Handler responsibilities (and nothing more):**
1. Extract and validate transport-level inputs (headers, URL params, request body).
2. Call exactly one core method.
3. Map the result to an HTTP response (status code, response body).

If you find yourself writing business logic in a handler (checking state, comparing fields, making decisions beyond "is this input valid HTTP?"), move it to core.

---

## Core Layer Ownership

- Core owns all sentinel errors. Handler only reads them (via `errors.Is`).
- Core owns all state machine transitions and validation logic.
- Core methods accept primitive types and context, not `*http.Request`.
- Core returns domain types (sqlc models or custom structs) and errors. Never `http.StatusCode` or `http.ResponseWriter`.
- If a core method needs data from the request beyond what its parameters provide, add a parameter -- don't pass the request.
