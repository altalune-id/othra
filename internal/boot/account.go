package boot

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/client"
	"altalune.id/othra/internal/consent"
	"altalune.id/othra/internal/controlplane"
	"altalune.id/othra/internal/enduser"
	"altalune.id/othra/internal/federation"
	"altalune.id/othra/internal/grant"
	"altalune.id/othra/internal/oauth/hosted"
	"altalune.id/othra/internal/pat"
	"altalune.id/othra/internal/resourceserver"
	"altalune.id/othra/internal/web"
	"altalune.id/othra/internal/web/layout"
)

// NOTE: account.v1 is one Connect service made of the domain slices; nil until every slice's service is built.
func accountSlices(svcs *Services) controlplane.AccountSlices {
	if svcs.EndUsers == nil || svcs.Federation == nil || svcs.Consents == nil || svcs.PersonalTokens == nil {
		return controlplane.AccountSlices{}
	}
	return controlplane.AccountSlices{
		Profile: enduser.NewAccountService(svcs.EndUsers),
		Methods: federation.NewAccountService(svcs.Federation),
		Apps:    consent.NewAccountService(svcs.Consents, consentDirectory{clients: svcs.Clients, servers: svcs.ResourceServers}),
		Tokens:  pat.NewAccountService(svcs.PersonalTokens),
	}
}

type accountRoutes struct {
	pattern string
	handler http.Handler
}

func (a accountRoutes) Register(mux *http.ServeMux) { mux.Handle(a.pattern, a.handler) }

func buildAccountPlane(basePath string, svcs *Services, resolver *hosted.Web, unexpected apperror.UnexpectedFunc) web.Registrar {
	slices := accountSlices(svcs)
	if !slices.Complete() {
		return nil
	}
	plane := controlplane.NewAccountPlane(slices, resolver, svcs.Validator, layout.Limits{Limiter: svcs.AuthLimiter}, unexpected)
	return accountRoutes{pattern: controlplane.Pattern(basePath), handler: plane.Handler(basePath)}
}

type endUserGrants struct{ grants *grant.Service }

// NOTE: the end reasons map onto the stored grant reasons here, so grant keeps the only list of them.
func (g endUserGrants) RevokeUserSession(ctx context.Context, p enduser.Project, userSession string, reason enduser.EndReason) error {
	why := grant.ReasonSignOut
	if reason == enduser.EndLogout {
		why = grant.ReasonEndSession
	}
	return g.grants.RevokeUserSession(ctx, grant.Project{OrgID: p.OrgID, ID: p.ID, IssuerURL: p.IssuerURL}, userSession, why)
}

type consentGrants struct{ grants *grant.Service }

func (g consentGrants) LiveFamilies(ctx context.Context, p consent.Project, endUserID uuid.UUID) ([]consent.Family, error) {
	fs, err := g.grants.LiveFamilies(ctx, grant.Project{OrgID: p.OrgID, ID: p.ID}, endUserID)
	if err != nil {
		return nil, err
	}
	out := make([]consent.Family, 0, len(fs))
	for _, f := range fs {
		out = append(out, consent.Family{
			ClientID: f.ClientID, ResourceServerID: f.ResourceServerID, Scopes: f.Scopes, CreatedAt: f.CreatedAt, LastUsedAt: f.LastUsedAt,
		})
	}
	return out, nil
}

func (consentGrants) ResourceScopes(scopes []string) []string {
	return grant.ResourceScopes(scopes)
}

func (g consentGrants) RevokeClient(ctx context.Context, p consent.Project, endUserID, clientID uuid.UUID) (bool, error) {
	return g.grants.RevokeClient(ctx, grant.Project{OrgID: p.OrgID, ID: p.ID}, endUserID, clientID)
}

type consentDirectory struct {
	clients *client.Service
	servers *resourceserver.Service
}

func (d consentDirectory) ClientID(ctx context.Context, p consent.Project, oauthID string) (uuid.UUID, bool, error) {
	c, err := d.clients.Get(ctx, p.OrgID, p.ID, oauthID)
	switch {
	case client.IsNotFoundError(err):
		return uuid.Nil, false, nil
	case err != nil:
		return uuid.Nil, false, err
	}
	return c.ID, true, nil
}

func (d consentDirectory) Clients(ctx context.Context, p consent.Project, ids []uuid.UUID) (map[uuid.UUID]consent.ClientInfo, error) {
	cs, err := d.clients.ByIDs(ctx, p.OrgID, p.ID, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]consent.ClientInfo, len(cs))
	for _, c := range cs {
		info := consent.ClientInfo{OAuthID: c.OAuthID(), Name: c.Name}
		if c.Managed() {
			info.MetadataHost = hostOf(c.MetadataURL)
		}
		out[c.ID] = info
	}
	return out, nil
}

func (d consentDirectory) Servers(ctx context.Context, p consent.Project) (map[uuid.UUID]consent.ServerInfo, error) {
	rs, err := d.servers.List(ctx, p.OrgID, p.ID)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]consent.ServerInfo, len(rs))
	for _, r := range rs {
		out[r.ID] = consent.ServerInfo{PublicID: r.PublicID, Name: r.Name, Identifier: r.Identifier, Descriptions: r.Descriptions}
	}
	return out, nil
}
