package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/internal/kernel/ratelimit"
	"altalune.id/othra/internal/kernel/ratelimit/memory"
	"altalune.id/othra/internal/kernel/ratelimit/ratelimittest"
	"altalune.id/othra/internal/web/layout"
)

type brokenLimiter struct{ calls int }

func (b *brokenLimiter) Allow(context.Context, string, ratelimit.Rule) (ratelimit.Result, error) {
	b.calls++
	return ratelimit.Result{}, errors.New("store down")
}

func (b *brokenLimiter) Reset(context.Context, string) error { return errors.New("store down") }

func memoryLimits(t *testing.T) (layout.Limits, *ratelimittest.Clock) {
	t.Helper()
	clock := ratelimittest.NewClock()
	return layout.Limits{Limiter: memory.New(10_000, memory.WithClock(clock.Now))}, clock
}

func TestRules(t *testing.T) {
	for _, r := range []ratelimit.Rule{LoginEmailRule(), LoginIPRule(), ResetEmailRule(), ResetIPRule(), ResetLinkIPRule()} {
		require.NoError(t, r.Validate(), r.Name)
	}
	assert.Equal(t, ratelimit.Rule{Name: "login-email", Rate: 10, Per: 15 * time.Minute, Burst: 5}, LoginEmailRule())
	assert.Equal(t, ratelimit.Rule{Name: "login-ip", Rate: 60, Per: 15 * time.Minute, Burst: 60}, LoginIPRule())
	assert.Equal(t, ratelimit.Rule{Name: "reset-email", Rate: 3, Per: time.Hour, Burst: 3}, ResetEmailRule())
	assert.Equal(t, ratelimit.Rule{Name: "reset-ip", Rate: 20, Per: time.Hour, Burst: 20}, ResetIPRule())
	assert.Equal(t, ratelimit.Rule{Name: "reset-link-ip", Rate: 20, Per: time.Hour, Burst: 20}, ResetLinkIPRule())
}
