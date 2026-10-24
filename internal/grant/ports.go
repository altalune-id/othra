package grant

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Projects runs fn once for every project, with ctx scoped to its org and the project.
type Projects interface {
	Each(ctx context.Context, fn func(ctx context.Context, orgID, projectID uuid.UUID) error) error
}

// AccessToken is the claim set of an at+jwt access token (RFC 9068 §2.2, owner D4: no profile claims).
type AccessToken struct {
	Audience  string
	Subject   string
	ClientID  string
	SessionID string
	Scopes    []string
	AuthTime  time.Time
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// AccessTokens signs the at+jwt access token of a grant bound to a resource server.
type AccessTokens interface {
	Sign(ctx context.Context, p Project, t AccessToken) (string, error)
}

// Audiences rechecks that a resource server still admits a client with scopes, and returns its identifier, the token's aud.
type Audiences interface {
	Audience(ctx context.Context, p Project, resourceServerID uuid.UUID, c Client, scopes []string) (identifier string, ok bool, err error)
}
