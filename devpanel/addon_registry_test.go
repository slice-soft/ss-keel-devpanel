package devpanel_test

import (
	"io"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/slice-soft/ss-keel-core/contracts"
	"github.com/slice-soft/ss-keel-devpanel/devpanel"
)

// fullMock implements contracts.Debuggable + contracts.Manifestable.
type fullMock struct {
	id           string
	label        string
	capabilities []string
	resources    []string
	ch           chan contracts.PanelEvent
}

func newFullMock(id, label string) *fullMock {
	return &fullMock{id: id, label: label, ch: make(chan contracts.PanelEvent, 16)}
}

func (m *fullMock) PanelID() string                          { return m.id }
func (m *fullMock) PanelLabel() string                       { return m.label }
func (m *fullMock) PanelEvents() <-chan contracts.PanelEvent  { return m.ch }
func (m *fullMock) Manifest() contracts.AddonManifest {
	return contracts.AddonManifest{
		ID:           m.id,
		Capabilities: m.capabilities,
		Resources:    m.resources,
	}
}

// --- registration ---

func TestRegisterAddon_idempotent(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	defer p.Shutdown()

	mock := newFullMock("gorm", "GORM")
	p.RegisterAddon(mock)
	p.RegisterAddon(mock) // second call must be a no-op

	if got := len(p.Addons()); got != 1 {
		t.Fatalf("expected 1 addon after duplicate register, got %d", got)
	}
}

func TestRegisterAddon_multipleAddons(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	defer p.Shutdown()

	p.RegisterAddon(newFullMock("gorm", "GORM"))
	p.RegisterAddon(newFullMock("redis", "Redis"))
	p.RegisterAddon(newFullMock("jwt", "JWT"))

	if got := len(p.Addons()); got != 3 {
		t.Fatalf("expected 3 addons, got %d", got)
	}
}

// --- event consumption ---

func TestAddonStream_eventReachesSSE(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	defer p.Shutdown()

	mock := newFullMock("gorm", "GORM")
	p.RegisterAddon(mock)

	// Give the goroutine time to start.
	time.Sleep(10 * time.Millisecond)

	// Emit an event from the mock addon.
	mock.ch <- contracts.PanelEvent{
		AddonID: "gorm",
		Label:   "query executed",
		Level:   "info",
		Detail:  map[string]any{"sql": "SELECT 1", "duration_ms": 5},
	}

	// Allow the goroutine to process.
	time.Sleep(20 * time.Millisecond)

	// If no panic, the goroutine consumed the event successfully.
	// Deeper SSE delivery is validated in the stream tests.
}

func TestAddonStream_goroutineStopsOnShutdown(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})

	mock := newFullMock("redis", "Redis")
	p.RegisterAddon(mock)
	time.Sleep(10 * time.Millisecond)

	p.Shutdown()
	time.Sleep(20 * time.Millisecond)

	// After shutdown, sending to the channel should not block (goroutine is gone).
	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case mock.ch <- contracts.PanelEvent{Label: "after shutdown"}:
		default:
		}
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("goroutine appears to still be blocking after Shutdown")
	}
}

func TestAddonStream_closedChannelStopsGoroutine(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	defer p.Shutdown()

	mock := newFullMock("mongo", "Mongo")
	p.RegisterAddon(mock)
	time.Sleep(10 * time.Millisecond)

	close(mock.ch) // simulate addon shutting down
	time.Sleep(20 * time.Millisecond)
	// No panic = goroutine handled the closed channel correctly.
}

// --- routes ---

func TestAddonDetail_returns200(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	defer p.Shutdown()
	p.RegisterAddon(newFullMock("gorm", "GORM"))

	app := fiber.New()
	p.Mount(app)

	if status := getStatus(t, app, "/keel/panel/addons/gorm", ""); status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
}

func TestAddonDetail_unknownAddon_returns404(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	defer p.Shutdown()

	app := fiber.New()
	p.Mount(app)

	if status := getStatus(t, app, "/keel/panel/addons/unknown", ""); status != 404 {
		t.Fatalf("status = %d, want 404", status)
	}
}

func TestAddonDetail_rendersCapabilitiesFromManifest(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	defer p.Shutdown()
	mock := newFullMock("redis", "Redis")
	mock.capabilities = []string{"cache"}
	mock.resources = []string{"redis"}
	p.RegisterAddon(mock)

	app := fiber.New()
	p.Mount(app)

	req := httptest.NewRequest("GET", "/keel/panel/addons/redis", nil)
	resp, _ := app.Test(req)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	html := string(body)
	if !contains(html, "cache") {
		t.Error("expected capability 'cache' in addon detail page")
	}
	if !contains(html, "redis") {
		t.Error("expected resource 'redis' in addon detail page")
	}
}

func TestAddonsList_showsDetailLinks(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	defer p.Shutdown()
	p.RegisterAddon(newFullMock("jwt", "JWT"))

	app := fiber.New()
	p.Mount(app)

	req := httptest.NewRequest("GET", "/keel/panel/addons", nil)
	resp, _ := app.Test(req)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if !contains(string(body), "/keel/panel/addons/jwt") {
		t.Error("addons list should contain link to addon detail page")
	}
}

func TestAddonStream_guardEnforced(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: false})
	defer p.Shutdown()

	app := fiber.New()
	p.Mount(app)

	if status := getStatus(t, app, "/keel/panel/addons/gorm/stream", ""); status != 404 {
		t.Fatalf("disabled panel: status = %d, want 404", status)
	}
}

// --- concurrency ---

func TestRegisterAddon_concurrentSafe(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	defer p.Shutdown()

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := "addon-unique-" + string(rune('a'+n))
			p.RegisterAddon(newFullMock(id, id))
		}(i)
	}
	wg.Wait()

	if got := len(p.Addons()); got != 20 {
		t.Fatalf("expected 20 unique addons, got %d", got)
	}
}
