package boot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	noopmetric "go.opentelemetry.io/otel/metric/noop"

	"altalune.id/othra/apperror"
	"altalune.id/othra/config"
	"altalune.id/othra/internal/grant"
	"altalune.id/othra/internal/kernel"
	"altalune.id/othra/internal/kernel/session"
	"altalune.id/othra/internal/testutil/fakes"
)

func TestGrantConfig(t *testing.T) {
	assert.Equal(t, grant.Config{
		CodeTTL: 30 * time.Second, AccessTokenTTL: 5 * time.Minute, RefreshIdle: 24 * time.Hour, RefreshAbsolute: 48 * time.Hour,
	}, grantConfig(config.OAuthConfig{
		CodeTTL: 30 * time.Second, AccessTokenTTL: 5 * time.Minute, IDTokenTTL: time.Minute,
		Refresh: config.OAuthRefreshConfig{Idle: 24 * time.Hour, Absolute: 48 * time.Hour},
	}))
	assert.Equal(t, grant.Config{
		CodeTTL: time.Minute, AccessTokenTTL: 15 * time.Minute, RefreshIdle: 30 * 24 * time.Hour, RefreshAbsolute: 90 * 24 * time.Hour,
	}, grantConfig(config.OAuthConfig{}), "a config literal without oauth still gives the spec defaults")
}

func TestBuildScheduler_RegistersTheGrantPrune(t *testing.T) {
	k := kernel.Kernel{Meter: noopmetric.NewMeterProvider().Meter("test"), Reporter: apperror.NewReporter(nil, false)}
	svcs := &Services{Sessions: session.NewMemoryStore(), Grants: buildGrants(k, config.Config{}, fakes.Transactor{})}
	r, err := buildScheduler(k, config.Config{Scheduler: config.SchedulerConfig{Enabled: true}}, svcs, testLogger())
	require.NoError(t, err)
	var names []string
	for _, j := range r.Jobs() {
		names = append(names, j.Name)
	}
	assert.Contains(t, names, "grant-prune")
}
