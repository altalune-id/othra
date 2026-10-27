package auth

import (
	"time"

	"github.com/altalune-id/othra/internal/kernel/ratelimit"
)

// LoginEmailRule counts failed sign-ins per email: 10 per 15 minutes, 5 at once.
func LoginEmailRule() ratelimit.Rule {
	return ratelimit.Rule{Name: "login-email", Rate: 10, Per: 15 * time.Minute, Burst: 5}
}

// LoginIPRule counts every sign-in per client IP: 60 per 15 minutes.
func LoginIPRule() ratelimit.Rule {
	return ratelimit.Rule{Name: "login-ip", Rate: 60, Per: 15 * time.Minute, Burst: 60}
}

// ResetEmailRule limits reset emails per address: 3 an hour.
func ResetEmailRule() ratelimit.Rule {
	return ratelimit.Rule{Name: "reset-email", Rate: 3, Per: time.Hour, Burst: 3}
}

// ResetIPRule limits reset requests per client IP: 20 an hour.
func ResetIPRule() ratelimit.Rule {
	return ratelimit.Rule{Name: "reset-ip", Rate: 20, Per: time.Hour, Burst: 20}
}

// ResetLinkIPRule limits opening reset links and setting the new password per client IP: 20 an hour.
func ResetLinkIPRule() ratelimit.Rule {
	return ratelimit.Rule{Name: "reset-link-ip", Rate: 20, Per: time.Hour, Burst: 20}
}
