package secret

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestMatchesAny(t *testing.T) {
	now := time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)
	past, future := now.Add(-time.Second), now.Add(time.Hour)
	stored := []Stored{
		Stored{ID: uuid.New()}.WithHash(Versioned("current")),
		Stored{ID: uuid.New(), ExpiresAt: &future}.WithHash(Versioned("grace")),
		Stored{ID: uuid.New(), ExpiresAt: &past}.WithHash(Versioned("expired")),
		{ID: uuid.New()},
	}
	assert.True(t, MatchesAny(stored, "current", now))
	assert.True(t, MatchesAny(stored, "grace", now), "an old secret works through its grace")
	assert.False(t, MatchesAny(stored, "expired", now))
	assert.False(t, MatchesAny(stored, "", now))
	assert.False(t, MatchesAny(stored, "other", now))
	assert.True(t, stored[0].Current())
	assert.False(t, stored[1].Current())
	assert.True(t, stored[1].Live(now))
	assert.False(t, stored[2].Live(now))
}

func TestRotate(t *testing.T) {
	now := time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)
	later := now.Add(time.Hour)
	cur, old := Stored{ID: uuid.New()}, Stored{ID: uuid.New(), ExpiresAt: &later}
	r := Rotate([]Stored{cur, old}, GraceDay, now)
	assert.Equal(t, []uuid.UUID{cur.ID}, r.Expire, "the current secret keeps working for the grace")
	assert.Equal(t, now.Add(GraceDay), r.At)
	assert.Equal(t, []uuid.UUID{old.ID}, r.Drop, "an older one in its grace ends at once")

	r = Rotate([]Stored{cur}, GraceNow, now)
	assert.Empty(t, r.Expire)
	assert.Equal(t, []uuid.UUID{cur.ID}, r.Drop, "no grace: the current one ends now")

	r = Rotate([]Stored{cur}, 30*24*time.Hour, now)
	assert.Equal(t, now.Add(MaxGrace), r.At, "the grace is capped")
	r = Rotate([]Stored{cur}, -time.Hour, now)
	assert.Equal(t, []uuid.UUID{cur.ID}, r.Drop)
}
