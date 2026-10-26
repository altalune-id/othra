package federation

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"altalune.id/othra/internal/upstream"
)

// Upstreams reads a project's upstream IdPs.
type Upstreams interface {
	// List returns the project's upstreams, enabled or not.
	List(ctx context.Context, orgID, projectID uuid.UUID) ([]*upstream.Upstream, error)
	// Credentials returns the enabled upstream of kind with its secret opened, or *upstream.NotFoundError.
	Credentials(ctx context.Context, orgID, projectID uuid.UUID, kind upstream.Kind) (*upstream.Credentials, error)
}

// EndUsers finds, creates and signs in end users, in the unit of work on ctx.
type EndUsers interface {
	// EndUserIDByEmail returns the project's end user with email; ok is false when there is none.
	EndUserIDByEmail(ctx context.Context, p Project, email string) (id uuid.UUID, ok bool, err error)
	// Create inserts a verified end user, or returns ErrEmailTaken; it never updates an existing one.
	Create(ctx context.Context, p Project, email, name string) (uuid.UUID, error)
	// SignIn starts the user session of req.EndUserID on the browser of req.BrowserSecret.
	SignIn(ctx context.Context, p Project, req SessionRequest) (*Session, error)
}

// SignInSwitch reads a project's email-link switch and locks the project row until the unit of work on ctx ends.
type SignInSwitch interface {
	LockEmailLink(ctx context.Context, projectID uuid.UUID) (bool, error)
}

// ErrEmailTaken reports a Create whose email another end user just took; federation answers it as a collision.
var ErrEmailTaken = errors.New("federation: the email belongs to another end user")
