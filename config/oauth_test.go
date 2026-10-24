package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_OAuthDefaults(t *testing.T) {
	c := validConfig(t)
	assert.Equal(t, OAuthConfig{
		CodeTTL:        time.Minute,
		AccessTokenTTL: 15 * time.Minute,
		IDTokenTTL:     5 * time.Minute,
		Refresh:        OAuthRefreshConfig{Idle: 30 * 24 * time.Hour, Absolute: 90 * 24 * time.Hour},
		CIMD:           OAuthCIMDConfig{FetchTimeout: 5 * time.Second, MaxBytes: 5120, CacheMin: 5 * time.Minute, CacheMax: 24 * time.Hour},
	}, c.OAuth)
	assert.Equal(t, 15*time.Minute, c.OAuth.MaxTokenLifetime())
}

func TestLoad_OAuthFromEnv(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("OTHRA_OAUTH_CODE_TTL", "30s")
	t.Setenv("OTHRA_OAUTH_ACCESS_TOKEN_TTL", "5m")
	t.Setenv("OTHRA_OAUTH_ID_TOKEN_TTL", "10m")
	t.Setenv("OTHRA_OAUTH_REFRESH_IDLE", "24h")
	t.Setenv("OTHRA_OAUTH_REFRESH_ABSOLUTE", "168h")
	t.Setenv("OTHRA_OAUTH_CIMD_FETCH_TIMEOUT", "3s")
	t.Setenv("OTHRA_OAUTH_CIMD_MAX_BYTES", "4096")
	t.Setenv("OTHRA_OAUTH_CIMD_CACHE_MIN", "10m")
	t.Setenv("OTHRA_OAUTH_CIMD_CACHE_MAX", "12h")
	c, err := Load(missingPath(t))
	require.NoError(t, err)
	assert.Equal(t, OAuthConfig{
		CodeTTL:        30 * time.Second,
		AccessTokenTTL: 5 * time.Minute,
		IDTokenTTL:     10 * time.Minute,
		Refresh:        OAuthRefreshConfig{Idle: 24 * time.Hour, Absolute: 7 * 24 * time.Hour},
		CIMD: OAuthCIMDConfig{
			FetchTimeout: 3 * time.Second, MaxBytes: 4096, CacheMin: 10 * time.Minute, CacheMax: 12 * time.Hour,
		},
	}, c.OAuth)
	assert.Equal(t, 10*time.Minute, c.OAuth.MaxTokenLifetime())
}

func TestValidate_OAuth(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate func(o *OAuthConfig)
		key    string
	}{
		"defaults ok":               {mutate: func(*OAuthConfig) {}},
		"code below 10s":            {mutate: func(o *OAuthConfig) { o.CodeTTL = 9 * time.Second }, key: "oauth.codeTTL"},
		"code above 10m":            {mutate: func(o *OAuthConfig) { o.CodeTTL = 11 * time.Minute }, key: "oauth.codeTTL"},
		"code exactly 10m ok":       {mutate: func(o *OAuthConfig) { o.CodeTTL = 10 * time.Minute }},
		"access below 1m":           {mutate: func(o *OAuthConfig) { o.AccessTokenTTL = 59 * time.Second }, key: "oauth.accessTokenTTL"},
		"access above 15m":          {mutate: func(o *OAuthConfig) { o.AccessTokenTTL = 16 * time.Minute }, key: "oauth.accessTokenTTL"},
		"access exactly 15m ok":     {mutate: func(o *OAuthConfig) { o.AccessTokenTTL = 15 * time.Minute }},
		"id token below 1m":         {mutate: func(o *OAuthConfig) { o.IDTokenTTL = 59 * time.Second }, key: "oauth.idTokenTTL"},
		"id token above 10m":        {mutate: func(o *OAuthConfig) { o.IDTokenTTL = 11 * time.Minute }, key: "oauth.idTokenTTL"},
		"id token exactly 10m ok":   {mutate: func(o *OAuthConfig) { o.IDTokenTTL = 10 * time.Minute }},
		"id token exactly 1m ok":    {mutate: func(o *OAuthConfig) { o.IDTokenTTL = time.Minute }},
		"refresh idle below 1h":     {mutate: func(o *OAuthConfig) { o.Refresh.Idle = 59 * time.Minute }, key: "oauth.refresh.idle"},
		"absolute below idle":       {mutate: func(o *OAuthConfig) { o.Refresh.Absolute = 24 * time.Hour }, key: "oauth.refresh.absolute"},
		"absolute above a year":     {mutate: func(o *OAuthConfig) { o.Refresh.Absolute = 366 * 24 * time.Hour }, key: "oauth.refresh.absolute"},
		"absolute equal to idle ok": {mutate: func(o *OAuthConfig) { o.Refresh.Absolute = o.Refresh.Idle }},
	} {
		t.Run(name, func(t *testing.T) {
			c := validConfig(t)
			tc.mutate(&c.OAuth)
			err := c.Validate()
			if tc.key == "" {
				assert.NoError(t, err)
				return
			}
			invalidSetting(tc.key)(t, err)
		})
	}
}

func TestValidate_KeysOverlapFollowsTheLongestTokenLifetime(t *testing.T) {
	c := validConfig(t)
	c.OAuth.AccessTokenTTL, c.OAuth.IDTokenTTL = 5*time.Minute, 10*time.Minute
	c.Keys.Rotation.Overlap = 14 * time.Minute
	invalidSetting("keys.rotation.overlap")(t, c.Validate())
	c.Keys.Rotation.Overlap = 10*time.Minute + JWKSMaxAge
	assert.NoError(t, c.Validate())
}

func TestValidate_OAuthCIMD(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate func(c *Config)
		key    string
	}{
		"defaults ok":                  {mutate: func(*Config) {}},
		"timeout below 1s":             {mutate: func(c *Config) { c.OAuth.CIMD.FetchTimeout = 500 * time.Millisecond }, key: "oauth.cimd.fetchTimeout"},
		"timeout above 10s":            {mutate: func(c *Config) { c.OAuth.CIMD.FetchTimeout = 11 * time.Second }, key: "oauth.cimd.fetchTimeout"},
		"more than 5 KB":               {mutate: func(c *Config) { c.OAuth.CIMD.MaxBytes = 5121 }, key: "oauth.cimd.maxBytes"},
		"less than 1 KB":               {mutate: func(c *Config) { c.OAuth.CIMD.MaxBytes = 1023 }, key: "oauth.cimd.maxBytes"},
		"cache min below 5m":           {mutate: func(c *Config) { c.OAuth.CIMD.CacheMin = 4 * time.Minute }, key: "oauth.cimd.cacheMin"},
		"cache min above 24h":          {mutate: func(c *Config) { c.OAuth.CIMD.CacheMin = 25 * time.Hour }, key: "oauth.cimd.cacheMin"},
		"cache max below min":          {mutate: func(c *Config) { c.OAuth.CIMD.CacheMin, c.OAuth.CIMD.CacheMax = time.Hour, 30*time.Minute }, key: "oauth.cimd.cacheMax"},
		"cache max above 24h":          {mutate: func(c *Config) { c.OAuth.CIMD.CacheMax = 25 * time.Hour }, key: "oauth.cimd.cacheMax"},
		"cache bounds at the limits":   {mutate: func(c *Config) { c.OAuth.CIMD.CacheMin, c.OAuth.CIMD.CacheMax = 5*time.Minute, 24*time.Hour }},
		"private hosts in production":  {mutate: func(c *Config) { c.OAuth.CIMD.AllowPrivateHosts = true }, key: "oauth.cimd.allowPrivateHosts"},
		"private hosts in development": {mutate: func(c *Config) { c.Env = EnvDevelopment; c.OAuth.CIMD.AllowPrivateHosts = true }},
	} {
		t.Run(name, func(t *testing.T) {
			c := validConfig(t)
			tc.mutate(c)
			err := c.Validate()
			if tc.key == "" {
				assert.NoError(t, err)
				return
			}
			invalidSetting(tc.key)(t, err)
		})
	}
}
