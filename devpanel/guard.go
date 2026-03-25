package devpanel

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// guard returns a Fiber middleware that enforces the panel's Enabled and
// Secret settings. It must be the first handler on every panel route.
//
//   - Enabled == false → 404 (panel does not exist in this environment)
//   - Secret set + wrong/missing token → 401
func (p *DevPanel) guard() fiber.Handler {
	return func(c *fiber.Ctx) error {
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
