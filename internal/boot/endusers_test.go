package boot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	noopmetric "go.opentelemetry.io/otel/metric/noop"

	"altalune.id/othra/apperror"
	"altalune.id/othra/config"
	"altalune.id/othra/internal/enduser"
	"altalune.id/othra/internal/kernel"
	"altalune.id/othra/internal/kernel/session"
	"altalune.id/othra/internal/testutil/fakes"
)

func TestEndUsersConfig(t *testing.T) {
	assert.Equal(t, enduser.Config{
		LinkTTL: 10 * time.Minute, ResendAfter: time.Minute, Idle: 24 * time.Hour, Absolute: 48 * time.Hour, MaxAccounts: 5,
	}, endUsersConfig(config.EndUsersConfig{
		MagicLink: config.EndUsersMagicLinkConfig{TTL: 10 * time.Minute, ResendAfter: time.Minute},
		Session:   config.EndUsersSessionConfig{Idle: 24 * time.Hour, Absolute: 48 * time.Hour, MaxAccounts: 5},
	}))
}

func TestEndUsersConfig_ZeroFallsBackToTheDefaults(t *testing.T) {
	assert.Equal(t, enduser.Config{
		LinkTTL: 15 * time.Minute, ResendAfter: time.Minute, Idle: 30 * 24 * time.Hour, Absolute: 90 * 24 * time.Hour, MaxAccounts: 20,
	}, endUsersConfig(config.EndUsersConfig{}), "a config literal without endUsers still gives live sessions")
}

func TestBuildScheduler_RegistersTheEndUserPrune(t *testing.T) {
	k := kernel.Kernel{Meter: noopmetric.NewMeterProvider().Meter("test"), Reporter: apperror.NewReporter(nil, false)}
	svcs := &Services{Sessions: session.NewMemoryStore(), EndUsers: buildEndUsers(k, config.Config{}, fakes.Transactor{}, nil)}
	r, err := buildScheduler(k, config.Config{Scheduler: config.SchedulerConfig{Enabled: true}}, svcs, testLogger())
	require.NoError(t, err)
	var names []string
	for _, j := range r.Jobs() {
		names = append(names, j.Name)
	}
	assert.Contains(t, names, "enduser-prune")
}
