package devpanel_test

import (
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/slice-soft/ss-keel-core/contracts"
	"github.com/slice-soft/ss-keel-devpanel/devpanel"
)

// --- X-Keel-Panel header ---

func TestPanelHeader_presentOnEnabledRoutes(t *testing.T) {
	_, app := panelApp(devpanel.Config{Enabled: true})

	req := httptest.NewRequest("GET", "/keel/panel/requests", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if h := resp.Header.Get("X-Keel-Panel"); h != "true" {
		t.Fatalf("X-Keel-Panel = %q, want %q", h, "true")
	}
}

func TestPanelHeader_presentWhenDisabled(t *testing.T) {
	_, app := panelApp(devpanel.Config{Enabled: false})

	req := httptest.NewRequest("GET", "/keel/panel/requests", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	if h := resp.Header.Get("X-Keel-Panel"); h != "true" {
		t.Fatalf("X-Keel-Panel = %q, want %q on 404", h, "true")
	}
}

func TestPanelHeader_presentWhenUnauthorized(t *testing.T) {
	_, app := panelApp(devpanel.Config{Enabled: true, Secret: "tok"})

	req := httptest.NewRequest("GET", "/keel/panel/requests", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode != 401 {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	if h := resp.Header.Get("X-Keel-Panel"); h != "true" {
		t.Fatalf("X-Keel-Panel = %q, want %q on 401", h, "true")
	}
}

// --- GlobalGuard ---

func TestGlobalGuard_blocksDisabledPanel(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: false})
	app := fiber.New()
	app.Use(p.GlobalGuard())
	p.Mount(app)

	// Panel route blocked at app level.
	if status := getStatus(t, app, "/keel/panel/requests", ""); status != 404 {
		t.Fatalf("status = %d, want 404", status)
	}
}

func TestGlobalGuard_passesNonPanelPaths(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: false})
	app := fiber.New()
	app.Use(p.GlobalGuard())
	p.Mount(app)
	app.Get("/api/health", func(c *fiber.Ctx) error { return c.SendString("ok") })

	// Non-panel route must not be affected.
	if status := getStatus(t, app, "/api/health", ""); status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
}

func TestGlobalGuard_passesEnabledPanel(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	app := fiber.New()
	app.Use(p.GlobalGuard())
	p.Mount(app)

	if status := getStatus(t, app, "/keel/panel/requests", ""); status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
}

func TestGlobalGuard_defenceInDepth(t *testing.T) {
	// Both GlobalGuard and the inner guard must independently block when disabled.
	// Removing the inner guard should still leave GlobalGuard protecting.
	p := devpanel.New(devpanel.Config{Enabled: false})
	app := fiber.New()
	app.Use(p.GlobalGuard())
	p.Mount(app)

	for _, path := range []string{
		"/keel/panel/requests",
		"/keel/panel/logs",
		"/keel/panel/routes",
		"/keel/panel/addons",
		"/keel/panel/config",
	} {
		if status := getStatus(t, app, path, ""); status != 404 {
			t.Errorf("%s: status = %d, want 404", path, status)
		}
	}
}

// --- end-to-end integration ---

// e2eAddon is a mock addon with a manifest (Manifestable + Debuggable).
type e2eAddon struct {
	id    string
	label string
	ch    chan contracts.PanelEvent
}

func (a *e2eAddon) PanelID() string                         { return a.id }
func (a *e2eAddon) PanelLabel() string                      { return a.label }
func (a *e2eAddon) PanelEvents() <-chan contracts.PanelEvent { return a.ch }
func (a *e2eAddon) Manifest() contracts.AddonManifest {
	return contracts.AddonManifest{
		ID:           a.id,
		Version:      "2.0.0",
		Capabilities: []string{"database"},
		Resources:    []string{"postgres"},
		EnvVars: []contracts.EnvVar{
			{Key: "DB_DSN", Required: true, Secret: true, Source: a.id},
		},
	}
}

func TestE2E_fullPanelFlow(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	defer p.Shutdown()

	addon := &e2eAddon{id: "gorm-e2e", label: "GORM E2E", ch: make(chan contracts.PanelEvent, 4)}
	p.RegisterAddon(addon)

	app := fiber.New()
	app.Use(p.RequestMiddleware())
	p.Mount(app)

	// A non-panel route that gets captured by the middleware.
	app.Get("/api/users", func(c *fiber.Ctx) error {
		return c.JSON([]string{"alice", "bob"})
	})

	// 1. Make a real request — it should be captured.
	if status := getStatus(t, app, "/api/users", ""); status != 200 {
		t.Fatalf("/api/users status = %d", status)
	}

	// 2. Write a log entry.
	p.Logger().Info("startup complete")

	// 3. Emit an event from the addon.
	addon.ch <- contracts.PanelEvent{
		Timestamp: time.Now(),
		AddonID:   "gorm-e2e",
		Label:     "SELECT users",
		Level:     "info",
		Detail:    map[string]any{"rows": 2, "ms": 1.5},
	}

	// Allow goroutine to process the event.
	time.Sleep(10 * time.Millisecond)

	// 4. Requests page must show the captured route.
	if html := pageHTML(t, app, "/keel/panel/requests", ""); !contains(html, "/api/users") {
		t.Error("requests page should contain /api/users")
	}

	// 5. Logs page must render (at least the empty state or the log we wrote).
	logsHTML := pageHTML(t, app, "/keel/panel/logs", "")
	if !contains(logsHTML, "Logs") {
		t.Error("logs page should render with 'Logs' heading")
	}

	// 6. Addons page must list our addon.
	addonsHTML := pageHTML(t, app, "/keel/panel/addons", "")
	if !contains(addonsHTML, "GORM E2E") {
		t.Error("addons page should show the addon label")
	}
	if !contains(addonsHTML, "database") {
		t.Error("addons page should show capabilities badge")
	}

	// 7. Addon detail page must render for our addon.
	detailHTML := pageHTML(t, app, "/keel/panel/addons/gorm-e2e", "")
	if !contains(detailHTML, "GORM E2E") {
		t.Error("addon detail page should show the addon label")
	}

	// 8. Config page must show runtime info and addon manifest data.
	configHTML := pageHTML(t, app, "/keel/panel/config", "")
	if !contains(configHTML, "Go version") {
		t.Error("config page should show Go version label")
	}
	if !contains(configHTML, "2.0.0") {
		t.Error("config page should show addon version 2.0.0")
	}
	if !contains(configHTML, "DB_DSN") {
		t.Error("config page should show addon env var")
	}

	// 9. Every panel response carries X-Keel-Panel: true.
	for _, path := range []string{
		"/keel/panel/requests",
		"/keel/panel/logs",
		"/keel/panel/routes",
		"/keel/panel/addons",
		"/keel/panel/config",
	} {
		req := httptest.NewRequest("GET", path, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("%s: app.Test: %v", path, err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if h := resp.Header.Get("X-Keel-Panel"); h != "true" {
			t.Errorf("%s: X-Keel-Panel = %q, want %q", path, h, "true")
		}
	}
}

// pageHTML is a helper that fetches a page and returns the HTML body.
func pageHTML(t *testing.T, app *fiber.App, path, auth string) string {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test %s: %v", path, err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return string(body)
}
