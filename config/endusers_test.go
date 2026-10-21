package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_EndUsersDefaults(t *testing.T) {
	c := validConfig(t)
	assert.Equal(t, EndUsersConfig{
		MagicLink: EndUsersMagicLinkConfig{TTL: 15 * time.Minute, ResendAfter: time.Minute},
		Session:   EndUsersSessionConfig{Idle: 30 * 24 * time.Hour, Absolute: 90 * 24 * time.Hour, MaxAccounts: 20},
		CSRF:      EndUsersCSRFConfig{MaxAge: time.Hour},
	}, c.EndUsers)
}

func TestLoad_EndUsersFromEnv(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("OTHRA_END_USERS_MAGIC_LINK_TTL", "10m")
	t.Setenv("OTHRA_END_USERS_MAGIC_LINK_RESEND_AFTER", "30s")
	t.Setenv("OTHRA_END_USERS_SESSION_IDLE", "24h")
	t.Setenv("OTHRA_END_USERS_SESSION_ABSOLUTE", "168h")
	t.Setenv("OTHRA_END_USERS_SESSION_MAX_ACCOUNTS", "5")
	t.Setenv("OTHRA_END_USERS_CSRF_MAX_AGE", "30m")
	c, err := Load(missingPath(t))
	require.NoError(t, err)
	assert.Equal(t, EndUsersConfig{
		MagicLink: EndUsersMagicLinkConfig{TTL: 10 * time.Minute, ResendAfter: 30 * time.Second},
		Session:   EndUsersSessionConfig{Idle: 24 * time.Hour, Absolute: 7 * 24 * time.Hour, MaxAccounts: 5},
		CSRF:      EndUsersCSRFConfig{MaxAge: 30 * time.Minute},
	}, c.EndUsers)
}

func TestValidate_EndUsers(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate func(e *EndUsersConfig)
		key    string
	}{
		"defaults ok":                {mutate: func(*EndUsersConfig) {}},
		"ttl below 5m":               {mutate: func(e *EndUsersConfig) { e.MagicLink.TTL = time.Minute }, key: "endUsers.magicLink.ttl"},
		"ttl above 30m":              {mutate: func(e *EndUsersConfig) { e.MagicLink.TTL = time.Hour }, key: "endUsers.magicLink.ttl"},
		"no resend cooldown ok":      {mutate: func(e *EndUsersConfig) { e.MagicLink.ResendAfter = 0 }},
		"negative resend":            {mutate: func(e *EndUsersConfig) { e.MagicLink.ResendAfter = -time.Second }, key: "endUsers.magicLink.resendAfter"},
		"resend not below the ttl":   {mutate: func(e *EndUsersConfig) { e.MagicLink.ResendAfter = 15 * time.Minute }, key: "endUsers.magicLink.resendAfter"},
		"idle below 1h":              {mutate: func(e *EndUsersConfig) { e.Session.Idle = time.Minute }, key: "endUsers.session.idle"},
		"absolute below idle":        {mutate: func(e *EndUsersConfig) { e.Session.Absolute = time.Hour }, key: "endUsers.session.absolute"},
		"no accounts":                {mutate: func(e *EndUsersConfig) { e.Session.MaxAccounts = 0 }, key: "endUsers.session.maxAccounts"},
		"too many accounts":          {mutate: func(e *EndUsersConfig) { e.Session.MaxAccounts = 51 }, key: "endUsers.session.maxAccounts"},
		"csrf below 5m":              {mutate: func(e *EndUsersConfig) { e.CSRF.MaxAge = time.Minute }, key: "endUsers.csrf.maxAge"},
		"csrf above 24h":             {mutate: func(e *EndUsersConfig) { e.CSRF.MaxAge = 48 * time.Hour }, key: "endUsers.csrf.maxAge"},
		"absolute equal to idle ok":  {mutate: func(e *EndUsersConfig) { e.Session.Absolute = e.Session.Idle }},
		"one account per browser ok": {mutate: func(e *EndUsersConfig) { e.Session.MaxAccounts = 1 }},
	} {
		t.Run(name, func(t *testing.T) {
			c := validConfig(t)
			tc.mutate(&c.EndUsers)
			err := c.Validate()
			if tc.key == "" {
				assert.NoError(t, err)
				return
			}
			invalidSetting(tc.key)(t, err)
		})
	}
}
