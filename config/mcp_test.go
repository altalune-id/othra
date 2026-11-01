package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mcpOAuthOn(c *Config) {
	c.MCP.Enabled = true
}

func TestLoad_MCPDefaults(t *testing.T) {
	c := validConfig(t)
	assert.Empty(t, c.MCP.Audience)
	assert.False(t, c.MCP.AudienceOverride)
	assert.Equal(t, time.Minute, c.MCP.ClockSkew)
	assert.Empty(t, c.MCP.ClientHosts)
}

func TestLoad_MCPFromEnv(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("OTHRA_MCP_AUDIENCE", "https://proxy.example.com/mcp")
	t.Setenv("OTHRA_MCP_AUDIENCE_OVERRIDE", "true")
	t.Setenv("OTHRA_MCP_CLOCK_SKEW", "45s")
	t.Setenv("OTHRA_MCP_CLIENT_HOSTS", "claude.ai,example.com")
	c, err := Load(missingPath(t))
	require.NoError(t, err)
	assert.Equal(t, "https://proxy.example.com/mcp", c.MCP.Audience)
	assert.True(t, c.MCP.AudienceOverride)
	assert.Equal(t, 45*time.Second, c.MCP.ClockSkew)
	assert.Equal(t, []string{"claude.ai", "example.com"}, c.MCP.ClientHosts)
}

func TestMountedMCP(t *testing.T) {
	assert.Equal(t, "https://othra.example.com/mcp", MountedMCP(HTTPConfig{BaseURL: "https://othra.example.com/"}))
	assert.Equal(t, "https://othra.example.com/othra/mcp", MountedMCP(HTTPConfig{BaseURL: "https://othra.example.com", BasePath: "/othra/"}))
	assert.Empty(t, MountedMCP(HTTPConfig{}))
}
