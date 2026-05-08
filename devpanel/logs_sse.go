package devpanel

import (
	"bufio"
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/slice-soft/ss-keel-devpanel/devpanel/ui"
)

// handleLogs renders the logs page with the current snapshot.
func (p *DevPanel) handleLogs() fiber.Handler {
	return func(c *fiber.Ctx) error {
		entries := p.Logs()
		rows := make([]ui.LogRow, len(entries))
		for i, e := range entries {
			rows[i] = ui.LogRow{
				Timestamp: e.Timestamp,
				Level:     string(e.Level),
				Message:   e.Message,
				RequestID: e.RequestID,
			}
		}
		streamURL := p.cfg.Path + "/logs/stream"
		return render(c, ui.Logs(p.buildNav("Logs"), rows, streamURL, p.assetBase()))
	}
}

// handleLogsStream is the SSE endpoint. Each connected client receives a
// "log" event for every new entry written via PanelLogger.
//
// The connection stays open until the client disconnects. Cleanup is
// guaranteed via the cancel func regardless of how the connection closes.
func (p *DevPanel) handleLogsStream() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("X-Accel-Buffering", "no")

		ch, cancel := p.logBcast.subscribe()

		c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
			defer cancel()

			// Force header flush so EventSource.onopen fires immediately,
			// even when the log buffer is empty (SSE comment is ignored by clients).
			_, _ = fmt.Fprint(w, ": ping\n\n")
			_ = w.Flush()

			// Send existing snapshot so the client is up-to-date on connect.
			for _, entry := range p.Logs() {
				if err := writeLogEvent(w, entry); err != nil {
					return
				}
			}
			_ = w.Flush()

			for entry := range ch {
				if err := writeLogEvent(w, entry); err != nil {
					return
				}
				_ = w.Flush()
			}
		})
		return nil
	}
}

func writeLogEvent(w *bufio.Writer, e LogEntry) error {
	data, err := json.Marshal(map[string]string{
		"level":      string(e.Level),
		"message":    e.Message,
		"time":       e.Timestamp.Format("15:04:05.000"),
		"request_id": e.RequestID,
	})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: log\ndata: %s\n\n", data)
	return err
}
