//go:build integration

package boot

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/internal/client"
	"altalune.id/othra/internal/enduser"
	"altalune.id/othra/internal/grant"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/keys"
	"altalune.id/othra/internal/oauth/hosted"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/resourceserver"
)

func postForm(t *testing.T, target string, form url.Values, header ...string) (int, http.Header, map[string]any) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, target, strings.NewReader(form.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://app.example")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var body map[string]any
	if len(raw) > 0 {
		require.NoError(t, json.Unmarshal(raw, &body), string(raw))
	}
	return resp.StatusCode, resp.Header, body
}

// SECURITY: through boot, a code buys an ID token signed by the project's key, userinfo answers its end user, and refresh rotates.
func TestTokenFlowThroughBoot(t *testing.T) {
	ts, svcs := newIssuerServer(t, "")
	ctx := t.Context()
	sys, err := svcs.Orgs.System(ctx)
	require.NoError(t, err)
	p, err := svcs.Projects.Create(ctx, project.CreateRequest{OrgID: sys.ID, Slug: "web", Name: "Web", IssuerHandle: "acme-web"})
	require.NoError(t, err)
	issuer := "https://id.example.test/p/acme-web"

	c, err := svcs.Clients.Create(ctx, sys.ID, p.ID, client.KindWeb, client.Settings{
		Name: "App", RedirectURIs: []string{"https://app.example/cb"}, RefreshTokens: true, ConsentPolicy: client.ConsentImplicit,
	})
	require.NoError(t, err)
	_, secret, err := svcs.Clients.GenerateSecret(ctx, sys.ID, p.ID, c.PublicID)
	require.NoError(t, err)

	eu := enduser.Project{OrgID: sys.ID, ID: p.ID, IssuerURL: issuer, EmailLink: true}
	link, err := svcs.EndUsers.IssueLink(ctx, eu, enduser.LinkRequest{Email: "ada@example.com", ReturnTo: "/p/acme-web/account/"})
	require.NoError(t, err)
	u, err := url.Parse(link.URL)
	require.NoError(t, err)
	signed, err := svcs.EndUsers.Redeem(ctx, eu, enduser.RedeemRequest{Token: u.Query().Get("t")})
	require.NoError(t, err)

	verifier := strings.Repeat("v", 43)
	sum := sha256.Sum256([]byte(verifier))
	gp := grant.Project{OrgID: sys.ID, ID: p.ID}
	code, err := svcs.Grants.IssueCode(ctx, gp, grant.CodeRequest{
		Client: c.PublicID, RedirectURI: "https://app.example/cb", Scopes: []string{"openid", "email", "offline_access"},
		CodeChallenge: base64.RawURLEncoding.EncodeToString(sum[:]), Nonce: "n-1",
		UserSession: signed.Account.PublicID, EndUser: signed.Account.Subject, AuthTime: signed.Account.AuthTime,
	})
	require.NoError(t, err)

	tokenURL := ts.URL + "/p/acme-web/oauth2/token"
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(c.PublicID+":"+url.QueryEscape(secret)))
	status, header, body := postForm(t, tokenURL, url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {"https://app.example/cb"}, "code_verifier": {verifier},
	}, "Authorization", auth)
	require.Equal(t, http.StatusOK, status, "%v", body)
	assert.Equal(t, "no-store", header.Get("Cache-Control"))
	assert.Equal(t, "no-cache", header.Get("Pragma"))
	assert.Equal(t, "*", header.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "Bearer", body["token_type"])
	access, _ := body["access_token"].(string)
	refresh, _ := body["refresh_token"].(string)
	idToken, _ := body["id_token"].(string)
	require.NotEmpty(t, access)
	require.NotEmpty(t, refresh)

	scoped := tenant.Into(ctx, tenant.Context{OrgID: sys.ID, ProjectID: p.ID})
	claims, err := keys.NewVerifier(svcs.Keys).VerifyIDTokenHint(scoped, p.ID, issuer, idToken)
	require.NoError(t, err, "the ID token verifies against the project's published key")
	assert.Equal(t, signed.Account.Subject, claims.Subject)
	assert.Equal(t, []string{c.PublicID}, claims.Audience)
	assert.Equal(t, "n-1", claims.Nonce)
	assert.Equal(t, keys.AtHash(access), claims.AtHash)
	assert.Equal(t, signed.Account.AuthTime.Unix(), claims.AuthTime)
	assert.Equal(t, "ada@example.com", claims.Email)
	assert.NotEmpty(t, claims.SessionID)
	assert.WithinDuration(t, time.Now().Add(5*time.Minute), time.Unix(claims.ExpiresAt, 0), time.Minute)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/p/acme-web/oauth2/userinfo", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+access)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, resp.Body.Close())
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	assert.JSONEq(t, `{"sub":"`+signed.Account.Subject+`","email":"ada@example.com","email_verified":true}`, string(raw))

	status, _, body = postForm(t, tokenURL, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}}, "Authorization", auth)
	require.Equal(t, http.StatusOK, status, "%v", body)
	assert.NotEqual(t, refresh, body["refresh_token"])
	status, _, body = postForm(t, tokenURL, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}}, "Authorization", auth)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "invalid_grant", body["error"], "a reused refresh token revokes the family")

	status, _, body = postForm(t, tokenURL, url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "code_verifier": {verifier},
	}, "Authorization", auth)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "invalid_grant", body["error"], "a code works once")

	status, header, body = postForm(t, ts.URL+"/p/acme-web/oauth2/revoke", url.Values{"token": {"unknown"}}, "Authorization", auth)
	assert.Equal(t, http.StatusOK, status, "RFC 7009 §2.2: an unknown token is fine: %v", body)
	assert.Equal(t, "no-store", header.Get("Cache-Control"))

	jwt, err := accessTokenSigner{signer: keys.NewSigner(svcs.Keys)}.Sign(ctx, grant.Project{OrgID: sys.ID, ID: p.ID, IssuerURL: issuer}, grant.AccessToken{
		Audience: "https://api.example/mcp", Subject: "sub-1", ClientID: c.PublicID, SessionID: "sid-1", Scopes: []string{"read", "write"},
		AuthTime: time.Now().Add(-time.Minute), IssuedAt: time.Now(), ExpiresAt: time.Now().Add(5 * time.Minute),
	})
	require.NoError(t, err)
	at, err := keys.NewVerifier(svcs.Keys, keys.WithExpectedIssuer(issuer), keys.WithExpectedType(keys.TypeAccessToken)).Verify(scoped, p.ID, jwt)
	require.NoError(t, err, "RFC 9068 §4: typ at+jwt, this issuer, the project's key")
	assert.Equal(t, []string{"https://api.example/mcp"}, []string(at.Audience))
	assert.Equal(t, "read write", at.Scope)
	assert.Len(t, at.JWTID, 21)
}

// OIDC Core §11 through the hosted adapters: sign-out keeps offline_access grants; end-session ends the hinted one whatever its scope.
func TestSignOutKeepsOfflineGrantsThroughBoot(t *testing.T) {
	_, svcs := newIssuerServer(t, "")
	ctx := t.Context()
	sys, err := svcs.Orgs.System(ctx)
	require.NoError(t, err)
	p, err := svcs.Projects.Create(ctx, project.CreateRequest{OrgID: sys.ID, Slug: "web", Name: "Web", IssuerHandle: "acme-web"})
	require.NoError(t, err)
	c, err := svcs.Clients.Create(ctx, sys.ID, p.ID, client.KindWeb, client.Settings{
		Name: "App", RedirectURIs: []string{"https://app.example/cb"}, RefreshTokens: true, ConsentPolicy: client.ConsentImplicit,
	})
	require.NoError(t, err)
	hp := hosted.Project{OrgID: sys.ID, ID: p.ID, IssuerURL: "https://id.example.test/p/acme-web", EmailLink: true}
	eu, gp := endUserProject(hp), grantProject(hp)

	signIn := func(email string) *enduser.Redeemed {
		link, err := svcs.EndUsers.IssueLink(ctx, eu, enduser.LinkRequest{Email: email, ReturnTo: "/p/acme-web/account/"})
		require.NoError(t, err)
		u, err := url.Parse(link.URL)
		require.NoError(t, err)
		r, err := svcs.EndUsers.Redeem(ctx, eu, enduser.RedeemRequest{Token: u.Query().Get("t")})
		require.NoError(t, err)
		return r
	}
	grantOf := func(r *enduser.Redeemed, scopes ...string) *grant.Tokens {
		code, err := svcs.Grants.IssueCode(ctx, gp, grant.CodeRequest{
			Client: c.PublicID, RedirectURI: "https://app.example/cb", Scopes: scopes,
			UserSession: r.Account.PublicID, EndUser: r.Account.Subject, AuthTime: r.Account.AuthTime,
		})
		require.NoError(t, err)
		got, err := svcs.Grants.Exchange(ctx, gp, grant.ExchangeRequest{Client: grant.Client{ID: c.ID, OAuthID: c.PublicID, Refresh: true}, Code: code})
		require.NoError(t, err)
		return got
	}
	live := func(sid string) bool {
		f, err := svcs.Grants.Family(ctx, gp, sid)
		require.NoError(t, err)
		return f != nil
	}

	ada := signIn("ada@example.com")
	plain, offline := grantOf(ada, "openid"), grantOf(ada, "openid", "offline_access")
	users := hostedUsers{users: svcs.EndUsers}
	require.NoError(t, users.SignOut(ctx, hp, ada.Secret, ada.Account.PublicID))
	assert.False(t, live(plain.Family.PublicID), "sign-out ends the session's plain grants")
	assert.True(t, live(offline.Family.PublicID), "an offline_access grant outlives the sign-in session")

	bob := signIn("bob@example.com")
	hinted, other := grantOf(bob, "openid", "offline_access"), grantOf(bob, "openid", "offline_access")
	oauth := hostedOAuth{clients: svcs.Clients, grants: svcs.Grants, users: svcs.EndUsers}
	require.NoError(t, oauth.EndSession(ctx, hp, hinted.Family.PublicID, bob.Account.PublicID))
	assert.False(t, live(hinted.Family.PublicID), "end-session ends the hinted grant whatever its scope")
	assert.True(t, live(other.Family.PublicID), "another offline_access grant of that session survives")
}

// RFC 8707, RFC 9068 and CIMD §6 through boot: a linked, trusted server gets an at+jwt for itself only, rechecked at refresh; discovery follows admission.
func TestResourceBoundTokenThroughBoot(t *testing.T) {
	ts, svcs := newIssuerServer(t, "")
	ctx := t.Context()
	sys, err := svcs.Orgs.System(ctx)
	require.NoError(t, err)
	p, err := svcs.Projects.Create(ctx, project.CreateRequest{OrgID: sys.ID, Slug: "web", Name: "Web", IssuerHandle: "acme-web"})
	require.NoError(t, err)
	issuer := "https://id.example.test/p/acme-web"
	scoped := tenant.Into(ctx, tenant.Context{OrgID: sys.ID, ProjectID: p.ID})

	c, err := svcs.Clients.Create(ctx, sys.ID, p.ID, client.KindWeb, client.Settings{
		Name: "App", RedirectURIs: []string{"https://app.example/cb"}, RefreshTokens: true, ConsentPolicy: client.ConsentImplicit,
	})
	require.NoError(t, err)
	_, secret, err := svcs.Clients.GenerateSecret(ctx, sys.ID, p.ID, c.PublicID)
	require.NoError(t, err)
	rs, err := svcs.ResourceServers.Create(ctx, resourceserver.CreateRequest{
		OrgID: sys.ID, ProjectID: p.ID, Name: "MCP", Identifier: "https://api.example.test/mcp", Scopes: []string{"read", "write"},
		Trusted: true, Person: uuid.New(),
	})
	require.NoError(t, err)
	_, err = svcs.ResourceServers.LinkClient(ctx, sys.ID, p.ID, c.PublicID, rs.PublicID, []string{"read"})
	require.NoError(t, err)

	discovery := func() map[string]any {
		_, body := fetch(t, http.MethodGet, ts.URL+"/p/acme-web/.well-known/openid-configuration")
		var doc map[string]any
		require.NoError(t, json.Unmarshal([]byte(body), &doc))
		return doc
	}
	assert.Equal(t, false, discovery()["client_id_metadata_document_supported"])
	_, err = svcs.ResourceServers.SetAdmission(ctx, sys.ID, p.ID, rs.PublicID, resourceserver.Admission{Accept: true})
	require.NoError(t, err)
	assert.Equal(t, true, discovery()["client_id_metadata_document_supported"], "admission drops the 30 s discovery cache at once")

	eu := enduser.Project{OrgID: sys.ID, ID: p.ID, IssuerURL: issuer, EmailLink: true}
	link, err := svcs.EndUsers.IssueLink(ctx, eu, enduser.LinkRequest{Email: "ada@example.com", ReturnTo: "/p/acme-web/account/"})
	require.NoError(t, err)
	u, err := url.Parse(link.URL)
	require.NoError(t, err)
	signed, err := svcs.EndUsers.Redeem(ctx, eu, enduser.RedeemRequest{Token: u.Query().Get("t")})
	require.NoError(t, err)
	code, err := svcs.Grants.IssueCode(ctx, grant.Project{OrgID: sys.ID, ID: p.ID}, grant.CodeRequest{
		Client: c.PublicID, RedirectURI: "https://app.example/cb", Scopes: []string{"openid", "offline_access", "read"},
		UserSession: signed.Account.PublicID, EndUser: signed.Account.Subject, AuthTime: signed.Account.AuthTime, ResourceServerID: &rs.ID,
	})
	require.NoError(t, err)

	tokenURL := ts.URL + "/p/acme-web/oauth2/token"
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(c.PublicID+":"+url.QueryEscape(secret)))
	status, _, body := postForm(t, tokenURL, url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {"https://app.example/cb"}, "resource": {rs.Identifier},
	}, "Authorization", auth)
	require.Equal(t, http.StatusOK, status, "%v", body)
	access, _ := body["access_token"].(string)
	refresh, _ := body["refresh_token"].(string)
	assert.Equal(t, "openid offline_access read", body["scope"])
	assert.NotEmpty(t, body["id_token"], "openid with a resource still gets an ID token")

	claims, err := keys.NewVerifier(svcs.Keys, keys.WithExpectedIssuer(issuer), keys.WithExpectedType(keys.TypeAccessToken)).Verify(scoped, p.ID, access)
	require.NoError(t, err, "RFC 9068 §4")
	assert.Equal(t, []string{rs.Identifier}, []string(claims.Audience))
	assert.Equal(t, "read", claims.Scope, "only the server's scopes")
	assert.Equal(t, c.PublicID, claims.ClientID)
	assert.Equal(t, signed.Account.Subject, claims.Subject)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/p/acme-web/oauth2/userinfo", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+access)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "an at+jwt for a resource server is not a userinfo token")

	status, _, body = postForm(t, tokenURL, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "resource": {"https://other.example/"}},
		"Authorization", auth)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "invalid_target", body["error"])

	require.NoError(t, svcs.ResourceServers.UnlinkClient(ctx, sys.ID, p.ID, c.PublicID, rs.PublicID))
	status, _, body = postForm(t, tokenURL, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}}, "Authorization", auth)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "invalid_grant", body["error"], "an unlinked client can't refresh")
}
