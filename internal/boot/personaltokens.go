package boot

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/altalune-id/othra/config"
	"github.com/altalune-id/othra/internal/client"
	"github.com/altalune-id/othra/internal/enduser"
	"github.com/altalune-id/othra/internal/kernel"
	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/internal/kernel/usage"
	"github.com/altalune-id/othra/internal/oauth/oidc"
	"github.com/altalune-id/othra/internal/pat"
	"github.com/altalune-id/othra/internal/resourceserver"
)

const tokenUsageInterval = 10 * time.Minute

func buildPersonalTokens(
	k kernel.Kernel, cfg config.Config, uow db.Transactor, servers *resourceserver.Service, endUsers *enduser.Service,
) (*pat.Service, *usage.Worker) {
	store := pat.NewStore(k.Pool, cfg.Postgres.Schema, cfg.Postgres.TablePrefix)
	recorder := usage.New("pat-usage", store, tokenUsageInterval, k.Log)
	svc := pat.NewService(store, uow, patServers{servers: servers}, patSubjects{endUsers: endUsers}, recorder, k.Reporter.Unexpected).
		WithLogger(orDiscard(k.Log))
	return svc, recorder
}

func buildIntrospection(svcs *Services) *oidc.Introspection {
	if svcs.ResourceServers == nil || svcs.PersonalTokens == nil || svcs.Clients == nil {
		return nil
	}
	return &oidc.Introspection{
		Callers:        introspectionCallers{servers: svcs.ResourceServers},
		PersonalTokens: introspectedPersonal{tokens: svcs.PersonalTokens},
		ClientIDs:      clientOAuthIDs{clients: svcs.Clients},
	}
}

type patServers struct{ servers *resourceserver.Service }

func (s patServers) toPatServer(r *resourceserver.ResourceServer, now time.Time) pat.Server {
	return pat.Server{
		ID: r.ID, PublicID: r.PublicID, Name: r.Name, Identifier: r.Identifier, Scopes: r.Scopes, Descriptions: r.Descriptions,
		Accepting: r.TakesPersonalTokens(now, s.servers.TrustAvailable()),
	}
}

func (s patServers) Accepting(ctx context.Context, p pat.Project) ([]pat.Server, error) {
	list, err := s.servers.PersonalTokenServers(ctx, p.OrgID, p.ID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := make([]pat.Server, 0, len(list))
	for _, r := range list {
		out = append(out, s.toPatServer(r, now))
	}
	return out, nil
}

func (s patServers) ByPublicID(ctx context.Context, p pat.Project, publicID string) (pat.Server, bool, error) {
	r, err := s.servers.Get(ctx, p.OrgID, p.ID, publicID)
	return s.found(r, err)
}

func (s patServers) ByID(ctx context.Context, p pat.Project, id uuid.UUID) (pat.Server, bool, error) {
	r, err := s.servers.ByID(ctx, p.OrgID, p.ID, id)
	return s.found(r, err)
}

func (s patServers) found(r *resourceserver.ResourceServer, err error) (pat.Server, bool, error) {
	switch {
	case resourceserver.IsNotFoundError(err):
		return pat.Server{}, false, nil
	case err != nil:
		return pat.Server{}, false, err
	}
	return s.toPatServer(r, time.Now()), true, nil
}

type patSubjects struct{ endUsers *enduser.Service }

func (s patSubjects) Subject(ctx context.Context, p pat.Project, endUserID uuid.UUID) (string, error) {
	u, err := s.endUsers.EndUser(ctx, enduser.Project{OrgID: p.OrgID, ID: p.ID}, endUserID)
	if err != nil || u == nil {
		return "", err
	}
	return u.Subject, nil
}

type introspectionCallers struct{ servers *resourceserver.Service }

func (c introspectionCallers) Authenticate(
	ctx context.Context, orgID, projectID uuid.UUID, publicID, presented string,
) (oidc.Introspector, bool, error) {
	r, ok, err := c.servers.Authenticate(ctx, orgID, projectID, publicID, presented)
	if err != nil || !ok {
		return oidc.Introspector{}, false, err
	}
	return oidc.Introspector{ID: r.ID, PublicID: r.PublicID, Identifier: r.Identifier}, true, nil
}

type introspectedPersonal struct{ tokens *pat.Service }

func (p introspectedPersonal) Introspect(
	ctx context.Context, orgID, projectID, callerID uuid.UUID, token string,
) (oidc.IntrospectedToken, bool, error) {
	in, ok, err := p.tokens.Introspect(ctx, pat.Project{OrgID: orgID, ID: projectID}, callerID, token)
	if err != nil || !ok {
		return oidc.IntrospectedToken{}, false, err
	}
	return oidc.IntrospectedToken{
		Scopes: in.Token.Scopes, Subject: in.Subject, Audience: in.Audience, IssuedAt: in.Token.CreatedAt, ExpiresAt: in.Token.ExpiresAt,
	}, true, nil
}

type clientOAuthIDs struct{ clients *client.Service }

func (c clientOAuthIDs) OAuthID(ctx context.Context, orgID, projectID, id uuid.UUID) (string, error) {
	list, err := c.clients.ByIDs(ctx, orgID, projectID, []uuid.UUID{id})
	if err != nil || len(list) == 0 {
		return "", err
	}
	return list[0].OAuthID(), nil
}
