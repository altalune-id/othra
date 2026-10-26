package boot

import (
	"log/slog"
	"net/http"
	"net/url"

	"altalune.id/othra/config"
	authv1mcp "altalune.id/othra/gen/go/auth/v1/authv1mcp"
	clientv1mcp "altalune.id/othra/gen/go/client/v1/clientv1mcp"
	invitev1mcp "altalune.id/othra/gen/go/invite/v1/invitev1mcp"
	keysv1mcp "altalune.id/othra/gen/go/keys/v1/keysv1mcp"
	memberv1mcp "altalune.id/othra/gen/go/member/v1/memberv1mcp"
	orgv1mcp "altalune.id/othra/gen/go/org/v1/orgv1mcp"
	projectv1mcp "altalune.id/othra/gen/go/project/v1/projectv1mcp"
	resourceserverv1mcp "altalune.id/othra/gen/go/resourceserver/v1/resourceserverv1mcp"
	upstreamv1mcp "altalune.id/othra/gen/go/upstream/v1/upstreamv1mcp"
	webhookv1mcp "altalune.id/othra/gen/go/webhook/v1/webhookv1mcp"
	"altalune.id/othra/internal/auth"
	"altalune.id/othra/internal/client"
	"altalune.id/othra/internal/keys"
	mcpinternal "altalune.id/othra/internal/mcp"
	mcpui "altalune.id/othra/internal/mcp/ui"
	"altalune.id/othra/internal/org"
	"altalune.id/othra/internal/org/invite"
	"altalune.id/othra/internal/org/member"
	"altalune.id/othra/internal/permission"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/resourceserver"
	"altalune.id/othra/internal/upstream"
	"altalune.id/othra/internal/webhook"
	rootmcp "altalune.id/othra/mcp"
	"altalune.id/othra/version"
)

// NOTE: nil when mcp.enabled is off; allowedHosts may arrive on loopback connections through a same-host proxy.
func buildMCPServer(cfg config.MCPConfig, svcs *Services, log *slog.Logger, allowedHosts ...string) *rootmcp.Server {
	if !cfg.Enabled {
		return nil
	}
	registry := rootmcp.NewRegistry()
	refs := newPublicIDs(svcs)
	validate := svcs.Validator.Validate
	orgv1mcp.RegisterOrgServiceTools(registry, org.NewControlPlane(svcs.Orgs), permission.For, validate)
	projectv1mcp.RegisterProjectServiceTools(registry, project.NewControlPlane(svcs.Projects, refs), permission.For, validate)
	webhookv1mcp.RegisterWebhookServiceTools(registry, webhook.NewControlPlane(svcs.Webhooks, refs), permission.For, validate)
	memberv1mcp.RegisterMemberServiceTools(registry, member.NewControlPlane(svcs.Members, refs), permission.For, validate)
	invitev1mcp.RegisterInviteServiceTools(registry, invite.NewControlPlane(svcs.Invites, refs), permission.For, validate)
	keysv1mcp.RegisterSigningKeyServiceTools(registry, keys.NewControlPlane(svcs.Keys, refs), permission.For, validate)
	clientv1mcp.RegisterClientServiceTools(registry, client.NewControlPlane(svcs.Clients, refs), permission.For, validate)
	rsCP := resourceserver.NewControlPlane(svcs.ResourceServers, refs)
	resourceserverv1mcp.RegisterResourceServerServiceTools(registry, rsCP, permission.For, validate)
	if svcs.Upstreams != nil {
		upstreamv1mcp.RegisterUpstreamServiceTools(registry, upstream.NewControlPlane(svcs.Upstreams, refs), permission.For, validate)
	}
	authSvc := auth.NewControlPlane(svcs.Orgs, svcs.Projects, svcs.Users).WithFileUploads(svcs.Caps.FileUploads)
	authv1mcp.RegisterAuthServiceTools(registry, authSvc, permission.For, validate)

	srv := rootmcp.NewServer(
		rootmcp.WithImplementation("othra", version.Default()),
		rootmcp.WithRegistry(registry),
		rootmcp.WithScopes(mcpinternal.ScopesFromContext),
		rootmcp.WithErrorMapper(mcpinternal.ErrorPayload),
		rootmcp.WithLogger(log),
		rootmcp.WithUI(cfg.AppsUI),
		rootmcp.WithAllowedHosts(allowedHosts...),
	)
	if cfg.AppsUI {
		srv.AddUIResource(rootmcp.UIResource{
			URI:           mcpui.ResourceURI,
			Name:          "Othra app",
			MIMEType:      rootmcp.MIMEApp,
			Body:          mcpui.Document(),
			PrefersBorder: new(bool),
		})
	}
	return srv
}

func mcpPublicHost(c config.HTTPConfig) []string {
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Hostname() == "" {
		return nil
	}
	return []string{u.Hostname()}
}

func mcpHTTPHandler(srv *rootmcp.Server) http.Handler {
	if srv == nil {
		return nil
	}
	return srv.Handler()
}
