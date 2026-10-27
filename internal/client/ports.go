package client

import (
	"context"

	"github.com/google/uuid"
)

// Refs turns a project's public id, in one read, into its ids and the console page of each of its clients.
type Refs interface {
	ClientProject(ctx context.Context, publicID string) (ProjectRef, bool, error)
}

// ProjectRef is a project's ids and the builder of its clients' console page URLs.
type ProjectRef struct {
	ID, OrgID  uuid.UUID
	ConsoleURL func(clientPublicID string) string
}

// IssuerLock freezes a project's issuer handle; it joins the caller's unit of work.
type IssuerLock interface {
	LockIssuerHandle(ctx context.Context, projectID uuid.UUID) error
	// IsSystem reports whether the project is the console project, which takes no client but its seeded one.
	IsSystem(ctx context.Context, projectID uuid.UUID) (bool, error)
}

// Projects runs fn once for every project, with ctx scoped to its org and the project.
type Projects interface {
	Each(ctx context.Context, fn func(ctx context.Context, orgID, projectID uuid.UUID) error) error
}

// MetadataFetcher GETs a client metadata document over https, follows no redirects and treats any status but 200 as an error.
type MetadataFetcher interface {
	FetchMetadata(ctx context.Context, url string) (*FetchedMetadata, error)
}

// FetchedMetadata is a fetched document body, at most MaxMetadataBytes+1 bytes, with the response headers Othra reads.
type FetchedMetadata struct {
	Body         []byte
	ContentType  string
	CacheControl string
}

// Links links an admitted metadata document client to the resource server that admitted it; it joins the caller's unit of work.
type Links interface {
	LinkMetadataClient(ctx context.Context, orgID, projectID, clientID, resourceServerID uuid.UUID, scopes []string) error
}
