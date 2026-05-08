package devpanel

import (
	"bufio"
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/slice-soft/ss-keel-core/contracts"
	"github.com/slice-soft/ss-keel-devpanel/devpanel/ui"
)

// handleAddonDetail renders the event stream page for a single addon.
func (p *DevPanel) handleAddonDetail() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		stream := p.addonStreamFor(id)
		if stream == nil {
			return c.SendStatus(fiber.StatusNotFound)
		}
		row := addonToRow(stream.addon, p.cfg.Path)
		streamURL := fmt.Sprintf("%s/addons/%s/stream", p.cfg.Path, id)
		return render(c, ui.AddonDetail(p.buildNav("Addons"), row, streamURL, p.assetBase()))
	}
}

// handleAddonStream is the SSE endpoint for a single addon's PanelEvents().
func (p *DevPanel) handleAddonStream() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		stream := p.addonStreamFor(id)
		if stream == nil {
			return c.SendStatus(fiber.StatusNotFound)
		}

		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("X-Accel-Buffering", "no")

		ch, cancel := stream.bcast.subscribe()

		c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
			defer cancel()

			// Force header flush so EventSource.onopen fires immediately.
			_, _ = fmt.Fprint(w, ": ping\n\n")
			_ = w.Flush()

			// Replay recent events so the client sees history on page load,
			// not just events that happen while the tab is open.
			for _, event := range stream.ring.snapshot() {
				if err := writeAddonEvent(w, event); err != nil {
					return
				}
			}
			_ = w.Flush()

			for event := range ch {
				if err := writeAddonEvent(w, event); err != nil {
					return
				}
				_ = w.Flush()
			}
		})
		return nil
	}
}

func writeAddonEvent(w *bufio.Writer, e contracts.PanelEvent) error {
	detail := make(map[string]string, len(e.Detail))
	for k, v := range e.Detail {
		detail[k] = fmt.Sprintf("%v", v)
	}
	data, err := json.Marshal(map[string]any{
		"time":   e.Timestamp.Format("15:04:05.000"),
		"level":  e.Level,
		"label":  e.Label,
		"detail": detail,
	})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: event\ndata: %s\n\n", data)
	return err
}

// addonToRow converts a Debuggable into a ui.AddonRow.
// If the addon also implements contracts.Manifestable the capabilities and
// resources are taken from its manifest.
func addonToRow(d contracts.Debuggable, basePath string) ui.AddonRow {
	row := ui.AddonRow{
		ID:        d.PanelID(),
		Label:     d.PanelLabel(),
		DetailURL: fmt.Sprintf("%s/addons/%s", basePath, d.PanelID()),
	}
	if m, ok := d.(contracts.Manifestable); ok {
		manifest := m.Manifest()
		row.Capabilities = manifest.Capabilities
		row.Resources = manifest.Resources
	}
	return row
}
