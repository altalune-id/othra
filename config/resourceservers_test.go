package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_ResourceServersDefaults(t *testing.T) {
	c := validConfig(t)
	assert.Equal(t, ResourceServersVerificationConfig{Interval: time.Hour, StaleAfter: 24 * time.Hour, Timeout: 5 * time.Second},
		c.ResourceServers.Verification)
}

func TestLoad_ResourceServersFromEnv(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("OTHRA_ENV", "development")
	t.Setenv("OTHRA_MODE", "selfhosted")
	t.Setenv("OTHRA_RESOURCE_SERVERS_VERIFICATION_INTERVAL", "30m")
	t.Setenv("OTHRA_RESOURCE_SERVERS_VERIFICATION_STALE_AFTER", "12h")
	t.Setenv("OTHRA_RESOURCE_SERVERS_VERIFICATION_TIMEOUT", "3s")
	t.Setenv("OTHRA_RESOURCE_SERVERS_VERIFICATION_ALLOW_PRIVATE_HOSTS", "true")
	c, err := Load(missingPath(t))
	require.NoError(t, err)
	assert.Equal(t, ResourceServersVerificationConfig{
		Interval: 30 * time.Minute, StaleAfter: 12 * time.Hour, Timeout: 3 * time.Second, AllowPrivateHosts: true,
	}, c.ResourceServers.Verification)
}

func TestValidate_ResourceServers(t *testing.T) {
	devSelfhosted := func(c *Config) { c.Env, c.Mode = EnvDevelopment, ModeSelfhosted }
	for name, tc := range map[string]struct {
		mutate func(c *Config)
		check  func(t *testing.T, err error)
	}{
		"defaults ok":           {mutate: func(*Config) {}, check: valid},
		"interval below 1m":     {mutate: func(c *Config) { c.ResourceServers.Verification.Interval = time.Second }, check: invalidSetting("resourceServers.verification.interval")},
		"stale before interval": {mutate: func(c *Config) { c.ResourceServers.Verification.StaleAfter = time.Minute }, check: invalidSetting("resourceServers.verification.staleAfter")},
		"timeout below 1s":      {mutate: func(c *Config) { c.ResourceServers.Verification.Timeout = time.Millisecond }, check: invalidSetting("resourceServers.verification.timeout")},
		"timeout above 1m":      {mutate: func(c *Config) { c.ResourceServers.Verification.Timeout = 2 * time.Minute }, check: invalidSetting("resourceServers.verification.timeout")},
		"private hosts in dev":  {mutate: func(c *Config) { devSelfhosted(c); c.ResourceServers.Verification.AllowPrivateHosts = true }, check: valid},
		"private hosts in prod": {mutate: func(c *Config) {
			c.Mode = ModeSelfhosted
			c.Env = EnvProduction
			c.ResourceServers.Verification.AllowPrivateHosts = true
		}, check: invalidSetting("resourceServers.verification.allowPrivateHosts")},
		"private hosts in cloud": {mutate: func(c *Config) {
			devSelfhosted(c)
			c.Mode = ModeCloud
			c.ResourceServers.Verification.AllowPrivateHosts = true
		}, check: invalidSetting("resourceServers.verification.allowPrivateHosts")},
		"no private hosts in prod": {mutate: func(c *Config) { c.Env = EnvProduction }, check: valid},
	} {
		t.Run(name, func(t *testing.T) {
			c := validConfig(t)
			tc.mutate(c)
			tc.check(t, c.validateResourceServers())
		})
	}
}
