package boot

import (
	"context"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/altalune-id/othra/config"
	"github.com/altalune-id/othra/internal/client"
	"github.com/altalune-id/othra/internal/consent"
	"github.com/altalune-id/othra/internal/kernel"
	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/internal/project"
	"github.com/altalune-id/othra/internal/resourceserver"
)

func buildConsents(k kernel.Kernel, cfg config.Config, uow db.Transactor, svcs *Services) *consent.Service {
	consents := consent.NewService(consent.NewStore(k.Pool, cfg.Postgres.Schema, cfg.Postgres.TablePrefix), uow,
		consentGrants{grants: svcs.Grants}, k.Reporter.Unexpected).WithLogger(orDiscard(k.Log))
	svcs.Clients.WithOnMetadataChange(func(ctx context.Context, orgID, projectID, clientID uuid.UUID) error {
		return consents.ForgetClient(ctx, consent.Project{OrgID: orgID, ID: projectID}, clientID)
	})
	return consents
}

func buildClients(k kernel.Kernel, cfg config.Config, uow db.Transactor, projects *project.Service) *client.Service {
	m := cfg.OAuth.CIMD
	meta := client.MetadataConfig{
		Cache:      client.MetadataCache{Min: m.CacheMin, Max: m.CacheMax, Default: defaultMetadataTTL(m)},
		IssuerHost: hostOf(publicBase(cfg.HTTP)), MaxBytes: m.MaxBytes,
	}
	return client.NewService(client.NewStore(k.Pool, cfg.Postgres.Schema, cfg.Postgres.TablePrefix), uow, projects,
		client.NewMetadataFetcher(m.FetchTimeout, m.AllowPrivateHosts), meta, k.Reporter.Unexpected).WithLogger(orDiscard(k.Log))
}

// NOTE: a document without a usable max-age is kept an hour, inside the configured bounds.
func defaultMetadataTTL(m config.OAuthCIMDConfig) time.Duration {
	return min(max(time.Hour, m.CacheMin), m.CacheMax)
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

type rsLinks struct{ servers *resourceserver.Service }

func (l rsLinks) LinkMetadataClient(ctx context.Context, orgID, projectID, clientID, serverID uuid.UUID, scopes []string) error {
	return l.servers.LinkMetadataClient(ctx, orgID, projectID, clientID, serverID, scopes)
}
