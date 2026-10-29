# Go change workflows

Read only the section relevant to the task. [Conventions](conventions.md) define repository rules; [patterns](patterns.md) link to implementation contracts. These procedures add investigation steps, not a second architecture or a general Go handbook.

## Debugging

Use when a test fails, a request behaves incorrectly, or work hangs or races.

1. Record expected and actual behavior, the failing input or command, and relevant environment. Preserve supplied failure evidence; do not rerun merely to confirm a reported failure. Isolate a reproduction when needed to distinguish causes.
2. Trace the request or value through callers, middleware, core and storage. Check existing validation and ownership before declaring a local snippet defective. Read resolved dependency source when its behavior matters.
3. Form one testable hypothesis. Choose the smallest observation that distinguishes it from alternatives, then change one cause at a time. Do not hide a failure with retries, ignored errors or extra guards without establishing the violated contract.
4. Use focused diagnostics. For example, from `backend/`, this runs the HTTP utility package without cached results:

   ```sh
   go test -race -count=1 ./pkg/httputil
   ```

   Substitute the affected package and use `-run` to select a test. For suspected order dependence, run that package with `-shuffle=on -count=10`; record the reported seed and replay with `-shuffle=<seed>`. Vary `-parallel` only when investigating shared test state; a passing serial run does not fix an unsafe parallel test. The race detector covers executed paths, not all possible interleavings.
5. For hangs, inspect goroutine stacks and identify the blocked operation, its owner and its expected release. Escalate to a debugger or targeted runtime diagnostics only when simpler evidence is insufficient. Stack dumps and profiles can expose sensitive data: keep them private and sanitized before sharing. This template does not expose a pprof endpoint; never assume one exists or add an unprotected diagnostic listener.
6. Verify the failing scenario after the fix. Keep a deterministic regression test when it captures a meaningful consumer-visible failure or invariant; follow the existing [testing rules](conventions.md#testing). Exercise changed runtime behavior as well as tests, then run `just check` from the repository root. State exactly what was observed and any verification limits.

## Refactoring

Use when changing names, signatures, interfaces or package boundaries.

1. Identify the behavior that must remain stable and the affected [layer contracts](architecture.md). Separate structural movement from intentional behavior changes; do not use the refactor to introduce a new architecture, dependency or compatibility policy.
2. Use available Go semantic tooling for definitions, references, implementations and renames. Check interface consumers, embedding and build-tag/platform variants explicitly: a static call hierarchy is not proof of every runtime caller, and results depend on the loaded build configuration. If tooling is unavailable, record that limit and inspect callers plus compiler feedback rather than treating a text match as semantic proof.
3. Migrate all affected callers, tests and documentation. Remove obsolete paths; do not leave aliases or wrappers unless compatibility is an explicit requirement. Regenerate sqlc output through the [documented workflow](patterns.md#database-queries-and-idempotency), never by hand.
4. Verify the affected behavior and run the repository gate. Coverage can locate unexercised code but cannot establish assertion quality or behavior preservation. Avoid new tests that only mirror forwarding or incidental implementation details.

## Targeted safety checks

Use only the checks implicated by the change, not as an obligatory whole-repository audit.

- **Interfaces and nil:** a typed nil pointer stored in an interface is not a nil interface. Check absence at the boundary where it is represented; do not return a typed nil error as success. A nil receiver can be valid when its method explicitly supports it.
- **Mutable data:** determine whether a returned or retained slice/map is borrowed, transferred or independently owned. Sub-slices and `append` can alias a backing array; limiting capacity prevents an append from reusing spare capacity but does not isolate existing elements. Clone when independent mutation or release of a large retained backing array requires it, not for every exported return value. Slice/map clones are shallow.
- **Copy safety:** do not copy a value containing a mutex, `sync.Once` or other no-copy state after use. Check receiver and parameter choices when changing such a type; consult the type's contract and `go vet` diagnostics.
- **Resources and cancellation:** identify who releases each successfully acquired resource, including partial-initialization failures. Use the [lifecycle rules](patterns.md#context-and-concurrency-ownership); cancellation alone does not permit closing dependencies still in use.
- **Numeric boundaries:** validate range before narrowing an externally supplied or computed integer when overflow would violate the contract. Choose floating-point tolerance from the domain rather than imposing a universal epsilon.

## Trust-boundary review

Use when changing input handling, identity, authorization, file/network access or externally visible output.

1. Identify what an untrusted caller controls and follow it to the sensitive operation. Include ownership checks, query scoping and response/log output, not just parsing. The example's [trusted-proxy requirements](patterns.md#errors-and-request-validation) are prerequisites, not an authentication implementation.
2. Verify protections in the actual call path and the layer that owns each invariant. Do not report a vulnerability from a suspicious fragment alone or require redundant validation in every layer. If exploitation depends on a missing deployment guarantee, state that condition explicitly.
3. Check failure paths against the existing [public-response](conventions.md#api-responses) and [logging](conventions.md#logging) contracts. Use adversarial boundary cases tied to the changed behavior; do not install scanners or expand into a full security audit as a side effect.

## Sources and maintenance

These are repository-owned adaptations, not installed upstream skills. See [selection rationale, pinned sources and license](go-guidance-sources.md) only when reviewing provenance or updating the guidance. Updates are deliberate reviews against current code and conventions, not automatic synchronization.
