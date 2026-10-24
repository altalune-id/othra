package hosted

import (
	"context"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/internal/consent"
	"altalune.id/othra/mailer"
)

// Project is the project a hosted request runs for, with the settings sign-in follows; every EndUsers call is scoped to it.
type Project struct {
	OrgID        uuid.UUID
	ID           uuid.UUID
	PublicID     string
	IssuerURL    string
	EmailLink    bool
	TermsVersion int32
	TermsURL     string
	PrivacyURL   string
}

// Link is a magic link just issued: URL holds the plaintext token, so it only goes into the mail.
type Link struct {
	Email  string
	URL    string
	SignUp bool
}

// IssueRequest asks for a magic link to Email that returns to ReturnTo.
type IssueRequest struct {
	Project  Project
	Email    string
	ReturnTo string
	IP       string
	// Compose builds the mail for the link; the service enqueues it in the link's unit of work. Nil sends no mail.
	Compose func(Link) (mailer.Message, error)
	// Allow runs once the resend cooldown has passed, before the link is stored; false issues nothing, silently. Nil allows.
	Allow func(ctx context.Context) (bool, error)
}

// Account is one end user signed in on this browser: a live user session.
type Account struct {
	UserSession string
	EndUserID   uuid.UUID
	EndUser     string
	Email       string
	Name        string
	AuthTime    time.Time
	SelectedAt  time.Time
}

// Browser is the browser session a cookie names, with its live accounts, the active one first.
type Browser struct {
	Accounts []Account
}

// Active returns the browser's active account: the one selected last.
func (b *Browser) Active() (Account, bool) {
	if b == nil || len(b.Accounts) == 0 {
		return Account{}, false
	}
	return b.Accounts[0], true
}

// Find returns the account of userSession on this browser.
func (b *Browser) Find(userSession string) (Account, bool) {
	if b == nil {
		return Account{}, false
	}
	for _, a := range b.Accounts {
		if a.UserSession == userSession {
			return a, true
		}
	}
	return Account{}, false
}

// LinkRef is a live magic link: its public id and the email it signs in.
type LinkRef struct {
	ID    string
	Email string
}

// RedeemRequest spends the link Token for the browser that holds BrowserSecret ("" when it has none).
type RedeemRequest struct {
	Project       Project
	Token         string
	BrowserSecret string
	IP            string
	UserAgent     string
}

// Redeemed is a spent link: the browser's rotated secret, the account now active and where to go next.
type Redeemed struct {
	BrowserSecret string
	ExpiresAt     time.Time
	Account       Account
	ReturnTo      string
}

// TermsState is where an end user stands with the project's current terms.
type TermsState struct {
	Needed  bool
	Updated bool
}

// TermsAcceptance records that the account of UserSession on the browser of BrowserSecret accepted the project's current terms.
type TermsAcceptance struct {
	Project       Project
	BrowserSecret string
	UserSession   string
	// Seen is the terms version the page showed; an older one is ErrTermsChanged.
	Seen      int32
	IP        string
	UserAgent string
}

// EndUsers is what the hosted pages need from the end-user domain; boot plugs in enduser.Service.
type EndUsers interface {
	// IssueLink stores a magic link and enqueues its mail; within the resend cooldown it sends nothing and returns ok false.
	IssueLink(ctx context.Context, req IssueRequest) (link Link, ok bool, err error)
	// LinkByToken returns the live link a mailed token names without spending it, or *enduser.LinkNotUsableError.
	LinkByToken(ctx context.Context, p Project, token string) (LinkRef, error)
	// Redeem spends a link once and signs its end user in on this browser, or returns *enduser.LinkNotUsableError.
	Redeem(ctx context.Context, req RedeemRequest) (*Redeemed, error)
	// Browser returns the live accounts of the browser of secret, touching its last use; an unknown secret has none.
	Browser(ctx context.Context, p Project, secret string) (*Browser, error)
	// Select makes userSession the browser's active account, or returns *enduser.SessionGoneError.
	Select(ctx context.Context, p Project, secret, userSession string) (Account, error)
	// SignOut ends userSession on this browser; an account already gone is no error.
	SignOut(ctx context.Context, p Project, secret, userSession string) error
	// TermsState reports whether the account of userSession on this browser still has to accept the project's terms.
	TermsState(ctx context.Context, p Project, secret, userSession string) (TermsState, error)
	// AcceptTerms appends the acceptance: *enduser.SessionGoneError for an account not on this browser, ErrTermsChanged for moved terms.
	AcceptTerms(ctx context.Context, a TermsAcceptance) error
}

// Client is what /authorize and end-session need to know about a registered OAuth client.
type Client struct {
	ID                     string
	RowID                  uuid.UUID
	MetadataHost           string
	Name                   string
	Native                 bool
	Confidential           bool
	RedirectURIs           []string
	PostLogoutRedirectURIs []string
	AllowedScopes          []string
	Refresh                bool
	PKCERequired           bool
	ConsentRequired        bool
}

// Clients reads the project's OAuth clients.
type Clients interface {
	// Client returns the project's client id, a metadata document client only when Admit already stored it; ok is false when there is none.
	Client(ctx context.Context, p Project, id string) (c Client, ok bool, err error)
}

// Resource is the resource server an authorization request names, with the scopes this client may ask of it.
type Resource struct {
	ID           uuid.UUID
	Identifier   string
	Name         string
	Scopes       []string
	Descriptions map[string]string
	// Catalog is every scope the server has, so a stored consent drops the ones it no longer has (R-6).
	Catalog []string
}

// Resources resolves a request's resource for a registered client.
type Resources interface {
	// Resource returns the server identifier names exactly; ok is false when it is unknown, not admissible or not linked to c.
	Resource(ctx context.Context, p Project, c Client, identifier string) (r Resource, ok bool, err error)
}

// Admissions admits Client ID Metadata Document clients (CIMD -02, ADR-0021).
type Admissions interface {
	// Admit returns the client of the document at clientURL and the one resource that admits it, fetching the document
	// when no fresh copy is stored; it fails with SIG010 or SIG011 before any redirect is trusted.
	Admit(ctx context.Context, p Project, clientURL string, resources []string) (Client, Resource, error)
}

// Consents reads and records what end users approved.
type Consents interface {
	Consent(ctx context.Context, p Project, key consent.Key) (*consent.Consent, error)
	Grant(ctx context.Context, p Project, req consent.GrantRequest) error
}

// CodeRequest is an authorization code to issue for a signed-in account.
type CodeRequest struct {
	Project       Project
	ClientID      string
	RedirectURI   string
	Scopes        []string
	CodeChallenge string
	Nonce         string
	UserSession   string
	EndUser       string
	AuthTime      time.Time
	// ResourceServerID is the server the code's access tokens are for, nil for an OIDC-only request (RFC 8707).
	ResourceServerID *uuid.UUID
}

// Hint is a verified id_token_hint: an ID token this issuer signed, expired or not.
type Hint struct {
	Subject  string
	SID      string
	Audience []string
}

// Grants issues codes and ends the grants a sign-out names.
type Grants interface {
	// IssueCode stores a code for req and returns it; the plaintext is never kept.
	IssueCode(ctx context.Context, req CodeRequest) (string, error)
	// VerifyHint checks that raw is an ID token this project issued, ignoring its expiry; ok is false when it is not.
	VerifyHint(ctx context.Context, p Project, raw string) (h Hint, ok bool, err error)
	// SessionOf returns the user session of the grant family sid in any state, empty when unknown or pruned; live reports the family is.
	SessionOf(ctx context.Context, p Project, sid string) (userSession string, live bool, err error)
	// EndSession revokes the grant family sid and ends userSession with its grants; anything already ended is no error.
	EndSession(ctx context.Context, p Project, sid, userSession string) error
}
