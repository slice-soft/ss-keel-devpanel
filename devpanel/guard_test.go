package devpanel_test

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/slice-soft/ss-keel-devpanel/devpanel"
)

func panelApp(cfg devpanel.Config) (*devpanel.DevPanel, *fiber.App) {
	p := devpanel.New(cfg)
	app := fiber.New()
	p.Mount(app)
	return p, app
}

func getStatus(t *testing.T, app *fiber.App, path, auth string) int {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp.StatusCode
}

// --- guard: Enabled ---

func TestGuard_disabled_returns404(t *testing.T) {
	_, app := panelApp(devpanel.Config{Enabled: false})

	for _, path := range []string{"/keel/panel/requests", "/keel/panel/routes", "/keel/panel/addons"} {
		if status := getStatus(t, app, path, ""); status != 404 {
			t.Fatalf("%s: status = %d, want 404", path, status)
		}
	}
}

func TestGuard_enabled_returns200(t *testing.T) {
	_, app := panelApp(devpanel.Config{Enabled: true})

	for _, path := range []string{"/keel/panel/requests", "/keel/panel/routes", "/keel/panel/addons"} {
		if status := getStatus(t, app, path, ""); status != 200 {
			t.Fatalf("%s: status = %d, want 200", path, status)
		}
	}
}

// --- guard: Secret ---

func TestGuard_secret_missingToken_returns401(t *testing.T) {
	_, app := panelApp(devpanel.Config{Enabled: true, Secret: "s3cr3t"})

	if status := getStatus(t, app, "/keel/panel/requests", ""); status != 401 {
		t.Fatalf("status = %d, want 401", status)
	}
}

func TestGuard_secret_wrongToken_returns401(t *testing.T) {
	_, app := panelApp(devpanel.Config{Enabled: true, Secret: "s3cr3t"})

	if status := getStatus(t, app, "/keel/panel/requests", "Bearer wrong"); status != 401 {
		t.Fatalf("status = %d, want 401", status)
	}
}

func TestGuard_secret_correctToken_returns200(t *testing.T) {
	_, app := panelApp(devpanel.Config{Enabled: true, Secret: "s3cr3t"})

	if status := getStatus(t, app, "/keel/panel/requests", "Bearer s3cr3t"); status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
}

func TestGuard_noSecret_returns200(t *testing.T) {
	_, app := panelApp(devpanel.Config{Enabled: true, Secret: ""})

	if status := getStatus(t, app, "/keel/panel/requests", ""); status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
}

// --- root redirect ---

func TestRoot_redirectsToRequests(t *testing.T) {
	_, app := panelApp(devpanel.Config{Enabled: true})

	req := httptest.NewRequest("GET", "/keel/panel/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode != 302 {
		t.Fatalf("status = %d, want 302", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/keel/panel/requests" {
		t.Fatalf("Location = %q, want %q", loc, "/keel/panel/requests")
	}
}

// --- custom path ---

func TestMount_customPath(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true, Path: "/dev"})
	app := fiber.New()
	p.Mount(app)

	if status := getStatus(t, app, "/dev/requests", ""); status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
	if status := getStatus(t, app, "/keel/panel/requests", ""); status != 404 {
		t.Fatalf("default path should 404, got %d", status)
	}
}
