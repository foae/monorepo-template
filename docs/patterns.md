# Service patterns

Read [conventions](conventions.md) before editing Go. Use this reference for one concern at a time; the linked source is authoritative, not a second copy of the implementation. For the module and layer boundaries, see [architecture](architecture.md).

## Configuration and startup

**Source:** [`cmd/example/main.go`](../backend/services/example/cmd/example/main.go), [`envutil/host.go`](../backend/pkg/envutil/host.go).

1. `main` installs the JSON logger before loading configuration and owns the process exit code.
2. `loadConfig` loads an optional working-directory `.env` without replacing existing environment variables, parses typed fields, and validates modes and durations.
3. Bind the HTTP listener synchronously, before opening database resources. A bind failure returns an error immediately; the process exits 1 rather than waiting for a signal.
4. Initialize optional tracing, PostgreSQL, migrations, core, and handlers. Database startup has a 30-second deadline and observes termination cancellation, including migration lock waits. Cancellation sends PostgreSQL a query-cancel request (bounded to one extra second), then closes the migration transport; closing the socket alone can leave server-side lock waits active. Begin serving only after initialization succeeds.

Libraries return errors; they never call `log.Fatal`, `os.Exit`, or panic for ordinary configuration failures. Errors containing supplied values must not cross the startup logging boundary. See [getting started](getting-started.md#configuration) for the complete environment contract.

`ReplaceHost` returns `(string, error)`. An unset `_HOST` leaves the value unchanged. Overrides preserve URL credentials, scheme, original port, path and query. DNS names, container aliases containing underscores, IPv4, bracketed/unbracketed IPv6, and IPv6 zones are supported. An override's explicit port is ignored; the original URL port wins. An invalid override returns a sanitized error, not a process exit. This helper accepts URLs, not PostgreSQL keyword/value DSNs.

## JSON logging and trace correlation

**Source:** [`observability/logger.go`](../backend/pkg/observability/logger.go), `buildLoggerMiddleware` in the entry point.

Use `observability.NewLogger` and `slog.*Context` when a context exists. `dev` enables Debug and above; `prod` enables Info, Warn and Error. Both modes emit JSON. Valid span contexts add `trace_id` and `span_id` in both modes, including records created after `With` or `WithGroup`.

Request logging retains route templates, status and latency, but omits bodies, headers, raw paths, path parameters, query strings and referrers. Do not add credentials, connection strings, user content or PII as custom attributes. Unknown database errors become generic HTTP errors; internal error text is not a public message. Server-side diagnostics retain safe `error_kind` classifications (such as deadlines, connection refusal, SQLSTATE and gRPC codes), not raw error payloads. Expected 4xx domain failures are not logged as internal errors.

**Deliberate exception:** the required top-level `_ "go.uber.org/automaxprocs"` import runs before `main` and emits its upstream plain-text diagnostic. It is always enabled, including development. Do not disable it by mode, call `maxprocs.Set` a second time, or claim its pre-main line is JSON. Its explicit setting takes precedence over Go's native container-aware dynamic `GOMAXPROCS` updates.

## HTTP middleware and panic boundaries

**Source:** [`httputil/recoverer.go`](../backend/pkg/httputil/recoverer.go), [`httputil/prometheus.go`](../backend/pkg/httputil/prometheus.go).

The OTel wrapper surrounds the chi router. Inside chi: request ID → request logger → metrics → recovery → handler. Recovery sits inside logging and metrics so an uncommitted panic becomes a measured/logged 500, not a false 200. It emits a fixed JSON response without the panic payload. Once headers are committed, it aborts the transport using `http.ErrAbortHandler`, rather than completing a truncated success or appending an error body. Hijacked connections are closed on panic. Panic diagnostics include static function/file/line frames, never panic values or runtime argument dumps.

Recovery is goroutine-local. New background goroutines require their own recovery and completion ownership; HTTP recovery cannot catch their panics. Do not use panic as normal control flow. Forwarded-IP headers are not trusted by default.

## Errors and request validation

**Source:** [`handler/item.go`](../backend/services/example/handler/item.go), [`handler/errors.go`](../backend/services/example/handler/errors.go).

Handlers bound JSON request bodies, reject trailing JSON, validate transport inputs, call core, then translate domain errors using `errors.Is`. Known failures have stable, safe messages; unrecognized errors return `500 internal_server_error` without `err.Error()`. Read [conventions](conventions.md#api-responses) for the response shape.

`X-Owner-Id` is an example trusted-proxy contract, **not authentication**. A deployment must prevent direct untrusted access and have its authentication proxy strip client-supplied owner headers before setting the authenticated identity. `/metrics` also needs an internal network or access policy. Replace this boundary when adopting the template if those conditions do not hold.

## Worker and graceful shutdown

**Source:** [`core/worker.go`](../backend/services/example/core/worker.go), `serve` in the entry point.

The sample worker waits one minute before its first task, then runs sequentially at `WORKER_INTERVAL`. A task panic is logged without its payload and recovered at the task boundary; later tasks can still run. Tasks must honor context cancellation. Do not detach work that outlives the task or hide a canceled context.

On SIGINT/SIGTERM:

1. Mark readiness unavailable and cancel the worker context.
2. Wait `HTTP_DRAIN_DELAY` for endpoint deregistration; HTTP requests remain accepted during this interval.
3. Drain HTTP and join the worker within the **same** `GRACEFUL_SHUTDOWN_TIMEOUT` budget, including the drain delay.
4. Only after both finish, close the database pool. Flush and shut down tracing with an additional maximum of five seconds.

A deadline failure cancels request contexts, closes HTTP connections, leaves dependencies alive for unfinished work until process exit, and exits 1. Set the orchestrator's termination grace above the application budget plus the tracing flush budget. Hijacked connections are not managed by `http.Server.Shutdown`; add explicit ownership before adding WebSockets or other hijacked protocols.

## Context and concurrency ownership

Read this when introducing concurrent work or changing deadlines, cancellation or resource lifetimes; the [worker and shutdown contract](#worker-and-graceful-shutdown) remains authoritative.

- Pass the caller's context through request work. Derive a child when an operation needs a tighter deadline; call its cancel function on every exit path unless ownership is explicitly transferred. In loops, release each iteration's resources promptly rather than accumulating deferred cleanup until the outer function returns.
- Cancellation requests a stop; it does not prove work has finished. Each goroutine needs an owner, an exit mechanism and a way to join it before closing dependencies. Keep work synchronous unless concurrency serves a concrete requirement, and bound concurrent work and queues.
- `context.WithoutCancel` removes cancellation and the deadline; it is not a background-job lifecycle. Work that must survive a request needs an explicit service-owned lifetime, bounds and shutdown ownership. Do not detach worker tasks.
- Assign channel closure to the producer or coordinator that can prove all sends have finished. Multiple producers must finish before closure; receivers do not close channels to request cancellation. Not every channel needs closing.
- A potentially blocking send or receive must have an exit path appropriate to its lifecycle. Do not add a `default` that silently drops required work. Choose buffering from the backpressure contract, not to hide a deadlock.
- Sending a pointer or slice does not copy the referenced data. Choose explicit ownership transfer, immutable sharing or synchronization; copy only when independent ownership requires it. Concurrent map reads are safe without writes; accesses involving writes need synchronization.

## OpenTelemetry: off by default

**Source:** [`observability/otel.go`](../backend/pkg/observability/otel.go).

An empty `OTEL_TRACING_ENDPOINT` leaves the global provider and propagator untouched and creates no exporter. Setting standard `OTEL_EXPORTER_OTLP_ENDPOINT` alone does not enable tracing in this template.

To enable tracing, set `OTEL_TRACING_ENDPOINT=collector:4317`. Use `[IPv6]:4317` for IPv6. The endpoint is a host and port, not a URL. TLS is on by default; set `OTEL_TRACING_INSECURE=true` only for an explicitly trusted plaintext collector, such as a local development collector. This is gRPC OTLP, not HTTP OTLP on port 4318.

`OTEL_TRACING_SAMPLE_RATIO` defaults to `0.1` and must be finite in `[0,1]` when enabled. Parent-based sampling preserves local and remote parent decisions; the ratio governs roots only. W3C `traceparent`/`tracestate` and baggage propagate. At an untrusted ingress, enforce a sampling policy there if clients must not select sampling. Health, readiness and metrics routes do not create spans. Resource fields include service name, version, environment and region. The provider owns its batch processor and exporter, so shutdown flushes once rather than independently shutting down each layer. Both internal SDK diagnostics and async exporter failures use sanitized JSON logging.

For local verification, use a gRPC collector, set ratio `1`, send a known `traceparent` on an API request, then terminate the service. Confirm exported server spans, matching log IDs and flush completion. Regression tests also exercise root/parent sampling, W3C propagation, disabled mode, logger grouping and a blocked collector's shutdown deadline.

## Database, queries and idempotency

**Source:** [`storage/postgres/client.go`](../backend/services/example/storage/postgres/client.go), [`q.sql`](../backend/services/example/storage/postgres/q.sql).

One shared pgx pool serves queries. Both PostgreSQL constructors take a startup context as their first argument. Migration startup owns a separate connection, interrupts its socket on cancellation (including library background-context lock calls), and closes all migration resources; failure closes the new pool. Pool connections and the initial ping have bounded timeouts. Migrations run before HTTP serving. Use append-only migration files and run `just sqlc-generate` after query/schema changes; never hand-edit generated code.

Creation uses the `(name, owner_id)` unique constraint and `ON CONFLICT ... DO UPDATE`: repeats return the same row and refresh `updated_at`, not a byte-identical response. Ownership is checked before get/delete; lists filter by owner in SQL. This small example has no pagination, state machine, external provisioning or rollback orchestration. Add those only when the application's contract requires them. For multi-step atomic operations, use pgx transactions and `WithTx`; do not close a transaction-bound client because it shares the owning pool.

## Build and delivery boundary

**Source:** [`backend/.dockerignore`](../backend/.dockerignore), [`Dockerfile`](../backend/services/example/Dockerfile), [CI](../.github/workflows/ci.yml).

Docker context is `backend/`. The allowlist admits module metadata, Go sources and embedded SQL migrations; it excludes vendor trees, local dependencies, binaries, tests and secret/config files, including nested `.env` files and directories. Explicitly extend the allowlist when adding other `go:embed` assets. The image is a static binary plus CA certificates in `scratch`, running as UID/GID 65532. Inject environment at runtime; never bake `.env` into the image.

Go and build images are version-pinned, images by digest and GitHub actions by commit. Refresh all linked pins together. `just check` is the Go/SQL verification gate; CI additionally builds the container. See [releasing](releasing.md) for publishing, which is separate from local verification.
