# Continuous Integration and Releases

`.github/workflows/ci.yml` runs on every push to `main`, every pull request, and every `v*` tag: gofmt, `go vet`, `go build`, `go test -race`, a `go mod tidy` cleanliness check, `sqlc diff`, and a Docker build of the example service.

Releases are SemVer tags on `main` published as GitHub releases. `just release-next patch|minor|major` (or `just release vX.Y.Z`) verifies the tree, runs the checks, creates an annotated tag, pushes it, and publishes the release with generated notes. Published tags are never moved.

## Release steps

1. Land the change on `main` and wait for CI to pass.
2. Choose the bump: `patch` for fixes and docs, `minor` for backward-compatible additions, `major` for changes that require consumers of the template to restructure.
3. Run `just release-next <patch|minor|major>`, or `just release vX.Y.Z` for an explicit version. The recipe refuses to run on a dirty tree, off `main`, when `main` is not in sync with `origin/main`, or when the tag already exists.
4. Confirm with `gh release view <tag>`.

`just version` prints the latest tag; `just next-version <level>` previews the next one.
