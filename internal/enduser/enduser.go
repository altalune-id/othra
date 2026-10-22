// Package enduser holds a project's end users: magic-link sign-in, browser and user sessions, and terms consent.
package enduser

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// MethodMagicLink is how a user session signed in.
const MethodMagicLink = "magic_link"

const (
	publicIDLen  = 21
	userAgentMax = 512
)

// Config holds the end-user sign-in limits.
type Config struct {
	LinkTTL     time.Duration
	ResendAfter time.Duration
	Idle        time.Duration
	Absolute    time.Duration
	MaxAccounts int
}

// Project is the project a hosted request runs for, as resolved from its issuer handle.
type Project struct {
	OrgID        uuid.UUID
	ID           uuid.UUID
	PublicID     string
	IssuerURL    string
	EmailLink    bool
	TermsVersion int
	TermsURL     string
	PrivacyURL   string
}

// TermsRequired reports whether the project asks end users to accept its terms.
func (p Project) TermsRequired() bool { return p.TermsVersion > 0 && p.TermsURL != "" }

// LinkRequest asks for a sign-in link for Email that continues to ReturnTo.
type LinkRequest struct {
	Email    string
	ReturnTo string
	IP       string
	// Allow runs only after the cooldown passes, right before issuing: false issues nothing, an error is returned as is.
	Allow func(ctx context.Context) (bool, error)
}

// Link is a magic link as stored; the token itself is never kept.
type Link struct {
	ID               uuid.UUID
	PublicID         string
	Email            string
	ReturnTo         string
	CreatedAt        time.Time
	ExpiresAt        time.Time
	UsedAt           *time.Time
	BrowserSessionID *uuid.UUID
}

// Usable reports whether l can still be redeemed at now.
func (l *Link) Usable(now time.Time) bool { return l.UsedAt == nil && now.Before(l.ExpiresAt) }

// IssuedLink is a fresh link; URL holds the token and is shown or mailed once. A resend inside the cooldown issues nothing.
type IssuedLink struct {
	URL       string
	Email     string
	ExpiresAt time.Time
	Issued    bool
	// SignUp is true when no end user has the email yet; only the email's wording uses it, never the page's answer.
	SignUp bool
}

// RedeemRequest redeems the link whose raw token is Token on the browser whose cookie secret is BrowserSecret, empty for a new browser.
type RedeemRequest struct {
	Token         string
	BrowserSecret string
	UserAgent     string
	IP            string
}

// Redeemed is a successful redeem: Secret is the browser's cookie secret to set, rotated unless the redeem was a repeat.
type Redeemed struct {
	Secret    string
	ExpiresAt time.Time
	Account   Account
	ReturnTo  string
}

// Account is one end user signed in on a browser.
type Account struct {
	UserSessionID uuid.UUID
	PublicID      string
	EndUserID     uuid.UUID
	Subject       string
	Email         string
	Name          string
	AuthTime      time.Time
	SelectedAt    time.Time
}

// EndUser is a project's end user as an ID token and userinfo describe it; Subject is the sub claim.
type EndUser struct {
	ID            uuid.UUID
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
}

// Session is a browser's live accounts, the most recently selected first; Active is nil when none is left.
type Session struct {
	BrowserSessionID uuid.UUID
	ExpiresAt        time.Time
	Accounts         []Account
}

// Active returns the account the browser last selected.
func (s *Session) Active() *Account {
	if s == nil || len(s.Accounts) == 0 {
		return nil
	}
	return &s.Accounts[0]
}

// Account returns the live account whose user session has publicID.
func (s *Session) Account(publicID string) (*Account, bool) {
	if s == nil {
		return nil, false
	}
	for i := range s.Accounts {
		if s.Accounts[i].PublicID == publicID {
			return &s.Accounts[i], true
		}
	}
	return nil, false
}

// TermsAcceptance is one append-only consent row.
type TermsAcceptance struct {
	EndUserID     uuid.UUID
	Version       int
	TermsURL      string
	PrivacyURL    string
	AcceptedAt    time.Time
	UserSessionID uuid.UUID
	// UserSessionPublicID keeps naming the session after prune clears UserSessionID, so the evidence survives.
	UserSessionPublicID string
	IP                  string
	UserAgent           string
}

// Browser is a browser session as stored.
type Browser struct {
	ID          uuid.UUID
	CreatedAt   time.Time
	LastSeenAt  time.Time
	AbsoluteEnd time.Time
}

// Live reports whether b is inside its idle and absolute windows at now.
func (b *Browser) Live(now time.Time, idle time.Duration) bool {
	return now.Before(b.AbsoluteEnd) && now.Before(b.LastSeenAt.Add(idle))
}

// ExpiresAt is when b ends unless it is used again: the earlier of its idle and absolute ends.
func (b *Browser) ExpiresAt(idle time.Duration) time.Time {
	return minTime(b.LastSeenAt.Add(idle), b.AbsoluteEnd)
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && s[n]&0xC0 == 0x80 {
		n--
	}
	return s[:n]
}
