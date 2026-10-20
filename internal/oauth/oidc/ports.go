package oidc

import (
	"context"

	"github.com/google/uuid"

	"altalune.id/othra/internal/project"
)

// Issuers resolves an issuer handle to its project before any tenant scope exists.
type Issuers interface {
	ResolveIssuer(ctx context.Context, handle string) (*project.Issuer, error)
}

// Publisher gives the public signing keys of a project.
type Publisher interface {
	// JWKS returns the project's JSON Web Key Set, read under the tenant scope on ctx; it never holds private key material.
	JWKS(ctx context.Context, projectID uuid.UUID) ([]byte, error)
}
