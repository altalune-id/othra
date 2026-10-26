package boot

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/config"
	"altalune.id/othra/httpclient"
	"altalune.id/othra/internal/enduser"
	"altalune.id/othra/internal/federation"
	"altalune.id/othra/internal/kernel"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/oauth/hosted"
	"altalune.id/othra/sealer"
)

const upstreamTimeout = 10 * time.Second

func buildFederation(
	k kernel.Kernel, cfg config.Config, uow db.Transactor, upstreams federation.Upstreams, users *enduser.Service,
	projects federation.SignInSwitch,
) *federation.Service {
	client := httpclient.New(
		httpclient.WithTimeout(upstreamTimeout), httpclient.WithoutRedirects(),
		httpclient.WithAllowPrivateHosts(cfg.Upstreams.AllowPrivateHosts),
	)
	store := federation.NewStore(k.Pool, cfg.Postgres.Schema, cfg.Postgres.TablePrefix)
	return federation.NewService(store, uow, upstreams, federatedUsers{users: users}, projects, federation.Config{
		GoogleIssuerOverride: cfg.Upstreams.Google.IssuerOverride, HTTPClient: client,
	}, k.Reporter.Unexpected).WithLogger(orDiscard(k.Log))
}

func wireFederation(web *hosted.Web, svcs *Services) *hosted.Web {
	if svcs.Federation == nil {
		return web
	}
	return web.WithFederation(svcs.Federation, federationSealer(svcs.StateSecret))
}

// SECURITY: a key sealer.New refuses leaves federation sealing nothing, so every start fails closed.
func federationSealer(secret []byte) sealer.Sealer {
	sl, err := sealer.New(purposeKey(secret, "othra-hosted-federation-v1"))
	if err != nil {
		return sealer.Disabled()
	}
	return sl
}

type federatedUsers struct{ users *enduser.Service }

var _ federation.EndUsers = federatedUsers{}

func federatedProject(p federation.Project) enduser.Project {
	return enduser.Project{OrgID: p.OrgID, ID: p.ID, PublicID: p.PublicID}
}

func (u federatedUsers) EndUserIDByEmail(ctx context.Context, p federation.Project, email string) (uuid.UUID, bool, error) {
	return u.users.EndUserIDByEmail(ctx, federatedProject(p), email)
}

func (u federatedUsers) Create(ctx context.Context, p federation.Project, email, name string) (uuid.UUID, error) {
	id, err := u.users.CreateFederated(ctx, federatedProject(p), email, name)
	if errors.Is(err, enduser.ErrEmailTaken) {
		return uuid.Nil, federation.ErrEmailTaken
	}
	return id, err
}

func (u federatedUsers) SignIn(ctx context.Context, p federation.Project, req federation.SessionRequest) (*federation.Session, error) {
	got, err := u.users.SignInFederated(ctx, federatedProject(p), enduser.FederatedSignIn{
		EndUserID: req.EndUserID, Method: req.Method, BrowserSecret: req.BrowserSecret, UserAgent: req.UserAgent,
	})
	if err != nil {
		return nil, err
	}
	a := got.Account
	return &federation.Session{
		BrowserSecret: got.Secret, ExpiresAt: got.ExpiresAt, EndUserID: a.EndUserID, UserSession: a.PublicID, Subject: a.Subject,
		Email: a.Email, Name: a.Name, AuthTime: a.AuthTime, SelectedAt: a.SelectedAt,
	}, nil
}
