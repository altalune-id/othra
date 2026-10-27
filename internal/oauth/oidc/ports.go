package oidc

import (
	"context"
	"time"

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

// CIMDAdmissions says whether a project has a verified resource server that admits CIMD clients.
type CIMDAdmissions interface {
	AcceptsCIMD(ctx context.Context, orgID, projectID uuid.UUID) (bool, error)
}

// Introspector is a resource server authenticated at the introspection endpoint: the only audience it may learn about.
type Introspector struct {
	ID         uuid.UUID
	PublicID   string
	Identifier string
}

// IntrospectionCallers authenticates resource servers by their introspection secret (RFC 7662 §2.1); boot plugs in resourceserver.Service.
type IntrospectionCallers interface {
	// Authenticate returns the resource server publicID when secret is one of its live introspection secrets; ok is false otherwise.
	Authenticate(ctx context.Context, orgID, projectID uuid.UUID, publicID, secret string) (rs Introspector, ok bool, err error)
}

// IntrospectedToken is a token that is active for the calling resource server; ClientID is empty for a personal token.
type IntrospectedToken struct {
	Scopes    []string
	ClientID  string
	Subject   string
	Audience  string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// PersonalTokens answers introspection for othra_pt_ tokens; boot plugs in pat.Service.
type PersonalTokens interface {
	// Introspect returns the token when it is active and bound to the resource server callerID; ok is false for every other case.
	Introspect(ctx context.Context, orgID, projectID, callerID uuid.UUID, token string) (t IntrospectedToken, ok bool, err error)
}

// ClientIDs reads the client_id a client presents in OAuth: its public id, or its metadata URL for a CIMD client.
type ClientIDs interface {
	// OAuthID returns the client's OAuth client_id, or "" when the client is gone.
	OAuthID(ctx context.Context, orgID, projectID, id uuid.UUID) (string, error)
}
