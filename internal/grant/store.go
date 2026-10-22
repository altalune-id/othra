package grant

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// NewCode is a code to store, named by the public ids of its client, user session and end user.
type NewCode struct {
	ID            uuid.UUID
	Hash          string
	Client        string
	UserSession   string
	EndUser       string
	RedirectURI   string
	Scopes        []string
	CodeChallenge string
	Nonce         string
	AuthTime      time.Time
	CreatedAt     time.Time
	ExpiresAt     time.Time
}

// Cutoffs are what Prune keeps: codes and tokens until Now, used codes after UsedCodes, families revoked after Revoked and used after Idle.
type Cutoffs struct {
	Now       time.Time
	UsedCodes time.Time
	Revoked   time.Time
	Idle      time.Time
}

// Store is the driven port for the grant tables; every call runs under the org and project on ctx.
type Store interface {
	// InsertCode stores c when its client exists and its user session is live and belongs to its end user, else *SessionGoneError.
	InsertCode(ctx context.Context, projectID uuid.UUID, c NewCode) error
	// CodeByHash locks and returns the code with hash, or nil.
	CodeByHash(ctx context.Context, projectID uuid.UUID, hash string) (*Code, error)
	// UseCode marks the code used at now by the family it was redeemed into.
	UseCode(ctx context.Context, projectID, codeID, familyID uuid.UUID, now time.Time) error

	// InsertFamily stores f.
	InsertFamily(ctx context.Context, orgID, projectID uuid.UUID, f *Family) error
	// FamilyBySID returns the family with the public id sid, or nil.
	FamilyBySID(ctx context.Context, projectID uuid.UUID, sid string) (*Family, error)
	// TouchFamily records a use of the family at now.
	TouchFamily(ctx context.Context, projectID, familyID uuid.UUID, now time.Time) error
	// RevokeFamily revokes the family for reason at now, unless it is already revoked.
	RevokeFamily(ctx context.Context, projectID, familyID uuid.UUID, reason string, now time.Time) error
	// RevokeUserSession revokes the live families of the user session userSession without offline_access, and drops its unused codes.
	RevokeUserSession(ctx context.Context, projectID uuid.UUID, userSession, reason string, now time.Time) error

	// InsertAccessToken stores an access token of the family by its hash.
	InsertAccessToken(ctx context.Context, orgID, projectID, familyID uuid.UUID, hash string, scopes []string, now, expires time.Time) error
	// AccessByHash returns the access token with hash and its family, or nil.
	AccessByHash(ctx context.Context, projectID uuid.UUID, hash string) (*Access, error)

	// InsertRefreshToken stores a refresh token of the family by its hash.
	InsertRefreshToken(ctx context.Context, orgID, projectID, familyID uuid.UUID, hash string, now, expires time.Time) error
	// RefreshByHash locks and returns the refresh token with hash and its family, or nil.
	RefreshByHash(ctx context.Context, projectID uuid.UUID, hash string) (*RefreshToken, *Family, error)
	// UseRefreshToken marks the refresh token used at now.
	UseRefreshToken(ctx context.Context, projectID, tokenID uuid.UUID, now time.Time) error

	// Prune deletes what is past the cutoffs and returns how many rows went.
	Prune(ctx context.Context, projectID uuid.UUID, c Cutoffs) (int, error)
}
