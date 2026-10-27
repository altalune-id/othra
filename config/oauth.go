package config

import "time"

// OAuthConfig sets the lifetimes of what the authorization server issues.
type OAuthConfig struct {
	CodeTTL        time.Duration      `yaml:"codeTTL"        mapstructure:"codeTTL"`
	AccessTokenTTL time.Duration      `yaml:"accessTokenTTL" mapstructure:"accessTokenTTL"`
	IDTokenTTL     time.Duration      `yaml:"idTokenTTL"     mapstructure:"idTokenTTL"`
	Refresh        OAuthRefreshConfig `yaml:"refresh"        mapstructure:"refresh"`
	CIMD           OAuthCIMDConfig    `yaml:"cimd"           mapstructure:"cimd"`
}

// OAuthCIMDConfig bounds the fetch and cache of Client ID Metadata Documents.
type OAuthCIMDConfig struct {
	FetchTimeout time.Duration `yaml:"fetchTimeout" mapstructure:"fetchTimeout"`
	MaxBytes     int           `yaml:"maxBytes"     mapstructure:"maxBytes"`
	CacheMin     time.Duration `yaml:"cacheMin"     mapstructure:"cacheMin"`
	CacheMax     time.Duration `yaml:"cacheMax"     mapstructure:"cacheMax"`
	// AllowPrivateHosts lets a development deployment fetch a metadata document from a private or loopback address.
	AllowPrivateHosts bool `yaml:"allowPrivateHosts" mapstructure:"allowPrivateHosts"`
}

// OAuthRefreshConfig bounds a refresh grant: how long it may sit unused, and how long it lives at most.
type OAuthRefreshConfig struct {
	Idle     time.Duration `yaml:"idle"     mapstructure:"idle"`
	Absolute time.Duration `yaml:"absolute" mapstructure:"absolute"`
}

// MaxTokenLifetime is the longest lifetime of a token a verifier may hold: the access or the ID token.
func (o OAuthConfig) MaxTokenLifetime() time.Duration {
	return max(o.AccessTokenTTL, o.IDTokenTTL)
}

// SECURITY: RFC 6749 §4.1.2 caps a code at 10 minutes; short token lifetimes keep a leaked token's window small.
func (c *Config) validateOAuth() error {
	o := c.OAuth
	switch {
	case o.CodeTTL < 10*time.Second || o.CodeTTL > 10*time.Minute:
		return &InvalidSettingError{Key: "oauth.codeTTL", Reason: "must be between 10s and 10m"}
	case o.AccessTokenTTL < time.Minute || o.AccessTokenTTL > 15*time.Minute:
		return &InvalidSettingError{Key: "oauth.accessTokenTTL", Reason: "must be between 1m and 15m"}
	case o.IDTokenTTL < time.Minute || o.IDTokenTTL > 10*time.Minute:
		return &InvalidSettingError{Key: "oauth.idTokenTTL", Reason: "must be between 1m and 10m"}
	case o.Refresh.Idle < time.Hour:
		return &InvalidSettingError{Key: "oauth.refresh.idle", Reason: "must be at least 1h"}
	case o.Refresh.Absolute < o.Refresh.Idle || o.Refresh.Absolute > 365*24*time.Hour:
		return &InvalidSettingError{Key: "oauth.refresh.absolute", Reason: "must be at least the idle timeout and at most 8760h"}
	}
	return c.validateCIMD()
}

// SECURITY: CIMD -02 §8.7, a client-controlled URL is fetched briefly, small, and never from a private host in production.
func (c *Config) validateCIMD() error {
	m := c.OAuth.CIMD
	switch {
	case m.FetchTimeout < time.Second || m.FetchTimeout > 10*time.Second:
		return &InvalidSettingError{Key: "oauth.cimd.fetchTimeout", Reason: "must be between 1s and 10s"}
	case m.MaxBytes < 1024 || m.MaxBytes > 5120:
		return &InvalidSettingError{Key: "oauth.cimd.maxBytes", Reason: "must be between 1024 and 5120"}
	case m.CacheMin < 5*time.Minute || m.CacheMin > 24*time.Hour:
		return &InvalidSettingError{Key: "oauth.cimd.cacheMin", Reason: "must be between 5m and 24h"}
	case m.CacheMax < m.CacheMin || m.CacheMax > 24*time.Hour:
		return &InvalidSettingError{Key: "oauth.cimd.cacheMax", Reason: "must be at least cacheMin and at most 24h"}
	case m.AllowPrivateHosts && (c.Env.IsProduction() || c.Mode == ModeCloud):
		return &InvalidSettingError{Key: "oauth.cimd.allowPrivateHosts", Reason: "is for development on self-hosted Othra only"}
	}
	return nil
}
