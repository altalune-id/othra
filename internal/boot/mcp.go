package boot

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/altalune-id/oidcrp"
	"github.com/google/uuid"

	"github.com/altalune-id/othra/config"
	authv1mcp "github.com/altalune-id/othra/gen/go/auth/v1/authv1mcp"
	clientv1mcp "github.com/altalune-id/othra/gen/go/client/v1/clientv1mcp"
	enduserv1mcp "github.com/altalune-id/othra/gen/go/enduser/v1/enduserv1mcp"
	invitev1mcp "github.com/altalune-id/othra/gen/go/invite/v1/invitev1mcp"
	keysv1mcp "github.com/altalune-id/othra/gen/go/keys/v1/keysv1mcp"
	memberv1mcp "github.com/altalune-id/othra/gen/go/member/v1/memberv1mcp"
	orgv1mcp "github.com/altalune-id/othra/gen/go/org/v1/orgv1mcp"
	projectv1mcp "github.com/altalune-id/othra/gen/go/project/v1/projectv1mcp"
	resourceserverv1mcp "github.com/altalune-id/othra/gen/go/resourceserver/v1/resourceserverv1mcp"
	upstreamv1mcp "github.com/altalune-id/othra/gen/go/upstream/v1/upstreamv1mcp"
	webhookv1mcp "github.com/altalune-id/othra/gen/go/webhook/v1/webhookv1mcp"
	"github.com/altalune-id/othra/internal/auth"
	"github.com/altalune-id/othra/internal/client"
	"github.com/altalune-id/othra/internal/controlplane"
	"github.com/altalune-id/othra/internal/kernel/tokens"
	"github.com/altalune-id/othra/internal/keys"
	mcpinternal "github.com/altalune-id/othra/internal/mcp"
	mcpui "github.com/altalune-id/othra/internal/mcp/ui"
	"github.com/altalune-id/othra/internal/oauth/oidc"
	"github.com/altalune-id/othra/internal/org"
	"github.com/altalune-id/othra/internal/org/invite"
	"github.com/altalune-id/othra/internal/org/member"
	"github.com/altalune-id/othra/internal/permission"
	"github.com/altalune-id/othra/internal/project"
	"github.com/altalune-id/othra/internal/resourceserver"
	"github.com/altalune-id/othra/internal/upstream"
	"github.com/altalune-id/othra/internal/webhook"
	rootmcp "github.com/altalune-id/othra/mcp"
	"github.com/altalune-id/othra/scope"
	"github.com/altalune-id/othra/version"
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
	if slices := accountSlices(svcs); slices.Complete() {
		admin := controlplane.NewEndUserService(svcs.EndUsers, svcs.Federation, slices, refs)
		enduserv1mcp.RegisterEndUserServiceTools(registry, admin, permission.For, validate)
	}
	authSvc := auth.NewControlPlane(svcs.Orgs, svcs.Projects, svcs.Users, svcs.Auth).
		WithFileUploads(svcs.Caps.FileUploads).WithPasswords(svcs.Caps.Passwords)
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

const mcpResourceName = "Othra MCP"

// NOTE: Othra is its own issuer: MCP clients sign in at /p/system, which needs MCP on and an issuer; basic mode has no console project.
func mcpOAuth(cfg config.Config, svcs *Services) bool {
	return cfg.MCP.Enabled && issuesTokens(svcs) && !svcs.Caps.BasicAuth
}

func mcpIssuer(svcs *Services) string { return svcs.Projects.IssuerURL(project.SystemHandle) }

func mcpAudience(cfg config.Config) string {
	if cfg.MCP.Audience != "" {
		return cfg.MCP.Audience
	}
	return publicBase(cfg.HTTP) + "/mcp"
}

// SECURITY: only catalog scopes a token may hold are offered, so a client never asks consent for one it can't use.
func mcpScopes() []string {
	out := []string{}
	for _, s := range scope.Names() {
		if scope.Holds(s, scope.HolderPerson) {
			out = append(out, s)
		}
	}
	return out
}

// NOTE: the one source of Othra's own resource server; install seeds it and every boot reconciles it.
func mcpSystemResource(cfg config.Config) (resourceserver.SystemResource, error) {
	if err := resourceserver.ValidateIdentifier(mcpAudience(cfg)); err != nil {
		return resourceserver.SystemResource{}, &config.InvalidSettingError{Key: "mcp.audience", Reason: err.Error()}
	}
	if err := resourceserver.ValidateClientHosts(cfg.MCP.ClientHosts); err != nil {
		return resourceserver.SystemResource{}, &config.InvalidSettingError{Key: "mcp.clientHosts", Reason: err.Error()}
	}
	return resourceserver.SystemResource{
		Name: mcpResourceName, Identifier: mcpAudience(cfg), Scopes: mcpScopes(), ClientHosts: cfg.MCP.ClientHosts,
	}, nil
}

func seedMCPResource(cfg config.Config, s *Services) func(ctx context.Context, orgID, projectID uuid.UUID) error {
	return func(ctx context.Context, orgID, projectID uuid.UUID) error {
		if !mcpOAuth(cfg, s) {
			return nil
		}
		want, err := mcpSystemResource(cfg)
		if err != nil {
			return err
		}
		_, err = s.ResourceServers.ReconcileSystem(ctx, orgID, projectID, want)
		return err
	}
}

func reconcileMCPResource(ctx context.Context, cfg config.Config, s *Services, log *slog.Logger) error {
	if !mcpOAuth(cfg, s) {
		return closeMCPResource(ctx, s, log)
	}
	want, err := mcpSystemResource(cfg)
	if err != nil {
		return err
	}
	sys, p, err := consoleProject(ctx, s)
	if err != nil || p == nil {
		return wrapBoot("console project", err)
	}
	changed, err := s.ResourceServers.ReconcileSystem(ctx, sys.ID, p.ID, want)
	if err != nil {
		return wrapBoot("reconcile the mcp resource server", err)
	}
	if changed {
		log.Info("boot: the mcp resource server now follows config", slog.String("identifier", want.Identifier))
	}
	return nil
}

// SECURITY: with MCP OAuth off, /p/system must not keep admitting or advertising clients for a resource nobody serves.
func closeMCPResource(ctx context.Context, s *Services, log *slog.Logger) error {
	if s.ResourceServers == nil {
		return nil
	}
	sys, p, err := consoleProject(ctx, s)
	if err != nil || p == nil {
		return wrapBoot("console project", err)
	}
	changed, err := s.ResourceServers.CloseSystem(ctx, sys.ID, p.ID)
	if err != nil {
		return wrapBoot("close the mcp resource server", err)
	}
	if changed {
		log.Info("boot: mcp is off, so the mcp resource server admits no more clients")
	}
	return nil
}

// SECURITY: RFC 8707: the verifier is pinned to the MCP audience, so a token minted for another resource is refused; nil without MCP OAuth.
func buildMCPTokens(cfg config.Config, svcs *Services, issuer http.Handler) (tokens.Verifier, error) {
	if !mcpOAuth(cfg, svcs) {
		return nil, nil
	}
	return newMCPVerifier(cfg, mcpIssuer(svcs), &http.Client{Transport: oidc.InProcess(issuer), Timeout: consoleRPTimeout})
}

func newMCPVerifier(cfg config.Config, issuer string, hc *http.Client) (tokens.Verifier, error) {
	tc := tokens.Config{Issuer: issuer, Audience: mcpAudience(cfg), ClockSkew: cfg.MCP.ClockSkew}
	return tokens.NewVerifier(tc, oidcrp.NewKeySource(issuer, hc))
}

// NOTE: zero without MCP OAuth; the RFC 9728 document is mounted at the root, outside http.basePath, as RFC 9728 Section 3.1 places it.
func buildMCPResource(cfg config.Config, svcs *Services, log *slog.Logger) (tokens.Resource, map[string]http.Handler, error) {
	if !mcpOAuth(cfg, svcs) {
		return tokens.Resource{}, nil, nil
	}
	return newMCPResource(cfg, mcpIssuer(svcs), log)
}

func newMCPResource(cfg config.Config, issuer string, log *slog.Logger) (tokens.Resource, map[string]http.Handler, error) {
	r, err := tokens.NewResource(mcpAudience(cfg))
	if err != nil {
		return tokens.Resource{}, nil, fmt.Errorf("boot: mcp resource: %w", err)
	}
	log.Info("boot: mcp oauth on", "audience", r.Identifier, "metadata", r.MetadataPath, "issuer", issuer,
		"client_hosts", len(cfg.MCP.ClientHosts))
	return r, map[string]http.Handler{r.MetadataPath: r.MetadataHandler(issuer, mcpScopes())}, nil
}

func toolScopes(reg *rootmcp.Registry) func(string) []string {
	return func(name string) []string {
		spec, ok := reg.Spec(name)
		if !ok {
			return nil
		}
		return spec.Scopes
	}
}
