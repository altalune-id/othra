package keys

import (
	"context"

	"github.com/google/uuid"
)

// Refs turns a project's public id into its id and org.
type Refs interface {
	ProjectID(ctx context.Context, publicID string) (id, orgID uuid.UUID, ok bool, err error)
}

// Projects runs fn once for every project, with ctx scoped to its org and the project.
type Projects interface {
	Each(ctx context.Context, fn func(ctx context.Context, orgID, projectID uuid.UUID) error) error
}
