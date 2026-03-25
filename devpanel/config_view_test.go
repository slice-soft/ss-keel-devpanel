package devpanel_test

import (
	"io"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/slice-soft/ss-keel-core/contracts"
	"github.com/slice-soft/ss-keel-devpanel/devpanel"
)

// manifestMock is a Debuggable + Manifestable addon for config tests.
type manifestMock struct {
	id       string
	label    string
	manifest contracts.AddonManifest
	ch       chan contracts.PanelEvent
}

func newManifestMock(id, label string, manifest contracts.AddonManifest) *manifestMock {
	return &manifestMock{id: id, label: label, manifest: manifest, ch: make(chan contracts.PanelEvent, 8)}
}

func (m *manifestMock) PanelID() string                         { return m.id }
func (m *manifestMock) PanelLabel() string                      { return m.label }
func (m *manifestMock) PanelEvents() <-chan contracts.PanelEvent { return m.ch }
func (m *manifestMock) Manifest() contracts.AddonManifest       { return m.manifest }

func configPage(t *testing.T, p *devpanel.DevPanel) string {
	t.Helper()
	app := fiber.New()
	p.Mount(app)
	url := p.Config().Path + "/config"
	req := httptest.NewRequest("GET", url, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return string(body)
}

// --- route ---

func TestConfigPage_returns200(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	defer p.Shutdown()

	app := fiber.New()
	p.Mount(app)

	if status := getStatus(t, app, "/keel/panel/config", ""); status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
}

// --- runtime info ---

func TestConfigPage_showsRuntimeInfo(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	defer p.Shutdown()

	html := configPage(t, p)

	if !contains(html, "Go version") {
		t.Error("config page should show Go version label")
	}
	if !contains(html, "Goroutines") {
		t.Error("config page should show goroutine count")
	}
	if !contains(html, "Heap alloc") {
		t.Error("config page should show heap alloc")
	}
	if !contains(html, "go1.") {
		t.Error("config page should contain actual Go version string (go1.x)")
	}
}

// --- env var redaction ---

func TestConfigPage_secretsRedacted(t *testing.T) {
	t.Setenv("KEEL_PANEL_SECRET", "super-secret-value")

	// No Secret in Config so the guard doesn't block the request —
	// the point of this test is to verify env var value redaction, not auth.
	p := devpanel.New(devpanel.Config{Enabled: true})
	defer p.Shutdown()

	html := configPage(t, p)

	if contains(html, "super-secret-value") {
		t.Error("secret value must not appear in config page HTML")
	}
	if !contains(html, "KEEL_PANEL_SECRET") {
		t.Error("secret key should appear (just not its value)")
	}
}

func TestConfigPage_nonSecretValueVisible(t *testing.T) {
	t.Setenv("KEEL_PANEL_PATH", "/custom-panel")

	p := devpanel.New(devpanel.Config{Enabled: true, Path: "/custom-panel"})
	defer p.Shutdown()

	html := configPage(t, p)

	if !contains(html, "/custom-panel") {
		t.Error("non-secret env var value should be visible in config page")
	}
}

func TestConfigPage_emptySecretNotRedacted(t *testing.T) {
	os.Unsetenv("KEEL_PANEL_SECRET")

	p := devpanel.New(devpanel.Config{Enabled: true})
	defer p.Shutdown()

	html := configPage(t, p)

	// When a secret has no value set, "••••••••" should not appear.
	if contains(html, "••••••••") {
		t.Error("empty secret should not show redaction placeholder")
	}
}

// --- addon manifest data ---

func TestConfigPage_addonCapabilitiesAndResources(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	defer p.Shutdown()

	p.RegisterAddon(newManifestMock("gorm", "GORM", contracts.AddonManifest{
		ID:           "gorm",
		Version:      "1.2.3",
		Capabilities: []string{"database"},
		Resources:    []string{"postgres"},
		EnvVars: []contracts.EnvVar{
			{Key: "DB_DSN", Required: true, Secret: true, Source: "gorm"},
		},
	}))

	html := configPage(t, p)

	if !contains(html, "database") {
		t.Error("config page should show addon capability 'database'")
	}
	if !contains(html, "postgres") {
		t.Error("config page should show addon resource 'postgres'")
	}
	if !contains(html, "1.2.3") {
		t.Error("config page should show addon version")
	}
	if !contains(html, "DB_DSN") {
		t.Error("config page should show addon env var key")
	}
}

func TestConfigPage_addonSecretEnvVarRedacted(t *testing.T) {
	t.Setenv("DB_DSN", "postgres://user:password@host/db")

	p := devpanel.New(devpanel.Config{Enabled: true})
	defer p.Shutdown()

	p.RegisterAddon(newManifestMock("gorm", "GORM", contracts.AddonManifest{
		ID: "gorm",
		EnvVars: []contracts.EnvVar{
			{Key: "DB_DSN", Required: true, Secret: true, Source: "gorm"},
		},
	}))

	html := configPage(t, p)

	if contains(html, "postgres://user:password@host/db") {
		t.Error("secret addon env var value must be redacted")
	}
	if !contains(html, "DB_DSN") {
		t.Error("secret key should still appear")
	}
}

func TestConfigPage_guardEnforced(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: false})
	defer p.Shutdown()

	app := fiber.New()
	p.Mount(app)

	if status := getStatus(t, app, "/keel/panel/config", ""); status != 404 {
		t.Fatalf("disabled panel: status = %d, want 404", status)
	}
}
