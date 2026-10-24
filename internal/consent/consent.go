// Package consent holds the scopes an end user approved for an OAuth client and a resource server, and decides when to ask again.
package consent

import (
	"slices"
	"time"

	"github.com/google/uuid"
)

// MaxScopes bounds the scopes one consent holds.
const MaxScopes = 72

// Consent is what an end user approved for one client and one resource server, or for the client alone when ResourceServerID is nil.
type Consent struct {
	ID        uuid.UUID
	PublicID  string
	OrgID     uuid.UUID
	ProjectID uuid.UUID
	Key       Key
	Scopes    []string
	GrantedAt time.Time
	UpdatedAt time.Time
	IP        string
	UserAgent string
}

// Key names one consent: an end user, a client and a resource server, or no resource server for an OIDC-only request.
type Key struct {
	EndUserID        uuid.UUID
	ClientID         uuid.UUID
	ResourceServerID *uuid.UUID
}

// Covers reports whether c already approved every scope; a nil consent covers nothing.
func (c *Consent) Covers(scopes []string) bool {
	if c == nil {
		return false
	}
	for _, s := range scopes {
		if !slices.Contains(c.Scopes, s) {
			return false
		}
	}
	return true
}

// Split returns the requested scopes c never approved, then those it did, each in request order.
func (c *Consent) Split(requested []string) (fresh, before []string) {
	for _, s := range requested {
		if c != nil && slices.Contains(c.Scopes, s) {
			before = append(before, s)
			continue
		}
		fresh = append(fresh, s)
	}
	return fresh, before
}

// Union returns have followed by the scopes of add it lacks, without repeats.
func Union(have, add []string) []string {
	out := make([]string, 0, len(have)+len(add))
	for _, s := range slices.Concat(have, add) {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

// Page is what an authorization request shows before a code is issued.
type Page int

// The pages: none issues the code at once; full lists the scopes; remembered is the one-click "Continue to {client}" page (D2).
const (
	PageNone Page = iota
	PageFull
	PageRemembered
)

// Request is the part of an authorization request the consent decision reads.
type Request struct {
	Implicit      bool
	Confidential  bool
	PromptConsent bool
	Scopes        []string
	// Marked and Since carry the re-entry marker: the request came back from a consent page shown at Since.
	Marked bool
	Since  time.Time
}

// Decide returns the page r shows given the stored consent c (E4, D2, OIDC Core §3.1.2.4).
func Decide(c *Consent, r Request) Page {
	switch {
	case r.Marked && c.Covers(r.Scopes) && c.UpdatedAt.Truncate(time.Millisecond).After(r.Since):
		return PageNone
	case r.PromptConsent:
		return PageFull
	case r.Implicit:
		return PageNone
	case !c.Covers(r.Scopes):
		return PageFull
	case r.Confidential:
		return PageNone
	default:
		return PageRemembered
	}
}
