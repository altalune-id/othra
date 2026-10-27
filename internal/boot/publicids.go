package boot

import (
	"context"

	"github.com/google/uuid"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/client"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/org"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/user"
	"altalune.id/othra/internal/webhook"
)

type publicIDs struct {
	orgs     *org.Service
	projects *project.Service
	users    *user.Service
	webhooks webhook.Store
	console  consolePages
}

func newPublicIDs(s *Services) publicIDs {
	return publicIDs{
		orgs:     s.Orgs,
		projects: s.Projects,
		users:    s.Users,
		webhooks: s.WebhookStore,
		console:  s.ConsolePages,
	}
}

func (p publicIDs) ClientProject(ctx context.Context, publicID string) (client.ProjectRef, bool, error) {
	r, err := p.projects.RefByPublicID(ctx, publicID)
	if err != nil {
		return client.ProjectRef{}, false, notFoundIsAbsent(err)
	}
	return client.ProjectRef{
		ID: r.ID, OrgID: r.OrgID,
		ConsoleURL: func(clientPublicID string) string { return p.console.client(r.OrgSlug, r.Slug, clientPublicID) },
	}, true, nil
}

func (p publicIDs) OrgID(ctx context.Context, publicID string) (uuid.UUID, bool, error) {
	o, err := p.orgs.ByPublicID(ctx, publicID)
	if err != nil {
		return uuid.Nil, false, notFoundIsAbsent(err)
	}
	return o.ID, true, nil
}

// IDByPublicID serves invite.OrgIDs; an unknown public id is an *org.NotFoundError.
func (p publicIDs) IDByPublicID(ctx context.Context, publicID string) (uuid.UUID, error) {
	o, err := p.orgs.ByPublicID(ctx, publicID)
	if err != nil {
		return uuid.Nil, err
	}
	return o.ID, nil
}

func (p publicIDs) OrgPublicID(ctx context.Context, id uuid.UUID) (string, error) {
	o, err := p.orgs.Get(ctx, id)
	if err != nil {
		return "", err
	}
	return o.PublicID, nil
}

func (p publicIDs) ProjectID(ctx context.Context, publicID string) (id, orgID uuid.UUID, ok bool, err error) {
	pr, err := p.projects.ByPublicID(ctx, publicID)
	if err != nil {
		return uuid.Nil, uuid.Nil, false, notFoundIsAbsent(err)
	}
	return pr.ID, pr.OrgID, true, nil
}

func (p publicIDs) ProjectPublicID(ctx context.Context, id uuid.UUID) (string, error) {
	pr, err := p.projects.Get(ctx, id)
	if err != nil {
		return "", err
	}
	return pr.PublicID, nil
}

func (p publicIDs) UserID(ctx context.Context, publicID string) (uuid.UUID, bool, error) {
	u, err := p.users.ByPublicID(ctx, publicID)
	if err != nil {
		return uuid.Nil, false, notFoundIsAbsent(err)
	}
	return u.ID, true, nil
}

func (p publicIDs) UserPublicID(ctx context.Context, id uuid.UUID) (string, error) {
	u, err := p.users.ByID(ctx, id)
	if err != nil {
		return "", err
	}
	return u.PublicID, nil
}

// ResourceID resolves a webhook endpoint's public id within orgID.
func (p publicIDs) ResourceID(ctx context.Context, orgID uuid.UUID, publicID string) (uuid.UUID, bool, error) {
	e, err := p.webhooks.ByPublicID(tenant.Into(ctx, tenant.Context{OrgID: orgID}), publicID)
	if err != nil {
		return uuid.Nil, false, notFoundIsAbsent(err)
	}
	return e.ID, true, nil
}

func (p publicIDs) ResourcePublicID(ctx context.Context, orgID, id uuid.UUID) (publicID string, ok bool, err error) {
	e, err := p.webhooks.ByID(tenant.Into(ctx, tenant.Context{OrgID: orgID}), id)
	if err != nil {
		return "", false, notFoundIsAbsent(err)
	}
	return e.PublicID, true, nil
}

func notFoundIsAbsent(err error) error {
	if isNotFound(err) {
		return nil
	}
	return err
}

func isNotFound(err error) bool {
	ae, ok := apperror.AsAppError(err)
	return ok && apperror.KindOf(ae.Code()) == apperror.KindNotFound
}
