package layout

import (
	"errors"
	"time"
)

// RateLimitedError signals too many attempts; it says nothing about whether the account exists.
type RateLimitedError struct{ RetryAfter time.Duration }

func (*RateLimitedError) Error() string { return "auth: rate limited" }

// IsRateLimitedError reports whether err wraps a *RateLimitedError.
func IsRateLimitedError(err error) bool {
	_, ok := errors.AsType[*RateLimitedError](err)
	return ok
}
