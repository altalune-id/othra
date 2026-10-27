package pat

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Store is the driven port for personal_tokens; every call runs under the org and project on ctx.
type Store interface {
	// Insert records t with the versioned hash of its secret.
	Insert(ctx context.Context, t *Token, hash string) error
	// ByHash returns the project's token with that hash, in any state, or nil.
	ByHash(ctx context.Context, projectID uuid.UUID, hash string) (*Token, error)
	// ListByEndUser returns the end user's tokens, newest first, including expired and revoked ones not yet pruned.
	ListByEndUser(ctx context.Context, projectID, endUserID uuid.UUID) ([]*Token, error)
	// LockEndUser locks the end user's row until the unit of work ends, so the live-token cap holds under concurrency; ok is false when gone.
	LockEndUser(ctx context.Context, projectID, endUserID uuid.UUID) (ok bool, err error)
	// CountLive counts the end user's tokens neither revoked nor expired at now.
	CountLive(ctx context.Context, projectID, endUserID uuid.UUID, now time.Time) (int, error)
	// Revoke sets revoked_at on the end user's token publicID; ok is false when there is no live one.
	Revoke(ctx context.Context, projectID, endUserID uuid.UUID, publicID string, now time.Time) (ok bool, err error)
	// TouchLastUsed moves each token's last_used_at forward to its time; it never moves it back.
	TouchLastUsed(ctx context.Context, used map[uuid.UUID]time.Time) error
	// Prune deletes the project's tokens expired before cutoff or revoked before cutoff.
	Prune(ctx context.Context, projectID uuid.UUID, cutoff time.Time) (int, error)
}
