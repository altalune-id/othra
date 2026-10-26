package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_UpstreamsDefaults(t *testing.T) {
	c := validConfig(t)
	assert.Equal(t, UpstreamsConfig{}, c.Upstreams, "nothing is seeded and no qa switch is on")
}

func TestLoad_UpstreamsFromEnv(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("OTHRA_ENV", "development")
	t.Setenv("OTHRA_UPSTREAMS_GOOGLE_CLIENT_ID", "1234.apps.googleusercontent.com")
	t.Setenv("OTHRA_UPSTREAMS_GOOGLE_CLIENT_SECRET", "GOCSPX-secret")
	t.Setenv("OTHRA_UPSTREAMS_GOOGLE_ISSUER_OVERRIDE", "https://fakeidp.test")
	t.Setenv("OTHRA_UPSTREAMS_ALLOW_PRIVATE_HOSTS", "true")
	c, err := Load(missingPath(t))
	require.NoError(t, err)
	assert.Equal(t, UpstreamsConfig{
		Google:            UpstreamsGoogleConfig{ClientID: "1234.apps.googleusercontent.com", ClientSecret: "GOCSPX-secret", IssuerOverride: "https://fakeidp.test"},
		AllowPrivateHosts: true,
	}, c.Upstreams)
}

func TestValidate_Upstreams(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate func(c *Config)
		check  func(t *testing.T, err error)
	}{
		"defaults ok": {mutate: func(*Config) {}, check: func(t *testing.T, err error) { assert.NoError(t, err) }},
		"both seeded ok": {
			mutate: func(c *Config) { c.Upstreams.Google = UpstreamsGoogleConfig{ClientID: "id", ClientSecret: "secret"} },
			check:  func(t *testing.T, err error) { assert.NoError(t, err) },
		},
		"an id without a secret": {
			mutate: func(c *Config) { c.Upstreams.Google.ClientID = "id" },
			check:  missingSetting("upstreams.google.clientId, upstreams.google.clientSecret"),
		},
		"a secret without an id": {
			mutate: func(c *Config) { c.Upstreams.Google.ClientSecret = "secret" },
			check:  missingSetting("upstreams.google.clientId, upstreams.google.clientSecret"),
		},
		"a long client id": {
			mutate: func(c *Config) {
				c.Upstreams.Google = UpstreamsGoogleConfig{ClientID: string(make([]byte, 256)), ClientSecret: "s"}
			},
			check: invalidSetting("upstreams.google.clientId"),
		},
		"an https override in development": {
			mutate: func(c *Config) { c.Env = EnvDevelopment; c.Upstreams.Google.IssuerOverride = "https://fakeidp.test" },
			check:  func(t *testing.T, err error) { assert.NoError(t, err) },
		},
		"private hosts in development": {
			mutate: func(c *Config) { c.Env = EnvDevelopment; c.Upstreams.AllowPrivateHosts = true },
			check:  func(t *testing.T, err error) { assert.NoError(t, err) },
		},
		"BL-8: an override in production": {
			mutate: func(c *Config) { c.Upstreams.Google.IssuerOverride = "https://fakeidp.test" },
			check:  invalidSetting("upstreams"),
		},
		"BL-8: private hosts in production": {
			mutate: func(c *Config) { c.Upstreams.AllowPrivateHosts = true },
			check:  invalidSetting("upstreams"),
		},
		"an http override in development": {
			mutate: func(c *Config) { c.Env = EnvDevelopment; c.Upstreams.Google.IssuerOverride = "http://localhost:9000" },
			check:  invalidSetting("upstreams.google.issuerOverride"),
		},
		"an override with a query": {
			mutate: func(c *Config) {
				c.Env = EnvDevelopment
				c.Upstreams.Google.IssuerOverride = "https://fakeidp.test/?x=1"
			},
			check: invalidSetting("upstreams.google.issuerOverride"),
		},
		"a relative override": {
			mutate: func(c *Config) { c.Env = EnvDevelopment; c.Upstreams.Google.IssuerOverride = "fakeidp" },
			check:  invalidSetting("upstreams.google.issuerOverride"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := validConfig(t)
			tc.mutate(c)
			tc.check(t, c.Validate())
		})
	}
}

func TestValidate_UpstreamsRefusedInCloud(t *testing.T) {
	c := validConfig(t)
	c.Mode = ModeCloud
	c.Upstreams.AllowPrivateHosts = true
	assert.Error(t, c.validateUpstreams())
	c.Upstreams = UpstreamsConfig{Google: UpstreamsGoogleConfig{IssuerOverride: "https://fakeidp.test"}}
	invalidSetting("upstreams")(t, c.validateUpstreams())
}
