package boot

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"

	"altalune.id/othra/config"
	"altalune.id/othra/internal/auth"
	"altalune.id/othra/internal/keys"
	"altalune.id/othra/internal/oauth/hosted"
	"altalune.id/othra/internal/oauth/oidc"
	"altalune.id/othra/oidcrp"
)

const consoleRPTimeout = 10 * time.Second

//nolint:gochecknoglobals // the scopes the console asks /p/system for; no refresh token, the console keeps its own session.
var consoleScopes = []string{"openid", "profile", "email"}

var errNotInstalled = errors.New("boot: the console project doesn't exist until install")

// NOTE: the console client exists only after install, so its relying party is built on first use and then kept.
type consoleRP struct {
	cfg       config.HTTPConfig
	svcs      *Services
	transport http.RoundTripper

	mu sync.Mutex
	rp *oidcrp.Client
}

// NOTE: the relying party reaches /p/system through issuer, the mux buildIssuerRoutes fills, never over the network.
func buildConsoleRP(cfg config.Config, svcs *Services, issuer http.Handler) auth.ConsoleRP {
	if !issuesTokens(svcs) {
		return nil
	}
	return newConsoleRP(cfg.HTTP, svcs, oidc.InProcess(issuer))
}

func newConsoleRP(cfg config.HTTPConfig, svcs *Services, transport http.RoundTripper) *consoleRP {
	return &consoleRP{cfg: cfg, svcs: svcs, transport: transport}
}

func (c *consoleRP) client(ctx context.Context) (*oidcrp.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rp != nil {
		return c.rp, nil
	}
	sys, p, err := consoleProject(ctx, c.svcs)
	switch {
	case err != nil:
		return nil, err
	case p == nil:
		return nil, errNotInstalled
	}
	cl, err := c.svcs.Clients.System(ctx, sys.ID, p.ID)
	if err != nil {
		return nil, err
	}
	c.rp = oidcrp.New(oidcrp.Config{
		Issuer: c.svcs.Projects.IssuerURL(p.IssuerHandle), ClientID: cl.OAuthID(), RedirectURI: consoleURIs(c.cfg).RedirectURI,
		Scopes: consoleScopes, HTTPClient: &http.Client{Transport: c.transport, Timeout: consoleRPTimeout},
	})
	return c.rp, nil
}

// AuthCodeURL implements auth.ConsoleRP.
func (c *consoleRP) AuthCodeURL(ctx context.Context, req oidcrp.AuthRequest) (string, error) {
	rp, err := c.client(ctx)
	if err != nil {
		return "", err
	}
	return rp.AuthCodeURL(ctx, req)
}

// Exchange implements auth.ConsoleRP.
func (c *consoleRP) Exchange(ctx context.Context, cb oidcrp.Callback, req oidcrp.AuthRequest) (*oidcrp.IDToken, error) {
	rp, err := c.client(ctx)
	if err != nil {
		return nil, err
	}
	return rp.Exchange(ctx, cb, req)
}

// EndSessionURL implements auth.ConsoleRP.
func (c *consoleRP) EndSessionURL(ctx context.Context, idTokenHint, postLogoutRedirectURI, state string) (string, error) {
	rp, err := c.client(ctx)
	if err != nil {
		return "", err
	}
	return rp.EndSessionURL(ctx, liveHint(ctx, rp.Keys(), idTokenHint), postLogoutRedirectURI, state)
}

// EndSystemSession implements auth.ConsoleRP: the /p/system grant the ID token's sid names, and its user session, end in process.
func (c *consoleRP) EndSystemSession(ctx context.Context, idToken string) error {
	sys, p, err := consoleProject(ctx, c.svcs)
	if err != nil || p == nil {
		return err
	}
	hp := hosted.Project{OrgID: sys.ID, ID: p.ID, IssuerURL: c.svcs.Projects.IssuerURL(p.IssuerHandle)}
	o := hostedOAuth{grants: c.svcs.Grants, hints: keys.NewVerifier(c.svcs.Keys), users: c.svcs.EndUsers}
	hint, ok, err := o.VerifyHint(ctx, hp, idToken)
	if err != nil || !ok || hint.SID == "" {
		return err
	}
	userSession, _, err := o.SessionOf(ctx, hp, hint.SID)
	if err != nil {
		return err
	}
	return o.EndSession(ctx, hp, hint.SID, userSession)
}

// NOTE: RPL §2: a hint signed by a key /p/system no longer publishes is left out, so after a rotation client_id alone ends the session.
func liveHint(ctx context.Context, src *oidcrp.KeySource, hint string) string {
	meta, err := src.Metadata(ctx)
	if err != nil {
		return ""
	}
	jws, err := jose.ParseSigned(hint, meta.SigningAlgorithms())
	if err != nil || len(jws.Signatures) != 1 {
		return ""
	}
	key, err := src.Key(ctx, jws.Signatures[0].Header.KeyID)
	if err != nil {
		return ""
	}
	if _, err := jws.Verify(key); err != nil {
		return ""
	}
	return hint
}
