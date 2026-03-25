package devpanel

import (
	"context"

	"github.com/slice-soft/ss-keel-core/contracts"
)

// addonStream holds the per-addon event broadcaster and the cancel function
// that stops the goroutine consuming PanelEvents().
type addonStream struct {
	addon  contracts.Debuggable
	bcast  *sseBroadcaster[contracts.PanelEvent]
	cancel context.CancelFunc
}

// startAddonStream spawns a goroutine that consumes events from the addon's
// PanelEvents() channel and fans them out to all SSE subscribers.
// It returns immediately; the goroutine runs until ctx is cancelled or the
// addon's channel is closed.
func (p *DevPanel) startAddonStream(ctx context.Context, d contracts.Debuggable) *addonStream {
	bcast := newSSEBroadcaster[contracts.PanelEvent]()
	addonCtx, cancel := context.WithCancel(ctx)

	go func() {
		defer cancel()
		ch := d.PanelEvents()
		for {
			select {
			case event, ok := <-ch:
				if !ok {
					return // addon closed its channel
				}
				bcast.broadcast(event)
			case <-addonCtx.Done():
				return
			}
		}
	}()

	return &addonStream{addon: d, bcast: bcast, cancel: cancel}
}

// addonStreamFor returns the stream for the addon with the given ID, or nil.
func (p *DevPanel) addonStreamFor(id string) *addonStream {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.streams[id]
}
