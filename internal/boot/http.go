package boot

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/config"
	"github.com/altalune-id/othra/httpserver"
	"github.com/altalune-id/othra/internal/auth"
	"github.com/altalune-id/othra/internal/controlplane"
	"github.com/altalune-id/othra/internal/install"
	"github.com/altalune-id/othra/internal/kernel/authn"
	"github.com/altalune-id/othra/internal/kernel/session"
	"github.com/altalune-id/othra/internal/keys"
	"github.com/altalune-id/othra/internal/legal"
	mcpinternal "github.com/altalune-id/othra/internal/mcp"
	"github.com/altalune-id/othra/internal/oauth/oidc"
	"github.com/altalune-id/othra/internal/org/invite"
	"github.com/altalune-id/othra/internal/permission"
	"github.com/altalune-id/othra/internal/web"
	"github.com/altalune-id/othra/internal/web/layout"
	"github.com/altalune-id/othra/internal/web/middleware"
	rootmcp "github.com/altalune-id/othra/mcp"
)

// SECURITY: the lockdown switch; /api, /mcp and the console refuse a request without a credential. Revert its commit to undo.
const requireCredentials = true

func httpConfig(c config.HTTPConfig) httpserver.Config {
	return httpserver.Config{Name: "http", Addr: c.Addr}
}

func buildAPIHandler(
	svcs *Services,
	unexpected apperror.UnexpectedFunc,
	cfg config.APIConfig,
	basePath string,
	log *slog.Logger,
) http.Handler {
	if svcs == nil || svcs.Orgs == nil {
		return nil
	}
	opts := controlplane.Options{
		OpenAPIEnabled: cfg.OpenAPI.Enabled,
		Authn: authn.InterceptorOptions{
			Chain:        apiChain(svcs),
			KeyPrefix:    svcs.KeyScheme.Prefix(),
			CookieSecret: svcs.StateSecret,
			Permissions:  permission.Table(),
			Required:     requireCredentials,
			Logger:       log,
		},
		Validator: svcs.Validator,
	}
	if cfg.OpenAPI.RequireBasicAuth {
		opts.OpenAPIBasicAuth = &controlplane.BasicAuth{User: cfg.OpenAPI.BasicAuthUser, Password: cfg.OpenAPI.BasicAuthPassword}
	}
	h := controlplane.New(controlplane.Services{
		Orgs:     svcs.Orgs,
		Projects: svcs.Projects,
		APIKeys:  svcs.APIKeys,
		Webhooks: svcs.Webhooks,
		Users:    svcs.Users,
		Members:  svcs.Members,
		Invites:  svcs.Invites,
		Auth:     svcs.Auth,
		Keys:     svcs.Keys,
		Clients:  svcs.Clients,

		ResourceServers: svcs.ResourceServers,
		Upstreams:       svcs.Upstreams,
		EndUsers:        svcs.EndUsers,
		Federation:      svcs.Federation,
		Account:         accountSlices(svcs),
		Refs:            newPublicIDs(svcs),
		FileUploads:     svcs.Caps.FileUploads,
		Passwords:       svcs.Caps.Passwords,
	}, unexpected, opts).Handler(basePath)
	if requireCredentials && svcs.Caps.BasicAuth {
		h = auth.BasicChallenge(h)
	}
	return h
}

// NOTE: nil before the lockdown; local mode sends a signed-out caller to /login, basic mode challenges.
func buildConsoleGate(cfg config.Config, svcs *Services) func(http.Handler) http.Handler {
	if !requireCredentials || svcs == nil {
		return nil
	}
	if svcs.Caps.BasicAuth {
		return auth.ConsoleGate(cfg.HTTP.BasePath, svcs.Auth)
	}
	return auth.ConsoleGate(cfg.HTTP.BasePath, nil)
}

func apiChain(svcs *Services) authn.Chain {
	chain := authn.Chain{svcs.KeyAuthn}
	if svcs.Caps.LocalLogin {
		chain = append(chain, auth.SessionAuthenticator{Sessions: svcs.Sessions, Auth: svcs.Auth})
	}
	if svcs.Caps.BasicAuth {
		chain = append(chain, auth.BasicAuthenticator{Auth: svcs.Auth})
	}
	return chain
}

func buildMCPHandler(srv *rootmcp.Server, svcs *Services, log *slog.Logger) http.Handler {
	h := mcpHTTPHandler(srv)
	if h == nil {
		return nil
	}
	return mcpinternal.Authenticate(mcpinternal.AuthOptions{
		Chain:     mcpChain(svcs),
		KeyPrefix: svcs.KeyScheme.Prefix(),
		Required:  requireCredentials,
		Logger:    log,
	})(h)
}

// SECURITY: no session authenticator — /mcp has no CSRF guard, so a browser cookie must never act on it.
func mcpChain(svcs *Services) authn.Chain {
	chain := authn.Chain{svcs.KeyAuthn}
	if svcs.Caps.BasicAuth {
		chain = append(chain, auth.BasicAuthenticator{Auth: svcs.Auth})
	}
	return chain
}

// SECURITY: behind a proxy the Host header is not the public origin, so the configured baseURL origin is trusted explicitly.
func buildCrossOrigin(baseURL string, crossSite []string) (*http.CrossOriginProtection, error) {
	cop := http.NewCrossOriginProtection()
	for _, pattern := range crossSite {
		cop.AddInsecureBypassPattern(pattern)
	}
	if baseURL == "" {
		return cop, nil
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("boot: http.baseURL: %w", err)
	}
	if err := cop.AddTrustedOrigin(u.Scheme + "://" + u.Host); err != nil {
		return nil, fmt.Errorf("boot: http.baseURL origin: %w", err)
	}
	return cop, nil
}

func warnSampleLegal(cfg config.Config, log *slog.Logger, serving bool) {
	if log == nil || !serving || cfg.Env == config.EnvDevelopment {
		return
	}
	var missing []string
	if cfg.Compliance.TermsURL == "" {
		missing = append(missing, "compliance.termsURL")
	}
	if cfg.Compliance.PrivacyURL == "" {
		missing = append(missing, "compliance.privacyURL")
	}
	if len(missing) == 0 {
		return
	}
	log.Warn("boot: serving the built-in sample terms/privacy; set compliance.termsURL and compliance.privacyURL",
		slog.String("missing", strings.Join(missing, " ")))
}

func buildIssuerRoutes(
	cfg config.Config, svcs *Services, inProcess *http.ServeMux, log *slog.Logger, unexpected apperror.UnexpectedFunc,
) (public, hosted []web.Registrar) {
	if svcs == nil || svcs.IssuerResolver == nil {
		return nil, nil
	}
	kit := layout.Kit{BasePath: cfg.HTTP.BasePath, Log: log, Unexpected: unexpected}
	resolver := svcs.IssuerResolver.WithLimiter(svcs.AuthLimiter, cfg.HTTP.TrustedProxyHops)
	issuers := oidc.NewWeb(resolver, kit, oidc.Config{
		PublicBase: publicBase(cfg.HTTP),
		BasePath:   cfg.HTTP.BasePath,
		CORS:       middleware.PublicCORS,
	})
	if issuesTokens(svcs) {
		issuers.WithTokens(oidc.Tokens{
			Clients: svcs.Clients, Grants: svcs.Grants, EndUsers: svcs.EndUsers, IDTokens: keys.NewSigner(svcs.Keys),
			IDTokenTTL: cfg.OAuth.IDTokenTTL, CORS: middleware.BearerCORS,
			Limits: authLimits(cfg, svcs),
		})
		if in := buildIntrospection(svcs); in != nil {
			issuers.WithIntrospection(*in)
			resolver.WithIntrospection()
		}
		issuers.Register(inProcess)
	}
	svcs.Keys.WithOnChange(resolver.Forget)
	svcs.ResourceServers.WithOnAdmissionChange(resolver.Forget)
	svcs.Projects.WithOnIssuerChange(resolver.Forget)
	if svcs.EndUsers == nil {
		return []web.Registrar{issuers}, nil
	}
	pages := buildHosted(cfg, svcs, kit, resolver)
	if plane := buildAccountPlane(cfg, svcs, pages, unexpected); plane != nil {
		return []web.Registrar{issuers}, []web.Registrar{pages, plane}
	}
	return []web.Registrar{issuers}, []web.Registrar{pages}
}

func issuesTokens(svcs *Services) bool {
	return svcs != nil && svcs.IssuerResolver != nil && svcs.Grants != nil && svcs.Clients != nil && svcs.EndUsers != nil && svcs.Keys != nil
}

func inviteMethods(cfg config.Config, rp auth.ConsoleRP) []string {
	var methods []string
	if cfg.PasswordsOn() {
		methods = append(methods, invite.MethodPassword)
	}
	if rp != nil {
		methods = append(methods, invite.MethodOthra)
	}
	return methods
}

func consoleWeb(
	cfg config.Config, svcs *Services, rp auth.ConsoleRP, kit layout.Kit, sessions auth.CookieSessions, limits layout.Limits,
) *auth.Web {
	w := auth.NewWeb(kit, svcs.Auth, sessions, newPublicIDs(svcs)).WithLimits(limits)
	if cfg.PasswordsOn() {
		w.WithResets(svcs.Resets)
	}
	if rp != nil {
		w.WithConsoleSignIn(auth.ConsoleSignIn{
			RP: rp, SignedOut: consoleURIs(cfg.HTTP).PostLogoutURI, PasswordOff: !cfg.PasswordsOn(),
			OpenSignup: cfg.OpenSignup(),
		})
	}
	return w
}

func buildRoutes(
	cfg config.Config, svcs *Services, rp auth.ConsoleRP, log *slog.Logger, unexpected apperror.UnexpectedFunc,
) []web.Registrar {
	kit := layout.Kit{BasePath: cfg.HTTP.BasePath, Log: log, Unexpected: unexpected}
	legalOpts := legal.WebOpts{Kit: kit, TermsURL: cfg.Compliance.TermsURL, PrivacyURL: cfg.Compliance.PrivacyURL}
	if svcs == nil || !svcs.Caps.LocalLogin {
		return []web.Registrar{legal.NewWeb(legalOpts)}
	}
	sessions := cookieSessions(cfg, svcs)
	limits := authLimits(cfg, svcs)
	legalOpts.Service = svcs.Legal
	legalOpts.Sessions = sessions
	legalOpts.RequireAcceptance = cfg.Compliance.RequireAcceptance
	return []web.Registrar{
		legal.NewWeb(legalOpts),
		consoleWeb(cfg, svcs, rp, kit, sessions, limits),
		install.NewWeb(install.WebOpts{
			Kit:       kit,
			Installer: svcs.Install,
			SignIn:    svcs.Auth,
			Sessions:  sessions,
			Token:     svcs.InstallToken,
			Required:  svcs.InstallRequired,
			Secret:    svcs.StateSecret,
			Secure:    cfg.HTTP.CookieSecure,
			Consent:   consentData(cfg),
		}),
		invite.NewWeb(invite.WebOpts{
			Kit:          kit,
			Invites:      svcs.Invites,
			Users:        svcs.Users,
			Members:      svcs.Members,
			Orgs:         memberOrgs{orgs: svcs.Orgs},
			OrgLinks:     memberOrgs{orgs: svcs.Orgs},
			People:       memberPeople{users: svcs.Users},
			SignIn:       inviteSignIn{svcs.Auth},
			Sessions:     sessions,
			Consent:      consentData(cfg),
			Methods:      inviteMethods(cfg, rp),
			StateSecret:  svcs.StateSecret,
			CookieSecure: cfg.HTTP.CookieSecure,
			Limits:       limits,
		}),
	}
}

func authLimits(cfg config.Config, svcs *Services) layout.Limits {
	return layout.Limits{Limiter: svcs.AuthLimiter, TrustedProxyHops: cfg.HTTP.TrustedProxyHops}
}

func consentData(cfg config.Config) layout.ConsentData {
	return layout.ConsentData{
		TermsURL:   cfg.Compliance.TermsURL,
		PrivacyURL: cfg.Compliance.PrivacyURL,
		Required:   cfg.Compliance.RequireAcceptance,
	}
}

func cookieSessions(cfg config.Config, svcs *Services) auth.CookieSessions {
	return auth.CookieSessions{
		Store:  svcs.Sessions,
		Secret: svcs.StateSecret,
		Cookie: session.CookieOptions{Path: layout.Path(cfg.HTTP.BasePath, ""), Secure: cfg.HTTP.CookieSecure},
		TTL:    cfg.Auth.SessionTTL,
	}
}

func buildSessionMiddleware(svcs *Services, unexpected apperror.UnexpectedFunc) func(http.Handler) http.Handler {
	if svcs == nil || !svcs.Caps.LocalLogin {
		return nil
	}
	return middleware.Session(middleware.SessionConfig{
		Store:      svcs.Sessions,
		Secret:     svcs.StateSecret,
		Auth:       svcs.Auth,
		Unexpected: unexpected,
	})
}

type httpSurfaces struct {
	log        *slog.Logger
	unexpected apperror.UnexpectedFunc
	probes     web.Registrar
	basePath   string
	api        http.Handler
	console    http.Handler
	portal     http.Handler
	mcp        http.Handler
	routes     []web.Registrar
	public     []web.Registrar
	hosted     []web.Registrar
	images     []string

	installRequired *atomic.Bool
	session         func(http.Handler) http.Handler
	consoleGate     func(http.Handler) http.Handler

	requireAcceptance bool
	termsUpdated      time.Time
	crossOrigin       *http.CrossOriginProtection
}

func buildHTTPHandler(s httpSurfaces) http.Handler {
	return web.NewServer(web.ServerOpts{
		APIHandler:   s.api,
		MCPHandler:   s.mcp,
		Console:      s.console,
		PortalAssets: s.portal,
		Routes:       s.routes,
		PublicRoutes: s.public,
		HostedPages:  s.hosted,

		HostedImageBases: s.images,
		BasePath:         s.basePath,
		Log:              s.log,
		Unexpected:       s.unexpected,
		Probes:           s.probes,

		Gate:        install.InstallGate(s.basePath, s.installRequired),
		Session:     s.session,
		SessionGate: legal.WelcomeGate(s.basePath, s.requireAcceptance, s.termsUpdated),
		ConsoleGate: s.consoleGate,
		CrossOrigin: s.crossOrigin,
	})
}

func hostedImageBases(s config.StorageConfig) []string {
	if s.Driver == "" {
		return nil
	}
	return []string{s.PublicBaseURL}
}
