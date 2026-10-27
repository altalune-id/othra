package boot

import (
	"cmp"
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/altalune-id/othra/config"
	"github.com/altalune-id/othra/internal/grant"
	"github.com/altalune-id/othra/internal/kernel"
	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/internal/kernel/tenant"
	"github.com/altalune-id/othra/internal/keys"
	"github.com/altalune-id/othra/internal/resourceserver"
)

const (
	grantCodeTTL         = time.Minute
	grantAccessTokenTTL  = 15 * time.Minute
	grantRefreshIdle     = 30 * 24 * time.Hour
	grantRefreshAbsolute = 90 * 24 * time.Hour
)

func grantConfig(c config.OAuthConfig) grant.Config {
	return grant.Config{
		CodeTTL:         cmp.Or(c.CodeTTL, grantCodeTTL),
		AccessTokenTTL:  cmp.Or(c.AccessTokenTTL, grantAccessTokenTTL),
		RefreshIdle:     cmp.Or(c.Refresh.Idle, grantRefreshIdle),
		RefreshAbsolute: cmp.Or(c.Refresh.Absolute, grantRefreshAbsolute),
	}
}

func buildGrants(k kernel.Kernel, cfg config.Config, uow db.Transactor, signing *keys.Service, audiences grant.Audiences) *grant.Service {
	store := grant.NewStore(k.Pool, cfg.Postgres.Schema, cfg.Postgres.TablePrefix)
	return grant.NewService(store, uow, accessTokenSigner{signer: keys.NewSigner(signing)}, audiences, grantConfig(cfg.OAuth),
		k.Reporter.Unexpected).WithLogger(orDiscard(k.Log))
}

type accessTokenSigner struct{ signer *keys.Signer }

var _ grant.AccessTokens = accessTokenSigner{}

func (a accessTokenSigner) Sign(ctx context.Context, p grant.Project, t grant.AccessToken) (string, error) {
	ctx = tenant.Into(ctx, tenant.Context{OrgID: p.OrgID, ProjectID: p.ID})
	tok, err := a.signer.Mint(ctx, p.ID, &keys.MintInput{
		Issuer: p.IssuerURL, Subject: t.Subject, Audience: []string{t.Audience}, ClientID: t.ClientID, SessionID: t.SessionID,
		Scope: strings.Join(t.Scopes, " "), AuthTime: t.AuthTime, IssuedAt: t.IssuedAt, ExpiresAt: t.ExpiresAt,
	})
	if err != nil {
		return "", err
	}
	return tok.Compact, nil
}

type grantAudiences struct{ servers *resourceserver.Service }

var _ grant.Audiences = grantAudiences{}

func (a grantAudiences) Audience(
	ctx context.Context, p grant.Project, serverID uuid.UUID, c grant.Client, scopes []string,
) (identifier string, ok bool, err error) {
	return a.servers.Admits(ctx, p.OrgID, p.ID, serverID, c.ID, c.CIMDHost, scopes)
}
