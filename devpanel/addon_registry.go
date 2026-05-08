package devpanel

import (
	"context"
	"sync"

	"github.com/slice-soft/ss-keel-core/contracts"
)

const addonEventRingSize = 50

// addonStream holds the per-addon event broadcaster, a ring buffer that
// retains the last addonEventRingSize events for late-joining SSE clients,
// and the cancel function that stops the goroutine consuming PanelEvents().
type addonStream struct {
	addon  contracts.Debuggable
	bcast  *sseBroadcaster[contracts.PanelEvent]
	ring   *eventRing
	cancel context.CancelFunc
}

// eventRing is a fixed-size thread-safe ring buffer for PanelEvent.
// It retains the last N events so new SSE clients can receive a replay.
type eventRing struct {
	mu      sync.RWMutex
	entries []contracts.PanelEvent
	size    int
	head    int
	count   int
}

func newEventRing(size int) *eventRing {
	return &eventRing{entries: make([]contracts.PanelEvent, size), size: size}
}

func (r *eventRing) push(e contracts.PanelEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[r.head] = e
	r.head = (r.head + 1) % r.size
	if r.count < r.size {
		r.count++
	}
}

func (r *eventRing) snapshot() []contracts.PanelEvent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.count == 0 {
		return nil
	}
	out := make([]contracts.PanelEvent, r.count)
	start := (r.head - r.count + r.size) % r.size
	for i := range r.count {
		out[i] = r.entries[(start+i)%r.size]
	}
	return out
}

// startAddonStream spawns a goroutine that consumes events from the addon's
// PanelEvents() channel, stores them in a ring buffer, and fans them out to
// all active SSE subscribers.
// It returns immediately; the goroutine runs until ctx is cancelled or the
// addon's channel is closed.
func (p *DevPanel) startAddonStream(ctx context.Context, d contracts.Debuggable) *addonStream {
	bcast := newSSEBroadcaster[contracts.PanelEvent]()
	ring := newEventRing(addonEventRingSize)
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
				ring.push(event)
				bcast.broadcast(event)
			case <-addonCtx.Done():
				return
			}
		}
	}()

	return &addonStream{addon: d, bcast: bcast, ring: ring, cancel: cancel}
}

// addonStreamFor returns the stream for the addon with the given ID, or nil.
func (p *DevPanel) addonStreamFor(id string) *addonStream {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.streams[id]
}
