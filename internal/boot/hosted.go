package boot

import (
	"cmp"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/config"
	"altalune.id/othra/internal/client"
	"altalune.id/othra/internal/consent"
	"altalune.id/othra/internal/enduser"
	"altalune.id/othra/internal/grant"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/keys"
	"altalune.id/othra/internal/oauth/hosted"
	"altalune.id/othra/internal/oauth/oidc"
	"altalune.id/othra/internal/resourceserver"
	"altalune.id/othra/internal/web/layout"
	"altalune.id/othra/mailer"
)

const (
	defaultCSRFMaxAge = time.Hour
	defaultLinkTTL    = 15 * time.Minute
)

func buildHosted(cfg config.Config, svcs *Services, kit layout.Kit, resolver *oidc.Resolver) *hosted.Web {
	linkTTL := cmp.Or(cfg.EndUsers.MagicLink.TTL, defaultLinkTTL)
	web := hosted.NewWeb(hostedUsers{users: svcs.EndUsers}, resolver, kit, hosted.Config{
		BasePath:   cfg.HTTP.BasePath,
		PublicBase: publicBase(cfg.HTTP),
		MailFrom:   cfg.Mail.From,
		CSRFKey:    purposeKey(svcs.StateSecret, "othra-hosted-csrf-v1"),
		CookieKey:  purposeKey(svcs.StateSecret, "othra-hosted-cookie-v1"),
		CSRFMaxAge: cmp.Or(cfg.EndUsers.CSRF.MaxAge, defaultCSRFMaxAge),
		LinkTTL:    linkTTL,
		LogoURL:    svcs.Projects.LogoURL,
	}).WithLimits(layout.Limits{Limiter: svcs.AuthLimiter, TrustedProxyHops: cfg.HTTP.TrustedProxyHops})
	if page, _ := buildPortal(cfg.HTTP.BasePath); page != nil {
		web.WithPortal(page)
	}
	if svcs.Grants != nil && svcs.Clients != nil && svcs.Keys != nil && svcs.ResourceServers != nil && svcs.Consents != nil {
		oauth := hostedOAuth{
			clients: svcs.Clients, grants: svcs.Grants, hints: keys.NewVerifier(svcs.Keys), users: svcs.EndUsers,
			servers: svcs.ResourceServers, consents: svcs.Consents,
		}
		web.WithOAuth(hosted.OAuth{
			Clients: oauth, Resources: oauth, Admissions: oauth, Consents: oauth, Grants: oauth,
			Marker: hosted.NewMarker(purposeKey(svcs.StateSecret, "othra-hosted-marker-v1"), linkTTL),
		})
	}
	return wireFederation(web, svcs)
}

// SECURITY: one key per purpose, derived from http.stateSecret, so a value signed for one use never verifies for another.
func purposeKey(secret []byte, purpose string) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(purpose))
	return h.Sum(nil)
}

type hostedUsers struct {
	users *enduser.Service
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
		UserSession: a.PublicID, EndUserID: a.EndUserID, EndUser: a.Subject, Email: a.Email, Name: a.Name, AuthTime: a.AuthTime,
		SelectedAt: a.SelectedAt, Method: a.Method,
	}
}

func (u hostedUsers) IssueLink(ctx context.Context, req hosted.IssueRequest) (hosted.Link, bool, error) {
	p, lr := endUserProject(req.Project), enduser.LinkRequest{
		Email: req.Email, ReturnTo: req.ReturnTo, IP: req.IP, Allow: req.Allow, Pending: req.Pending,
	}
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
	return hosted.Link{ID: l.PublicID, Email: l.Email, URL: l.URL, SignUp: l.SignUp}
}

func (u hostedUsers) LinkByToken(ctx context.Context, p hosted.Project, token string) (hosted.LinkRef, error) {
	l, err := u.users.LinkByToken(ctx, endUserProject(p), token)
	if err != nil {
		return hosted.LinkRef{}, err
	}
	return hostedLinkRef(l), nil
}

func hostedLinkRef(l *enduser.Link) hosted.LinkRef {
	return hosted.LinkRef{ID: l.PublicID, Email: l.Email, Pending: l.Pending}
}

func (u hostedUsers) Redeem(ctx context.Context, req hosted.RedeemRequest) (*hosted.Redeemed, error) {
	rr := enduser.RedeemRequest{Token: req.Token, BrowserSecret: req.BrowserSecret, UserAgent: req.UserAgent, IP: req.IP}
	if req.Confirm != nil {
		rr.Confirm = func(ctx context.Context, l *enduser.Link, endUserID uuid.UUID, created bool) (bool, error) {
			return req.Confirm(ctx, hostedLinkRef(l), endUserID, created)
		}
	}
	got, err := u.users.Redeem(ctx, endUserProject(req.Project), rr)
	if err != nil {
		return nil, err
	}
	return &hosted.Redeemed{
		BrowserSecret: got.Secret, ExpiresAt: got.ExpiresAt, Account: hostedAccount(got.Account), ReturnTo: got.ReturnTo,
	}, nil
}

func (u hostedUsers) FillName(ctx context.Context, p hosted.Project, endUserID uuid.UUID, name string) error {
	return u.users.FillName(ctx, endUserProject(p), endUserID, name)
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

type hostedOAuth struct {
	clients  *client.Service
	grants   *grant.Service
	hints    *keys.Verifier
	users    *enduser.Service
	servers  *resourceserver.Service
	consents *consent.Service
}

var (
	_ hosted.Clients    = hostedOAuth{}
	_ hosted.Grants     = hostedOAuth{}
	_ hosted.Resources  = hostedOAuth{}
	_ hosted.Admissions = hostedOAuth{}
	_ hosted.Consents   = hostedOAuth{}
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
	return hostedClient(c), true, nil
}

func hostedClient(c *client.Client) hosted.Client {
	out := hosted.Client{
		ID: c.OAuthID(), RowID: c.ID, Name: c.Name, Native: c.ApplicationType == client.AppNative, Confidential: c.UsesSecret(),
		RedirectURIs: c.RedirectURIs, PostLogoutRedirectURIs: c.PostLogoutRedirectURIs, AllowedScopes: c.AllowedScopes,
		Refresh: slices.Contains(c.GrantTypes, client.GrantRefreshToken), PKCERequired: c.PKCERequired,
		ConsentRequired: c.ConsentPolicy == client.ConsentRequire,
	}
	if c.Managed() {
		out.MetadataHost = hostOf(c.MetadataURL)
	}
	return out
}

func hostedResource(r *resourceserver.ResourceServer, scopes []string) hosted.Resource {
	return hosted.Resource{
		ID: r.ID, Identifier: r.Identifier, Name: r.Name, Scopes: scopes, Descriptions: r.Descriptions, Catalog: r.Scopes,
	}
}

func (o hostedOAuth) Resource(ctx context.Context, p hosted.Project, c hosted.Client, identifier string) (hosted.Resource, bool, error) {
	r, pin, ok, err := o.servers.Resolve(ctx, p.OrgID, p.ID, c.RowID, identifier)
	if err != nil || !ok {
		return hosted.Resource{}, false, err
	}
	return hostedResource(r, pin), true, nil
}

// SECURITY: M2, ADR-0021 D1: exactly one resource, admitting the client_id host, before any fetch; every miss is the same SIG010.
func (o hostedOAuth) Admit(
	ctx context.Context, p hosted.Project, clientURL string, resources []string,
) (hosted.Client, hosted.Resource, error) {
	resources = slices.Compact(slices.Clone(resources))
	host, err := o.clients.MetadataHost(clientURL)
	if err != nil {
		return hosted.Client{}, hosted.Resource{}, err
	}
	if len(resources) != 1 {
		return hosted.Client{}, hosted.Resource{}, &client.NotAdmittedError{Reason: client.AdmissionResource}
	}
	r, ok, err := o.servers.ForMetadataClient(ctx, p.OrgID, p.ID, resources[0], host)
	switch {
	case err != nil:
		return hosted.Client{}, hosted.Resource{}, err
	case !ok:
		return hosted.Client{}, hosted.Resource{}, &client.NotAdmittedError{Reason: client.AdmissionOff}
	}
	scopes := r.MetadataClientScopes()
	c, err := o.clients.Admit(ctx, p.OrgID, p.ID, client.AdmitRequest{URL: clientURL, ResourceServerID: r.ID, Scopes: scopes})
	if err != nil {
		return hosted.Client{}, hosted.Resource{}, err
	}
	return hostedClient(c), hostedResource(r, scopes), nil
}

func consentProject(p hosted.Project) consent.Project {
	return consent.Project{OrgID: p.OrgID, ID: p.ID, PublicID: p.PublicID}
}

func (o hostedOAuth) Consent(ctx context.Context, p hosted.Project, key consent.Key) (*consent.Consent, error) {
	return o.consents.Find(ctx, consentProject(p), key)
}

func (o hostedOAuth) Grant(ctx context.Context, p hosted.Project, req consent.GrantRequest) error {
	_, err := o.consents.Grant(ctx, consentProject(p), req)
	return err
}

func (o hostedOAuth) IssueCode(ctx context.Context, req hosted.CodeRequest) (string, error) {
	clientID := req.ClientID
	if client.IsMetadataURL(clientID) {
		c, err := o.clients.Get(ctx, req.Project.OrgID, req.Project.ID, clientID)
		if err != nil {
			return "", err
		}
		clientID = c.PublicID
	}
	code, err := o.grants.IssueCode(ctx, grantProject(req.Project), grant.CodeRequest{
		Client: clientID, ResourceServerID: req.ResourceServerID, RedirectURI: req.RedirectURI, Scopes: req.Scopes,
		CodeChallenge: req.CodeChallenge, Nonce: req.Nonce,
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
	return o.users.EndSession(ctx, endUserProject(p), userSession)
}
