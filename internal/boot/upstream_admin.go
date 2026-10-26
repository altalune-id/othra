package boot

import (
	"context"

	"github.com/google/uuid"

	"altalune.id/othra/config"
	"altalune.id/othra/internal/kernel"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/upstream"
	"altalune.id/othra/sealer"
)

func buildUpstreams(k kernel.Kernel, cfg config.Config, uow db.Transactor, sl sealer.Sealer, projects *project.Service) *upstream.Service {
	return upstream.NewService(upstream.NewStore(k.Pool, cfg.Postgres.Schema, cfg.Postgres.TablePrefix), uow, sl,
		upstreamGuard{projects: projects}, k.Reporter.Unexpected).
		WithLogger(orDiscard(k.Log)).WithIssuers(upstreamIssuers{projects: projects})
}

// NOTE: M7 F13: the project install creates gets Google from upstreams.google, once; every other project is set up in the console.
func seedUpstreams(cfg config.UpstreamsConfig, upstreams *upstream.Service) func(ctx context.Context, orgID, projectID uuid.UUID) error {
	return func(ctx context.Context, orgID, projectID uuid.UUID) error {
		return upstreams.Seed(ctx, orgID, projectID, upstream.KindGoogle, cfg.Google.ClientID, cfg.Google.ClientSecret)
	}
}

type upstreamGuard struct{ projects *project.Service }

func (g upstreamGuard) EmailLinkOn(ctx context.Context, projectID uuid.UUID) (bool, error) {
	return g.projects.LockEmailLink(ctx, projectID)
}

type upstreamIssuers struct{ projects *project.Service }

func (i upstreamIssuers) IssuerURL(ctx context.Context, projectID uuid.UUID) (string, error) {
	p, err := i.projects.Get(ctx, projectID)
	if err != nil {
		return "", err
	}
	return i.projects.IssuerURL(p.IssuerHandle), nil
}

// NOTE: F8: projectSignInMethods answers the email-link guard: an enabled upstream, and whether every end user linked one.
type projectSignInMethods struct {
	upstreams *upstream.Service
	linked    endUsersLinked
}

type endUsersLinked interface {
	EveryEndUserLinked(ctx context.Context, orgID, projectID uuid.UUID) (bool, error)
}

func (m projectSignInMethods) UpstreamEnabled(ctx context.Context, orgID, projectID uuid.UUID) (bool, error) {
	return m.upstreams.AnyEnabled(ctx, orgID, projectID)
}

func (m projectSignInMethods) AnyEndUserUnlinked(ctx context.Context, orgID, projectID uuid.UUID) (bool, error) {
	if m.linked == nil {
		return true, nil
	}
	all, err := m.linked.EveryEndUserLinked(ctx, orgID, projectID)
	return !all, err
}
