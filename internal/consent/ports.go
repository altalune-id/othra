package consent

import (
	"context"

	"github.com/google/uuid"
)

// Grants is what the end user's apps hold besides their consents: live grant families, and their revocation.
type Grants interface {
	// LiveFamilies returns the end user's live grant families.
	LiveFamilies(ctx context.Context, p Project, endUserID uuid.UUID) ([]Family, error)
	// RevokeClient revokes every grant of the end user's client in the unit of work on ctx, reporting whether any was live.
	RevokeClient(ctx context.Context, p Project, endUserID, clientID uuid.UUID) (bool, error)
	// ResourceScopes returns scopes without the OpenID Connect ones, which are shown as signing the end user in.
	ResourceScopes(scopes []string) []string
}

// Directory names the clients and resource servers a connected app refers to; boot wires it over client and resourceserver.
type Directory interface {
	// ClientID returns the client whose client_id (a public id or a metadata document URL) is oauthID; ok is false when unknown.
	ClientID(ctx context.Context, p Project, oauthID string) (id uuid.UUID, ok bool, err error)
	// Clients describes the clients ids names; an unknown id is left out.
	Clients(ctx context.Context, p Project, ids []uuid.UUID) (map[uuid.UUID]ClientInfo, error)
	// Servers describes the project's resource servers.
	Servers(ctx context.Context, p Project) (map[uuid.UUID]ServerInfo, error)
}
