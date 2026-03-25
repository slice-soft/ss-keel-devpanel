package devpanel_test

import (
	"testing"

	"github.com/slice-soft/ss-keel-core/contracts"
	"github.com/slice-soft/ss-keel-devpanel/devpanel"
)

// --- mock ---

type debuggableMock struct {
	id    string
	label string
	ch    chan contracts.PanelEvent
}

func (d *debuggableMock) PanelID() string                    { return d.id }
func (d *debuggableMock) PanelLabel() string                 { return d.label }
func (d *debuggableMock) PanelEvents() <-chan contracts.PanelEvent { return d.ch }

// --- tests ---

func TestNew_defaults(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})

	if p.Config().Path != "/keel/panel" {
		t.Fatalf("Path = %q, want %q", p.Config().Path, "/keel/panel")
	}
}

func TestNew_customPath(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true, Path: "/dev"})

	if p.Config().Path != "/dev" {
		t.Fatalf("Path = %q, want %q", p.Config().Path, "/dev")
	}
}

func TestID(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})

	if p.ID() != "devpanel" {
		t.Fatalf("ID() = %q, want %q", p.ID(), "devpanel")
	}
}

func TestRegisterAddon(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})

	a := &debuggableMock{id: "gorm", label: "GORM"}
	b := &debuggableMock{id: "redis", label: "Redis"}

	p.RegisterAddon(a)
	p.RegisterAddon(b)

	addons := p.Addons()
	if len(addons) != 2 {
		t.Fatalf("Addons() len = %d, want 2", len(addons))
	}
	if addons[0].PanelID() != "gorm" {
		t.Fatalf("addons[0].PanelID() = %q, want %q", addons[0].PanelID(), "gorm")
	}
	if addons[1].PanelID() != "redis" {
		t.Fatalf("addons[1].PanelID() = %q, want %q", addons[1].PanelID(), "redis")
	}
}

func TestAddons_returnsCopy(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	p.RegisterAddon(&debuggableMock{id: "jwt"})

	first := p.Addons()
	p.RegisterAddon(&debuggableMock{id: "mongo"})
	second := p.Addons()

	if len(first) != 1 {
		t.Fatalf("first snapshot len = %d, want 1", len(first))
	}
	if len(second) != 2 {
		t.Fatalf("second snapshot len = %d, want 2", len(second))
	}
}

func TestManifest(t *testing.T) {
	p := devpanel.New(devpanel.Config{Enabled: true})
	m := p.Manifest()

	if m.ID != "devpanel" {
		t.Fatalf("Manifest.ID = %q, want %q", m.ID, "devpanel")
	}
	if len(m.Capabilities) != 1 || m.Capabilities[0] != "observability" {
		t.Fatalf("Manifest.Capabilities = %v, want [observability]", m.Capabilities)
	}
	if len(m.Resources) != 0 {
		t.Fatalf("Manifest.Resources = %v, want []", m.Resources)
	}

	keys := make(map[string]bool)
	for _, ev := range m.EnvVars {
		keys[ev.Key] = true
		if ev.Source != "devpanel" {
			t.Fatalf("EnvVar %q Source = %q, want %q", ev.Key, ev.Source, "devpanel")
		}
	}
	for _, expected := range []string{"KEEL_PANEL_ENABLED", "KEEL_PANEL_SECRET", "KEEL_PANEL_PATH"} {
		if !keys[expected] {
			t.Fatalf("missing env var %q in manifest", expected)
		}
	}

	for _, ev := range m.EnvVars {
		if ev.Key == "KEEL_PANEL_SECRET" && !ev.Secret {
			t.Fatal("KEEL_PANEL_SECRET should be marked as secret")
		}
	}
}
