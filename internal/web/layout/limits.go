package layout

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/kernel/ratelimit"
)

const limiterDownRetry = 5 * time.Second

// LimiterDown is the text a form shows when the rate limiter itself fails.
const LimiterDown = "Sign-in is temporarily unavailable. Try again in a moment."

// Limits rate limits the pre-sign-in forms by client IP and email; the zero value limits nothing.
type Limits struct {
	Limiter          ratelimit.Limiter
	TrustedProxyHops int
}

// LimitIP counts r against rule by its client IP: a *RateLimitedError when over, or the limiter's own error.
func (l Limits) LimitIP(r *http.Request, rule ratelimit.Rule) error {
	return l.Allow(r.Context(), ratelimit.Key(rule, ratelimit.IPSubject(ClientIP(r, l.TrustedProxyHops))), rule)
}

// LimitEmail counts one attempt for email against rule; the key holds a hash, not the email.
func (l Limits) LimitEmail(ctx context.Context, email string, rule ratelimit.Rule) error {
	return l.Allow(ctx, emailKey(email, rule), rule)
}

// ForgetEmail clears email's count under rule.
func (l Limits) ForgetEmail(ctx context.Context, email string, rule ratelimit.Rule) error {
	return l.limiter().Reset(ctx, emailKey(email, rule))
}

// Allow counts one attempt for key against rule: a *RateLimitedError when over, or the limiter's own error.
func (l Limits) Allow(ctx context.Context, key string, rule ratelimit.Rule) error {
	res, err := l.limiter().Allow(ctx, key, rule)
	if err != nil {
		return err
	}
	if !res.Allowed {
		return &RateLimitedError{RetryAfter: res.RetryAfter}
	}
	return nil
}

func (l Limits) limiter() ratelimit.Limiter {
	if l.Limiter == nil {
		return ratelimit.Disabled()
	}
	return l.Limiter
}

func emailKey(email string, rule ratelimit.Rule) string {
	return ratelimit.Key(rule, ratelimit.EmailSubject(email))
}

// RateLimitedError signals too many attempts; it says nothing about whether the account exists.
type RateLimitedError struct{ RetryAfter time.Duration }

func (*RateLimitedError) Error() string { return "auth: rate limited" }

// IsRateLimitedError reports whether err wraps a *RateLimitedError.
func IsRateLimitedError(err error) bool {
	_, ok := errors.AsType[*RateLimitedError](err)
	return ok
}

// RetryAfterSeconds is RetryAfter rounded up to whole seconds, for a Retry-After header.
func (e *RateLimitedError) RetryAfterSeconds() int {
	return int((e.RetryAfter + time.Second - 1) / time.Second)
}

// Message is the generic text shown to the person, in whole minutes.
func (e *RateLimitedError) Message() string {
	minutes := int((e.RetryAfter + time.Minute - 1) / time.Minute)
	if minutes <= 1 {
		return "Too many attempts. Try again in a minute."
	}
	return "Too many attempts. Try again in " + strconv.Itoa(minutes) + " minutes."
}

// ToAppError maps RateLimitedError to AUT003 with the wait in seconds.
func (e *RateLimitedError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeAuthRateLimited, e.Message()).
		WithMeta(map[string]string{"retry_after": strconv.Itoa(e.RetryAfterSeconds())}).
		WithCause(e)
}

// WriteLimited answers a refused form: 429 for a *RateLimitedError, else 503. SECURITY: fails closed and never shows the error.
func WriteLimited(w http.ResponseWriter, r *http.Request, kit Kit, title string, err error) {
	w.Header().Set("Cache-Control", "no-store")
	if rl, ok := errors.AsType[*RateLimitedError](err); ok {
		w.Header().Set("Retry-After", strconv.Itoa(rl.RetryAfterSeconds()))
		kit.Render(w, r, http.StatusTooManyRequests, Error(kit.Layout(title), rl.Message()))
		return
	}
	LimiterFailed(w, r, kit, err)
	kit.Render(w, r, http.StatusServiceUnavailable, Error(kit.Layout(title), LimiterDown))
}

// LimiterFailed reports a broken rate limiter and sets the Retry-After for the 503 the caller writes.
func LimiterFailed(w http.ResponseWriter, r *http.Request, kit Kit, err error) {
	w.Header().Set("Retry-After", strconv.Itoa(int(limiterDownRetry/time.Second)))
	if kit.Unexpected != nil {
		_ = kit.Unexpected(r.Context(), "web: rate limit", err)
	}
}
