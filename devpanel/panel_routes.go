package devpanel

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/slice-soft/ss-keel-devpanel/devpanel/ui"
)

//go:embed assets/*
var assetsFS embed.FS

// Mount registers all panel routes on the given Fiber router under Config.Path.
// Call this after creating the DevPanel and before starting the server.
//
//	panel := devpanel.New(cfg)
//	panel.Mount(app.Fiber())
func (p *DevPanel) Mount(router *fiber.App) {
	p.fiberApp = router
	g := router.Group(p.cfg.Path, p.guard())

	// Rate limiter — 120 requests per minute per IP for panel endpoints.
	g.Use(limiter.New(limiter.Config{
		Max:        120,
		Expiration: 1 * time.Minute,
	}))

	// Static assets — served under <path>/assets/
	sub, _ := fs.Sub(assetsFS, "assets")
	g.Use("/assets", filesystem.New(filesystem.Config{
		Root: http.FS(sub),
	}))

	// Pages
	g.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect(p.cfg.Path + "/requests")
	})
	g.Get("/requests", p.handleRequests())
	g.Get("/logs", p.handleLogs())
	g.Get("/logs/stream", p.handleLogsStream())
	g.Get("/routes", p.handleRoutes())
	g.Get("/addons", p.handleAddons())
	g.Get("/addons/:id", p.handleAddonDetail())
	g.Get("/addons/:id/stream", p.handleAddonStream())
	g.Get("/config", p.handleConfig())
}

// render writes a templ component to the Fiber response.
func render(c *fiber.Ctx, component templ.Component) error {
	c.Set("Content-Type", "text/html; charset=utf-8")
	return component.Render(c.Context(), c.Response().BodyWriter())
}

func (p *DevPanel) buildNav(active string) []ui.NavItem {
	base := p.cfg.Path
	items := []struct{ label, path string }{
		{"Requests", base + "/requests"},
		{"Logs", base + "/logs"},
		{"Routes", base + "/routes"},
		{"Addons", base + "/addons"},
		{"Config", base + "/config"},
	}
	nav := make([]ui.NavItem, len(items))
	for i, it := range items {
		nav[i] = ui.NavItem{Label: it.label, Path: it.path, Active: it.label == active}
	}
	return nav
}

func (p *DevPanel) assetBase() string {
	return p.cfg.Path + "/assets"
}

func (p *DevPanel) handleRequests() fiber.Handler {
	return func(c *fiber.Ctx) error {
		entries := p.Requests()
		rows := make([]ui.RequestRow, len(entries))
		for i, e := range entries {
			rows[i] = ui.RequestRow{
				ID:        e.ID,
				Timestamp: e.Timestamp,
				Method:    e.Method,
				Path:      e.Path,
				Status:    e.Status,
				LatencyMS: float64(e.Latency.Microseconds()) / 1000.0,
				RequestID: e.RequestID,
			}
		}
		return render(c, ui.Requests(p.buildNav("Requests"), rows, p.assetBase()))
	}
}

func (p *DevPanel) handleRoutes() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var rows []ui.RouteRow
		if p.fiberApp != nil {
			for _, r := range p.fiberApp.GetRoutes(true) {
				if strings.HasPrefix(r.Path, p.cfg.Path) {
					continue
				}
				rows = append(rows, ui.RouteRow{
					Method: r.Method,
					Path:   r.Path,
				})
			}
		}
		return render(c, ui.Routes(p.buildNav("Routes"), rows, p.assetBase()))
	}
}

func (p *DevPanel) handleAddons() fiber.Handler {
	return func(c *fiber.Ctx) error {
		registered := p.Addons()
		rows := make([]ui.AddonRow, len(registered))
		for i, a := range registered {
			rows[i] = addonToRow(a, p.cfg.Path)
		}
		return render(c, ui.Addons(p.buildNav("Addons"), rows, p.assetBase()))
	}
}
