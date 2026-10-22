package client

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Store is the driven port for the oauth_clients and oauth_client_secrets tables; every call runs under the org and project on ctx.
type Store interface {
	// Insert records a new client.
	Insert(ctx context.Context, c *Client) error
	// Update saves a client's settings and updated_at.
	Update(ctx context.Context, c *Client) error
	// Delete removes a client with its secrets and links, or returns *NotFoundError.
	Delete(ctx context.Context, projectID, id uuid.UUID) error
	// List returns the project's clients, oldest first, each with its live secrets at now, the current one first.
	List(ctx context.Context, projectID uuid.UUID, now time.Time) ([]*Client, error)
	// ByPublicID returns one client with its live secrets at now, the current one first, or *NotFoundError.
	ByPublicID(ctx context.Context, projectID uuid.UUID, publicID string, now time.Time) (*Client, error)
	// ForAuthentication returns one client with its live secrets at now and their hashes, for VerifySecret only, or *NotFoundError.
	ForAuthentication(ctx context.Context, projectID uuid.UUID, publicID string, now time.Time) (*Client, error)
	// Lock locks a client's row and returns it with its live secrets at now, or *NotFoundError.
	Lock(ctx context.Context, projectID uuid.UUID, publicID string, now time.Time) (*Client, error)
	// InsertSecret records secret s of client c by its hash.
	InsertSecret(ctx context.Context, c *Client, s Secret, hash string) error
	// ExpireSecret makes the secret id of client clientID stop working at at.
	ExpireSecret(ctx context.Context, projectID, clientID, id uuid.UUID, at time.Time) error
	// DeleteSecrets removes the client's secrets with the given ids and every secret that expired at or before now.
	DeleteSecrets(ctx context.Context, projectID, clientID uuid.UUID, ids []uuid.UUID, now time.Time) error
}
