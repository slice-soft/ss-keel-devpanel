<img src="https://cdn.slicesoft.dev/boat.svg" width="400" />

# ss-keel-devpanel

Observability dev panel for [Keel](https://keel-go.dev) applications.
Provides a real-time UI to inspect requests, logs, addon events, routes, and configuration — all embedded in your Go binary.

[![CI](https://github.com/slice-soft/ss-keel-devpanel/actions/workflows/ci.yml/badge.svg)](https://github.com/slice-soft/ss-keel-devpanel/actions)
[![Release](https://img.shields.io/github/v/release/slice-soft/ss-keel-devpanel)](https://github.com/slice-soft/ss-keel-devpanel/releases)
![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)
[![Go Report Card](https://goreportcard.com/badge/github.com/slice-soft/ss-keel-devpanel)](https://goreportcard.com/report/github.com/slice-soft/ss-keel-devpanel)
[![Go Reference](https://pkg.go.dev/badge/github.com/slice-soft/ss-keel-devpanel.svg)](https://pkg.go.dev/github.com/slice-soft/ss-keel-devpanel)
![License](https://img.shields.io/badge/License-MIT-green)
![Made in Colombia](https://img.shields.io/badge/Made%20in-Colombia-FCD116?labelColor=003893)

---

## Features

- **Requests** — ring buffer (256 entries) with method, path, status, latency, request ID
- **Logs** — ring buffer (512 entries) with level filter, search, and live SSE stream
- **Addon events** — per-addon SSE stream from any `contracts.Debuggable` addon
- **Routes** — registered Fiber routes at a glance
- **Config** — env vars (secrets auto-redacted), Go runtime stats, addon manifests
- **Guard** — `Enabled` flag + optional Bearer token; returns 404 when disabled
- **Rate limiting** — 120 req/min per IP on all panel endpoints
- **Header** — every panel response includes `X-Keel-Panel: true`
- **Zero external deps** — htmx and CSS embedded via `//go:embed`

---

## 🚀 Installation

```bash
keel add devpanel
```

The CLI will:
- Add `ss-keel-devpanel` to your Go module
- Create `cmd/setup_devpanel.go` with the initialization function
- Inject one line into `cmd/main.go` before your modules are registered
- Add `KEEL_PANEL_ENABLED`, `KEEL_PANEL_SECRET`, and `KEEL_PANEL_PATH` to `.env`

Or manually:

```bash
go get github.com/slice-soft/ss-keel-devpanel
```

---

## Quick start

```go
import (
    "github.com/gofiber/fiber/v2"
    "github.com/slice-soft/ss-keel-devpanel/devpanel"
)

func main() {
    panel := devpanel.New(devpanel.Config{
        Enabled: true,                   // set false in production
        Secret:  os.Getenv("PANEL_SECRET"), // optional Bearer token
        Path:    "/keel/panel",          // default path
    })
    defer panel.Shutdown()

    app := fiber.New()

    // Capture incoming requests (place before your routes).
    app.Use(panel.RequestMiddleware())

    // Optional: defence-in-depth guard at app level.
    app.Use(panel.GlobalGuard())

    // Mount the panel UI.
    panel.Mount(app)

    // Your application routes.
    app.Get("/api/users", handleUsers)

    app.Listen(":3000")
}
```

Open `http://localhost:3000/keel/panel` in your browser.

---

## Logger

Use `PanelLogger` to associate logs with requests in the panel:

```go
logger := panel.Logger()

app.Use(func(c *fiber.Ctx) error {
    reqLogger := logger.WithRequestID(c.Get("X-Request-ID"))
    c.Locals("logger", reqLogger)
    return c.Next()
})

// In a handler:
log := c.Locals("logger").(*devpanel.PanelLogger)
log.Info("user fetched: %s", userID)
```

---

## Addon integration

Any addon implementing `contracts.Debuggable` can self-register:

```go
func (a *MyAddon) Register(app *keel.App) error {
    if panel, ok := app.GetAddon("devpanel").(contracts.PanelRegistry); ok {
        panel.RegisterAddon(a)
    }
    return nil
}
```

Addons that also implement `contracts.Manifestable` expose version, capabilities, resources, and env vars on the Config page.

---

## Configuration

| Field     | Type   | Default        | Description                              |
|-----------|--------|----------------|------------------------------------------|
| `Enabled` | bool   | `false`        | Enable the panel. Set `false` in prod.   |
| `Secret`  | string | `""`           | Bearer token. Empty = no auth required.  |
| `Path`    | string | `/keel/panel`  | URL prefix for all panel routes.         |

Env vars declared in the panel's own manifest:

| Key                  | Secret | Default        |
|----------------------|--------|----------------|
| `KEEL_PANEL_ENABLED` | no     | `true`         |
| `KEEL_PANEL_SECRET`  | yes    | *(empty)*      |
| `KEEL_PANEL_PATH`    | no     | `/keel/panel`  |

---

## Security

- **Always set `Enabled: false` in production** unless you intend to expose the panel.
- Use `Secret` with a strong random token and HTTPS when exposing the panel.
- `GlobalGuard()` provides an extra layer at the app middleware level (defence in depth).
- Secret env var values are automatically redacted (`••••••••`) on the Config page.
- Addon event `Detail` fields are displayed as-is — addons must not put sensitive data in events.

---

## 🤚 CI/CD and releases

Every PR runs `go test -race ./...` and `go vet ./...` via the reusable pipeline in `ss-pipeline`. Releases are created automatically by `release-please` on every merge to `main`.

---

## 💡 Recommendations

- **Disable in production** — set `KEEL_PANEL_ENABLED=false` or `Enabled: false` in all production environments.
- **Protect with a secret** — set `KEEL_PANEL_SECRET` to a strong random token and serve the app over HTTPS whenever the panel is enabled.
- **Register your addons** — any addon implementing `contracts.Debuggable` can self-register with the panel to stream live events; addons implementing `contracts.Manifestable` also appear in the Config tab.

---

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for local setup.
The base workflow, commit conventions, and community standards live in [ss-community](https://github.com/slice-soft/ss-community/blob/main/CONTRIBUTING.md).

## Community

| Document | |
|---|---|
| [CONTRIBUTING.md](https://github.com/slice-soft/ss-community/blob/main/CONTRIBUTING.md) | Workflow, commit conventions, and PR guidelines |
| [GOVERNANCE.md](https://github.com/slice-soft/ss-community/blob/main/GOVERNANCE.md) | Decision-making, roles, and release process |
| [CODE_OF_CONDUCT.md](https://github.com/slice-soft/ss-community/blob/main/CODE_OF_CONDUCT.md) | Community standards |
| [VERSIONING.md](https://github.com/slice-soft/ss-community/blob/main/VERSIONING.md) | SemVer policy and breaking changes |
| [SECURITY.md](https://github.com/slice-soft/ss-community/blob/main/SECURITY.md) | How to report vulnerabilities |

## License

MIT License — see [LICENSE](LICENSE) for details.

## Links

- Website: [keel-go.dev](https://keel-go.dev)
- Documentation: [docs.keel-go.dev](https://docs.keel-go.dev)
- GitHub: [github.com/slice-soft/ss-keel-devpanel](https://github.com/slice-soft/ss-keel-devpanel)

---

Made by [SliceSoft](https://slicesoft.dev) — Colombia
