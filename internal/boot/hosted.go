package boot

import (
	"cmp"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"slices"
	"time"

	"altalune.id/othra/config"
	"altalune.id/othra/internal/client"
	"altalune.id/othra/internal/enduser"
	"altalune.id/othra/internal/grant"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/keys"
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
	linkTTL := cmp.Or(cfg.EndUsers.MagicLink.TTL, defaultLinkTTL)
	web := hosted.NewWeb(hostedUsers{users: svcs.EndUsers, grants: svcs.Grants}, resolver, kit, hosted.Config{
		BasePath:   cfg.HTTP.BasePath,
		PublicBase: publicBase(cfg.HTTP),
		MailFrom:   cfg.Mail.From,
		CSRFKey:    purposeKey(svcs.StateSecret, "othra-hosted-csrf-v1"),
		CookieKey:  purposeKey(svcs.StateSecret, "othra-hosted-cookie-v1"),
		CSRFMaxAge: cmp.Or(cfg.EndUsers.CSRF.MaxAge, defaultCSRFMaxAge),
		LinkTTL:    linkTTL,
		LogoURL:    svcs.Projects.LogoURL,
	}).WithLimits(layout.Limits{Limiter: svcs.AuthLimiter, TrustedProxyHops: cfg.HTTP.TrustedProxyHops})
	if svcs.Grants != nil && svcs.Clients != nil && svcs.Keys != nil {
		oauth := hostedOAuth{clients: svcs.Clients, grants: svcs.Grants, hints: keys.NewVerifier(svcs.Keys), users: svcs.EndUsers}
		web.WithOAuth(oauth, oauth, hosted.NewMarker(purposeKey(svcs.StateSecret, "othra-hosted-marker-v1"), linkTTL))
	}
	return web
}

// SECURITY: one key per purpose, derived from http.stateSecret, so a value signed for one use never verifies for another.
func purposeKey(secret []byte, purpose string) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(purpose))
	return h.Sum(nil)
}

type hostedUsers struct {
	users  *enduser.Service
	grants *grant.Service
}

var _ hosted.EndUsers = hostedUsers{}

func endUserProject(p hosted.Project) enduser.Project {
	return enduser.Project{
		OrgID: p.OrgID, ID: p.ID, PublicID: p.PublicID, IssuerURL: p.IssuerURL, EmailLink: p.EmailLink, TermsVersion: int(p.TermsVersion),
		TermsURL: p.TermsURL, PrivacyURL: p.PrivacyURL,
	}
}

func hostedAccount(a enduser.Account) hosted.Account {
	return hosted.Account{
		UserSession: a.PublicID, EndUser: a.Subject, Email: a.Email, Name: a.Name, AuthTime: a.AuthTime, SelectedAt: a.SelectedAt,
	}
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

// NOTE: G16: signing an account out also revokes the grants issued under that user session.
func (u hostedUsers) SignOut(ctx context.Context, p hosted.Project, secret, userSession string) error {
	if err := u.users.SignOut(ctx, endUserProject(p), secret, userSession); err != nil || u.grants == nil {
		return err
	}
	return u.grants.RevokeUserSession(ctx, grantProject(p), userSession, grant.ReasonSignOut)
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

type hostedOAuth struct {
	clients *client.Service
	grants  *grant.Service
	hints   *keys.Verifier
	users   *enduser.Service
}

var (
	_ hosted.Clients = hostedOAuth{}
	_ hosted.Grants  = hostedOAuth{}
)

func grantProject(p hosted.Project) grant.Project { return grant.Project{OrgID: p.OrgID, ID: p.ID} }

func (o hostedOAuth) Client(ctx context.Context, p hosted.Project, id string) (hosted.Client, bool, error) {
	c, err := o.clients.Get(ctx, p.OrgID, p.ID, id)
	switch {
	case client.IsNotFoundError(err):
		return hosted.Client{}, false, nil
	case err != nil:
		return hosted.Client{}, false, err
	}
	return hosted.Client{
		ID: c.PublicID, Name: c.Name, Native: c.ApplicationType == client.AppNative, Confidential: c.UsesSecret(),
		RedirectURIs: c.RedirectURIs, PostLogoutRedirectURIs: c.PostLogoutRedirectURIs, AllowedScopes: c.AllowedScopes,
		Refresh: slices.Contains(c.GrantTypes, client.GrantRefreshToken), PKCERequired: c.PKCERequired,
		ConsentRequired: c.ConsentPolicy == client.ConsentRequire,
	}, true, nil
}

func (o hostedOAuth) IssueCode(ctx context.Context, req hosted.CodeRequest) (string, error) {
	code, err := o.grants.IssueCode(ctx, grantProject(req.Project), grant.CodeRequest{
		Client: req.ClientID, RedirectURI: req.RedirectURI, Scopes: req.Scopes, CodeChallenge: req.CodeChallenge, Nonce: req.Nonce,
		UserSession: req.UserSession, EndUser: req.EndUser, AuthTime: req.AuthTime,
	})
	if grant.IsSessionGoneError(err) {
		return "", &enduser.SessionGoneError{}
	}
	return code, err
}

func (o hostedOAuth) VerifyHint(ctx context.Context, p hosted.Project, raw string) (hosted.Hint, bool, error) {
	claims, err := o.hints.VerifyIDTokenHint(tenant.Into(ctx, tenant.Context{OrgID: p.OrgID, ProjectID: p.ID}), p.ID, p.IssuerURL, raw)
	switch {
	case errors.Is(err, keys.ErrInvalidToken):
		return hosted.Hint{}, false, nil
	case err != nil:
		return hosted.Hint{}, false, err
	}
	return hosted.Hint{Subject: claims.Subject, SID: claims.SessionID, Audience: claims.Audience}, true, nil
}

func (o hostedOAuth) SessionOf(ctx context.Context, p hosted.Project, sid string) (userSession string, live bool, err error) {
	if sid == "" {
		return "", false, nil
	}
	f, err := o.grants.AnyFamily(ctx, grantProject(p), sid)
	if err != nil || f == nil {
		return "", false, err
	}
	return f.UserSession, f.Live(time.Now()), nil
}

func (o hostedOAuth) EndSession(ctx context.Context, p hosted.Project, sid, userSession string) error {
	if _, err := o.grants.RevokeFamily(ctx, grantProject(p), sid, grant.ReasonEndSession); err != nil {
		return err
	}
	if userSession == "" {
		return nil
	}
	if err := o.users.EndSession(ctx, endUserProject(p), userSession); err != nil {
		return err
	}
	return o.grants.RevokeUserSession(ctx, grantProject(p), userSession, grant.ReasonEndSession)
}
