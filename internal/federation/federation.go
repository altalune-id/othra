// Package federation signs end users in through a project's upstream IdPs and links their identities (authl ADR-0014).
package federation

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/internal/upstream"
)

// Project is the project a hosted request runs for.
type Project struct {
	OrgID    uuid.UUID
	ID       uuid.UUID
	PublicID string
}

// Identity is an upstream's assertion about a person, keyed by the canonical (Issuer, Subject) (OIDC §5.7).
type Identity struct {
	UpstreamID    uuid.UUID
	Issuer        string
	Subject       string
	Email         string
	EmailVerified bool
	// HostedDomain is Google's hd claim, the Workspace domain; empty for a consumer account (P8, R-1).
	HostedDomain string
	Name         string
	PublicID     string
	Kind         upstream.Kind
	LinkedAt     time.Time
	LastUsedAt   time.Time
}

// SignInMethod is one way an end user signs in: the project's email link (an empty Kind), or an upstream IdP, linked or linkable.
type SignInMethod struct {
	Kind       upstream.Kind
	Linked     bool
	Identity   Identity
	CanRemove  bool
	LastMethod bool
}

// Config is what federation needs from the deployment; boot maps upstreams.* into it.
type Config struct {
	// GoogleIssuerOverride points Google at a qa fake IdP; empty in production.
	GoogleIssuerOverride string
	// HTTPClient reaches the upstream IdPs; it must refuse redirects and private hosts.
	HTTPClient *http.Client
}

// SessionRequest signs EndUserID in on the browser of BrowserSecret after a federated sign-in.
type SessionRequest struct {
	EndUserID     uuid.UUID
	Method        string
	BrowserSecret string
	UserAgent     string
}

// Session is the user session a federated sign-in started, with the browser's rotated secret.
type Session struct {
	BrowserSecret string
	ExpiresAt     time.Time
	EndUserID     uuid.UUID
	UserSession   string
	Subject       string
	Email         string
	Name          string
	AuthTime      time.Time
	SelectedAt    time.Time
}

type storedIdentity struct {
	ID        uuid.UUID
	PublicID  string
	EndUserID uuid.UUID
	Identity
}
