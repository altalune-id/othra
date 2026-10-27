// Package pat holds end users' personal tokens: opaque bearer tokens for one resource server, validated only by introspection.
package pat

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/secret"
)

// Prefix starts every personal token, so secret scanners find a leaked one and introspection routes it here.
const Prefix = "othra_pt_" //nolint:gosec // G101: a prefix, not a credential.

// The personal-token limits.
const (
	MaxLive     = 20
	MaxLifetime = 90 * 24 * time.Hour
	nameMax     = 100
)

// Scheme mints personal tokens: the prefix, 256 random bits in base62 and a CRC32 checksum.
var Scheme = secret.Scheme{Prefix: Prefix, Checksum: true} //nolint:gochecknoglobals // the personal-token format, fixed.

// Expiry is how long a new token lives.
type Expiry int

// The expiries a person picks from.
const (
	Expiry7Days  Expiry = 7
	Expiry30Days Expiry = 30
	Expiry90Days Expiry = 90
)

// Duration is the lifetime the expiry stands for.
func (e Expiry) Duration() time.Duration { return time.Duration(e) * 24 * time.Hour }

// Valid reports whether e is one of the offered expiries.
func (e Expiry) Valid() bool { return e == Expiry7Days || e == Expiry30Days || e == Expiry90Days }

// Project is the project a token belongs to; every call is scoped to it.
type Project struct {
	OrgID    uuid.UUID
	ID       uuid.UUID
	PublicID string
	Name     string
}

// State is where a token stands.
type State string

// The states of a token.
const (
	StateActive  State = "active"
	StateExpired State = "expired"
	StateRevoked State = "revoked"
)

// Token is one personal token; the secret itself is never kept.
type Token struct {
	ID               uuid.UUID
	PublicID         string
	OrgID            uuid.UUID
	ProjectID        uuid.UUID
	EndUserID        uuid.UUID
	ResourceServerID uuid.UUID
	Name             string
	Hint             string
	Scopes           []string
	CreatedAt        time.Time
	ExpiresAt        time.Time
	LastUsedAt       *time.Time
	RevokedAt        *time.Time
}

// State reports the token's state at now.
func (t *Token) State(now time.Time) State {
	switch {
	case t.RevokedAt != nil:
		return StateRevoked
	case !now.Before(t.ExpiresAt):
		return StateExpired
	}
	return StateActive
}

// Server is a resource server as personal tokens see it: the audience and the scopes a token may carry.
type Server struct {
	ID           uuid.UUID
	PublicID     string
	Name         string
	Identifier   string
	Scopes       []string
	Descriptions map[string]string
	// Accepting is true when the server takes personal tokens and has an introspection secret.
	Accepting bool
}

// NormalizeName trims name and checks it is 1 to 100 characters (the API-key rule).
func NormalizeName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	return name, name != "" && utf8.RuneCountInString(name) <= nameMax
}

// ScopesWithin reports whether scopes is non-empty and a subset of the server's catalog.
func (s *Server) ScopesWithin(scopes []string) bool {
	if len(scopes) == 0 {
		return false
	}
	for _, sc := range scopes {
		if !slices.Contains(s.Scopes, sc) {
			return false
		}
	}
	return true
}
