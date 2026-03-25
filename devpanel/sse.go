package devpanel

import (
	"sync"
)

// sseBroadcaster fans out messages to all active SSE clients.
// Each subscriber gets its own buffered channel; slow clients are skipped
// (non-blocking send) so one lagging connection cannot block the others.
type sseBroadcaster[T any] struct {
	mu      sync.RWMutex
	clients map[chan T]struct{}
}

func newSSEBroadcaster[T any]() *sseBroadcaster[T] {
	return &sseBroadcaster[T]{clients: make(map[chan T]struct{})}
}

// subscribe returns a channel that receives broadcast values and a cancel
// function the caller must invoke when the client disconnects.
func (b *sseBroadcaster[T]) subscribe() (<-chan T, func()) {
	ch := make(chan T, 64)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		delete(b.clients, ch)
		b.mu.Unlock()
		close(ch)
	}
	return ch, cancel
}

// broadcast sends v to all subscribers. Slow subscribers are skipped.
func (b *sseBroadcaster[T]) broadcast(v T) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- v:
		default:
		}
	}
}

// count returns the number of active subscribers.
func (b *sseBroadcaster[T]) count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}
