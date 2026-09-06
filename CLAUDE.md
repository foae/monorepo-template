# Working in this repository

## Layout

- `backend/` is the single Go module (`monorepo-template/backend`). Shared packages live in `backend/pkg/`, services in `backend/services/<name>/`.
- `docs/` holds the architecture, pattern reference, conventions and the guide for adding a service. Read `docs/conventions.md` before changing Go code.
- `infra/` holds per-component infrastructure configs.

## Verification

Run `just check` from the repo root before committing. It runs exactly what CI runs: gofmt, go vet, go mod tidy (must be clean), go build, go test with the race detector, and `sqlc diff`.

After editing `q.sql` or a migration, regenerate with `just sqlc-generate` from `backend/services/example/` (or the service in question) and commit the generated files.

## Versioning and releases

The repository is versioned as a whole with SemVer tags (`vMAJOR.MINOR.PATCH`). There are no independently versioned components. Every change that lands on `main` and is meant to be consumed must be followed by a release:

1. Merge or push the change to `main` and wait for CI to pass.
2. Pick the bump: `patch` for fixes and doc-only changes, `minor` for new patterns, packages or recipes that stay backward compatible, `major` for changes that require consumers of the template to restructure (layer model, module layout, justfile contract).
3. Run `just release-next <patch|minor|major>` (or `just release vX.Y.Z` for an explicit version). The recipe refuses to run on a dirty tree, off `main`, when local `main` differs from `origin/main`, or when the tag already exists. It runs `just check`, creates an annotated tag, pushes it and publishes a GitHub release with generated notes.
4. Confirm with `gh release view <tag>`.

Never move or delete a published tag. If a release is wrong, publish a new patch version.

`just version` prints the latest tag and `just next-version <level>` previews the next one without releasing.

## Commit hygiene

- No AI or tool attribution in commit messages, trailers or code comments.
- Keep secrets and private infrastructure details out of the repository. Local configuration goes in `.env` files or `.private/`, both git-ignored; commit sanitized `.env.example` files instead.
