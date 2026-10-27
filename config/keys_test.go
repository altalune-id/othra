package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_KeysDefaults(t *testing.T) {
	c := validConfig(t)
	assert.Equal(t, KeysConfig{RSABits: 2048, Rotation: KeysRotationConfig{PrePublish: 10 * time.Minute, Overlap: 7 * 24 * time.Hour}}, c.Keys)
}

func TestLoad_KeysFromEnv(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("OTHRA_KEYS_RSA_BITS", "3072")
	t.Setenv("OTHRA_KEYS_ROTATION_PRE_PUBLISH", "15m")
	t.Setenv("OTHRA_KEYS_ROTATION_OVERLAP", "24h")
	c, err := Load(missingPath(t))
	require.NoError(t, err)
	assert.Equal(t, KeysConfig{RSABits: 3072, Rotation: KeysRotationConfig{PrePublish: 15 * time.Minute, Overlap: 24 * time.Hour}}, c.Keys)
}

func TestValidate_Keys(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate func(c *Config)
		check  func(t *testing.T, err error)
	}{
		"defaults ok":            {mutate: func(*Config) {}, check: func(t *testing.T, err error) { assert.NoError(t, err) }},
		"3072 bits ok":           {mutate: func(c *Config) { c.Keys.RSABits = 3072 }, check: func(t *testing.T, err error) { assert.NoError(t, err) }},
		"1024 bits":              {mutate: func(c *Config) { c.Keys.RSABits = 1024 }, check: invalidSetting("keys.rsaBits")},
		"4096 bits":              {mutate: func(c *Config) { c.Keys.RSABits = 4096 }, check: invalidSetting("keys.rsaBits")},
		"pre-publish below 5m":   {mutate: func(c *Config) { c.Keys.Rotation.PrePublish = 4 * time.Minute }, check: invalidSetting("keys.rotation.prePublish")},
		"pre-publish exactly 5m": {mutate: func(c *Config) { c.Keys.Rotation.PrePublish = JWKSMaxAge }, check: func(t *testing.T, err error) { assert.NoError(t, err) }},
		"overlap below 20m":      {mutate: func(c *Config) { c.Keys.Rotation.Overlap = 19 * time.Minute }, check: invalidSetting("keys.rotation.overlap")},
		"overlap exactly 20m":    {mutate: func(c *Config) { c.Keys.Rotation.Overlap = 20 * time.Minute }, check: func(t *testing.T, err error) { assert.NoError(t, err) }},
		"production needs a base url": {
			mutate: func(c *Config) { c.HTTP.BaseURL = "" },
			check:  missingSetting("http.baseURL"),
		},
		"production refuses an http issuer": {
			mutate: func(c *Config) { c.HTTP.BaseURL = "http://othra.example.com" },
			check:  invalidSetting("http.baseURL"),
		},
		"development with no base url": {
			mutate: func(c *Config) { c.Env = EnvDevelopment; c.HTTP.BaseURL = "" },
			check:  func(t *testing.T, err error) { assert.NoError(t, err) },
		},
		"a trailing slash is fine": {
			mutate: func(c *Config) { c.HTTP.BaseURL = "https://othra.example.com/" },
			check:  func(t *testing.T, err error) { assert.NoError(t, err) },
		},
		"a path belongs in http.basePath": {
			mutate: func(c *Config) { c.HTTP.BaseURL = "https://othra.example.com/auth" },
			check:  issuerBaseError,
		},
		"a path is refused in development too": {
			mutate: func(c *Config) { c.Env = EnvDevelopment; c.HTTP.BaseURL = "http://localhost:5150/auth" },
			check:  issuerBaseError,
		},
		"credentials": {
			mutate: func(c *Config) { c.HTTP.BaseURL = "https://user:pw-secret@othra.example.com" },
			check:  issuerBaseError,
		},
		"a query": {
			mutate: func(c *Config) { c.Env = EnvDevelopment; c.HTTP.BaseURL = "http://localhost:5150/?x=1" },
			check:  issuerBaseError,
		},
		"a fragment": {
			mutate: func(c *Config) { c.HTTP.BaseURL = "https://othra.example.com#top" },
			check:  issuerBaseError,
		},
		"not a url in development": {
			mutate: func(c *Config) { c.Env = EnvDevelopment; c.HTTP.BaseURL = "localhost:5150" },
			check:  issuerBaseError,
		},
		"development allows an http issuer": {
			mutate: func(c *Config) { c.Env = EnvDevelopment; c.HTTP.BaseURL = "http://localhost:5150" },
			check:  func(t *testing.T, err error) { assert.NoError(t, err) },
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := validConfig(t)
			c.HTTP.BaseURL = "https://othra.example.com"
			tc.mutate(c)
			tc.check(t, c.validateKeys())
		})
	}
}

func issuerBaseError(t *testing.T, err error) {
	t.Helper()
	invalidSetting("http.baseURL")(t, err)
	assert.NotContains(t, err.Error(), "pw-secret", "the url is not echoed")
}
