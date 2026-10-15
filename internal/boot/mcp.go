package boot

import (
	"log/slog"
	"net/http"

	"altalune.id/othra/config"
	agentv1mcp "altalune.id/othra/gen/go/agent/v1/agentv1mcp"
	authv1mcp "altalune.id/othra/gen/go/auth/v1/authv1mcp"
	invitev1mcp "altalune.id/othra/gen/go/invite/v1/invitev1mcp"
	memberv1mcp "altalune.id/othra/gen/go/member/v1/memberv1mcp"
	orgv1mcp "altalune.id/othra/gen/go/org/v1/orgv1mcp"
	projectv1mcp "altalune.id/othra/gen/go/project/v1/projectv1mcp"
	webhookv1mcp "altalune.id/othra/gen/go/webhook/v1/webhookv1mcp"
	"altalune.id/othra/internal/agent"
	"altalune.id/othra/internal/auth"
	mcpinternal "altalune.id/othra/internal/mcp"
	mcpui "altalune.id/othra/internal/mcp/ui"
	"altalune.id/othra/internal/org"
	"altalune.id/othra/internal/org/invite"
	"altalune.id/othra/internal/org/member"
	"altalune.id/othra/internal/permission"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/webhook"
	rootmcp "altalune.id/othra/mcp"
	"altalune.id/othra/version"
)

// NOTE: nil when mcp.enabled is off.
func buildMCPServer(cfg config.MCPConfig, svcs *Services, log *slog.Logger) *rootmcp.Server {
	if !cfg.Enabled {
		return nil
	}
	registry := rootmcp.NewRegistry()
	refs := newPublicIDs(svcs)
	validate := svcs.Validator.Validate
	orgv1mcp.RegisterOrgServiceTools(registry, org.NewControlPlane(svcs.Orgs), permission.For, validate)
	projectv1mcp.RegisterProjectServiceTools(registry, project.NewControlPlane(svcs.Projects, refs), permission.For, validate)
	agentv1mcp.RegisterAgentServiceTools(registry, agent.NewControlPlane(svcs.Agents, refs), permission.For, validate)
	webhookv1mcp.RegisterWebhookServiceTools(registry, webhook.NewControlPlane(svcs.Webhooks, refs), permission.For, validate)
	memberv1mcp.RegisterMemberServiceTools(registry, member.NewControlPlane(svcs.Members, refs), permission.For, validate)
	invitev1mcp.RegisterInviteServiceTools(registry, invite.NewControlPlane(svcs.Invites, refs), permission.For, validate)
	authv1mcp.RegisterAuthServiceTools(registry, auth.NewControlPlane(svcs.Orgs, svcs.Projects, svcs.Users), permission.For, validate)

	srv := rootmcp.NewServer(
		rootmcp.WithImplementation("othra", version.Default()),
		rootmcp.WithRegistry(registry),
		rootmcp.WithScopes(mcpinternal.ScopesFromContext),
		rootmcp.WithErrorMapper(mcpinternal.ErrorPayload),
		rootmcp.WithLogger(log),
		rootmcp.WithUI(cfg.AppsUI),
	)
	if cfg.AppsUI {
		srv.AddUIResource(rootmcp.UIResource{
			URI:           mcpui.ResourceURI,
			Name:          "othra app",
			MIMEType:      rootmcp.MIMEApp,
			Body:          mcpui.Document(),
			PrefersBorder: new(bool),
		})
	}
	return srv
}

func mcpHTTPHandler(srv *rootmcp.Server) http.Handler {
	if srv == nil {
		return nil
	}
	return srv.Handler()
}
