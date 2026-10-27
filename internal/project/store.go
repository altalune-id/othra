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
	RefByPublicID(ctx context.Context, publicID string) (*Ref, error)
	BySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Project, error)
	List(ctx context.Context, orgID uuid.UUID) ([]*Project, error)
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
	// LockSignIn locks the project id's row until the unit of work ends and returns its email-link switch.
	LockSignIn(ctx context.Context, id uuid.UUID) (emailLink bool, err error)
	// SetLogoFile points the project id's logo at fileID, or at none when nil, and returns the file it pointed at before.
	SetLogoFile(ctx context.Context, id uuid.UUID, fileID *uuid.UUID) (previous *uuid.UUID, err error)
}
