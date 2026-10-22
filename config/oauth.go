package config

import "time"

// OAuthConfig sets the lifetimes of what the authorization server issues.
type OAuthConfig struct {
	CodeTTL        time.Duration      `yaml:"codeTTL"        mapstructure:"codeTTL"`
	AccessTokenTTL time.Duration      `yaml:"accessTokenTTL" mapstructure:"accessTokenTTL"`
	IDTokenTTL     time.Duration      `yaml:"idTokenTTL"     mapstructure:"idTokenTTL"`
	Refresh        OAuthRefreshConfig `yaml:"refresh"        mapstructure:"refresh"`
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
	return nil
}
