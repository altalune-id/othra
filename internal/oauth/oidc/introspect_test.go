package oidc_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/internal/grant"
	"github.com/altalune-id/othra/internal/oauth/oidc"
)

const (
	introspectURL = "/p/acme-web/oauth2/introspect"
	rsSecret      = "othra_rs_secret"
	personalToken = "othra_pt_token"
)

type callers struct {
	rs  map[string]oidc.Introspector
	err error
}

func (c *callers) Authenticate(_ context.Context, _, _ uuid.UUID, publicID, presented string) (oidc.Introspector, bool, error) {
	rs, ok := c.rs[publicID]
	return rs, ok && presented == rsSecret, c.err
}

type personalTokens struct {
	token  oidc.IntrospectedToken
	active bool
	caller uuid.UUID
	err    error
}

func (p *personalTokens) Introspect(_ context.Context, _, _, callerID uuid.UUID, _ string) (oidc.IntrospectedToken, bool, error) {
	p.caller = callerID
	return p.token, p.active, p.err
}

type clientIDs struct {
	id  string
	err error
}

func (c clientIDs) OAuthID(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (string, error) {
	return c.id, c.err
}

type introspectFixture struct {
	*tokenFixture
	callers  *callers
	personal *personalTokens
	s1, s2   oidc.Introspector
}

func newIntrospectFixture(t *testing.T) *introspectFixture {
	t.Helper()
	f := &introspectFixture{tokenFixture: newTokenFixture(t)}
	f.s1 = oidc.Introspector{ID: uuid.New(), PublicID: "s1", Identifier: "https://mcp1.test/mcp"}
	f.s2 = oidc.Introspector{ID: uuid.New(), PublicID: "s2", Identifier: "https://mcp2.test/mcp"}
	f.callers = &callers{rs: map[string]oidc.Introspector{"s1": f.s1, "s2": f.s2}}
	f.personal = &personalTokens{}
	f.web.WithIntrospection(oidc.Introspection{Callers: f.callers, PersonalTokens: f.personal, ClientIDs: clientIDs{id: "web-1"}})
	f.mux = http.NewServeMux()
	f.web.Register(f.mux)
	iat, exp := time.Unix(1700000000, 0), time.Unix(1700000900, 0)
	f.grants.access = &grant.Access{
		Family: f.grants.tokens.Family, Scopes: []string{"openid", "offline_access", "sample:read", "sample:write"}, IssuedAt: iat, ExpiresAt: exp, ResourceServerID: &f.s1.ID,
	}
	return f
}

func (f *introspectFixture) as(t *testing.T, rs, token string) string {
	t.Helper()
	rec := f.post(t, introspectURL, url.Values{"token": {token}}, "Authorization", basic(rs, rsSecret))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return rec.Body.String()
}

// RFC 7662 §2.2: an access token bound to the caller answers exactly the agreed members, never cached.
func TestIntrospect_ActiveAccessToken(t *testing.T) {
	f := newIntrospectFixture(t)
	rec := f.post(t, introspectURL, url.Values{"token": {"at-1"}, "token_type_hint": {"refresh_token"}}, "Authorization", basic("s1", rsSecret))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"active":true,"scope":"sample:read sample:write","client_id":"web-1","sub":"sub-1",
		"aud":"https://mcp1.test/mcp","iss":"https://id.example.com/p/acme-web","exp":1700000900,"iat":1700000000,
		"token_type":"Bearer"}`, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "no-cache", rec.Header().Get("Pragma"))
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	rec = f.post(t, introspectURL, url.Values{"token": {"at-1"}, "client_id": {"s1"}, "client_secret": {rsSecret}})
	assert.Equal(t, http.StatusOK, rec.Code, "client_secret_post works alone")
	assert.Contains(t, rec.Body.String(), `"active":true`)
}

// RFC 7662 §2.2: every token not active for the caller answers the one identical body.
func TestIntrospect_InactiveIsOneBody(t *testing.T) {
	const inactive = `{"active":false}`
	f := newIntrospectFixture(t)
	assert.Equal(t, inactive, f.as(t, "s2", "at-1"), "another resource server's token")

	other := f.grants.access
	f.grants.access = &grant.Access{Family: other.Family, Scopes: []string{"openid"}, ExpiresAt: other.ExpiresAt}
	assert.Equal(t, inactive, f.as(t, "s1", "at-1"), "an OIDC token bound to no resource server")

	f.grants.access = nil
	assert.Equal(t, inactive, f.as(t, "s1", "random"), "unknown, revoked or expired")

	f.grants.access = other
	user := f.users.user
	f.users.user = nil
	assert.Equal(t, inactive, f.as(t, "s1", "at-1"), "an end user that is gone")

	f.users.user = user
	f.web.WithIntrospection(oidc.Introspection{Callers: f.callers, PersonalTokens: f.personal, ClientIDs: clientIDs{}})
	assert.Equal(t, inactive, f.as(t, "s1", "at-1"), "a client that is gone")
}

// a personal token goes to its own lookup with the caller's id and answers without client_id.
func TestIntrospect_PersonalToken(t *testing.T) {
	f := newIntrospectFixture(t)
	assert.Equal(t, `{"active":false}`, f.as(t, "s1", personalToken))
	assert.Equal(t, f.s1.ID, f.personal.caller)

	f.personal.active = true
	f.personal.token = oidc.IntrospectedToken{
		Scopes: []string{"sample:read"}, Subject: "sub-1", Audience: "https://mcp1.test/mcp",
		IssuedAt: time.Unix(1700000000, 0), ExpiresAt: time.Unix(1702592000, 0),
	}
	assert.JSONEq(t, `{"active":true,"scope":"sample:read","sub":"sub-1","aud":"https://mcp1.test/mcp",
		"iss":"https://id.example.com/p/acme-web","exp":1702592000,"iat":1700000000,"token_type":"Bearer"}`, f.as(t, "s1", personalToken))
}

// RFC 7662 §2.1: only a resource server with its own live secret may introspect; every failure is one 401.
func TestIntrospect_CallerAuthentication(t *testing.T) {
	f := newIntrospectFixture(t)
	cases := map[string][]string{
		"no credential":   nil,
		"wrong secret":    {"Authorization", basic("s1", "wrong")},
		"unknown server":  {"Authorization", basic("s9", rsSecret)},
		"an OAuth client": {"Authorization", basic("basic", secret)},
		"bad encoding":    {"Authorization", "Basic " + "czElemo6eA=="},
	}
	for name, header := range cases {
		rec := f.post(t, introspectURL, url.Values{"token": {"at-1"}}, header...)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, name)
		assert.Equal(t, "invalid_client", oauthErr(t, rec), name)
	}
	rec := f.post(t, introspectURL, url.Values{"token": {"at-1"}, "client_id": {"s1"}})
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "none is not a method here")

	rec = f.post(t, introspectURL, url.Values{"token": {"at-1"}}, "Authorization", basic("s1", "wrong"))
	assert.Equal(t, `Basic realm="https://id.example.com/p/acme-web"`, rec.Header().Get("WWW-Authenticate"))

	rec = f.post(t, introspectURL, url.Values{"token": {"at-1"}, "client_secret": {rsSecret}}, "Authorization", basic("s1", rsSecret))
	assert.Equal(t, http.StatusBadRequest, rec.Code, "Basic and post together, as at /token and /revoke")
	assert.Equal(t, "invalid_request", oauthErr(t, rec))
}

// RFC 7662 §2.1, §4: POST with a form only, token required; GET never carries a token into logs.
func TestIntrospect_RequestErrors(t *testing.T) {
	f := newIntrospectFixture(t)
	get := f.do(t, http.MethodGet, introspectURL+"?token=at-1")
	assert.Equal(t, http.StatusMethodNotAllowed, get.Code)
	assert.Equal(t, "POST", get.Header().Get("Allow"))

	rec := f.post(t, introspectURL, url.Values{}, "Authorization", basic("s1", rsSecret))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "invalid_request", oauthErr(t, rec))

	rec = f.post(t, introspectURL, url.Values{"token": {"at-1"}}, "Authorization", basic("s1", rsSecret), "Content-Type", "application/json")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "invalid_request", oauthErr(t, rec))
}

// A broken store answers 503, never a guess about the token.
func TestIntrospect_Unavailable(t *testing.T) {
	f := newIntrospectFixture(t)
	f.callers.err = errors.New("db down")
	rec := f.post(t, introspectURL, url.Values{"token": {"at-1"}}, "Authorization", basic("s1", rsSecret))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	f.callers.err = nil
	f.personal.err = errors.New("db down")
	rec = f.post(t, introspectURL, url.Values{"token": {personalToken}}, "Authorization", basic("s1", rsSecret))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "5", rec.Header().Get("Retry-After"))

	f.grants.err = errors.New("db down")
	rec = f.post(t, introspectURL, url.Values{"token": {"at-1"}}, "Authorization", basic("s1", rsSecret))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

// one rule per client IP, counted before authentication, and one per authenticated resource server.
func TestIntrospect_RateLimited(t *testing.T) {
	f := newIntrospectFixture(t)
	f.as(t, "s1", "at-1")
	p := "project:" + f.acme.ProjectID.String()
	require.Len(t, f.limiter.keys, 2)
	assert.Contains(t, f.limiter.keys[0], "oauth-introspect-ip:"+p)
	assert.Contains(t, f.limiter.keys[1], "oauth-introspect-server:"+p)
	assert.Contains(t, f.limiter.keys[1], "server:s1")

	f.limiter.keys = nil
	f.post(t, introspectURL, url.Values{"token": {"at-1"}}, "Authorization", basic("s1", "wrong"))
	assert.Len(t, f.limiter.keys, 1, "a failed authentication counts on the IP rule only")

	f.limiter.allow = false
	rec := f.post(t, introspectURL, url.Values{"token": {"at-1"}}, "Authorization", basic("s1", rsSecret))
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}

// discovery advertises the endpoint and its two methods only when introspection is wired.
func TestDiscovery_IntrospectionWhenWired(t *testing.T) {
	f := newIntrospectFixture(t)
	f.resolver.WithIntrospection()
	var doc map[string]any
	require.NoError(t, json.Unmarshal(f.do(t, http.MethodGet, "/p/acme-web/.well-known/openid-configuration").Body.Bytes(), &doc))
	assert.Equal(t, "https://id.example.com/p/acme-web/oauth2/introspect", doc["introspection_endpoint"])
	assert.Equal(t, []any{"client_secret_basic", "client_secret_post"}, doc["introspection_endpoint_auth_methods_supported"])

	plain := newFixture(t, "")
	body := plain.do(t, http.MethodGet, "/p/acme-web/.well-known/openid-configuration").Body.String()
	assert.NotContains(t, body, "introspection")
	assert.Equal(t, http.StatusNotFound, plain.do(t, http.MethodPost, introspectURL).Code, "not mounted without Introspection")
}

// a personal token is no userinfo token, and /revoke leaves it alone with 200.
func TestPersonalTokens_ElsewhereRefused(t *testing.T) {
	f := newIntrospectFixture(t)
	f.grants.access = nil
	rec := f.userinfo(t, http.MethodGet, "", "Authorization", "Bearer "+personalToken)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Header().Get("WWW-Authenticate"), `error="invalid_token"`)

	rec = f.post(t, revokeURL, url.Values{"token": {personalToken}}, "Authorization", basic("basic", secret))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.False(t, f.personal.active)
}
