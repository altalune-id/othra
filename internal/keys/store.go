package keys

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Store is the driven port for the signing_keys table; every call runs under the org and project on ctx.
type Store interface {
	// Insert records k with its sealed private key.
	Insert(ctx context.Context, k *Key, sealed []byte) error
	// Update saves k's status and timestamps.
	Update(ctx context.Context, k *Key) error
	// Live locks and returns the project's next and active keys, either nil when absent.
	Live(ctx context.Context, projectID uuid.UUID) (next, active *Key, err error)
	// List returns every key of the project, newest first.
	List(ctx context.Context, projectID uuid.UUID) ([]*Key, error)
	// Published returns the next, active and retiring keys, oldest first, then by kid.
	Published(ctx context.Context, projectID uuid.UUID) ([]*Key, error)
	// Active returns the active key and its sealed private key, or *NoActiveKeyError.
	Active(ctx context.Context, projectID uuid.UUID) (*Key, []byte, error)
	// Retiring returns the retiring keys whose retire_after is at or before now.
	Retiring(ctx context.Context, projectID uuid.UUID, now time.Time) ([]*Key, error)
	// DeleteRetired deletes the retired keys retired before cutoff, with their sealed private keys.
	DeleteRetired(ctx context.Context, projectID uuid.UUID, cutoff time.Time) (int64, error)
}
