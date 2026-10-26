package enduser

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Store is the driven port for the end-user tables; every call runs under the org and project on ctx.
type Store interface {
	// HasRecentLink reports whether email has an unused link created after since.
	HasRecentLink(ctx context.Context, projectID uuid.UUID, email string, since time.Time) (bool, error)
	// DeleteUnusedLinks removes the unused links of email created at or before before.
	DeleteUnusedLinks(ctx context.Context, projectID uuid.UUID, email string, before time.Time) error
	// InsertLink records l by the hash of its token, or returns ErrLiveLink when another request just issued one for the email.
	InsertLink(ctx context.Context, orgID, projectID uuid.UUID, l *Link, tokenHash, ip string) error
	// LinkByTokenHash returns the link with tokenHash, or *LinkNotUsableError.
	LinkByTokenHash(ctx context.Context, projectID uuid.UUID, tokenHash string) (*Link, error)
	// LinkByPublicID returns the link publicID, or *LinkNotUsableError.
	LinkByPublicID(ctx context.Context, projectID uuid.UUID, publicID string) (*Link, error)
	// UseLink marks the link with tokenHash used by browserID at now when it is unused and unexpired, or returns *LinkNotUsableError.
	UseLink(ctx context.Context, projectID uuid.UUID, tokenHash string, browserID uuid.UUID, now time.Time) (*Link, error)

	// BrowserBySecretHash returns the browser session with secretHash that has not ended, or nil.
	BrowserBySecretHash(ctx context.Context, projectID uuid.UUID, secretHash string) (*Browser, error)
	// InsertBrowser records b by the hash of its secret.
	InsertBrowser(ctx context.Context, orgID, projectID uuid.UUID, b *Browser, secretHash, userAgent string) error
	// RotateBrowser replaces b's secret hash and marks it seen at now.
	RotateBrowser(ctx context.Context, projectID, browserID uuid.UUID, secretHash string, now time.Time) error
	// TouchBrowser marks the browser seen at now.
	TouchBrowser(ctx context.Context, projectID, browserID uuid.UUID, now time.Time) error

	// EndUserByID returns the end user id, or nil.
	EndUserByID(ctx context.Context, projectID, id uuid.UUID) (*EndUser, error)
	// EndUserIDByEmail returns the id of the project's end user with email; ok is false when there is none.
	EndUserIDByEmail(ctx context.Context, projectID uuid.UUID, email string) (id uuid.UUID, ok bool, err error)
	// CreateEndUser inserts a verified end user with email and name, or returns ErrEmailTaken; it never updates an existing one.
	CreateEndUser(ctx context.Context, orgID, projectID uuid.UUID, email, name string, now time.Time) (uuid.UUID, error)
	// FillName sets the end user's name at now only while it is empty.
	FillName(ctx context.Context, projectID, endUserID uuid.UUID, name string, now time.Time) error
	// EndUserExists reports whether the project has an end user with email.
	EndUserExists(ctx context.Context, projectID uuid.UUID, email string) (bool, error)
	// SaveEndUser finds or creates the end user with email, verifying it and recording a sign-in at now.
	SaveEndUser(ctx context.Context, orgID, projectID uuid.UUID, email string, now time.Time) (endUserID uuid.UUID, err error)

	// Accounts returns the browser's live accounts, the most recently selected first.
	Accounts(ctx context.Context, projectID, browserID uuid.UUID) ([]Account, error)
	// SignIn starts, or refreshes, the user session of endUserID on the browser by method and selects it at now.
	SignIn(ctx context.Context, orgID, projectID, browserID, endUserID uuid.UUID, method string, now time.Time) error
	// Select marks the live user session publicID of the browser selected at now, reporting whether it was live.
	Select(ctx context.Context, projectID, browserID uuid.UUID, publicID string, now time.Time) (bool, error)
	// EndUserSession ends the live user session publicID of the browser at now.
	EndUserSession(ctx context.Context, projectID, browserID uuid.UUID, publicID string, now time.Time) error
	// EndUserSessionAnywhere ends the live user session publicID at now, on whichever browser holds it.
	EndUserSessionAnywhere(ctx context.Context, projectID uuid.UUID, publicID string, now time.Time) error
	// KeepNewest ends every live user session of the browser beyond the keep most recently selected.
	KeepNewest(ctx context.Context, projectID, browserID uuid.UUID, keep int, now time.Time) error

	// LatestTerms returns the highest terms version endUserID accepted, or 0.
	LatestTerms(ctx context.Context, projectID, endUserID uuid.UUID) (int, error)
	// InsertTerms appends a terms acceptance; accepting the same version again changes nothing.
	InsertTerms(ctx context.Context, orgID, projectID uuid.UUID, t TermsAcceptance) error

	// Prune deletes the project's expired links and sessions, and ended user sessions; it returns how many rows went.
	Prune(ctx context.Context, projectID uuid.UUID, now time.Time, idle time.Duration) (int, error)
}
