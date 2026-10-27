package upstream

import (
	"time"

	"github.com/google/uuid"
)

// Kind is an upstream IdP kind; its issuer, endpoints and scopes are fixed here, never configured.
type Kind string

// KindGoogle is Google's OpenID Connect.
const KindGoogle Kind = "google"

// Spec is a kind's fixed facts: the canonical issuer stored on identities, its other iss forms, and how to re-authenticate.
type Spec struct {
	Kind          Kind
	Issuer        string
	AcceptIssuers []string
	Scopes        []string
	// Prompt is sent on every start, so the person picks the account each time (Google has no prompt=login).
	Prompt string
}

// SpecOf returns kind's fixed facts.
func SpecOf(kind Kind) (Spec, bool) {
	if kind != KindGoogle {
		return Spec{}, false
	}
	return Spec{
		Kind: KindGoogle, Issuer: "https://accounts.google.com", AcceptIssuers: []string{"accounts.google.com"},
		Scopes: []string{"openid", "email", "profile"}, Prompt: "select_account",
	}, true
}

// Upstream is a project's configured upstream IdP; its client secret stays sealed and is only opened for a sign-in.
type Upstream struct {
	ID               uuid.UUID
	PublicID         string
	OrgID            uuid.UUID
	ProjectID        uuid.UUID
	Kind             Kind
	ClientID         string
	Enabled          bool
	IdentitiesLinked bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Credentials is an enabled upstream with its opened client secret, for the federation round trip only.
type Credentials struct {
	Upstream     Upstream
	ClientSecret string
}

// RedirectURI is where the upstream sends the browser back: the project's issuer URL plus the kind's callback path.
func RedirectURI(issuerURL string, kind Kind) string {
	return issuerURL + "/federation/" + string(kind) + "/callback"
}
