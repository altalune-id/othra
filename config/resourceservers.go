package config

import "time"

// ResourceServersConfig configures resource servers.
type ResourceServersConfig struct {
	Verification ResourceServersVerificationConfig `yaml:"verification" mapstructure:"verification"`
}

// ResourceServersVerificationConfig configures the RFC 9728 ownership check and its scheduled re-check.
type ResourceServersVerificationConfig struct {
	Interval   time.Duration `yaml:"interval"   mapstructure:"interval"`
	StaleAfter time.Duration `yaml:"staleAfter" mapstructure:"staleAfter"`
	Timeout    time.Duration `yaml:"timeout"    mapstructure:"timeout"`
	// AllowPrivateHosts lets a development deployment verify a resource server on a private or loopback address.
	AllowPrivateHosts bool `yaml:"allowPrivateHosts" mapstructure:"allowPrivateHosts"`
}

func (c *Config) validateResourceServers() error {
	v := c.ResourceServers.Verification
	switch {
	case v.Interval < time.Minute:
		return &InvalidSettingError{Key: "resourceServers.verification.interval", Reason: "must be at least 1m"}
	case v.StaleAfter < v.Interval:
		return &InvalidSettingError{Key: "resourceServers.verification.staleAfter", Reason: "must be at least the interval"}
	case v.Timeout < time.Second || v.Timeout > time.Minute:
		return &InvalidSettingError{Key: "resourceServers.verification.timeout", Reason: "must be between 1s and 1m"}
	case v.AllowPrivateHosts && (c.Env.IsProduction() || c.Mode == ModeCloud):
		return &InvalidSettingError{
			Key: "resourceServers.verification.allowPrivateHosts", Reason: "is for development on self-hosted Othra only",
		}
	}
	return nil
}
