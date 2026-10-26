package config

import "net/url"

// UpstreamsConfig configures upstream IdPs: Google from config for the install project, and deployer-only qa switches.
type UpstreamsConfig struct {
	Google UpstreamsGoogleConfig `yaml:"google" mapstructure:"google"`
	// AllowPrivateHosts lets a development deployment reach an upstream IdP on a private or loopback address.
	AllowPrivateHosts bool `yaml:"allowPrivateHosts" mapstructure:"allowPrivateHosts"`
}

// UpstreamsGoogleConfig seeds the install project's Google upstream once, and can point Google at a qa fake IdP.
type UpstreamsGoogleConfig struct {
	ClientID     string `yaml:"clientId"     mapstructure:"clientId"`
	ClientSecret string `yaml:"clientSecret" mapstructure:"clientSecret" awareness:"secret"`
	// IssuerOverride replaces Google's issuer, for qa against a fake IdP; never set it in production.
	IssuerOverride string `yaml:"issuerOverride" mapstructure:"issuerOverride"`
}

// SECURITY: the qa switches change whom Othra trusts, so cloud and production refuse them, as the CIMD and RS switches do (authl gap 3).
func (c *Config) validateUpstreams() error {
	u := c.Upstreams
	switch {
	case (u.Google.ClientID == "") != (u.Google.ClientSecret == ""):
		return &MissingSettingError{
			Keys: "upstreams.google.clientId, upstreams.google.clientSecret", Reason: "set both to seed the install project's Google, or neither",
		}
	case len(u.Google.ClientID) > 255:
		return &InvalidSettingError{Key: "upstreams.google.clientId", Reason: "must be at most 255 characters"}
	case (c.Mode == ModeCloud || c.Env.IsProduction()) && (u.Google.IssuerOverride != "" || u.AllowPrivateHosts):
		return &InvalidSettingError{
			Key: "upstreams", Reason: "issuerOverride and allowPrivateHosts are for qa outside production, on self-hosted Othra only",
		}
	case u.Google.IssuerOverride != "" && !absoluteHTTPS(u.Google.IssuerOverride):
		return &InvalidSettingError{Key: "upstreams.google.issuerOverride", Reason: "must be an absolute https:// URL with no query or fragment"}
	}
	return nil
}

// NOTE: an identity's issuer is stored as https only (end_user_identities_issuer_https), so a fake IdP serves TLS too.
func absoluteHTTPS(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.RawQuery == "" && u.Fragment == "" && u.User == nil
}
