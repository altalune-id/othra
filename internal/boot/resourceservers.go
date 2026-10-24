package boot

import (
	"cmp"
	"context"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/config"
	"altalune.id/othra/internal/client"
	"altalune.id/othra/internal/kernel"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/resourceserver"
	"altalune.id/othra/internal/user"
)

const (
	defaultVerifyInterval   = time.Hour
	defaultVerifyStaleAfter = 24 * time.Hour
	defaultVerifyTimeout    = 5 * time.Second
)

func resourceServersConfig(cfg config.Config) resourceserver.Config {
	v := cfg.ResourceServers.Verification
	return resourceserver.Config{
		Interval:       cmp.Or(v.Interval, defaultVerifyInterval),
		StaleAfter:     cmp.Or(v.StaleAfter, defaultVerifyStaleAfter),
		TrustAvailable: cfg.Mode == config.ModeSelfhosted,
	}
}

func buildResourceServers(
	k kernel.Kernel, cfg config.Config, uow db.Transactor, projects *project.Service, users *user.Service, clients *client.Service,
) *resourceserver.Service {
	v := cfg.ResourceServers.Verification
	return resourceserver.NewService(resourceserver.Deps{
		Store:      resourceserver.NewStore(k.Pool, cfg.Postgres.Schema, cfg.Postgres.TablePrefix),
		Transactor: uow,
		Issuers:    rsIssuers{projects: projects},
		People:     rsPeople{users: users},
		Clients:    rsClients{clients: clients},
		Checker:    resourceserver.NewVerifier(cmp.Or(v.Timeout, defaultVerifyTimeout), v.AllowPrivateHosts),
		Unexpected: k.Reporter.Unexpected,
	}, resourceServersConfig(cfg)).WithLogger(orDiscard(k.Log))
}

type rsIssuers struct{ projects *project.Service }

func (i rsIssuers) Project(ctx context.Context, projectID uuid.UUID) (resourceserver.ProjectRef, error) {
	p, err := i.projects.Get(ctx, projectID)
	if err != nil {
		return resourceserver.ProjectRef{}, err
	}
	return resourceserver.ProjectRef{PublicID: p.PublicID, Issuer: i.projects.IssuerURL(p.IssuerHandle)}, nil
}

type rsPeople struct{ users *user.Service }

func (p rsPeople) UserPublicID(ctx context.Context, userID uuid.UUID) (string, error) {
	u, err := p.users.ByID(ctx, userID)
	if err != nil {
		return "", err
	}
	return u.PublicID, nil
}

func (i rsIssuers) LockIssuerHandle(ctx context.Context, projectID uuid.UUID) error {
	return i.projects.LockIssuerHandle(ctx, projectID)
}

type rsClients struct{ clients *client.Service }

func (c rsClients) Lookup(ctx context.Context, orgID, projectID uuid.UUID, publicID string) (resourceserver.ClientRef, bool, error) {
	got, err := c.clients.Get(ctx, orgID, projectID, publicID)
	switch {
	case client.IsNotFoundError(err):
		return resourceserver.ClientRef{}, false, nil
	case err != nil:
		return resourceserver.ClientRef{}, false, err
	}
	return clientRef(got), true, nil
}

func clientRef(c *client.Client) resourceserver.ClientRef {
	return resourceserver.ClientRef{
		ID: c.ID, PublicID: c.PublicID, Name: c.Name, MetadataURL: c.MetadataURL, MetadataFetchedAt: c.MetadataFetchedAt,
	}
}

func (c rsClients) Refs(ctx context.Context, orgID, projectID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]resourceserver.ClientRef, error) {
	list, err := c.clients.ByIDs(ctx, orgID, projectID, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]resourceserver.ClientRef, len(list))
	for _, got := range list {
		out[got.ID] = clientRef(got)
	}
	return out, nil
}
