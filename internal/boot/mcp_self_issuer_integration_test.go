//go:build integration

package boot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/config"
	"github.com/altalune-id/othra/internal/auth"
	"github.com/altalune-id/othra/internal/client"
	"github.com/altalune-id/othra/internal/kernel"
	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/internal/project"
	"github.com/altalune-id/othra/internal/testutil/pgtest"
)

const (
	claudeClientID = "https://claude.ai/oauth/claude-code-client-metadata"
	claudeCallback = "http://127.0.0.1:53999/callback"
)

type stubMetadata struct{}

func (stubMetadata) FetchMetadata(context.Context, string) (*client.FetchedMetadata, error) {
	doc, err := json.Marshal(map[string]any{
		"client_id": claudeClientID, "client_name": "Claude Code", "redirect_uris": []string{"http://127.0.0.1/callback"},
		"grant_types": []string{"authorization_code", "refresh_token"}, "response_types": []string{"code"},
		"token_endpoint_auth_method": "none",
	})
	return &client.FetchedMetadata{Body: doc, ContentType: "application/json"}, err
}

type mcpIssuerFixture struct {
	*signInFixture
	resource string
}

func newMCPIssuerFixture(t *testing.T) *mcpIssuerFixture {
	t.Helper()
	srv := httptest.NewUnstartedServer(nil)
	srv.StartTLS()
	t.Cleanup(srv.Close)
	cfg := devLocalConfig()
	cfg.HTTP.BaseURL = srv.URL
	cfg.HTTP.StateSecret = strings.Repeat("s", 40)
	cfg.Secrets.EncryptionKey = testKeyHex
	cfg.Mail.From = "Othra <no-reply@othra.example>"
	cfg.EndUsers = config.EndUsersConfig{
		MagicLink: config.EndUsersMagicLinkConfig{TTL: 15 * time.Minute, ResendAfter: time.Minute},
		Session:   config.EndUsersSessionConfig{Idle: 30 * 24 * time.Hour, Absolute: 90 * 24 * time.Hour, MaxAccounts: 20},
		CSRF:      config.EndUsersCSRFConfig{MaxAge: time.Hour},
	}
	cfg.Auth.Genesis = config.GenesisConfig{Email: "root@example.com", Password: "genesis-password"}
	cfg.MCP = config.MCPConfig{Enabled: true, ClockSkew: time.Minute, ClientHosts: []string{"claude.ai"}}
	pool, sqlDB, schemaName := pgtest.MigratedRLS(t)
	cfg.Postgres.Schema = schemaName
	svcs := buildServices(kernel.Kernel{Pool: pool, Reporter: apperror.NewReporter(nil, false)}, cfg)
	svcs.Clients = client.NewService(client.NewStore(pool, schemaName, cfg.Postgres.TablePrefix), db.NewTransactor(pool), svcs.Projects, stubMetadata{},
		client.MetadataConfig{Cache: client.MetadataCache{Min: time.Hour, Max: time.Hour, Default: time.Hour}, IssuerHost: "othra.invalid", MaxBytes: 5120},
		apperror.NewReporter(nil, false).Unexpected).WithLinks(rsLinks{servers: svcs.ResourceServers})
	require.NoError(t, buildAuthSecrets(cfg, svcs, testLogger()))
	svcs.Caps.EnvGenesis = true
	require.NoError(t, firstBoot(t.Context(), cfg, svcs, testLogger(), true))
	required, err := installRequired(t.Context(), svcs)
	require.NoError(t, err)

	inProcess := http.NewServeMux()
	public, hosted := buildIssuerRoutes(cfg, svcs, inProcess, nil, nil)
	svcs.MCPTokens, err = buildMCPTokens(cfg, svcs, inProcess)
	require.NoError(t, err)
	require.NotNil(t, svcs.MCPTokens)
	mcpServer := buildMCPServer(cfg.MCP, svcs, testLogger(), mcpPublicHost(cfg.HTTP)...)
	resource, docs, err := buildMCPResource(cfg, svcs, testLogger())
	require.NoError(t, err)
	srv.Config.Handler = buildHTTPHandler(httpSurfaces{
		routes: buildRoutes(cfg, svcs, nil, nil, nil), public: public, hosted: hosted,
		mcp: buildMCPHandler(mcpServer, svcs, resource, testLogger()), mcpDocs: docs,
		session: buildSessionMiddleware(svcs, nil), installRequired: required,
	})
	f := &signInFixture{t: t, srv: srv, svcs: svcs, db: sqlDB, pool: pool, cfg: cfg}
	f.c = f.client()
	return &mcpIssuerFixture{signInFixture: f, resource: resource.Identifier}
}

func (f *mcpIssuerFixture) mcp(token, body string) (int, http.Header, string) {
	f.t.Helper()
	req, err := http.NewRequestWithContext(f.t.Context(), http.MethodPost, f.srv.URL+"/mcp", strings.NewReader(body))
	require.NoError(f.t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := f.c.Do(req)
	require.NoError(f.t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(f.t, err)
	return resp.StatusCode, resp.Header, string(raw)
}

func (f *mcpIssuerFixture) token(form url.Values) map[string]any {
	f.t.Helper()
	p := f.do(f.c, http.MethodPost, "/p/system/oauth2/token", form)
	require.Equal(f.t, http.StatusOK, p.status, p.body)
	var out map[string]any
	require.NoError(f.t, json.Unmarshal([]byte(p.body), &out))
	return out
}

func (f *mcpIssuerFixture) follow(body string) signInPage {
	f.t.Helper()
	m := continueRE.FindStringSubmatch(body)
	require.NotEmpty(f.t, m, body)
	return f.do(f.c, http.MethodGet, strings.TrimPrefix(strings.ReplaceAll(m[1], "&amp;", "&"), f.srv.URL), nil)
}

func (f *mcpIssuerFixture) signIn(q url.Values, email string) string {
	f.t.Helper()
	first := f.do(f.c, http.MethodGet, "/p/system/oauth2/authorize?"+q.Encode(), nil)
	require.Equal(f.t, http.StatusSeeOther, first.status, first.body)
	login := f.do(f.c, http.MethodGet, first.header.Get("Location"), nil)
	sent := f.submit(f.c, login, "/login", url.Values{"email": {email}})
	require.Equal(f.t, http.StatusSeeOther, sent.status, sent.body)
	link := signInLinkRE.FindString(claimMail(f.t, f.svcs)[0].TextBody)
	moved := f.do(f.c, http.MethodGet, strings.TrimPrefix(link, f.srv.URL), nil)
	verify := f.do(f.c, http.MethodGet, moved.header.Get("Location"), nil)
	done := f.submit(f.c, verify, "/login/verify", nil)
	toConsent := f.follow(done.body)
	require.Equal(f.t, http.StatusSeeOther, toConsent.status, toConsent.body)
	page := f.do(f.c, http.MethodGet, toConsent.header.Get("Location"), nil)
	require.Equal(f.t, http.StatusOK, page.status, page.body)
	assert.Contains(f.t, page.body, "Claude Code")
	assert.Contains(f.t, page.body, "claude.ai")
	back := f.follow(f.submit(f.c, page, "/consent", nil).body)
	require.Equal(f.t, http.StatusFound, back.status, back.body)
	loc, err := url.Parse(back.header.Get("Location"))
	require.NoError(f.t, err)
	require.NotEmpty(f.t, loc.Query().Get("code"), loc.String())
	return loc.Query().Get("code")
}

// A CIMD client with only the MCP URL signs in at /p/system; the first token, then a refreshed one, act as the linked console person.
func TestMCPOAuth_SelfIssuedTokenAndRefresh(t *testing.T) {
	f := newMCPIssuerFixture(t)
	whoami := toolCall("auth_whoami", map[string]any{})

	status, header, _ := f.mcp("", whoami)
	require.Equal(t, http.StatusUnauthorized, status)
	metadata := f.srv.URL + "/.well-known/oauth-protected-resource/mcp"
	assert.Equal(t, `Bearer resource_metadata="`+metadata+`"`, header.Get("WWW-Authenticate"))
	doc := f.do(f.c, http.MethodGet, "/.well-known/oauth-protected-resource/mcp", nil)
	require.Equal(t, http.StatusOK, doc.status, doc.body)
	assert.Contains(t, doc.body, f.srv.URL+"/p/system")
	assert.NotContains(t, doc.body, "apikey:write")
	discovery := f.do(f.c, http.MethodGet, "/.well-known/oauth-authorization-server/p/system", nil)
	require.Equal(t, http.StatusOK, discovery.status, discovery.body)
	assert.Contains(t, discovery.body, `"client_id_metadata_document_supported":true`)

	code := f.signIn(url.Values{
		"client_id": {claudeClientID}, "redirect_uri": {claudeCallback}, "response_type": {"code"},
		"scope": {"member:read offline_access"}, "resource": {f.resource}, "state": {"st"},
		"code_challenge": {oauthChallenge}, "code_challenge_method": {"S256"},
	}, "root@example.com")
	first := f.token(url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "client_id": {claudeClientID}, "redirect_uri": {claudeCallback},
		"code_verifier": {oauthVerifier}, "resource": {f.resource},
	})
	access, _ := first["access_token"].(string)
	refresh, _ := first["refresh_token"].(string)
	require.NotEmpty(t, access)
	require.NotEmpty(t, refresh, "offline_access buys a refresh token")

	status, header, body := f.mcp(access, whoami)
	assert.Equal(t, http.StatusUnauthorized, status, "no console link yet: %s", body)
	assert.Contains(t, header.Get("WWW-Authenticate"), `error="invalid_token"`)

	subject := f.subject(t, access)
	root, err := f.svcs.Users.FindByEmail(t.Context(), "root@example.com")
	require.NoError(t, err)
	linked, err := f.svcs.Users.LinkIDP(t.Context(), root.ID, auth.ConsoleIssuer, subject)
	require.NoError(t, err)
	require.True(t, linked)

	status, _, body = f.mcp(access, whoami)
	require.Equal(t, http.StatusOK, status, body)
	assert.Contains(t, body, "root@example.com")

	status, _, body = f.mcp(access, toolCall("member_list", map[string]any{"orgId": f.systemOrgPublicID(t)}))
	require.Equal(t, http.StatusOK, status, body)
	assert.NotContains(t, body, `"isError":true`)

	status, header, body = f.mcp(access, toolCall("member_invite", map[string]any{"orgId": f.systemOrgPublicID(t), "email": "new@example.com"}))
	assert.Equal(t, http.StatusForbidden, status, body)
	assert.Contains(t, header.Get("WWW-Authenticate"), `error="insufficient_scope", scope="member:write"`)

	next := f.token(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {claudeClientID}, "resource": {f.resource}})
	renewed, _ := next["access_token"].(string)
	rotated, _ := next["refresh_token"].(string)
	require.NotEmpty(t, renewed)
	assert.NotEqual(t, refresh, rotated, "refresh rotates")
	status, _, body = f.mcp(renewed, whoami)
	require.Equal(t, http.StatusOK, status, body)
	assert.Contains(t, body, "root@example.com")

	replay := f.do(f.c, http.MethodPost, "/p/system/oauth2/token", url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {claudeClientID}})
	assert.Equal(t, http.StatusBadRequest, replay.status, replay.body)

	moved := f.cfg
	moved.HTTP.BaseURL = "https://othra.example.test"
	require.NoError(t, reconcileMCPResource(t.Context(), moved, f.svcs, testLogger()))
	sys, err := f.svcs.Orgs.System(t.Context())
	require.NoError(t, err)
	p, err := f.svcs.Projects.BySlug(t.Context(), sys.ID, project.SystemSlug)
	require.NoError(t, err)
	list, err := f.svcs.ResourceServers.List(t.Context(), sys.ID, p.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "https://othra.example.test/mcp", list[0].Identifier, "the server follows http.baseURL")

	off := f.cfg
	off.MCP.Enabled = false
	require.NoError(t, reconcileMCPResource(t.Context(), off, f.svcs, testLogger()))
	discovery = f.do(f.c, http.MethodGet, "/.well-known/oauth-authorization-server/p/system", nil)
	require.Equal(t, http.StatusOK, discovery.status, discovery.body)
	assert.Contains(t, discovery.body, `"client_id_metadata_document_supported":false`, "turning MCP off closes admission")
	refused := f.do(f.c, http.MethodGet, "/p/system/oauth2/authorize?"+url.Values{
		"client_id": {claudeClientID}, "redirect_uri": {claudeCallback}, "response_type": {"code"},
		"resource": {"https://othra.example.test/mcp"}, "state": {"st"},
		"code_challenge": {oauthChallenge}, "code_challenge_method": {"S256"},
	}.Encode(), nil)
	assert.NotEqual(t, http.StatusSeeOther, refused.status, "a metadata document client is refused: %s", refused.body)
}

func (f *mcpIssuerFixture) systemOrgPublicID(t *testing.T) string {
	t.Helper()
	sys, err := f.svcs.Orgs.System(t.Context())
	require.NoError(t, err)
	return sys.PublicID
}

func (f *mcpIssuerFixture) subject(t *testing.T, access string) string {
	t.Helper()
	p, err := f.svcs.MCPTokens.Verify(t.Context(), access)
	require.NoError(t, err)
	return p.IDPSubject
}
