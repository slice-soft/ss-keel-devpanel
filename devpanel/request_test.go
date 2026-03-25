package devpanel_test

import (
	"io"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/slice-soft/ss-keel-devpanel/devpanel"
)

// --- ring buffer tests ---

func TestRequestBuffer_pushAndSnapshot(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	app := fiber.New()
	app.Use(p.RequestMiddleware())
	app.Get("/hello", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest("GET", "/hello", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	entries := p.Requests()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Method != "GET" {
		t.Fatalf("Method = %q, want GET", e.Method)
	}
	if e.Path != "/hello" {
		t.Fatalf("Path = %q, want /hello", e.Path)
	}
	if e.Status != 200 {
		t.Fatalf("Status = %d, want 200", e.Status)
	}
	if e.Latency <= 0 {
		t.Fatal("Latency should be > 0")
	}
	if e.RequestID == "" {
		t.Fatal("RequestID should not be empty")
	}
	if e.ID == "" {
		t.Fatal("ID should not be empty")
	}
}

func TestRequestBuffer_ignoresPanelRoutes(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true, Path: "/keel/panel"})
	app := fiber.New()
	app.Use(p.RequestMiddleware())
	app.Get("/keel/panel/requests", func(c *fiber.Ctx) error { return c.SendStatus(200) })
	app.Get("/api/users", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	for _, path := range []string{"/keel/panel/requests", "/keel/panel/requests", "/api/users"} {
		req := httptest.NewRequest("GET", path, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test(%s): %v", path, err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	entries := p.Requests()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (only /api/users), got %d", len(entries))
	}
	if entries[0].Path != "/api/users" {
		t.Fatalf("Path = %q, want /api/users", entries[0].Path)
	}
}

func TestRequestBuffer_propagatesXRequestID(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	app := fiber.New()
	app.Use(p.RequestMiddleware())
	app.Get("/ping", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest("GET", "/ping", nil)
	req.Header.Set("X-Request-ID", "test-id-123")
	resp, _ := app.Test(req)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	entries := p.Requests()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].RequestID != "test-id-123" {
		t.Fatalf("RequestID = %q, want %q", entries[0].RequestID, "test-id-123")
	}
}

func TestRequestBuffer_ringOverwrite(t *testing.T) {
	// Use a small buffer via multiple requests to verify oldest is overwritten.
	// We can't set buffer size from outside, so we push 300 requests (> 256).
	p := devpanel.New(devpanel.Config{Enabled: true})
	app := fiber.New()
	app.Use(p.RequestMiddleware())
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	for i := 0; i < 300; i++ {
		req := httptest.NewRequest("GET", "/x", nil)
		resp, _ := app.Test(req)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	entries := p.Requests()
	if len(entries) != 256 {
		t.Fatalf("expected 256 entries (buffer cap), got %d", len(entries))
	}
}

func TestRequestBuffer_concurrentSafe(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	app := fiber.New()
	app.Use(p.RequestMiddleware())
	app.Get("/race", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/race", nil)
			resp, _ := app.Test(req)
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}()
	}
	wg.Wait()

	entries := p.Requests()
	if len(entries) == 0 {
		t.Fatal("expected entries after concurrent requests")
	}
}
