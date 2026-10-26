package federation

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Store is the driven port for end_user_identities; every call runs under the org and project on ctx.
type Store interface {
	// BySubject returns the identity (issuer, subject), or nil.
	BySubject(ctx context.Context, projectID uuid.UUID, issuer, subject string) (*storedIdentity, error)
	// Insert links id to endUserID, or returns errSubjectTaken or errUpstreamTaken.
	Insert(ctx context.Context, orgID, projectID, endUserID uuid.UUID, id Identity, publicID string, now time.Time) error
	// Touch records a sign-in by the identity and what the upstream says about its email now.
	Touch(ctx context.Context, projectID, id uuid.UUID, email string, emailVerified bool, now time.Time) error
	// ListByEndUser returns the end user's identities with their upstream kind, oldest first.
	ListByEndUser(ctx context.Context, projectID, endUserID uuid.UUID) ([]Identity, error)
	// LockEndUser locks the end user's row for the unit of work; ok is false when it is gone.
	LockEndUser(ctx context.Context, projectID, endUserID uuid.UUID) (ok bool, err error)
	// Delete removes the end user's identity publicID; ok is false when there is none.
	Delete(ctx context.Context, projectID, endUserID uuid.UUID, publicID string) (ok bool, err error)
	// EveryEndUserLinked reports whether every end user of the project has an identity.
	EveryEndUserLinked(ctx context.Context, projectID uuid.UUID) (bool, error)
}

var (
	errSubjectTaken  = errors.New("federation: the (issuer, subject) is linked")
	errUpstreamTaken = errors.New("federation: the end user already has an identity of this upstream")
)
