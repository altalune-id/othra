package boot

import (
	"context"

	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/org"
	"altalune.id/othra/internal/project"
)

type projectTenants struct {
	orgs     *org.Service
	projects *project.Service
}

func (p projectTenants) Each(ctx context.Context, fn func(ctx context.Context, orgID, projectID uuid.UUID) error) error {
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
