package config

import (
	"net/url"
	"time"
)

// JWKSMaxAge is how long every verifier may cache a project's JWKS.
const JWKSMaxAge = 5 * time.Minute

// KeysConfig configures each project's token signing keys.
type KeysConfig struct {
	RSABits  int                `yaml:"rsaBits"  mapstructure:"rsaBits"`
	Rotation KeysRotationConfig `yaml:"rotation" mapstructure:"rotation"`
}

// KeysRotationConfig bounds a rotation: how long a new key is published before it signs, and how long the old one stays published.
type KeysRotationConfig struct {
	PrePublish time.Duration `yaml:"prePublish" mapstructure:"prePublish"`
	Overlap    time.Duration `yaml:"overlap"    mapstructure:"overlap"`
}

func (c *Config) validateKeys() error {
	if err := c.validateIssuerBase(); err != nil {
		return err
	}
	k := c.Keys
	switch {
	case k.RSABits != 2048 && k.RSABits != 3072:
		return &InvalidSettingError{Key: "keys.rsaBits", Reason: "must be 2048 or 3072"}
	case k.Rotation.PrePublish < JWKSMaxAge:
		return &InvalidSettingError{Key: "keys.rotation.prePublish", Reason: "must be at least the JWKS max-age, 5m"}
	case k.Rotation.Overlap < c.OAuth.MaxTokenLifetime()+JWKSMaxAge:
		return &InvalidSettingError{
			Key:    "keys.rotation.overlap",
			Reason: "must be at least the longest token lifetime plus the JWKS max-age, " + (c.OAuth.MaxTokenLifetime() + JWKSMaxAge).String(),
		}
	}
	return nil
}

// SECURITY: every project issuer is http.baseURL + basePath + /p/<handle>; it is an origin only, and https outside development.
func (c *Config) validateIssuerBase() error {
	raw, production := c.HTTP.BaseURL, c.Env.IsProduction()
	if raw == "" {
		if production {
			return &MissingSettingError{Keys: "http.baseURL", Reason: "every project's issuer URL starts with it in production"}
		}
		return nil
	}
	u, err := url.Parse(raw)
	switch {
	case err != nil, u.Host == "", u.Scheme != "https" && (production || u.Scheme != "http"):
		reason := "must be an absolute https:// URL in production; it is every project's issuer"
		if !production {
			reason = "must be an absolute http:// or https:// URL"
		}
		return &InvalidSettingError{Key: "http.baseURL", Reason: reason}
	case u.User != nil, u.RawQuery != "", u.Fragment != "", u.ForceQuery:
		return &InvalidSettingError{Key: "http.baseURL", Reason: "must not carry credentials, a query or a fragment"}
	case u.Path != "" && u.Path != "/":
		return &InvalidSettingError{Key: "http.baseURL", Reason: "must be an origin; put a path prefix in http.basePath"}
	}
	return nil
}
