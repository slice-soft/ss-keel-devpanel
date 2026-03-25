# Contributing to ss-keel-devpanel

The base contributing guide — workflow, commit conventions, PR guidelines, and community standards — lives in [ss-community](https://github.com/slice-soft/ss-community/blob/main/CONTRIBUTING.md). Read it first.

This document covers only what is specific to this repository.

---

## Requirements

- Go 1.25+
- Git
- [`templ`](https://templ.guide) CLI (for template changes)

## Getting started

```bash
git clone https://github.com/slice-soft/ss-keel-devpanel.git
cd ss-keel-devpanel
go mod download
```

## Running tests

```bash
# All tests with race detector (required before every PR)
go test -race ./...

# Verbose output
go test -race -v ./devpanel/...
```

## Working with templ templates

The `devpanel/ui/*.templ` files are Go HTML templates compiled by `templ`.
After editing any `.templ` file, regenerate the `*_templ.go` files:

```bash
templ generate
```

The generated `*_templ.go` files must be committed alongside the `.templ` source.

## Repository structure

```
ss-keel-devpanel/
├── devpanel/
│   ├── ui/                  # Templ templates + generated Go files
│   ├── assets/              # Embedded static files (htmx, CSS)
│   ├── devpanel.go          # Core struct, lifecycle, contracts
│   ├── config.go            # Config type with defaults
│   ├── guard.go             # Enabled/Secret middleware + GlobalGuard
│   ├── middleware.go        # Request capture middleware
│   ├── logger.go            # PanelLogger + logBuffer
│   ├── sse.go               # Generic sseBroadcaster[T]
│   ├── addon_registry.go    # Per-addon goroutine + stream
│   ├── addon_sse.go         # Addon SSE + detail handlers
│   ├── logs_sse.go          # Logs page + SSE stream
│   ├── config_view.go       # Config page handler
│   └── panel_routes.go      # Mount() — all routes registered here
├── .github/workflows/
│   ├── ci.yml               # PR validation
│   └── release.yml          # release-please + tag
└── go.mod
```

## Commit conventions

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(config): add env var redaction for secret values
fix(guard): return 404 instead of 403 when disabled
test(e2e): add full panel flow integration test
docs: update README with security guidelines
```

## Creating a branch

```bash
git checkout -b feat/your-feature-name
```

Open a PR against `main`. The CI pipeline runs `go test -race ./...` and `go vet ./...` automatically.
