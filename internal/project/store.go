package project

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Store persists Project aggregates.
type Store interface {
	Reader
	Writer
}

// Reader reads Project aggregates; implementations may use a read replica.
type Reader interface {
	ByID(ctx context.Context, id uuid.UUID) (*Project, error)
	ByPublicID(ctx context.Context, publicID string) (*Project, error)
	BySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Project, error)
	List(ctx context.Context, orgID uuid.UUID) ([]*Project, error)
	// Logo returns the project's logo, or nil when it has none.
	Logo(ctx context.Context, projectID uuid.UUID) (*Logo, error)
	// IssuerHandleExists reports whether any org's project uses handle.
	IssuerHandleExists(ctx context.Context, handle string) (bool, error)
	// ResolveIssuer finds the project behind handle before any tenant scope exists.
	ResolveIssuer(ctx context.Context, handle string) (*Issuer, error)
}

// Writer writes Project aggregates.
type Writer interface {
	Save(ctx context.Context, p *Project) error
	// SetIssuerHandle changes the handle of the project id while it is unlocked.
	SetIssuerHandle(ctx context.Context, id uuid.UUID, handle string) error
	// LockIssuerHandle freezes the handle of the project id at at; a locked handle keeps its first lock time.
	LockIssuerHandle(ctx context.Context, id uuid.UUID, at time.Time) error
	SaveLogo(ctx context.Context, l *Logo) error
	DeleteLogo(ctx context.Context, projectID uuid.UUID) error
}
