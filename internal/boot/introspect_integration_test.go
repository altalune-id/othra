//go:build integration

package boot

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/internal/client"
	"github.com/altalune-id/othra/internal/enduser"
	"github.com/altalune-id/othra/internal/grant"
	"github.com/altalune-id/othra/internal/pat"
	"github.com/altalune-id/othra/internal/project"
	"github.com/altalune-id/othra/internal/resourceserver"
)

func oauthBasic(id, secret string) string {
	return basicAuth(url.QueryEscape(id), url.QueryEscape(secret))
}

// RFC 7662 through boot: a resource server learns only about its own tokens, and a revoked personal token dies at once.
func TestIntrospectionThroughBoot(t *testing.T) {
	ts, svcs := newIssuerServer(t, "")
	ctx := t.Context()
	sys, err := svcs.Orgs.System(ctx)
	require.NoError(t, err)
	p, err := svcs.Projects.Create(ctx, project.CreateRequest{OrgID: sys.ID, Slug: "web", Name: "Web", IssuerHandle: "acme-web"})
	require.NoError(t, err)
	issuer := "https://id.example.test/p/acme-web"

	c, err := svcs.Clients.Create(ctx, sys.ID, p.ID, client.KindWeb, client.Settings{
		Name: "App", RedirectURIs: []string{"https://app.example/cb"}, ConsentPolicy: client.ConsentImplicit,
	})
	require.NoError(t, err)
	_, clientSecret, err := svcs.Clients.GenerateSecret(ctx, sys.ID, p.ID, c.PublicID)
	require.NoError(t, err)
	server := func(name, identifier string) (*resourceserver.ResourceServer, string) {
		rs, err := svcs.ResourceServers.Create(ctx, resourceserver.CreateRequest{
			OrgID: sys.ID, ProjectID: p.ID, Name: name, Identifier: identifier, Scopes: []string{"read", "write"}, Trusted: true, Person: uuid.New(),
		})
		require.NoError(t, err)
		_, s, err := svcs.ResourceServers.GenerateIntrospectionSecret(ctx, sys.ID, p.ID, rs.PublicID)
		require.NoError(t, err)
		return rs, s
	}
	s1, s1Secret := server("MCP 1", "https://mcp1.example.test/mcp")
	s2, s2Secret := server("MCP 2", "https://mcp2.example.test/mcp")
	_, err = svcs.ResourceServers.LinkClient(ctx, sys.ID, p.ID, c.PublicID, s1.PublicID, []string{"read"})
	require.NoError(t, err)

	_, body := fetch(t, http.MethodGet, ts.URL+"/p/acme-web/.well-known/openid-configuration")
	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(body), &doc))
	assert.Equal(t, issuer+"/oauth2/introspect", doc["introspection_endpoint"])
	assert.Equal(t, []any{"client_secret_basic", "client_secret_post"}, doc["introspection_endpoint_auth_methods_supported"])

	eu := enduser.Project{OrgID: sys.ID, ID: p.ID, IssuerURL: issuer, EmailLink: true}
	link, err := svcs.EndUsers.IssueLink(ctx, eu, enduser.LinkRequest{Email: "ada@example.com", ReturnTo: "/p/acme-web/account/"})
	require.NoError(t, err)
	u, err := url.Parse(link.URL)
	require.NoError(t, err)
	signed, err := svcs.EndUsers.Redeem(ctx, eu, enduser.RedeemRequest{Token: u.Query().Get("t")})
	require.NoError(t, err)
	code, err := svcs.Grants.IssueCode(ctx, grant.Project{OrgID: sys.ID, ID: p.ID}, grant.CodeRequest{
		Client: c.PublicID, RedirectURI: "https://app.example/cb", Scopes: []string{"read"},
		UserSession: signed.Account.PublicID, EndUser: signed.Account.Subject, AuthTime: signed.Account.AuthTime, ResourceServerID: &s1.ID,
	})
	require.NoError(t, err)
	clientAuth := oauthBasic(c.PublicID, clientSecret)
	status, _, tokens := postForm(t, ts.URL+"/p/acme-web/oauth2/token", url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {"https://app.example/cb"}, "resource": {s1.Identifier},
	}, "Authorization", clientAuth)
	require.Equal(t, http.StatusOK, status, "%v", tokens)
	access, _ := tokens["access_token"].(string)

	introspectURL := ts.URL + "/p/acme-web/oauth2/introspect"
	introspect := func(id, secret, token string) (int, http.Header, map[string]any) {
		t.Helper()
		return postForm(t, introspectURL, url.Values{"token": {token}}, "Authorization", oauthBasic(id, secret))
	}
	status, header, got := introspect(s1.PublicID, s1Secret, access)
	require.Equal(t, http.StatusOK, status, "%v", got)
	assert.Equal(t, "no-store", header.Get("Cache-Control"))
	assert.Equal(t, true, got["active"])
	assert.Equal(t, "read", got["scope"])
	assert.Equal(t, c.PublicID, got["client_id"])
	assert.Equal(t, signed.Account.Subject, got["sub"])
	assert.Equal(t, s1.Identifier, got["aud"])
	assert.Equal(t, issuer, got["iss"])
	assert.Equal(t, "Bearer", got["token_type"])
	assert.Len(t, got, 9)

	_, _, got = introspect(s2.PublicID, s2Secret, access)
	assert.Equal(t, map[string]any{"active": false}, got, "another resource server's token")
	status, _, got = introspect(s1.PublicID, s2Secret, access)
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "invalid_client", got["error"])

	_, err = svcs.ResourceServers.SetPersonalTokenAccess(ctx, sys.ID, p.ID, s1.PublicID, true)
	require.NoError(t, err)
	pp := pat.Project{OrgID: sys.ID, ID: p.ID, PublicID: p.PublicID, Name: p.Name}
	created, err := svcs.PersonalTokens.Create(ctx, pp, pat.CreateRequest{
		EndUserID: signed.Account.EndUserID, Name: "CLI", ServerPublicID: s1.PublicID,
		Scopes: []string{"read"}, Expiry: pat.Expiry30Days,
	})
	require.NoError(t, err)

	_, _, got = introspect(s1.PublicID, s1Secret, created.Secret)
	assert.Equal(t, true, got["active"])
	assert.Equal(t, signed.Account.Subject, got["sub"])
	assert.Equal(t, s1.Identifier, got["aud"])
	assert.NotContains(t, got, "client_id", "a personal token has no client")
	_, _, got = introspect(s2.PublicID, s2Secret, created.Secret)
	assert.Equal(t, map[string]any{"active": false}, got)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/p/acme-web/oauth2/userinfo", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+created.Secret)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "not a userinfo token")
	status, _, _ = postForm(t, ts.URL+"/p/acme-web/oauth2/revoke", url.Values{"token": {created.Secret}}, "Authorization", clientAuth)
	assert.Equal(t, http.StatusOK, status)
	_, _, got = introspect(s1.PublicID, s1Secret, created.Secret)
	assert.Equal(t, true, got["active"], "/revoke leaves a personal token alone")

	require.NoError(t, svcs.PersonalTokens.Revoke(ctx, pp, signed.Account.EndUserID, created.Token.PublicID))
	_, _, got = introspect(s1.PublicID, s1Secret, created.Secret)
	assert.Equal(t, map[string]any{"active": false}, got, "revoked: inactive on the very next call")

	live, err := svcs.PersonalTokens.Create(ctx, pp, pat.CreateRequest{
		EndUserID: signed.Account.EndUserID, Name: "CLI 2", ServerPublicID: s1.PublicID, Scopes: []string{"read"}, Expiry: pat.Expiry30Days,
	})
	require.NoError(t, err)
	_, err = svcs.ResourceServers.SetTrust(ctx, sys.ID, p.ID, s1.PublicID, false, uuid.New())
	require.NoError(t, err)
	_, _, got = introspect(s1.PublicID, s1Secret, live.Secret)
	assert.Equal(t, map[string]any{"active": false}, got, "a server no longer admissible takes no personal tokens")
	_, err = svcs.PersonalTokens.Create(ctx, pp, pat.CreateRequest{
		EndUserID: signed.Account.EndUserID, Name: "CLI 3", ServerPublicID: s1.PublicID, Scopes: []string{"read"}, Expiry: pat.Expiry30Days,
	})
	assert.True(t, pat.IsServerClosedError(err), "%v", err)
}
