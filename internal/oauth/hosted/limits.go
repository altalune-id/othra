package hosted

import (
	"time"

	"altalune.id/othra/internal/kernel/ratelimit"
)

// IssueIPRules limit magic-link requests per client IP and project (m4-spec RL1): 10 a minute and 50 an hour.
func IssueIPRules() []ratelimit.Rule {
	return []ratelimit.Rule{
		{Name: "hosted-issue-ip-minute", Rate: 10, Per: time.Minute, Burst: 10},
		{Name: "hosted-issue-ip-hour", Rate: 50, Per: time.Hour, Burst: 50},
	}
}

// IssueEmailRules limit magic links to one email per project (m4-spec RL2), refused silently: 3 in 15 minutes and 10 a day.
func IssueEmailRules() []ratelimit.Rule {
	return []ratelimit.Rule{
		{Name: "hosted-issue-email-15m", Rate: 3, Per: 15 * time.Minute, Burst: 3},
		{Name: "hosted-issue-email-day", Rate: 10, Per: 24 * time.Hour, Burst: 10},
	}
}

// RedeemIPRule limits magic-link redeems per client IP and project (m4-spec RL3): 30 a minute.
func RedeemIPRule() ratelimit.Rule {
	return ratelimit.Rule{Name: "hosted-redeem-ip", Rate: 30, Per: time.Minute, Burst: 30}
}

// FormIPRule limits the picker, sign-out and terms forms per client IP and project: 60 an hour.
func FormIPRule() ratelimit.Rule {
	return ratelimit.Rule{Name: "hosted-form-ip", Rate: 60, Per: time.Hour, Burst: 60}
}

// LinkSessionRule limits starting to link an upstream per signed-in account (user session) and project: 10 an hour.
func LinkSessionRule() ratelimit.Rule {
	return ratelimit.Rule{Name: "hosted-link-session", Rate: 10, Per: time.Hour, Burst: 10}
}

// AuthorizeIPRule limits authorization requests per client IP and project: 60 a minute.
func AuthorizeIPRule() ratelimit.Rule {
	return ratelimit.Rule{Name: "hosted-authorize-ip", Rate: 60, Per: time.Minute, Burst: 60}
}
