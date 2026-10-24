package boot

import (
	"context"
	"net/url"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/config"
	"altalune.id/othra/internal/client"
	"altalune.id/othra/internal/consent"
	"altalune.id/othra/internal/kernel"
	"altalune.id/othra/internal/resourceserver"
)

func buildConsents(k kernel.Kernel, cfg config.Config, svcs *Services) *consent.Service {
	consents := consent.NewService(consent.NewStore(k.Pool, cfg.Postgres.Schema, cfg.Postgres.TablePrefix), k.Reporter.Unexpected).
		WithLogger(orDiscard(k.Log))
	m := cfg.OAuth.CIMD
	svcs.Clients.WithMetadata(client.NewMetadataFetcher(m.FetchTimeout, m.AllowPrivateHosts), rsLinks{servers: svcs.ResourceServers},
		client.MetadataConfig{
			Cache:      client.MetadataCache{Min: m.CacheMin, Max: m.CacheMax, Default: defaultMetadataTTL(m)},
			IssuerHost: hostOf(publicBase(cfg.HTTP)), MaxBytes: m.MaxBytes,
		}).
		WithOnMetadataChange(func(ctx context.Context, orgID, projectID, clientID uuid.UUID) error {
			return consents.ForgetClient(ctx, consent.Project{OrgID: orgID, ID: projectID}, clientID)
		})
	return consents
}

// NOTE: ADR-0021 D2: a document without a usable max-age is kept an hour, inside the configured bounds.
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
