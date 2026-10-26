package upstream

import (
	"context"

	"github.com/google/uuid"
)

// SignInGuard reads a project's email-link switch and locks the project row until the caller's unit of work ends.
type SignInGuard interface {
	EmailLinkOn(ctx context.Context, projectID uuid.UUID) (bool, error)
}

// Refs turns a project's public id into its id and org.
type Refs interface {
	ProjectID(ctx context.Context, publicID string) (id, orgID uuid.UUID, ok bool, err error)
}

// Issuers gives a project's issuer URL, which the redirect URI an admin registers at the upstream starts with (P11).
type Issuers interface {
	IssuerURL(ctx context.Context, projectID uuid.UUID) (string, error)
}
