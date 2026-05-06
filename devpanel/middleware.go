package devpanel

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	keelcore "github.com/slice-soft/ss-keel-core/core"
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

		// Skip panel's own routes and the parent group prefix (Fiber generates
		// an internal redirect from /keel/ → /keel/panel before mounting).
		panelBase := strings.TrimSuffix(p.cfg.Path, "/")
		if path == panelBase || strings.HasPrefix(path, panelBase+"/") {
			return c.Next()
		}
		if idx := strings.LastIndex(panelBase, "/"); idx > 0 {
			groupBase := panelBase[:idx+1]
			if path == groupBase || path == panelBase[:idx] {
				return c.Next()
			}
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
			Status:    resolveStatus(c, err),
			Latency:   latency,
			RequestID: requestID,
		})

		return err
	}
}

// resolveStatus returns the true HTTP status code for the request.
// c.Response().StatusCode() reads 200 before Fiber's error handler runs,
// so we inspect the returned error directly when one is present.
func resolveStatus(c *fiber.Ctx, err error) int {
	if err != nil {
		var ke *keelcore.KError
		if errors.As(err, &ke) {
			return ke.StatusCode
		}
		if fe, ok := err.(*fiber.Error); ok {
			return fe.Code
		}
	}
	return c.Response().StatusCode()
}

// Requests returns a snapshot of captured requests, oldest first.
func (p *DevPanel) Requests() []RequestEntry {
	if p.requests == nil {
		return nil
	}
	return p.requests.snapshot()
}
