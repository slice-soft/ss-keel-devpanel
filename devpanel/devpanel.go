package devpanel

import (
	"context"
	"sync"

	"github.com/slice-soft/ss-keel-core/contracts"
)

// DevPanel is the observability addon for Keel applications.
// It implements contracts.Addon, contracts.PanelRegistry and contracts.Manifestable.
//
// Debuggable addons register themselves by calling RegisterAddon during their
// own Register step:
//
//	if panel, ok := app.GetAddon("devpanel").(contracts.PanelRegistry); ok {
//	    panel.RegisterAddon(myAddon)
//	}
type DevPanel struct {
	cfg      Config
	mu       sync.RWMutex
	addons   []contracts.Debuggable
	streams  map[string]*addonStream // keyed by PanelID
	requests *requestBuffer
	logs     *logBuffer
	logBcast *sseBroadcaster[LogEntry]
	ctx      context.Context
	cancel   context.CancelFunc
}

// Compile-time assertions.
var (
	_ contracts.Addon         = (*DevPanel)(nil)
	_ contracts.PanelRegistry = (*DevPanel)(nil)
	_ contracts.Manifestable  = (*DevPanel)(nil)
)

// New creates a new DevPanel with the given configuration.
func New(cfg Config) *DevPanel {
	cfg.setDefaults()
	ctx, cancel := context.WithCancel(context.Background())
	return &DevPanel{
		cfg:      cfg,
		streams:  make(map[string]*addonStream),
		logs:     newLogBuffer(logBufferSize),
		logBcast: newSSEBroadcaster[LogEntry](),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Shutdown stops all addon goroutines. Call it when the application shuts down.
func (p *DevPanel) Shutdown() {
	p.cancel()
}

// Logger returns the panel's PanelLogger, which implements contracts.Logger.
// Use Logger().WithRequestID(id) inside request handlers to associate log
// entries with a specific request.
func (p *DevPanel) Logger() *PanelLogger {
	return &PanelLogger{buf: p.logs, bcast: p.logBcast}
}

// Logs returns a snapshot of captured log entries, oldest first.
func (p *DevPanel) Logs() []LogEntry {
	return p.logs.snapshot()
}

// ID returns the unique identifier for this addon.
func (p *DevPanel) ID() string { return "devpanel" }

// RegisterAddon adds a Debuggable addon to the panel registry and starts a
// goroutine that consumes its PanelEvents() channel.
// Safe for concurrent use.
func (p *DevPanel) RegisterAddon(d contracts.Debuggable) {
	p.mu.Lock()
	defer p.mu.Unlock()
	id := d.PanelID()
	if _, exists := p.streams[id]; exists {
		return // already registered — idempotent
	}
	p.addons = append(p.addons, d)
	p.streams[id] = p.startAddonStream(p.ctx, d)
}

// Addons returns a snapshot of all registered Debuggable addons.
// Safe for concurrent use.
func (p *DevPanel) Addons() []contracts.Debuggable {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]contracts.Debuggable, len(p.addons))
	copy(result, p.addons)
	return result
}

// Config returns the panel configuration.
func (p *DevPanel) Config() Config { return p.cfg }

// Manifest returns the addon metadata for the Keel CLI.
func (p *DevPanel) Manifest() contracts.AddonManifest {
	return contracts.AddonManifest{
		ID:           "devpanel",
		Version:      "1.0.0",
		Capabilities: []string{"observability"},
		Resources:    []string{},
		EnvVars: []contracts.EnvVar{
			{
				Key:         "KEEL_PANEL_ENABLED",
				ConfigKey:   "panel.enabled",
				Description: "Enable the dev panel UI (should be false in production)",
				Required:    false,
				Secret:      false,
				Default:     "true",
				Source:      "devpanel",
			},
			{
				Key:         "KEEL_PANEL_SECRET",
				ConfigKey:   "panel.secret",
				Description: "Bearer token required to access the panel. Leave empty to disable auth.",
				Required:    false,
				Secret:      true,
				Default:     "",
				Source:      "devpanel",
			},
			{
				Key:         "KEEL_PANEL_PATH",
				ConfigKey:   "panel.path",
				Description: "URL prefix for all panel routes",
				Required:    false,
				Secret:      false,
				Default:     "/keel/panel",
				Source:      "devpanel",
			},
		},
	}
}
