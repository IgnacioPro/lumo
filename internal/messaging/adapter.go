package messaging

// ConfigAdapter adapts Config to the configGetter interface used by providers
type ConfigAdapter struct {
	*Config
}

// NewConfigAdapter creates a new config adapter
func NewConfigAdapter(cfg *Config) *ConfigAdapter {
	return &ConfigAdapter{Config: cfg}
}

// GetURL returns the messaging URL
func (c *ConfigAdapter) GetURL() string {
	return c.URL
}

// GetUsername returns the username
func (c *ConfigAdapter) GetUsername() string {
	return c.Username
}

// GetPassword returns the password
func (c *ConfigAdapter) GetPassword() string {
	return c.Password
}

// IsTLSEnabled returns whether TLS is enabled
func (c *ConfigAdapter) IsTLSEnabled() bool {
	return c.TLS.Enabled
}

// IsTLSInsecure returns whether to skip TLS verification
func (c *ConfigAdapter) IsTLSInsecure() bool {
	return c.TLS.InsecureSkipVerify
}
