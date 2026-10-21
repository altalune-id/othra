package boot

import (
	"cmp"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"time"

	"altalune.id/othra/config"
	"altalune.id/othra/internal/enduser"
	"altalune.id/othra/internal/oauth/hosted"
	"altalune.id/othra/internal/oauth/oidc"
	"altalune.id/othra/internal/web/layout"
	"altalune.id/othra/mailer"
)

const (
	defaultCSRFMaxAge = time.Hour
	defaultLinkTTL    = 15 * time.Minute
)

func buildHosted(cfg config.Config, svcs *Services, kit layout.Kit, resolver *oidc.Resolver) *hosted.Web {
	return hosted.NewWeb(hostedUsers{users: svcs.EndUsers}, resolver, kit, hosted.Config{
		BasePath:   cfg.HTTP.BasePath,
		PublicBase: publicBase(cfg.HTTP),
		MailFrom:   cfg.Mail.From,
		CSRFKey:    purposeKey(svcs.StateSecret, "othra-hosted-csrf-v1"),
		CookieKey:  purposeKey(svcs.StateSecret, "othra-hosted-cookie-v1"),
		CSRFMaxAge: cmp.Or(cfg.EndUsers.CSRF.MaxAge, defaultCSRFMaxAge),
		LinkTTL:    cmp.Or(cfg.EndUsers.MagicLink.TTL, defaultLinkTTL),
		LogoURL:    svcs.Projects.LogoURL,
	}).WithLimits(layout.Limits{Limiter: svcs.AuthLimiter, TrustedProxyHops: cfg.HTTP.TrustedProxyHops})
}

// SECURITY: one key per purpose, derived from http.stateSecret, so a value signed for one use never verifies for another.
func purposeKey(secret []byte, purpose string) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(purpose))
	return h.Sum(nil)
}

type hostedUsers struct{ users *enduser.Service }

var _ hosted.EndUsers = hostedUsers{}

func endUserProject(p hosted.Project) enduser.Project {
	return enduser.Project{
		OrgID: p.OrgID, ID: p.ID, PublicID: p.PublicID, IssuerURL: p.IssuerURL, EmailLink: p.EmailLink, TermsVersion: int(p.TermsVersion),
		TermsURL: p.TermsURL, PrivacyURL: p.PrivacyURL,
	}
}

func hostedAccount(a enduser.Account) hosted.Account {
	return hosted.Account{UserSession: a.PublicID, EndUser: a.Subject, Email: a.Email, Name: a.Name, AuthTime: a.AuthTime}
}

func (u hostedUsers) IssueLink(ctx context.Context, req hosted.IssueRequest) (hosted.Link, bool, error) {
	p, lr := endUserProject(req.Project), enduser.LinkRequest{Email: req.Email, ReturnTo: req.ReturnTo, IP: req.IP, Allow: req.Allow}
	var issued enduser.IssuedLink
	var err error
	if req.Compose == nil {
		issued, err = u.users.IssueLink(ctx, p, lr)
	} else {
		issued, err = u.users.SendLink(ctx, p, lr, func(_ context.Context, l enduser.IssuedLink) (mailer.Message, error) {
			return req.Compose(hostedLink(l))
		})
	}
	if err != nil {
		return hosted.Link{}, false, err
	}
	return hostedLink(issued), issued.Issued, nil
}

func hostedLink(l enduser.IssuedLink) hosted.Link {
	return hosted.Link{Email: l.Email, URL: l.URL, SignUp: l.SignUp}
}

func (u hostedUsers) LinkByToken(ctx context.Context, p hosted.Project, token string) (hosted.LinkRef, error) {
	l, err := u.users.LinkByToken(ctx, endUserProject(p), token)
	if err != nil {
		return hosted.LinkRef{}, err
	}
	return hosted.LinkRef{ID: l.PublicID, Email: l.Email}, nil
}

func (u hostedUsers) Redeem(ctx context.Context, req hosted.RedeemRequest) (*hosted.Redeemed, error) {
	got, err := u.users.Redeem(ctx, endUserProject(req.Project), enduser.RedeemRequest{
		Token: req.Token, BrowserSecret: req.BrowserSecret, UserAgent: req.UserAgent, IP: req.IP,
	})
	if err != nil {
		return nil, err
	}
	return &hosted.Redeemed{
		BrowserSecret: got.Secret, ExpiresAt: got.ExpiresAt, Account: hostedAccount(got.Account), ReturnTo: got.ReturnTo,
	}, nil
}

func (u hostedUsers) Browser(ctx context.Context, p hosted.Project, secret string) (*hosted.Browser, error) {
	sess, err := u.users.Session(ctx, endUserProject(p), secret)
	if err != nil || sess == nil {
		return &hosted.Browser{}, err
	}
	out := &hosted.Browser{Accounts: make([]hosted.Account, 0, len(sess.Accounts))}
	for _, a := range sess.Accounts {
		out.Accounts = append(out.Accounts, hostedAccount(a))
	}
	return out, nil
}

func (u hostedUsers) Select(ctx context.Context, p hosted.Project, secret, userSession string) (hosted.Account, error) {
	a, err := u.users.Select(ctx, endUserProject(p), secret, userSession)
	if err != nil {
		return hosted.Account{}, err
	}
	return hostedAccount(*a), nil
}

func (u hostedUsers) SignOut(ctx context.Context, p hosted.Project, secret, userSession string) error {
	return u.users.SignOut(ctx, endUserProject(p), secret, userSession)
}

func (u hostedUsers) TermsState(ctx context.Context, p hosted.Project, secret, userSession string) (hosted.TermsState, error) {
	ep := endUserProject(p)
	sess, err := u.users.Session(ctx, ep, secret)
	if err != nil {
		return hosted.TermsState{}, err
	}
	a, ok := sess.Account(userSession)
	if !ok {
		return hosted.TermsState{}, &enduser.SessionGoneError{}
	}
	needed, updated, err := u.users.NeedsTerms(ctx, ep, *a)
	return hosted.TermsState{Needed: needed, Updated: updated}, err
}

func (u hostedUsers) AcceptTerms(ctx context.Context, a hosted.TermsAcceptance) error {
	_, err := u.users.AcceptTerms(ctx, endUserProject(a.Project), a.BrowserSecret, a.UserSession, int(a.Seen), a.IP, a.UserAgent)
	if enduser.IsTermsChangedError(err) {
		return hosted.ErrTermsChanged
	}
	return err
}
