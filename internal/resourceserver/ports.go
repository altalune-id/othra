package resourceserver

import (
	"context"

	"github.com/google/uuid"
)

// Refs turns a project's public id into its id and org.
type Refs interface {
	ProjectID(ctx context.Context, publicID string) (id, orgID uuid.UUID, ok bool, err error)
}

// ProjectRef is a project's public id and its issuer, the value its resource servers list in authorization_servers.
type ProjectRef struct {
	PublicID string
	Issuer   string
}

// Issuers gives a project's public id and issuer and locks its issuer handle; both run under the tenant scope on ctx.
type Issuers interface {
	// Project returns the project's public id and issuer.
	Project(ctx context.Context, projectID uuid.UUID) (ProjectRef, error)
	// LockIssuerHandle freezes the project's issuer handle; it joins the caller's unit of work.
	LockIssuerHandle(ctx context.Context, projectID uuid.UUID) error
}

// Clients reads the project's OAuth clients that links name.
type Clients interface {
	// Lookup returns the project's client with publicID; ok is false when there is none.
	Lookup(ctx context.Context, orgID, projectID uuid.UUID, publicID string) (ref ClientRef, ok bool, err error)
	// Refs returns the project's clients by id; an id with no client is left out.
	Refs(ctx context.Context, orgID, projectID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]ClientRef, error)
}

// People turns a user's id into the public id a log line names.
type People interface {
	UserPublicID(ctx context.Context, userID uuid.UUID) (string, error)
}

// Checker runs one RFC 9728 check of identifier against issuer and returns nil when it passes.
type Checker interface {
	Check(ctx context.Context, identifier, issuer string) *Failure
}

// Projects runs fn once for every project, with ctx scoped to its org and the project.
type Projects interface {
	Each(ctx context.Context, fn func(ctx context.Context, orgID, projectID uuid.UUID) error) error
}
