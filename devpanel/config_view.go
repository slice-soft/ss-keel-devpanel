package devpanel

import (
	"os"
	"runtime"

	"github.com/gofiber/fiber/v2"
	"github.com/slice-soft/ss-keel-core/config"
	"github.com/slice-soft/ss-keel-core/contracts"
	"github.com/slice-soft/ss-keel-devpanel/devpanel/ui"
)

const redacted = "••••••••"

// handleConfig renders the config page with env vars, runtime info, and addon
// manifest data. Secret env vars have their values automatically redacted.
func (p *DevPanel) handleConfig() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return render(c, ui.Config(
			p.buildNav("Config"),
			p.buildEnvVarRows(),
			p.buildRuntimeInfo(),
			p.buildConfigAddonRows(),
		))
	}
}

// buildEnvVarRows collects env vars from the panel's own manifest and from
// every registered Manifestable addon. Secret values are redacted.
func (p *DevPanel) buildEnvVarRows() []ui.EnvVarRow {
	var vars []contracts.EnvVar

	// Panel's own env vars.
	for _, ev := range p.Manifest().EnvVars {
		vars = append(vars, ev)
	}

	// Each registered addon that implements Manifestable.
	for _, addon := range p.Addons() {
		if m, ok := addon.(contracts.Manifestable); ok {
			vars = append(vars, m.Manifest().EnvVars...)
		}
	}

	rows := make([]ui.EnvVarRow, len(vars))
	for i, v := range vars {
		value := os.Getenv(v.Key)
		if value == "" && v.ConfigKey != "" {
			if resolved, ok := config.LookupString(v.ConfigKey); ok {
				value = resolved
			}
		}
		if v.Secret && value != "" {
			value = redacted
		}
		if value == "" {
			value = v.Default
		}
		rows[i] = ui.EnvVarRow{
			Key:         v.Key,
			Value:       value,
			Source:      v.Source,
			Required:    v.Required,
			Secret:      v.Secret,
			Description: v.Description,
		}
	}
	return rows
}

// buildRuntimeInfo reads live Go runtime statistics.
func (p *DevPanel) buildRuntimeInfo() ui.RuntimeInfo {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ui.RuntimeInfo{
		GoVersion:   runtime.Version(),
		Goroutines:  runtime.NumGoroutine(),
		HeapAllocMB: float64(ms.HeapAlloc) / 1024 / 1024,
		SysMB:       float64(ms.Sys) / 1024 / 1024,
	}
}

// buildConfigAddonRows builds the addon table rows for the config page.
// Only addons that implement Manifestable have version/capability data.
func (p *DevPanel) buildConfigAddonRows() []ui.ConfigAddonRow {
	addons := p.Addons()
	rows := make([]ui.ConfigAddonRow, len(addons))
	for i, addon := range addons {
		row := ui.ConfigAddonRow{ID: addon.PanelID()}
		if m, ok := addon.(contracts.Manifestable); ok {
			manifest := m.Manifest()
			row.Version = manifest.Version
			row.Capabilities = manifest.Capabilities
			row.Resources = manifest.Resources
		}
		rows[i] = row
	}
	return rows
}
