package config

import "time"

// EndUsersConfig configures how a project's end users sign in on the hosted pages.
type EndUsersConfig struct {
	MagicLink EndUsersMagicLinkConfig `yaml:"magicLink" mapstructure:"magicLink"`
	Session   EndUsersSessionConfig   `yaml:"session"   mapstructure:"session"`
	CSRF      EndUsersCSRFConfig      `yaml:"csrf"      mapstructure:"csrf"`
}

// EndUsersMagicLinkConfig configures the sign-in link sent by email.
type EndUsersMagicLinkConfig struct {
	TTL         time.Duration `yaml:"ttl"         mapstructure:"ttl"`
	ResendAfter time.Duration `yaml:"resendAfter" mapstructure:"resendAfter"`
}

// EndUsersSessionConfig configures the end-user browser sessions.
type EndUsersSessionConfig struct {
	Idle        time.Duration `yaml:"idle"        mapstructure:"idle"`
	Absolute    time.Duration `yaml:"absolute"    mapstructure:"absolute"`
	MaxAccounts int           `yaml:"maxAccounts" mapstructure:"maxAccounts"`
}

// EndUsersCSRFConfig configures the CSRF token of the hosted forms.
type EndUsersCSRFConfig struct {
	MaxAge time.Duration `yaml:"maxAge" mapstructure:"maxAge"`
}

func (c *Config) validateEndUsers() error {
	e := c.EndUsers
	switch {
	case e.MagicLink.TTL < 5*time.Minute || e.MagicLink.TTL > 30*time.Minute:
		return &InvalidSettingError{Key: "endUsers.magicLink.ttl", Reason: "must be between 5m and 30m"}
	case e.MagicLink.ResendAfter < 0 || e.MagicLink.ResendAfter >= e.MagicLink.TTL:
		return &InvalidSettingError{Key: "endUsers.magicLink.resendAfter", Reason: "must be at least 0 and less than the ttl"}
	case e.Session.Idle < time.Hour:
		return &InvalidSettingError{Key: "endUsers.session.idle", Reason: "must be at least 1h"}
	case e.Session.Absolute < e.Session.Idle:
		return &InvalidSettingError{Key: "endUsers.session.absolute", Reason: "must be at least the idle timeout"}
	case e.Session.MaxAccounts < 1 || e.Session.MaxAccounts > 50:
		return &InvalidSettingError{Key: "endUsers.session.maxAccounts", Reason: "must be between 1 and 50"}
	case e.CSRF.MaxAge < 5*time.Minute || e.CSRF.MaxAge > 24*time.Hour:
		return &InvalidSettingError{Key: "endUsers.csrf.maxAge", Reason: "must be between 5m and 24h"}
	}
	return nil
}
