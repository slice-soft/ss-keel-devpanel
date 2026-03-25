package devpanel

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const requestBufferSize = 256

// RequestMiddleware returns a Fiber middleware that captures incoming HTTP
// requests into the panel's ring buffer.
//
// Requests whose path starts with the panel's own Config.Path are ignored
// so the panel does not record its own traffic.
func (p *DevPanel) RequestMiddleware() fiber.Handler {
	buf := newRequestBuffer(requestBufferSize)
	p.requests = buf

	return func(c *fiber.Ctx) error {
		path := c.Path()

		// Skip panel's own routes.
		if strings.HasPrefix(path, p.cfg.Path) {
			return c.Next()
		}

		start := time.Now()
		err := c.Next()
		latency := time.Since(start)

		requestID := c.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		buf.push(RequestEntry{
			ID:        uuid.New().String(),
			Timestamp: start,
			Method:    c.Method(),
			Path:      path,
			Status:    c.Response().StatusCode(),
			Latency:   latency,
			RequestID: requestID,
		})

		return err
	}
}

// Requests returns a snapshot of captured requests, oldest first.
func (p *DevPanel) Requests() []RequestEntry {
	if p.requests == nil {
		return nil
	}
	return p.requests.snapshot()
}
