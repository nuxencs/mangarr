# Development

## Prereqs

- Go `1.26.0` (see [`go.mod`](../go.mod))
- Network access for source/API integration
- Chrome/Chromium runtime dependencies for browser-based adapters (Rod)

## Local Setup

```bash
git clone <repo>
cd mangarr
go mod download
```

## Run Commands

```bash
go run . --help
go run . download --help
go run . monitor --help
go run . version
```

## Config

- Monitor mode requires `config.yaml`.

Lookup order when `-c` is not passed:

1. `./config.yaml`
2. `$HOME/.config/mangarr/config.yaml`
3. `$HOME/.mangarr/config.yaml`

Environment overrides use `MANGARR__` prefix:

- `MANGARR__DOWNLOAD_LOCATION`
- `MANGARR__NAMING_TEMPLATE`
- `MANGARR__CHECK_INTERVAL`
- `MANGARR__PPROF_ENABLED`
- `MANGARR__PPROF_ADDRESS`
- `MANGARR__LOG_LEVEL`
- `MANGARR__LOG_PATH`
- `MANGARR__LOG_MAX_SIZE`
- `MANGARR__LOG_MAX_BACKUPS`

## Test + Build Gate

```bash
go test ./...
go test -race ./...
go build ./...
```

Current unit test coverage is focused in:

- `internal/parse`
- `internal/files`
- `internal/download`

For source-adapter behavior, rely on integration-style manual verification against live services.

## Release + CI

- Workflow file: `.github/workflows/release.yml`
- PRs run GoReleaser build validation + Docker build matrix.
- Tag pushes (`v*`) trigger publish.
- Markdown/config-only changes are ignored by workflow path filters.

## Docker

Local compose:

```bash
docker compose up -d
docker compose logs -f
```

Image build uses `ci.Dockerfile` and publishes to GHCR.
