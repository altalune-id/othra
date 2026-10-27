package resourceserver

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/altalune-id/othra/internal/kernel/secret"
)

// Store is the driven port for the resource_servers and oauth_client_resources tables; every call runs under the org and project on ctx.
type Store interface {
	// Insert records r, or returns *IdentifierTakenError.
	Insert(ctx context.Context, r *ResourceServer) error
	// Update saves r's name, scopes and their descriptions, verification, trust and admission.
	Update(ctx context.Context, r *ResourceServer) error
	// Delete removes the server id.
	Delete(ctx context.Context, projectID, id uuid.UUID) error
	// List returns the project's servers, by name.
	List(ctx context.Context, projectID uuid.UUID) ([]*ResourceServer, error)
	// ByIdentifier returns the project's server whose identifier is exactly identifier, or *NotFoundError.
	ByIdentifier(ctx context.Context, projectID uuid.UUID, identifier string) (*ResourceServer, error)
	// ByID returns the project's server id, or *NotFoundError.
	ByID(ctx context.Context, projectID, id uuid.UUID) (*ResourceServer, error)
	// ByPublicID returns the project's server publicID, or *NotFoundError.
	ByPublicID(ctx context.Context, projectID uuid.UUID, publicID string) (*ResourceServer, error)
	// Lock is ByPublicID that also locks the row until the unit of work ends.
	Lock(ctx context.Context, projectID uuid.UUID, publicID string) (*ResourceServer, error)
	// Due returns up to limit untrusted servers never checked or last checked before cutoff, oldest first.
	Due(ctx context.Context, projectID uuid.UUID, cutoff time.Time, limit int) ([]*ResourceServer, error)
	// Links returns the project's links of clientID or of serverID (whichever is not uuid.Nil), with their servers.
	Links(ctx context.Context, projectID, clientID, serverID uuid.UUID) ([]*Link, error)
	// UpsertLink records l, replacing the scopes of an existing link; an unknown client returns *ClientNotFoundError.
	UpsertLink(ctx context.Context, l *Link) error
	// UpdateLink replaces the scopes of l and reports whether the link exists.
	UpdateLink(ctx context.Context, l *Link) (bool, error)
	// DeleteLink removes the link between clientID and serverID, if any.
	DeleteLink(ctx context.Context, projectID, clientID, serverID uuid.UUID) error
	// IntrospectionSecrets returns the servers' live introspection secrets at now, oldest first; withHash only to authenticate.
	IntrospectionSecrets(
		ctx context.Context, projectID uuid.UUID, ids []uuid.UUID, now time.Time, withHash bool,
	) (map[uuid.UUID][]secret.Stored, error)
	// InsertIntrospectionSecret records secret s of server r by its Versioned hash.
	InsertIntrospectionSecret(ctx context.Context, r *ResourceServer, s secret.Stored, hash string) error
	// ExpireIntrospectionSecret makes the secret id of server serverID stop working at at.
	ExpireIntrospectionSecret(ctx context.Context, projectID, serverID, id uuid.UUID, at time.Time) error
	// DeleteIntrospectionSecrets removes the server's secrets ids and every one that expired at or before now.
	DeleteIntrospectionSecrets(ctx context.Context, projectID, serverID uuid.UUID, ids []uuid.UUID, now time.Time) error
}
