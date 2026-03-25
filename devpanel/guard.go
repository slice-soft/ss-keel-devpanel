package devpanel

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

const panelHeader = "X-Keel-Panel"

// guard returns a Fiber middleware that enforces the panel's Enabled and
// Secret settings. It must be the first handler on every panel route.
//
//   - Always sets X-Keel-Panel: true on the response.
//   - Enabled == false → 404 (panel does not exist in this environment)
//   - Secret set + wrong/missing token → 401
func (p *DevPanel) guard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set(panelHeader, "true")
		if !p.cfg.Enabled {
			return c.SendStatus(fiber.StatusNotFound)
		}
		if p.cfg.Secret != "" {
			auth := c.Get("Authorization")
			token, found := strings.CutPrefix(auth, "Bearer ")
			if !found || token != p.cfg.Secret {
				return c.SendStatus(fiber.StatusUnauthorized)
			}
		}
		return c.Next()
	}
}

// GlobalGuard returns an app-level middleware that blocks panel routes before
// they reach the route group. Use it for defence in depth alongside Mount():
//
//	app.Use(panel.GlobalGuard())
//	panel.Mount(app)
//
// When Enabled is false, any request whose path starts with Config.Path
// receives a 404 immediately. Non-panel paths are unaffected.
func (p *DevPanel) GlobalGuard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if strings.HasPrefix(c.Path(), p.cfg.Path) && !p.cfg.Enabled {
			c.Set(panelHeader, "true")
			return c.SendStatus(fiber.StatusNotFound)
		}
		return c.Next()
	}
}
