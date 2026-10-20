package boot

import (
	"context"

	"github.com/google/uuid"

	"altalune.id/othra/config"
	"altalune.id/othra/internal/kernel"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/keys"
	"altalune.id/othra/internal/org"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/sealer"
)

const defaultRSABits = 2048

func keysConfig(c config.KeysConfig) keys.Config {
	bits := c.RSABits
	if bits == 0 {
		bits = defaultRSABits
	}
	return keys.Config{RSABits: bits, PrePublish: c.Rotation.PrePublish, Overlap: c.Rotation.Overlap}
}

func buildKeys(k kernel.Kernel, cfg config.Config, uow db.Transactor, sl sealer.Sealer) *keys.Service {
	store := keys.NewStore(k.Pool, cfg.Postgres.Schema, cfg.Postgres.TablePrefix)
	return keys.NewService(store, uow, sl, keysConfig(cfg.Keys), k.Reporter.Unexpected).WithLogger(orDiscard(k.Log))
}

type keyProjects struct {
	orgs     *org.Service
	projects *project.Service
}

func (p keyProjects) Each(ctx context.Context, fn func(ctx context.Context, orgID, projectID uuid.UUID) error) error {
	orgIDs, err := p.orgs.IDs(ctx)
	if err != nil {
		return err
	}
	for _, orgID := range orgIDs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		scoped := tenant.Into(ctx, tenant.Context{OrgID: orgID})
		list, err := p.projects.List(scoped, orgID)
		if err != nil {
			return err
		}
		for _, pr := range list {
			if err := fn(tenant.Into(ctx, tenant.Context{OrgID: orgID, ProjectID: pr.ID}), orgID, pr.ID); err != nil {
				return err
			}
		}
	}
	return nil
}
