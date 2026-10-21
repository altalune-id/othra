package hosted

import (
	"context"
	"time"

	"github.com/google/uuid"

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
	EndUser     string
	Email       string
	Name        string
	AuthTime    time.Time
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
