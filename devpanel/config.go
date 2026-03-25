package devpanel

// Config holds the configuration for the dev panel.
// The panel should only be enabled in non-production environments.
type Config struct {
	// Enabled controls whether the panel is active.
	// Defaults to true; set to false in production.
	Enabled bool `keel:"panel.enabled,required"`

	// Secret is an optional bearer token that protects the panel routes.
	// When set, all requests must include: Authorization: Bearer <secret>
	Secret string `keel:"panel.secret"`

	// Path is the URL prefix for all panel routes.
	// Defaults to "/keel/panel".
	Path string `keel:"panel.path,required"`
}

func (c *Config) setDefaults() {
	if c.Path == "" {
		c.Path = "/keel/panel"
	}
}
