package devpanel_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/slice-soft/ss-keel-core/contracts"
	"github.com/slice-soft/ss-keel-devpanel/devpanel"
)

// compile-time assertion: PanelLogger implements contracts.Logger
var _ contracts.Logger = (*devpanel.PanelLogger)(nil)

func TestLogger_levels(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	l := p.Logger()

	l.Debug("debug msg")
	l.Info("info msg")
	l.Warn("warn msg")
	l.Error("error msg")

	entries := p.Logs()
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}

	cases := []struct {
		level devpanel.LogLevel
		msg   string
	}{
		{devpanel.LogLevelDebug, "debug msg"},
		{devpanel.LogLevelInfo, "info msg"},
		{devpanel.LogLevelWarn, "warn msg"},
		{devpanel.LogLevelError, "error msg"},
	}
	for i, c := range cases {
		if entries[i].Level != c.level {
			t.Fatalf("[%d] Level = %q, want %q", i, entries[i].Level, c.level)
		}
		if entries[i].Message != c.msg {
			t.Fatalf("[%d] Message = %q, want %q", i, entries[i].Message, c.msg)
		}
	}
}

func TestLogger_formatArgs(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	p.Logger().Info("user %d logged in from %s", 42, "192.168.1.1")

	entries := p.Logs()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	want := "user 42 logged in from 192.168.1.1"
	if entries[0].Message != want {
		t.Fatalf("Message = %q, want %q", entries[0].Message, want)
	}
}

func TestLogger_withRequestID(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})

	scoped := p.Logger().WithRequestID("req-abc")
	scoped.Info("scoped log")

	p.Logger().Warn("unscoped log")

	entries := p.Logs()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].RequestID != "req-abc" {
		t.Fatalf("entries[0].RequestID = %q, want %q", entries[0].RequestID, "req-abc")
	}
	if entries[1].RequestID != "" {
		t.Fatalf("entries[1].RequestID = %q, want empty", entries[1].RequestID)
	}
}

func TestLogger_withRequestID_sharedBuffer(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	base := p.Logger()

	base.Info("base log")
	base.WithRequestID("req-1").Error("scoped error")
	base.WithRequestID("req-2").Warn("another scoped")

	entries := p.Logs()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[1].RequestID != "req-1" {
		t.Fatalf("entries[1].RequestID = %q, want %q", entries[1].RequestID, "req-1")
	}
	if entries[2].RequestID != "req-2" {
		t.Fatalf("entries[2].RequestID = %q, want %q", entries[2].RequestID, "req-2")
	}
}

func TestLogger_ringOverwrite(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	l := p.Logger()

	for i := 0; i < 600; i++ {
		l.Info("msg %d", i)
	}

	entries := p.Logs()
	if len(entries) != 512 {
		t.Fatalf("expected 512 entries (buffer cap), got %d", len(entries))
	}
	// oldest visible entry should be msg 88 (600-512)
	want := fmt.Sprintf("msg %d", 600-512)
	if entries[0].Message != want {
		t.Fatalf("oldest entry = %q, want %q", entries[0].Message, want)
	}
}

func TestLogger_concurrentSafe(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	l := p.Logger()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			l.WithRequestID(fmt.Sprintf("req-%d", n)).Info("goroutine %d", n)
		}(i)
	}
	wg.Wait()

	entries := p.Logs()
	if len(entries) == 0 {
		t.Fatal("expected log entries after concurrent writes")
	}
}
