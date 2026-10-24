package consent

import (
	"context"

	"github.com/google/uuid"
)

// Store is the driven port for scope_consents; every call runs under the org and project on ctx.
type Store interface {
	// Find returns the consent key names, or nil.
	Find(ctx context.Context, projectID uuid.UUID, key Key) (*Consent, error)
	// Upsert inserts c, or adds its scopes to the stored consent of c.Key, first dropping the stored scopes not in keep (nil keeps
	// them all), and stamps UpdatedAt; it returns the stored consent.
	Upsert(ctx context.Context, c *Consent, keep []string) (*Consent, error)
	// DeleteClient removes every consent of clientID.
	DeleteClient(ctx context.Context, projectID, clientID uuid.UUID) error
	// Delete removes the consent key names, reporting whether there was one.
	Delete(ctx context.Context, projectID uuid.UUID, key Key) (bool, error)
}
