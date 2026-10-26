package upstream

import (
	"context"

	"github.com/google/uuid"
)

// Store is the driven port for upstream_providers; every call runs under the org and project on ctx.
type Store interface {
	// Insert records u with its sealed secret, or returns *ExistsError when the project has the kind.
	Insert(ctx context.Context, u *Upstream, sealed []byte) error
	// Update saves u's client id, enabled flag and updated_at, and its sealed secret when sealed is not nil.
	Update(ctx context.Context, u *Upstream, sealed []byte) error
	// Delete removes the upstream id, or returns *HasIdentitiesError while end users have linked it.
	Delete(ctx context.Context, projectID, id uuid.UUID) error
	// List returns the project's upstreams by kind.
	List(ctx context.Context, projectID uuid.UUID) ([]*Upstream, error)
	// ByPublicID returns the project's upstream publicID, or *NotFoundError.
	ByPublicID(ctx context.Context, projectID uuid.UUID, publicID string) (*Upstream, error)
	// Lock is ByPublicID that also locks the row until the unit of work ends.
	Lock(ctx context.Context, projectID uuid.UUID, publicID string) (*Upstream, error)
	// ByKind returns the project's upstream of kind, or *NotFoundError.
	ByKind(ctx context.Context, projectID uuid.UUID, kind Kind) (*Upstream, error)
	// Secret returns the sealed client secret of the upstream id.
	Secret(ctx context.Context, projectID, id uuid.UUID) ([]byte, error)
}
