package devpanel

import (
	"sync"
	"time"
)

// RequestEntry holds the captured data of a single HTTP request.
type RequestEntry struct {
	ID        string
	Timestamp time.Time
	Method    string
	Path      string
	Status    int
	Latency   time.Duration
	RequestID string
}

// requestBuffer is a fixed-size thread-safe ring buffer for RequestEntry.
// When full, the oldest entry is overwritten.
type requestBuffer struct {
	mu      sync.RWMutex
	entries []RequestEntry
	size    int
	head    int // index of the next write position
	count   int // number of valid entries
}

func newRequestBuffer(size int) *requestBuffer {
	return &requestBuffer{
		entries: make([]RequestEntry, size),
		size:    size,
	}
}

// push adds an entry to the buffer, overwriting the oldest if full.
func (b *requestBuffer) push(e RequestEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries[b.head] = e
	b.head = (b.head + 1) % b.size
	if b.count < b.size {
		b.count++
	}
}

// snapshot returns all valid entries ordered from oldest to newest.
func (b *requestBuffer) snapshot() []RequestEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return nil
	}

	out := make([]RequestEntry, b.count)
	start := (b.head - b.count + b.size) % b.size
	for i := 0; i < b.count; i++ {
		out[i] = b.entries[(start+i)%b.size]
	}
	return out
}
