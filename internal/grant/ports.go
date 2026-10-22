package grant

import (
	"context"

	"github.com/google/uuid"
)

// Projects runs fn once for every project, with ctx scoped to its org and the project.
type Projects interface {
	Each(ctx context.Context, fn func(ctx context.Context, orgID, projectID uuid.UUID) error) error
}
