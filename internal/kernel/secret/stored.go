package secret

import (
	"time"

	"github.com/google/uuid"
)

// The graces a rotation may give the secret it replaces; MaxGrace is the longest.
const (
	GraceNow  time.Duration = 0
	GraceHour               = time.Hour
	GraceDay                = 24 * time.Hour
	MaxGrace                = 7 * 24 * time.Hour
)

// Stored is one secret a holder keeps: the current one while ExpiresAt is nil, an old one working until ExpiresAt.
type Stored struct {
	ID        uuid.UUID
	PublicID  string
	Hint      string
	CreatedAt time.Time
	ExpiresAt *time.Time
	hash      string
}

// WithHash returns s carrying its stored Versioned hash, for authentication reads only.
func (s Stored) WithHash(hash string) Stored {
	s.hash = hash
	return s
}

// Current reports whether s is the current secret rather than an old one in its grace.
func (s Stored) Current() bool { return s.ExpiresAt == nil }

// Live reports whether s still works at now.
func (s Stored) Live(now time.Time) bool { return s.ExpiresAt == nil || now.Before(*s.ExpiresAt) }

// MatchesAny reports whether presented matches a secret that is live at now; every live secret is compared, in constant time.
func MatchesAny(stored []Stored, presented string, now time.Time) bool {
	if presented == "" {
		return false
	}
	ok := false
	for _, s := range stored {
		if s.hash == "" || !s.Live(now) {
			continue
		}
		if Matches(s.hash, presented) {
			ok = true
		}
	}
	return ok
}

// Rotation is what rotating in a new secret does to the old ones: Expire stop working at At, Drop stop now.
type Rotation struct {
	Expire []uuid.UUID
	At     time.Time
	Drop   []uuid.UUID
}

// Rotate plans a rotation at now: the current secret works for grace (clamped to [0, MaxGrace]); any other goes at once.
func Rotate(old []Stored, grace time.Duration, now time.Time) Rotation {
	grace = min(max(grace, 0), MaxGrace)
	r := Rotation{At: now.Add(grace)}
	for _, s := range old {
		if s.Current() && grace > 0 {
			r.Expire = append(r.Expire, s.ID)
			continue
		}
		r.Drop = append(r.Drop, s.ID)
	}
	return r
}
