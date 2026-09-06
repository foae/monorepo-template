# Repository-level recipes: verification and release management.
# Per-module recipes live in backend/justfile; per-service ones in backend/services/<name>/justfile.

# List available recipes.
default:
    @just --list

# Run every check CI runs (format, vet, tidy, build, test, sqlc diff).
check:
    #!/usr/bin/env bash
    set -euo pipefail
    cd backend
    unformatted=$(gofmt -l .)
    if [[ -n "$unformatted" ]]; then echo "unformatted files:"; echo "$unformatted"; exit 1; fi
    go vet ./...
    before=$(cat go.mod go.sum | sha256sum)
    go mod tidy
    if [[ "$before" != "$(cat go.mod go.sum | sha256sum)" ]]; then echo "go mod tidy changed go.mod/go.sum; review and commit the result"; exit 1; fi
    go build ./...
    go test -race ./...
    sqlc diff -f services/example/storage/postgres/sqlc.yaml
    echo "all checks passed"

# Print the latest release tag (or "none").
version:
    @git describe --tags --abbrev=0 --match 'v*' 2>/dev/null || echo none

# Print the next version for a bump level: patch (default), minor or major.
next-version level="patch":
    #!/usr/bin/env bash
    set -euo pipefail
    cur=$(git describe --tags --abbrev=0 --match 'v*' 2>/dev/null || echo v0.0.0)
    IFS=. read -r major minor patch <<< "${cur#v}"
    case "{{level}}" in
        major) major=$((major+1)); minor=0; patch=0 ;;
        minor) minor=$((minor+1)); patch=0 ;;
        patch) patch=$((patch+1)) ;;
        *) echo "level must be patch, minor or major" >&2; exit 1 ;;
    esac
    echo "v$major.$minor.$patch"

# Tag and publish a release: verifies a clean, pushed main; runs checks;
# creates an annotated tag; pushes it; publishes a GitHub release with generated notes.
#   just release v1.2.3
release version:
    #!/usr/bin/env bash
    set -euo pipefail
    v="{{version}}"
    [[ "$v" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "version must look like v1.2.3, got '$v'"; exit 1; }
    branch=$(git rev-parse --abbrev-ref HEAD)
    [[ "$branch" == "main" ]] || { echo "release from main only (on '$branch')"; exit 1; }
    [[ -z "$(git status --porcelain)" ]] || { echo "working tree is not clean"; exit 1; }
    git fetch --quiet origin main --tags
    [[ "$(git rev-parse HEAD)" == "$(git rev-parse origin/main)" ]] || { echo "local main differs from origin/main; push or pull first"; exit 1; }
    if git rev-parse -q --verify "refs/tags/$v" >/dev/null || git ls-remote --exit-code --tags origin "refs/tags/$v" >/dev/null 2>&1; then
        echo "tag $v already exists; published tags are never moved"; exit 1
    fi
    just check
    prev=$(git describe --tags --abbrev=0 --match 'v*' 2>/dev/null || true)
    git tag -a "$v" -m "Release $v"
    git push origin "$v"
    if [[ -n "$prev" ]]; then
        gh release create "$v" --title "$v" --generate-notes --notes-start-tag "$prev" --verify-tag
    else
        gh release create "$v" --title "$v" --generate-notes --verify-tag
    fi
    echo "released $v"

# Bump and release in one step: just release-next patch|minor|major
release-next level="patch":
    just release "$(just next-version {{level}})"
