package oidc_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
	ksecret "altalune.id/othra/internal/kernel/secret"
	"altalune.id/othra/internal/keys"
	"altalune.id/othra/internal/oauth/oidc"
	"altalune.id/othra/internal/web/layout"
)

const secret = "s3cret/+="

type clients map[string]*client.Client

func (c clients) Authenticate(_ context.Context, _, _ uuid.UUID, publicID string) (*client.Client, error) {
	if publicID == "boom" {
		return nil, errors.New("boom")
	}
	cl, ok := c[publicID]
	if !ok {
		return nil, &client.NotFoundError{}
	}
	return cl, nil
}

type grants struct {
	exchange *grant.ExchangeRequest
	refresh  *grant.RefreshRequest
	tokens   *grant.Tokens
	access   *grant.Access
	err      error
	revoked  []string
}

func (g *grants) Exchange(_ context.Context, _ grant.Project, req grant.ExchangeRequest) (*grant.Tokens, error) {
	g.exchange = &req
	return g.tokens, g.err
}

func (g *grants) Refresh(_ context.Context, _ grant.Project, req grant.RefreshRequest) (*grant.Tokens, error) {
	g.refresh = &req
	return g.tokens, g.err
}

func (g *grants) Revoke(_ context.Context, _ grant.Project, c grant.Client, token, hint string) error {
	g.revoked = append(g.revoked, c.OAuthID+" "+token+" "+hint)
	return g.err
}

func (g *grants) Access(context.Context, grant.Project, string) (*grant.Access, error) {
	if g.access == nil {
		return nil, &grant.InvalidTokenError{}
	}
	return g.access, g.err
}

type endUsers struct {
	user *enduser.EndUser
	err  error
}

func (e endUsers) EndUser(context.Context, enduser.Project, uuid.UUID) (*enduser.EndUser, error) {
	return e.user, e.err
}

type idTokens struct {
	in  *keys.MintIDTokenInput
	err error
}

func (s *idTokens) MintIDToken(_ context.Context, _ uuid.UUID, in *keys.MintIDTokenInput) (*keys.Token, error) {
	s.in = in
	if s.err != nil {
		return nil, s.err
	}
	return &keys.Token{Compact: "id.token.sig"}, nil
}

type tokenFixture struct {
	*fixture
	grants   *grants
	idTokens *idTokens
	users    *endUsers
	limiter  *limiter
}

func webClient(id, method string, grantTypes ...string) *client.Client {
	return &client.Client{
		ID: uuid.New(), PublicID: id, TokenEndpointAuthMethod: method, GrantTypes: grantTypes,
		Secrets: []client.Secret{(client.Secret{}).WithHash(ksecret.Versioned(secret))},
	}
}

func newTokenFixture(t *testing.T) *tokenFixture {
	t.Helper()
	f := &tokenFixture{fixture: newFixture(t, ""), idTokens: &idTokens{}, limiter: &limiter{allow: true}}
	family := &grant.Family{PublicID: "sid-1", EndUserID: uuid.New(), Scopes: []string{"openid", "email", "profile", "offline_access"}, AuthTime: time.Unix(1700000000, 0)}
	f.grants = &grants{tokens: &grant.Tokens{
		Family: family, AccessToken: "at-1", ExpiresAt: time.Now().Add(15 * time.Minute), Scopes: family.Scopes, RefreshToken: "rt-1", Nonce: "n-1",
	}}
	f.users = &endUsers{user: &enduser.EndUser{Subject: "sub-1", Email: "ada@example.com", EmailVerified: true, Name: "Ada"}}
	f.web.WithTokens(oidc.Tokens{
		Clients: clients{
			"basic": webClient("basic", client.AuthClientSecretBasic, "authorization_code", "refresh_token"),
			"post":  webClient("post", client.AuthClientSecretPost, "authorization_code"),
			"pct": {
				ID: uuid.New(), PublicID: "pct", TokenEndpointAuthMethod: client.AuthClientSecretBasic, GrantTypes: []string{"authorization_code"},
				Secrets: []client.Secret{(client.Secret{}).WithHash(ksecret.Versioned("a%zz"))},
			},
			"cli": {ID: uuid.New(), PublicID: "cli", TokenEndpointAuthMethod: client.AuthNone, GrantTypes: []string{"authorization_code", "refresh_token"}},
		},
		Grants: f.grants, EndUsers: f.users, IDTokens: f.idTokens, IDTokenTTL: 5 * time.Minute,
		Limits: layout.Limits{Limiter: f.limiter},
	})
	f.mux = http.NewServeMux()
	f.web.Register(f.mux)
	return f
}

func (f *tokenFixture) post(t *testing.T, path string, form url.Values, header ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, req)
	return rec
}

func basic(id, pw string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(url.QueryEscape(id)+":"+url.QueryEscape(pw)))
}

func oauthErr(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	return body.Error
}

const tokenURL = "/p/acme-web/oauth2/token"

func codeForm(extra ...string) url.Values {
	v := url.Values{"grant_type": {"authorization_code"}, "code": {"c-1"}, "redirect_uri": {"https://a.example/cb"}, "code_verifier": {"v-1"}}
	for i := 0; i+1 < len(extra); i += 2 {
		v.Set(extra[i], extra[i+1])
	}
	return v
}

// OIDC Core §3.1.3.3: the code grant answers the tokens, never cached, with an ID token for its client.
func TestToken_CodeGrantWithBasic(t *testing.T) {
	f := newTokenFixture(t)
	rec := f.post(t, tokenURL, codeForm(), "Authorization", basic("basic", secret))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "no-cache", rec.Header().Get("Pragma"))
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "at-1", body["access_token"])
	assert.Equal(t, "Bearer", body["token_type"])
	assert.Equal(t, "openid email profile offline_access", body["scope"])
	assert.Equal(t, "rt-1", body["refresh_token"])
	assert.Equal(t, "id.token.sig", body["id_token"])
	assert.InDelta(t, 900, body["expires_in"], 2)

	require.NotNil(t, f.grants.exchange)
	assert.Equal(t, grant.ExchangeRequest{
		Client: f.grants.exchange.Client, Code: "c-1", RedirectURI: "https://a.example/cb", CodeVerifier: "v-1",
	}, *f.grants.exchange)
	assert.Equal(t, "basic", f.grants.exchange.Client.OAuthID)
	assert.True(t, f.grants.exchange.Client.Refresh)

	in := f.idTokens.in
	require.NotNil(t, in)
	assert.Equal(t, "https://id.example.com/p/acme-web", in.Issuer)
	assert.Equal(t, "sub-1", in.Subject)
	assert.Equal(t, []string{"basic"}, in.Audience)
	assert.Equal(t, "n-1", in.Nonce)
	assert.Equal(t, "sid-1", in.SessionID)
	assert.Equal(t, "at-1", in.AccessToken)
	assert.Equal(t, time.Unix(1700000000, 0), in.AuthTime)
	assert.Equal(t, 5*time.Minute, in.ExpiresAt.Sub(in.IssuedAt))
	assert.Equal(t, &keys.Profile{Email: "ada@example.com", EmailVerified: true, Name: "Ada", WithEmail: true, WithName: true}, in.Profile)
}

func TestToken_PostAndNoneClients(t *testing.T) {
	f := newTokenFixture(t)
	rec := f.post(t, tokenURL, codeForm("client_id", "post", "client_secret", secret))
	assert.Equal(t, http.StatusOK, rec.Code, "client_secret_post (D2): %s", rec.Body.String())
	assert.False(t, f.grants.exchange.Client.Refresh)

	rec = f.post(t, tokenURL, codeForm("client_id", "cli"))
	assert.Equal(t, http.StatusOK, rec.Code, "a public client names itself with client_id (S8): %s", rec.Body.String())

	rec = f.post(t, tokenURL, codeForm("client_id", "basic"), "Authorization", basic("basic", secret))
	assert.Equal(t, http.StatusOK, rec.Code, "client_id beside Basic is the same client")
}

// SECURITY: RFC 6749 §5.2, every client authentication failure is one 401 invalid_client; Basic gets its challenge.
func TestToken_InvalidClient(t *testing.T) {
	f := newTokenFixture(t)
	for name, tc := range map[string]struct {
		form   url.Values
		header []string
		basic  bool
	}{
		"no client":                                {form: codeForm()},
		"unknown":                                  {form: codeForm("client_id", "nope")},
		"a wrong secret":                           {form: codeForm(), header: []string{"Authorization", basic("basic", "wrong")}, basic: true},
		"basic for a post client":                  {form: codeForm(), header: []string{"Authorization", basic("post", secret)}, basic: true},
		"post for a basic client":                  {form: codeForm("client_id", "basic", "client_secret", secret)},
		"a public client's secret":                 {form: codeForm("client_id", "cli", "client_secret", secret)},
		"a confidential client without its secret": {form: codeForm("client_id", "basic")},
		"a secret that is not form-encoded": {
			form: codeForm(), header: []string{"Authorization", "Basic " + base64.StdEncoding.EncodeToString([]byte("pct:a%zz"))}, basic: true,
		},
		"a malformed form-encoded id": {
			form: codeForm(), header: []string{"Authorization", "Basic " + base64.StdEncoding.EncodeToString([]byte("bas%ic:"+url.QueryEscape(secret)))}, basic: true,
		},
	} {
		rec := f.post(t, tokenURL, tc.form, tc.header...)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, name)
		assert.Equal(t, "invalid_client", oauthErr(t, rec), name)
		if tc.basic {
			assert.Equal(t, `Basic realm="https://id.example.com/p/acme-web"`, rec.Header().Get("WWW-Authenticate"), name)
		} else {
			assert.Empty(t, rec.Header().Get("WWW-Authenticate"), name)
		}
	}
	assert.Nil(t, f.grants.exchange, "no failed client reaches the grant")

	rec := f.post(t, tokenURL, codeForm("client_id", "boom"))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "server_error", oauthErr(t, rec))
}

func TestToken_RequestErrors(t *testing.T) {
	f := newTokenFixture(t)
	auth := []string{"Authorization", basic("basic", secret)}
	for name, tc := range map[string]struct {
		form   url.Values
		header []string
		status int
		code   string
	}{
		"two methods":         {codeForm("client_secret", secret), auth, http.StatusBadRequest, "invalid_request"},
		"another client_id":   {codeForm("client_id", "post"), auth, http.StatusBadRequest, "invalid_request"},
		"no code":             {codeForm("code", ""), auth, http.StatusBadRequest, "invalid_request"},
		"no refresh token":    {url.Values{"grant_type": {"refresh_token"}}, auth, http.StatusBadRequest, "invalid_request"},
		"an unknown grant":    {url.Values{"grant_type": {"password"}}, auth, http.StatusBadRequest, "unsupported_grant_type"},
		"no grant":            {url.Values{}, auth, http.StatusBadRequest, "invalid_request"},
		"a grant not allowed": {url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"r"}, "client_id": {"post"}, "client_secret": {secret}}, nil, http.StatusBadRequest, "unauthorized_client"},
	} {
		rec := f.post(t, tokenURL, tc.form, tc.header...)
		assert.Equal(t, tc.status, rec.Code, name)
		assert.Equal(t, tc.code, oauthErr(t, rec), name)
	}

	dup := httptest.NewRequest(http.MethodPost, tokenURL, strings.NewReader("grant_type=authorization_code&code=a&evil%3Cscript%3E=1&evil%3Cscript%3E=2"))
	dup.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	dup.Header.Set("Authorization", basic("basic", secret))
	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, dup)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "RFC 6749 §3.2: a repeated parameter")
	assert.Equal(t, "invalid_request", oauthErr(t, rec))
	assert.JSONEq(t, `{"error":"invalid_request","error_description":"a request parameter appears more than once"}`, rec.Body.String(),
		"SECURITY: the description is fixed text, never the parameter name")

	asJSON := httptest.NewRequest(http.MethodPost, tokenURL, strings.NewReader(`{"grant_type":"authorization_code"}`))
	asJSON.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	f.mux.ServeHTTP(rec, asJSON)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	huge := httptest.NewRequest(http.MethodPost, tokenURL, strings.NewReader("code="+strings.Repeat("a", 17<<10)))
	huge.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	f.mux.ServeHTTP(rec, huge)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "the body is capped at 16 KB")
}

func TestToken_GrantErrors(t *testing.T) {
	f := newTokenFixture(t)
	auth := []string{"Authorization", basic("basic", secret)}
	for err, code := range map[error]string{
		&grant.InvalidGrantError{}:   "invalid_grant",
		&grant.InvalidScopeError{}:   "invalid_scope",
		&grant.InvalidRequestError{}: "invalid_request",
		errors.New("boom"):           "server_error",
	} {
		f.grants.err = err
		rec := f.post(t, tokenURL, codeForm(), auth...)
		assert.Equal(t, code, oauthErr(t, rec))
	}
	f.grants.err = nil

	f.idTokens.err = errors.New("no key")
	rec := f.post(t, tokenURL, codeForm(), auth...)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "server_error", oauthErr(t, rec))
	f.idTokens.err = nil

	f.users.user = nil
	rec = f.post(t, tokenURL, codeForm(), auth...)
	assert.Equal(t, http.StatusInternalServerError, rec.Code, "an end user deleted mid-exchange")
	assert.Contains(t, f.reported, "oidc: id token end user")
}

// RFC 6749 §6: a refresh may narrow the scope; without openid there is no ID token.
func TestToken_RefreshGrant(t *testing.T) {
	f := newTokenFixture(t)
	f.grants.tokens.Scopes = []string{"offline_access"}
	f.grants.tokens.Family.Scopes = []string{"offline_access"}
	f.grants.tokens.Nonce = ""
	rec := f.post(t, tokenURL, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"rt-0"}, "scope": {"offline_access"}, "client_id": {"cli"}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotNil(t, f.grants.refresh)
	assert.Equal(t, "rt-0", f.grants.refresh.RefreshToken)
	assert.Equal(t, []string{"offline_access"}, f.grants.refresh.Scopes)
	assert.Nil(t, f.idTokens.in, "no openid, no ID token")
	assert.NotContains(t, rec.Body.String(), "id_token")

	rec = f.post(t, tokenURL, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"rt-0"}, "client_id": {"cli"}})
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Nil(t, f.grants.refresh.Scopes, "no scope keeps the grant's")
}

func TestToken_RateLimited(t *testing.T) {
	f := newTokenFixture(t)
	f.limiter.allow = false
	rec := f.post(t, tokenURL, codeForm(), "Authorization", basic("basic", secret))
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "2", rec.Header().Get("Retry-After"))
	assert.Contains(t, f.limiter.keys[0], "oauth-token-ip:project:"+f.acme.ProjectID.String())

	f.limiter.allow, f.limiter.err = true, errors.New("redis down")
	rec = f.post(t, tokenURL, codeForm(), "Authorization", basic("basic", secret))
	assert.Equal(t, http.StatusOK, rec.Code, "a broken limiter lets requests through")
	assert.Contains(t, f.reported, "oidc: rate limit")
}

const userinfoURL = "/p/acme-web/oauth2/userinfo"

func (f *tokenFixture) userinfo(t *testing.T, method string, body string, header ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, userinfoURL, strings.NewReader(body))
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, req)
	return rec
}

// OIDC Core §5.3: userinfo answers sub and the claims the token's scopes allow.
func TestUserinfo(t *testing.T) {
	f := newTokenFixture(t)
	f.grants.access = &grant.Access{Family: f.grants.tokens.Family, Scopes: []string{"openid", "email"}}

	rec := f.userinfo(t, http.MethodGet, "", "Authorization", "Bearer at-1")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"sub":"sub-1","email":"ada@example.com","email_verified":true}`, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))

	f.grants.access.Scopes = []string{"openid", "profile"}
	rec = f.userinfo(t, http.MethodPost, "access_token=at-1", "Content-Type", "application/x-www-form-urlencoded")
	require.Equal(t, http.StatusOK, rec.Code, "RFC 6750 §2.2: the form body")
	assert.JSONEq(t, `{"sub":"sub-1","name":"Ada"}`, rec.Body.String())

	rec = f.userinfo(t, http.MethodPost, "access_token=at-1", "Content-Type", "application/x-www-form-urlencoded", "Authorization", "Bearer at-1")
	assert.Equal(t, http.StatusBadRequest, rec.Code, "never both")
	assert.Contains(t, rec.Header().Get("WWW-Authenticate"), `error="invalid_request"`)

	rec = f.userinfo(t, http.MethodGet, "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, `Bearer realm="https://id.example.com/p/acme-web"`, rec.Header().Get("WWW-Authenticate"), "RFC 6750 §3.1: no error without a token")
	rec = f.userinfo(t, http.MethodGet, "", "Authorization", "Basic abc")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	f.grants.access.Scopes = []string{"email"}
	rec = f.userinfo(t, http.MethodGet, "", "Authorization", "Bearer at-1")
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Header().Get("WWW-Authenticate"), `error="insufficient_scope"`)

	f.grants.access.Scopes = []string{"openid"}
	f.users.user = nil
	rec = f.userinfo(t, http.MethodGet, "", "Authorization", "Bearer at-1")
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "an end user that is gone")

	f.grants.access = nil
	rec = f.userinfo(t, http.MethodGet, "", "Authorization", "bearer at-x")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Header().Get("WWW-Authenticate"), `error="invalid_token"`)

	rec = f.userinfo(t, http.MethodDelete, "")
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.Equal(t, "GET, POST, OPTIONS", rec.Header().Get("Allow"))
}

// RFC 8707 §2: a resource the grant isn't bound to is invalid_target; the resource reaches the grant on code and refresh.
func TestToken_Resource(t *testing.T) {
	f := newTokenFixture(t)
	auth := []string{"Authorization", basic("basic", secret)}
	rec := f.post(t, tokenURL, codeForm("resource", "https://api.example/mcp"), auth...)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "https://api.example/mcp", f.grants.exchange.Resource)

	rec = f.post(t, tokenURL, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"r"}, "resource": {"https://api.example/mcp"}}, auth...)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "https://api.example/mcp", f.grants.refresh.Resource)

	f.grants.err = &grant.InvalidTargetError{}
	rec = f.post(t, tokenURL, codeForm("resource", "https://other.example/"), auth...)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "invalid_target", oauthErr(t, rec))
}

// E8-3: a refresh that narrows openid away gets no ID token.
func TestToken_RefreshWithoutOpenIDGetsNoIDToken(t *testing.T) {
	f := newTokenFixture(t)
	f.grants.tokens.Scopes = []string{"offline_access"}
	f.grants.tokens.Nonce = ""
	rec := f.post(t, tokenURL, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"r"}, "scope": {"offline_access"}},
		"Authorization", basic("basic", secret))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Nil(t, f.idTokens.in, "the family still holds openid, but this request narrowed it away")
	assert.NotContains(t, rec.Body.String(), "id_token")
}

// RFC 6750 §3.1: a token bound to a resource server is not a userinfo token.
func TestUserinfo_RefusesResourceTokens(t *testing.T) {
	f := newTokenFixture(t)
	rs := uuid.New()
	f.grants.access = &grant.Access{Family: f.grants.tokens.Family, Scopes: []string{"openid"}, ResourceServerID: &rs}
	rec := f.userinfo(t, http.MethodGet, "", "Authorization", "Bearer at-1")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Header().Get("WWW-Authenticate"), `error="invalid_token"`)
}

const revokeURL = "/p/acme-web/oauth2/revoke"

// RFC 7009: the /token client authentication, token required, 200 with no body whatever happened.
func TestRevoke(t *testing.T) {
	f := newTokenFixture(t)
	rec := f.post(t, revokeURL, url.Values{"token": {"rt-1"}, "token_type_hint": {"refresh_token"}}, "Authorization", basic("basic", secret))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, []string{"basic rt-1 refresh_token"}, f.grants.revoked)

	rec = f.post(t, revokeURL, url.Values{"token": {"at-1"}, "client_id": {"cli"}})
	assert.Equal(t, http.StatusOK, rec.Code, "a public client revokes with its client_id")

	rec = f.post(t, revokeURL, url.Values{"token": {"at-1"}, "client_id": {"basic"}, "client_secret": {"wrong"}})
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "invalid_client", oauthErr(t, rec))

	rec = f.post(t, revokeURL, url.Values{"client_id": {"cli"}})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "invalid_request", oauthErr(t, rec))

	f.grants.err = errors.New("db down")
	rec = f.post(t, revokeURL, url.Values{"token": {"x"}, "client_id": {"cli"}})
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	get := f.do(t, http.MethodGet, revokeURL)
	assert.Equal(t, http.StatusMethodNotAllowed, get.Code)
}

// CIMD: a shadow client authenticates with none; its OAuth client_id is its metadata URL, in the ID token aud and to the grant.
func TestToken_CIMDClient(t *testing.T) {
	f := newTokenFixture(t)
	metadata := "https://claude.ai/oauth/claude-code-client-metadata"
	f.web.WithTokens(oidc.Tokens{
		Clients: clients{metadata: {
			ID: uuid.New(), PublicID: "shadow", MetadataURL: metadata, TokenEndpointAuthMethod: client.AuthNone,
			GrantTypes: []string{"authorization_code", "refresh_token"},
		}},
		Grants: f.grants, EndUsers: f.users, IDTokens: f.idTokens, Limits: layout.Limits{Limiter: f.limiter},
	})
	f.mux = http.NewServeMux()
	f.web.Register(f.mux)
	rec := f.post(t, tokenURL, codeForm("client_id", metadata))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, metadata, f.grants.exchange.Client.OAuthID)
	assert.Equal(t, "claude.ai", f.grants.exchange.Client.CIMDHost)
	assert.Equal(t, []string{metadata}, f.idTokens.in.Audience)
}

// R-8, RFC 8707 §2: two different resources at /token or refresh are invalid_target; a repeated equal one counts once.
func TestToken_RepeatedResource(t *testing.T) {
	f := newTokenFixture(t)
	auth := []string{"Authorization", basic("basic", secret)}
	for name, form := range map[string]url.Values{
		"code":    codeForm(),
		"refresh": {"grant_type": {"refresh_token"}, "refresh_token": {"r"}},
	} {
		form["resource"] = []string{"https://a.example/mcp", "https://b.example/mcp"}
		rec := f.post(t, tokenURL, form, auth...)
		assert.Equal(t, http.StatusBadRequest, rec.Code, name)
		assert.Equal(t, "invalid_target", oauthErr(t, rec), name)

		form["resource"] = []string{"https://a.example/mcp", "https://a.example/mcp"}
		rec = f.post(t, tokenURL, form, auth...)
		assert.Equal(t, http.StatusOK, rec.Code, "%s: %s", name, rec.Body.String())
	}
	assert.Equal(t, "https://a.example/mcp", f.grants.exchange.Resource)
	assert.Equal(t, "https://a.example/mcp", f.grants.refresh.Resource)
}
