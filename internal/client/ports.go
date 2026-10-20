package client

import (
	"context"

	"github.com/google/uuid"
)

// Refs turns a project's public id into its id and org.
type Refs interface {
	ProjectID(ctx context.Context, publicID string) (id, orgID uuid.UUID, ok bool, err error)
}

// IssuerLock freezes a project's issuer handle; it joins the caller's unit of work.
type IssuerLock interface {
	LockIssuerHandle(ctx context.Context, projectID uuid.UUID) error
}
