package pat

import (
	"context"

	"github.com/google/uuid"
)

// Servers reads the project's resource servers as audiences of personal tokens; boot plugs in resourceserver.Service.
type Servers interface {
	// Accepting returns the servers that take personal tokens now: switched on, with an introspection secret.
	Accepting(ctx context.Context, p Project) ([]Server, error)
	// ByPublicID returns the server publicID in any state; ok is false when it is unknown.
	ByPublicID(ctx context.Context, p Project, publicID string) (s Server, ok bool, err error)
	// ByID returns the server id in any state; ok is false when it is gone.
	ByID(ctx context.Context, p Project, id uuid.UUID) (s Server, ok bool, err error)
}

// Subjects turns an end user into the sub claim every Othra token carries: the end user's public id.
type Subjects interface {
	// Subject returns the end user's sub, or "" when the end user is gone.
	Subject(ctx context.Context, p Project, endUserID uuid.UUID) (string, error)
}

// Projects runs fn once for every project, with ctx scoped to its org and the project.
type Projects interface {
	Each(ctx context.Context, fn func(ctx context.Context, orgID, projectID uuid.UUID) error) error
}
