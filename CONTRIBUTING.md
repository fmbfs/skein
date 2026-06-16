# Contributing to skein

Thanks for your interest. skein is in early development, so the architecture
is still settling. **Open an issue before starting significant work** — it
saves you from building something that conflicts with in-flight design.

## Vocabulary

skein uses a consistent textile vocabulary. Please use these terms in code,
comments, and docs:

| Concept | Term |
|---------|------|
| Focal symbol | thread |
| Navigate to node | follow |
| Navigation history | spool |
| Pinned collection | bundle |
| Blank start state | tangle |
| Depth | ply |
| Fast mode | draw |

## Development setup

```bash
git clone https://github.com/fmbfs/skein
cd skein
go mod download
go build ./cmd/skein

# Run tests
go test ./internal/...           # unit
go test ./tests/e2e/...          # E2E (needs clangd installed)

# Lint
golangci-lint run
```

## Requirements for development

- Go >= 1.22
- clangd >= 14 (for E2E tests)
- cmake + bear (for generating fixture compile_commands.json)

## Branch naming

```
feat/short-description
fix/short-description
chore/short-description
docs/short-description
```

## Commit convention

Conventional commits, enforced by CI:

```
feat(lsp): add callHierarchy/incomingCalls support
fix(tui): correct spool back-navigation on empty history
chore(ci): pin golangci-lint to v1.59
```

Scopes: `lsp`, `compositor`, `tui`, `draw`, `ci`, `docs`.

## Pull requests

- Fill out the PR template.
- All CI jobs must pass (lint, unit, E2E, build, audit).
- Coverage must stay at or above 85% (unit + E2E merged).
- New public functions and types need doc comments.
- Update `CHANGELOG.md` under `[Unreleased]`.
- Update `docs/SPEC.md` if behaviour changes.
- Sign your commits (`git commit -S`).

## Code style

- Standard `gofmt` + `goimports`.
- `golangci-lint` config is in `.golangci.yml`; it is the source of truth.
- Comments end with a period (enforced by `godot`).
