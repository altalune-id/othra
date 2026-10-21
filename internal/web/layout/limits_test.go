package layout

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/kernel/ratelimit"
	"altalune.id/othra/internal/kernel/ratelimit/memory"
	"altalune.id/othra/internal/kernel/ratelimit/ratelimittest"
)

var (
	ipRule    = ratelimit.Rule{Name: "login-ip", Rate: 60, Per: 15 * time.Minute, Burst: 60}
	emailRule = ratelimit.Rule{Name: "login-email", Rate: 10, Per: 15 * time.Minute, Burst: 5}
)

type brokenLimiter struct{ calls int }

func (b *brokenLimiter) Allow(context.Context, string, ratelimit.Rule) (ratelimit.Result, error) {
	b.calls++
	return ratelimit.Result{}, errors.New("store down")
}

func (b *brokenLimiter) Reset(context.Context, string) error { return errors.New("store down") }

type recordingLimiter struct{ keys []string }

func (r *recordingLimiter) Allow(_ context.Context, key string, _ ratelimit.Rule) (ratelimit.Result, error) {
	r.keys = append(r.keys, key)
	return ratelimit.Result{Allowed: true}, nil
}

func (r *recordingLimiter) Reset(context.Context, string) error { return nil }

func memoryLimits(t *testing.T) (Limits, *ratelimittest.Clock) {
	t.Helper()
	clock := ratelimittest.NewClock()
	return Limits{Limiter: memory.New(10_000, memory.WithClock(clock.Now))}, clock
}

func TestLimits_ZeroValueLimitsNothing(t *testing.T) {
	var l Limits
	r := httptest.NewRequest(http.MethodPost, "/login", nil)
	for range 1000 {
		require.NoError(t, l.LimitIP(r, ipRule))
		require.NoError(t, l.LimitEmail(t.Context(), "a@x.io", emailRule))
	}
	require.NoError(t, l.ForgetEmail(t.Context(), "a@x.io", emailRule))
}

func TestLimits_KeysByClientIPAndEmailHash(t *testing.T) {
	rec := &recordingLimiter{}
	l := Limits{Limiter: rec, TrustedProxyHops: 2}
	r := httptest.NewRequest(http.MethodPost, "/login", nil)
	r.RemoteAddr = "10.0.0.1:1"
	r.Header.Set("X-Forwarded-For", "6.6.6.6, 203.0.113.9, 198.51.100.7")
	require.NoError(t, l.LimitIP(r, ipRule))
	require.NoError(t, l.LimitEmail(t.Context(), " Ada@Example.com", emailRule))
	assert.Equal(t, []string{
		"login-ip:ip:203.0.113.9",
		ratelimit.Key(emailRule, ratelimit.EmailSubject("ada@example.com")),
	}, rec.keys)
}

func TestLimits_OverTheLimit(t *testing.T) {
	l, _ := memoryLimits(t)
	rule := emailRule
	for range rule.Burst {
		require.NoError(t, l.LimitEmail(t.Context(), "a@x.io", rule))
	}
	err := l.LimitEmail(t.Context(), "a@x.io", rule)
	var rl *RateLimitedError
	require.ErrorAs(t, err, &rl)
	assert.Equal(t, 90*time.Second, rl.RetryAfter)

	require.NoError(t, l.ForgetEmail(t.Context(), "a@x.io", rule))
	assert.NoError(t, l.LimitEmail(t.Context(), "a@x.io", rule))
}

func TestLimits_ABrokenLimiterIsAnError(t *testing.T) {
	l := Limits{Limiter: &brokenLimiter{}}
	err := l.LimitIP(httptest.NewRequest(http.MethodPost, "/login", nil), ipRule)
	require.Error(t, err)
	assert.False(t, IsRateLimitedError(err))
	assert.Error(t, l.LimitEmail(t.Context(), "a@x.io", emailRule))
	assert.Error(t, l.ForgetEmail(t.Context(), "a@x.io", emailRule))
}

func TestWriteLimited(t *testing.T) {
	var reported []string
	kit := Kit{Unexpected: func(_ context.Context, msg string, _ error, _ ...any) *apperror.AppError {
		reported = append(reported, msg)
		return nil
	}}

	rec := httptest.NewRecorder()
	WriteLimited(rec, httptest.NewRequest(http.MethodGet, "/x", nil), kit, "Invite", &RateLimitedError{RetryAfter: 61 * time.Second})
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "61", rec.Header().Get("Retry-After"))
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Contains(t, rec.Body.String(), "Too many attempts. Try again in 2 minutes.")
	assert.Empty(t, reported)

	rec = httptest.NewRecorder()
	WriteLimited(rec, httptest.NewRequest(http.MethodGet, "/x", nil), kit, "Invite", errors.New("store down"))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "5", rec.Header().Get("Retry-After"))
	assert.Contains(t, rec.Body.String(), "Sign-in is temporarily unavailable")
	assert.NotContains(t, rec.Body.String(), "store down")
	assert.Equal(t, []string{"web: rate limit"}, reported)
}

func TestRateLimitedError(t *testing.T) {
	err := &RateLimitedError{RetryAfter: 90 * time.Second}
	assert.True(t, IsRateLimitedError(err))
	assert.False(t, IsRateLimitedError(assert.AnError))
	assert.Equal(t, 90, err.RetryAfterSeconds())
	ae := err.ToAppError()
	assert.Equal(t, apperror.CodeAuthRateLimited, ae.Code())
	assert.Equal(t, "Too many attempts. Try again in 2 minutes.", ae.Message())
	assert.Equal(t, map[string]string{"retry_after": "90"}, ae.Meta())

	for d, want := range map[time.Duration]string{
		0:                "Too many attempts. Try again in a minute.",
		30 * time.Second: "Too many attempts. Try again in a minute.",
		time.Minute:      "Too many attempts. Try again in a minute.",
		61 * time.Second: "Too many attempts. Try again in 2 minutes.",
		15 * time.Minute: "Too many attempts. Try again in 15 minutes.",
	} {
		assert.Equal(t, want, (&RateLimitedError{RetryAfter: d}).Message(), d)
	}
}
