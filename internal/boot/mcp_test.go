package boot

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/altalune-id/oidcrp/oidcrptest"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/config"
	authv1mcp "github.com/altalune-id/othra/gen/go/auth/v1/authv1mcp"
	clientv1mcp "github.com/altalune-id/othra/gen/go/client/v1/clientv1mcp"
	invitev1mcp "github.com/altalune-id/othra/gen/go/invite/v1/invitev1mcp"
	keysv1mcp "github.com/altalune-id/othra/gen/go/keys/v1/keysv1mcp"
	memberv1mcp "github.com/altalune-id/othra/gen/go/member/v1/memberv1mcp"
	orgv1connect "github.com/altalune-id/othra/gen/go/org/v1/orgv1connect"
	orgv1mcp "github.com/altalune-id/othra/gen/go/org/v1/orgv1mcp"
	projectv1mcp "github.com/altalune-id/othra/gen/go/project/v1/projectv1mcp"
	resourceserverv1mcp "github.com/altalune-id/othra/gen/go/resourceserver/v1/resourceserverv1mcp"
	webhookv1mcp "github.com/altalune-id/othra/gen/go/webhook/v1/webhookv1mcp"
	"github.com/altalune-id/othra/internal/apikey"
	"github.com/altalune-id/othra/internal/auth"
	"github.com/altalune-id/othra/internal/client"
	"github.com/altalune-id/othra/internal/controlplane"
	"github.com/altalune-id/othra/internal/enduser"
	"github.com/altalune-id/othra/internal/grant"
	"github.com/altalune-id/othra/internal/kernel/authz"
	"github.com/altalune-id/othra/internal/kernel/capabilities"
	"github.com/altalune-id/othra/internal/kernel/session"
	"github.com/altalune-id/othra/internal/kernel/tokens"
	"github.com/altalune-id/othra/internal/keys"
	mcpinternal "github.com/altalune-id/othra/internal/mcp"
	"github.com/altalune-id/othra/internal/oauth/oidc"
	"github.com/altalune-id/othra/internal/org"
	"github.com/altalune-id/othra/internal/permission"
	"github.com/altalune-id/othra/internal/testutil/fakes"
	rootmcp "github.com/altalune-id/othra/mcp"
	"github.com/altalune-id/othra/scope"
)

func TestBuildMCPServer_RegistersWhoami(t *testing.T) {
	srv := buildMCPServer(config.MCPConfig{Enabled: true}, &Services{Orgs: org.NewService(&fakes.OrgStore{}, fakes.Transactor{}, fakes.NoMemberRoles{}, nil, nil)}, nil)
	require.NotNil(t, srv)
	assert.Contains(t, srv.Registry().Names(), authv1mcp.AuthWhoamiToolName)
}

// SECURITY: the MCP chain never includes the session cookie authenticator.
func TestMCPChain(t *testing.T) {
	local := &Services{Caps: capabilities.Capabilities{LocalLogin: true}, KeyAuthn: &apikey.Authenticator{}, Auth: &auth.Service{}}
	chain := mcpChain(local)
	require.Len(t, chain, 1)
	assert.IsType(t, &apikey.Authenticator{}, chain[0])

	basic := &Services{Caps: capabilities.Capabilities{BasicAuth: true}, KeyAuthn: &apikey.Authenticator{}, Auth: &auth.Service{}}
	chain = mcpChain(basic)
	require.Len(t, chain, 2)
	assert.IsType(t, auth.BasicAuthenticator{}, chain[1])
}

func TestMCPChain_TokensWhenMCPOAuthIsOn(t *testing.T) {
	svcs := &Services{KeyAuthn: &apikey.Authenticator{}, Auth: &auth.Service{}, MCPTokens: fakeTokens{}}
	chain := mcpChain(svcs)
	require.Len(t, chain, 2)
	assert.Equal(t, auth.TokenAuthenticator{Verifier: fakeTokens{}, Auth: svcs.Auth}, chain[1])
}

type fakeTokens struct{}

func (fakeTokens) Verify(context.Context, string) (session.Principal, error) {
	return session.Principal{}, &tokens.InvalidTokenError{}
}

func mcpOAuthConfig() config.Config {
	cfg := devLocalConfig()
	cfg.HTTP.BaseURL = "https://othra.example.com"
	cfg.MCP.Enabled = true
	return cfg
}

func TestMCPAudience(t *testing.T) {
	cfg := mcpOAuthConfig()
	assert.Equal(t, "https://othra.example.com/mcp", mcpAudience(cfg))
	cfg.HTTP.BasePath = "/othra"
	assert.Equal(t, "https://othra.example.com/othra/mcp", mcpAudience(cfg))
	cfg.MCP.Audience = "https://proxy.example.com/mcp"
	assert.Equal(t, "https://proxy.example.com/mcp", mcpAudience(cfg))
}

func TestBuildMCPTokens_OnlyWithMCPOAuth(t *testing.T) {
	off := mcpOAuthConfig()
	off.MCP.Enabled = false
	v, err := buildMCPTokens(off, &Services{}, nil)
	require.NoError(t, err)
	assert.Nil(t, v)

	v, err = buildMCPTokens(mcpOAuthConfig(), &Services{}, nil)
	require.NoError(t, err)
	assert.Nil(t, v, "no issuer signs tokens")
}

func TestMCPOAuth_OffInBasicMode(t *testing.T) {
	svcs := &Services{
		IssuerResolver: &oidc.Resolver{}, Grants: &grant.Service{}, Clients: &client.Service{}, EndUsers: &enduser.Service{}, Keys: &keys.Service{},
	}
	require.True(t, mcpOAuth(mcpOAuthConfig(), svcs))
	svcs.Caps.BasicAuth = true
	assert.False(t, mcpOAuth(mcpOAuthConfig(), svcs))
}

func TestNewMCPVerifier_TakesTheClockSkew(t *testing.T) {
	idp := oidcrptest.New(t)
	now := time.Now()
	late := idp.AccessToken(map[string]any{
		"aud": "https://othra.example.com/mcp", "iat": now.Add(-time.Hour).Unix(), "exp": now.Add(-45 * time.Second).Unix(),
	})
	for skew, ok := range map[time.Duration]bool{time.Minute: true, 10 * time.Second: false} {
		cfg := mcpOAuthConfig()
		cfg.MCP.ClockSkew = skew
		v, err := newMCPVerifier(cfg, idp.Issuer(), idp.Client())
		require.NoError(t, err)
		_, err = v.Verify(t.Context(), late)
		assert.Equal(t, ok, err == nil, "skew %s: %v", skew, err)
	}
}

// SECURITY: RFC 8707: the verifier takes only tokens minted for this MCP endpoint.
func TestNewMCPVerifier_PinsTheAudience(t *testing.T) {
	idp := oidcrptest.New(t)
	v, err := newMCPVerifier(mcpOAuthConfig(), idp.Issuer(), idp.Client())
	require.NoError(t, err)

	p, err := v.Verify(t.Context(), idp.AccessToken(map[string]any{"aud": "https://othra.example.com/mcp"}))
	require.NoError(t, err)
	assert.Equal(t, session.SourceToken, p.Source)

	_, err = v.Verify(t.Context(), idp.AccessToken(map[string]any{"aud": "https://othra.example.com/api"}))
	assert.ErrorAs(t, err, new(*tokens.InvalidTokenError))
}

// SECURITY: the RFC 9728 document offers only catalog scopes a token may hold.
func TestMCPScopes_OfferOnlyTokenScopes(t *testing.T) {
	assert.Contains(t, mcpScopes(), scope.MemberWrite)
	assert.Contains(t, mcpScopes(), scope.WebhookRead)
	assert.NotContains(t, mcpScopes(), scope.ApikeyWrite)
	noop := func(context.Context, json.RawMessage) (json.RawMessage, error) { return nil, nil }
	reg := rootmcp.NewRegistry()
	reg.Register(rootmcp.ToolSpec{Name: "member_invite", Scopes: []string{scope.MemberWrite}, Handler: noop}, nil)
	assert.Equal(t, []string{scope.MemberWrite}, toolScopes(reg)("member_invite"))
	assert.Nil(t, toolScopes(reg)("nope"))
}

func TestMCPSystemResource(t *testing.T) {
	cfg := mcpOAuthConfig()
	cfg.MCP.ClientHosts = []string{"claude.ai"}
	r, err := mcpSystemResource(cfg)
	require.NoError(t, err)
	assert.Equal(t, "https://othra.example.com/mcp", r.Identifier)
	assert.Equal(t, mcpScopes(), r.Scopes)
	assert.Equal(t, []string{"claude.ai"}, r.ClientHosts)

	cfg.MCP.ClientHosts = []string{"Claude.AI"}
	_, err = mcpSystemResource(cfg)
	var bad *config.InvalidSettingError
	require.ErrorAs(t, err, &bad)
	assert.Equal(t, "mcp.clientHosts", bad.Key)

	cfg = mcpOAuthConfig()
	cfg.MCP.Audience, cfg.MCP.AudienceOverride = "http://proxy.example.com/mcp", true
	_, err = mcpSystemResource(cfg)
	require.ErrorAs(t, err, &bad)
	assert.Equal(t, "mcp.audience", bad.Key)

	cfg = mcpOAuthConfig()
	cfg.HTTP.BaseURL = "http://localhost:5150"
	_, err = mcpSystemResource(cfg)
	require.NoError(t, err, "plain http on localhost stays valid")
}

func TestBuildMCPResource_OffWithoutMCPOAuth(t *testing.T) {
	log, buf := bufferLogger()
	r, docs, err := buildMCPResource(mcpOAuthConfig(), &Services{}, log)
	require.NoError(t, err)
	assert.Equal(t, tokens.Resource{}, r)
	assert.Nil(t, docs)
	assert.Empty(t, buf.String())
}

func TestNewMCPResource(t *testing.T) {
	log, buf := bufferLogger()
	cfg := mcpOAuthConfig()
	cfg.HTTP.BasePath = "/othra"
	const issuer = "https://othra.example.com/othra/p/system"
	r, docs, err := newMCPResource(cfg, issuer, log)
	require.NoError(t, err)
	assert.Equal(t, "https://othra.example.com/othra/mcp", r.Identifier)
	assert.Contains(t, buf.String(), `msg="boot: mcp oauth on" audience=https://othra.example.com/othra/mcp`)
	assert.Contains(t, buf.String(), "metadata=/.well-known/oauth-protected-resource/othra/mcp")
	assert.Contains(t, buf.String(), "issuer="+issuer)
	require.Contains(t, docs, "/.well-known/oauth-protected-resource/othra/mcp")

	rec := httptest.NewRecorder()
	docs[r.MetadataPath].ServeHTTP(rec, httptest.NewRequest(http.MethodGet, r.MetadataPath, nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var doc struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
		ScopesSupported      []string `json:"scopes_supported"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &doc))
	assert.Equal(t, r.Identifier, doc.Resource)
	assert.Equal(t, []string{issuer}, doc.AuthorizationServers)
	assert.Contains(t, doc.ScopesSupported, scope.MemberWrite)
	assert.NotContains(t, doc.ScopesSupported, scope.ApikeyWrite)

	cfg.MCP.Audience = "urn:othra:mcp"
	_, _, err = newMCPResource(cfg, issuer, log)
	assert.ErrorAs(t, err, new(*tokens.ResourceInvalidError))
}

func TestBuildMCPHandler(t *testing.T) {
	svcs := &Services{Orgs: org.NewService(&fakes.OrgStore{}, fakes.Transactor{}, fakes.NoMemberRoles{}, nil, nil), KeyScheme: apikey.NewScheme(""), KeyAuthn: &apikey.Authenticator{}}
	assert.Nil(t, buildMCPHandler(nil, svcs, tokens.Resource{}, nil))

	h := buildMCPHandler(buildMCPServer(config.MCPConfig{Enabled: true}, svcs, nil), svcs, tokens.Resource{}, nil)
	require.NotNil(t, h)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Token nope")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "Bearer", rec.Header().Get("WWW-Authenticate"))
}

func TestApp_RunMCPActsAsSystem(t *testing.T) {
	seen := make(chan session.Principal, 1)
	reg := rootmcp.NewRegistry()
	reg.Register(rootmcp.ToolSpec{Name: "probe", Scopes: []string{scope.WebhookRead}, Handler: func(ctx context.Context, _ json.RawMessage) (json.RawMessage, error) {
		seen <- session.PrincipalFrom(ctx)
		return json.RawMessage(`{}`), nil
	}}, nil)
	app := &App{MCP: rootmcp.NewServer(rootmcp.WithRegistry(reg), rootmcp.WithScopes(mcpinternal.ScopesFromContext))}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	clientT, serverT := sdkmcp.NewInMemoryTransports()
	go func() { _ = app.runMCP(ctx, serverT) }()
	cs, err := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "t", Version: "v0"}, nil).Connect(ctx, clientT, nil)
	require.NoError(t, err)
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(ctx, &sdkmcp.CallToolParams{Name: "probe"})
	require.NoError(t, err)
	require.False(t, res.IsError, "%+v", res.Content)
	assert.Equal(t, session.System(), <-seen)
}

// SECURITY: every published tool takes its permissions from permission.Table, through its RPC.
func TestMCPTools_PermissionsComeFromTheTable(t *testing.T) {
	srv := buildMCPServer(config.MCPConfig{Enabled: true}, &Services{Orgs: org.NewService(&fakes.OrgStore{}, fakes.Transactor{}, fakes.NoMemberRoles{}, nil, nil)}, nil)
	require.NotNil(t, srv)
	inTable := map[string]bool{}
	for _, perms := range permission.Table() {
		for _, p := range perms {
			inTable[p] = true
		}
	}
	want := map[string][]string{
		orgv1mcp.OrgCreateToolName:              {scope.OrgWrite},
		orgv1mcp.OrgGetToolName:                 {scope.OrgRead},
		orgv1mcp.OrgListToolName:                {scope.OrgRead},
		projectv1mcp.ProjectCreateToolName:      {scope.ProjectWrite},
		projectv1mcp.ProjectGetToolName:         {scope.ProjectRead},
		projectv1mcp.ProjectListToolName:        {scope.ProjectRead},
		webhookv1mcp.WebhookCreateToolName:      {scope.WebhookWrite},
		webhookv1mcp.WebhookListToolName:        {scope.WebhookRead},
		webhookv1mcp.WebhookDeliveriesToolName:  {scope.WebhookRead},
		webhookv1mcp.WebhookRetryFailedToolName: {scope.WebhookWrite},
		memberv1mcp.MemberListToolName:          {scope.MemberRead},
		invitev1mcp.MemberInviteToolName:        {scope.MemberWrite},
		invitev1mcp.MemberInviteConfirmToolName: {scope.MemberWrite},
		invitev1mcp.MemberInviteDiscardToolName: {scope.MemberWrite},
		authv1mcp.AuthWhoamiToolName:            {authz.AnyAuthenticated},
		keysv1mcp.SigningKeyListToolName:        {scope.ProjectRead},
		keysv1mcp.SigningKeyRotateToolName:      {scope.ProjectWrite},
		clientv1mcp.ClientListToolName:          {scope.ProjectRead},
		clientv1mcp.ClientGetToolName:           {scope.ProjectRead},
		clientv1mcp.ClientCreateToolName:        {scope.ProjectWrite},
		clientv1mcp.ClientUpdateToolName:        {scope.ProjectWrite},
		clientv1mcp.ClientDeleteToolName:        {scope.ProjectWrite},

		resourceserverv1mcp.ResourceServerListToolName:       {scope.ProjectRead},
		resourceserverv1mcp.ResourceServerGetToolName:        {scope.ProjectRead},
		resourceserverv1mcp.ResourceServerCreateToolName:     {scope.ProjectWrite},
		resourceserverv1mcp.ResourceServerUpdateToolName:     {scope.ProjectWrite},
		resourceserverv1mcp.ResourceServerDeleteToolName:     {scope.ProjectWrite},
		resourceserverv1mcp.ResourceServerVerifyToolName:     {scope.ProjectWrite},
		resourceserverv1mcp.ResourceServerLinkListToolName:   {scope.ProjectRead},
		resourceserverv1mcp.ResourceServerLinkCreateToolName: {scope.ProjectWrite},
		resourceserverv1mcp.ResourceServerLinkUpdateToolName: {scope.ProjectWrite},
		resourceserverv1mcp.ResourceServerLinkDeleteToolName: {scope.ProjectWrite},
	}
	names := srv.Registry().Names()
	assert.Len(t, names, len(want), "the published tool list")
	for _, name := range names {
		spec, ok := srv.Registry().Spec(name)
		require.True(t, ok)
		assert.NotEmpty(t, spec.Scopes, "%s has no permission", name)
		for _, p := range spec.Scopes {
			assert.True(t, inTable[p], "%s needs %q, which permission.Table does not name", name, p)
		}
		assert.Equal(t, want[name], spec.Scopes, name)
	}
}

// SECURITY: a generated tool whose RPC is missing from the table is denied and hidden.
func TestMCPTools_UnmappedProcedureIsDenied(t *testing.T) {
	reg := rootmcp.NewRegistry()
	orgv1mcp.RegisterOrgServiceTools(reg, org.NewControlPlane(org.NewService(&fakes.OrgStore{}, fakes.Transactor{}, fakes.NoMemberRoles{}, nil, nil)), func(procedure string) []string {
		if procedure == orgv1connect.OrgServiceGetOrgProcedure {
			return nil
		}
		return permission.For(procedure)
	}, (*controlplane.Validator)(nil).Validate)
	app := &App{MCP: rootmcp.NewServer(rootmcp.WithRegistry(reg), rootmcp.WithScopes(mcpinternal.ScopesFromContext))}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	clientT, serverT := sdkmcp.NewInMemoryTransports()
	go func() { _ = app.runMCP(ctx, serverT) }()
	cs, err := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "t", Version: "v0"}, nil).Connect(ctx, clientT, nil)
	require.NoError(t, err)
	defer func() { _ = cs.Close() }()

	list, err := cs.ListTools(ctx, nil)
	require.NoError(t, err)
	var listed []string
	for _, tool := range list.Tools {
		listed = append(listed, tool.Name)
	}
	assert.Equal(t, []string{orgv1mcp.OrgCreateToolName, orgv1mcp.OrgListToolName}, listed, "system sees every mapped tool, not the unmapped one")

	res, err := cs.CallTool(ctx, &sdkmcp.CallToolParams{Name: orgv1mcp.OrgGetToolName, Arguments: map[string]any{"id": "x"}})
	require.NoError(t, err)
	assert.True(t, res.IsError, "even the system operator cannot call a tool whose RPC has no permission")
}

// SECURITY: the model must never be offered the tool that sends a drafted invite.
func TestMCPTools_InviteConfirmIsForTheAppOnly(t *testing.T) {
	list := func(appsUI bool) map[string]*sdkmcp.Tool {
		srv := buildMCPServer(config.MCPConfig{Enabled: true, AppsUI: appsUI}, &Services{Orgs: org.NewService(&fakes.OrgStore{}, fakes.Transactor{}, fakes.NoMemberRoles{}, nil, nil)}, nil)
		app := &App{MCP: srv}
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)
		clientT, serverT := sdkmcp.NewInMemoryTransports()
		go func() { _ = app.runMCP(ctx, serverT) }()
		cs, err := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "t", Version: "v0"}, nil).Connect(ctx, clientT, nil)
		require.NoError(t, err)
		t.Cleanup(func() { _ = cs.Close() })
		res, err := cs.ListTools(ctx, nil)
		require.NoError(t, err)
		out := map[string]*sdkmcp.Tool{}
		for _, tool := range res.Tools {
			out[tool.Name] = tool
		}
		return out
	}

	withApp := list(true)
	for _, name := range []string{invitev1mcp.MemberInviteConfirmToolName, invitev1mcp.MemberInviteDiscardToolName} {
		require.Contains(t, withApp, name)
		ui, ok := withApp[name].Meta["ui"].(map[string]any)
		require.True(t, ok, name)
		assert.Equal(t, []any{"app"}, ui["visibility"], "%s is hidden from the model", name)
	}
	draft, ok := withApp[invitev1mcp.MemberInviteToolName].Meta["ui"].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, draft, "visibility", "the model may draft")

	without := list(false)
	assert.Contains(t, without, invitev1mcp.MemberInviteToolName)
	assert.NotContains(t, without, invitev1mcp.MemberInviteConfirmToolName, "no app, no confirm tool")
	assert.NotContains(t, without, invitev1mcp.MemberInviteDiscardToolName)
}

// e2e Finding 1: /mcp behind a same-host proxy accepts http.baseURL's host on a loopback connection, and only that one.
func TestMCPPublicHost(t *testing.T) {
	assert.Equal(t, []string{"id.example.com"}, mcpPublicHost(config.HTTPConfig{BaseURL: "https://id.example.com"}))
	assert.Equal(t, []string{"id.example.com"}, mcpPublicHost(config.HTTPConfig{BaseURL: "https://id.example.com:8443"}))
	assert.Nil(t, mcpPublicHost(config.HTTPConfig{}))

	srv := buildMCPServer(config.MCPConfig{Enabled: true}, &Services{Orgs: org.NewService(&fakes.OrgStore{}, fakes.Transactor{}, fakes.NoMemberRoles{}, nil, nil)}, nil,
		mcpPublicHost(config.HTTPConfig{BaseURL: "https://id.example.com"})...)
	for host, want := range map[string]int{"id.example.com": http.StatusOK, "evil.example": http.StatusForbidden} {
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`))
		req.Host = host
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, net.Addr(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})))
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		assert.Equal(t, want, rec.Code, host)
	}
}
