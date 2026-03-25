package devpanel

import (
	"fmt"
	"sync"
	"time"
)

const logBufferSize = 512

// LogLevel represents the severity of a log entry.
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// LogEntry holds a single captured log line.
type LogEntry struct {
	Timestamp time.Time
	Level     LogLevel
	Message   string
	RequestID string // empty when the log has no associated request
}

// logBuffer is a fixed-size thread-safe ring buffer for LogEntry.
type logBuffer struct {
	mu      sync.RWMutex
	entries []LogEntry
	size    int
	head    int
	count   int
}

func newLogBuffer(size int) *logBuffer {
	return &logBuffer{
		entries: make([]LogEntry, size),
		size:    size,
	}
}

func (b *logBuffer) push(e LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries[b.head] = e
	b.head = (b.head + 1) % b.size
	if b.count < b.size {
		b.count++
	}
}

func (b *logBuffer) snapshot() []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return nil
	}

	out := make([]LogEntry, b.count)
	start := (b.head - b.count + b.size) % b.size
	for i := 0; i < b.count; i++ {
		out[i] = b.entries[(start+i)%b.size]
	}
	return out
}

// PanelLogger implements contracts.Logger and captures every log line into
// the panel's ring buffer. New entries are also broadcast to active SSE clients.
//
// Use WithRequestID to get a scoped copy that tags entries with a request ID:
//
//	logger := panel.Logger().WithRequestID(c.Locals("request_id").(string))
type PanelLogger struct {
	buf       *logBuffer
	bcast     *sseBroadcaster[LogEntry]
	requestID string
}

// WithRequestID returns a shallow copy of PanelLogger that tags every entry
// it writes with the given request ID. The underlying buffer and broadcaster
// are shared.
func (l *PanelLogger) WithRequestID(id string) *PanelLogger {
	return &PanelLogger{buf: l.buf, bcast: l.bcast, requestID: id}
}

func (l *PanelLogger) log(level LogLevel, format string, args ...interface{}) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   fmt.Sprintf(format, args...),
		RequestID: l.requestID,
	}
	l.buf.push(entry)
	if l.bcast != nil {
		l.bcast.broadcast(entry)
	}
}

func (l *PanelLogger) Debug(format string, args ...interface{}) { l.log(LogLevelDebug, format, args...) }
func (l *PanelLogger) Info(format string, args ...interface{})  { l.log(LogLevelInfo, format, args...) }
func (l *PanelLogger) Warn(format string, args ...interface{})  { l.log(LogLevelWarn, format, args...) }
func (l *PanelLogger) Error(format string, args ...interface{}) { l.log(LogLevelError, format, args...) }
