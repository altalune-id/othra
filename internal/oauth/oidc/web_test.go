package oidc_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/kernel/ratelimit"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/oauth/oidc"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/web/layout"
)

const jwksBody = `{"keys":[{"kty":"RSA","kid":"k1","use":"sig","alg":"RS256","n":"AQAB","e":"AQAB"}]}`

type issuers struct {
	mu       sync.Mutex
	byHandle map[string]*project.Issuer
	calls    int
	err      error
}

func (f *issuers) ResolveIssuer(_ context.Context, handle string) (*project.Issuer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	i, ok := f.byHandle[handle]
	if !ok {
		return nil, &project.NotFoundError{}
	}
	return i, nil
}

type publisher struct {
	mu    sync.Mutex
	body  string
	calls int
	scope tenant.Context
	err   error
}

func (f *publisher) JWKS(ctx context.Context, _ uuid.UUID) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.scope, _ = tenant.From(ctx)
	if f.err != nil {
		return nil, f.err
	}
	return []byte(f.body), nil
}

type limiter struct {
	allow bool
	err   error
	keys  []string
}

func (l *limiter) Allow(_ context.Context, key string, _ ratelimit.Rule) (ratelimit.Result, error) {
	l.keys = append(l.keys, key)
	if l.err != nil {
		return ratelimit.Result{}, l.err
	}
	return ratelimit.Result{Allowed: l.allow, RetryAfter: 2 * time.Second}, nil
}

func (l *limiter) Reset(context.Context, string) error { return nil }

type fixture struct {
	issuers   *issuers
	keys      *publisher
	web       *oidc.Web
	mux       *http.ServeMux
	resolver  *oidc.Resolver
	acme      *project.Issuer
	reported  []string
	corsCalls int
}

func newFixture(t *testing.T, basePath string) *fixture {
	t.Helper()
	acme := &project.Issuer{ProjectID: uuid.New(), OrgID: uuid.New(), Handle: "acme-web", Name: "Web", DisplayName: "Acme"}
	f := &fixture{
		issuers: &issuers{byHandle: map[string]*project.Issuer{"acme-web": acme}},
		keys:    &publisher{body: jwksBody},
		acme:    acme,
	}
	kit := layout.Kit{BasePath: basePath, Unexpected: func(_ context.Context, msg string, err error, _ ...any) *apperror.AppError {
		f.reported = append(f.reported, msg)
		return apperror.New(apperror.CodeUnexpected, "unexpected").WithCause(err)
	}}
	f.resolver = oidc.NewResolver(f.issuers, f.keys, "https://id.example.com"+basePath, kit.Unexpected)
	f.web = oidc.NewWeb(f.resolver, kit, oidc.Config{
		PublicBase: "https://id.example.com" + basePath,
		BasePath:   basePath,
		CORS: func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				f.corsCalls++
				w.Header().Set("Access-Control-Allow-Origin", "*")
				next.ServeHTTP(w, r)
			})
		},
	})
	f.mux = http.NewServeMux()
	f.web.Register(f.mux)
	return f
}

func (f *fixture) do(t *testing.T, method, path string, header ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, req)
	return rec
}

func TestDiscovery_GoldenAndIdenticalOnBothPaths(t *testing.T) {
	for _, basePath := range []string{"", "/auth"} {
		f := newFixture(t, basePath)
		oidcDoc := f.do(t, http.MethodGet, basePath+"/p/acme-web/.well-known/openid-configuration")
		rfc8414 := f.do(t, http.MethodGet, "/.well-known/oauth-authorization-server"+basePath+"/p/acme-web")
		for _, rec := range []*httptest.ResponseRecorder{oidcDoc, rfc8414} {
			require.Equal(t, http.StatusOK, rec.Code, basePath)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			assert.Equal(t, "public, max-age=300", rec.Header().Get("Cache-Control"))
			assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
			assert.NotEmpty(t, rec.Header().Get("ETag"))
		}
		assert.Equal(t, oidcDoc.Body.String(), rfc8414.Body.String(), "one builder, the same bytes on both paths")
		assert.Equal(t, oidcDoc.Header().Get("ETag"), rfc8414.Header().Get("ETag"))

		golden, err := os.ReadFile(filepath.Join("testdata", "discovery"+strings.ReplaceAll(basePath, "/", "-")+".json"))
		require.NoError(t, err)
		assert.JSONEq(t, string(golden), oidcDoc.Body.String())
		assert.Equal(t, strings.TrimSpace(string(golden)), oidcDoc.Body.String(), "byte-identical to the golden file")
	}
}

func TestDiscovery_HasEveryRequiredField(t *testing.T) {
	f := newFixture(t, "")
	var doc map[string]any
	require.NoError(t, json.Unmarshal(f.do(t, http.MethodGet, "/p/acme-web/.well-known/openid-configuration").Body.Bytes(), &doc))

	oidcRequired := []string{
		"issuer", "authorization_endpoint", "token_endpoint", "jwks_uri",
		"response_types_supported", "subject_types_supported", "id_token_signing_alg_values_supported",
	}
	rfc8414Required := []string{"issuer", "authorization_endpoint", "token_endpoint", "response_types_supported"}
	for _, field := range append(oidcRequired, rfc8414Required...) {
		assert.NotEmpty(t, doc[field], field)
	}
	issuer, _ := doc["issuer"].(string)
	assert.Equal(t, "https://id.example.com/p/acme-web", issuer)
	assert.NotContains(t, issuer, "?")
	assert.NotContains(t, issuer, "#")
	assert.False(t, strings.HasSuffix(issuer, "/"))
	for _, field := range []string{"authorization_endpoint", "token_endpoint", "jwks_uri", "userinfo_endpoint", "end_session_endpoint", "revocation_endpoint"} {
		u, _ := doc[field].(string)
		assert.True(t, strings.HasPrefix(u, issuer+"/"), "%s lives under the issuer: %s", field, u)
	}
	assert.Equal(t, []any{"RS256"}, doc["id_token_signing_alg_values_supported"])
	assert.Equal(t, []any{"S256"}, doc["code_challenge_methods_supported"])
	assert.Equal(t, []any{"query"}, doc["response_modes_supported"], "the profile bans the fragment mode the default would imply")
	assert.Equal(t, []any{"client_secret_basic", "client_secret_post", "none"}, doc["token_endpoint_auth_methods_supported"])
	assert.Equal(t, []any{"authorization_code", "refresh_token"}, doc["grant_types_supported"])
	assert.Equal(t, true, doc["authorization_response_iss_parameter_supported"], "RFC 9207")
	assert.Equal(t, []any{"none", "login", "consent", "select_account"}, doc["prompt_values_supported"], "M6: the consent page shows on prompt=consent")
	assert.Equal(t, issuer+"/oauth2/revoke", doc["revocation_endpoint"])
	assert.Equal(t, []any{"client_secret_basic", "client_secret_post", "none"}, doc["revocation_endpoint_auth_methods_supported"])
	assert.Equal(t, false, doc["client_id_metadata_document_supported"], "CIMD §6: always sent, false until a verified server admits CIMD apps")
	assert.Equal(t, []any{"en-US", "id-ID"}, doc["ui_locales_supported"], "the hosted pages' locales")
	for _, field := range []string{"request_parameter_supported", "request_uri_parameter_supported", "claims_parameter_supported"} {
		assert.Equal(t, false, doc[field], "%s: request objects and the claims parameter are refused", field)
	}
	assert.Len(t, doc, 24, "nothing is advertised beyond what M6 answers")
}

// K14: every URL a discovery document advertises answers, or answers the spec error; never 404 or 500.
func TestDiscovery_EveryAdvertisedURLAnswers(t *testing.T) {
	for _, basePath := range []string{"", "/auth"} {
		f := newFixture(t, basePath)
		var doc oidc.Document
		require.NoError(t, json.Unmarshal(f.do(t, http.MethodGet, basePath+"/p/acme-web/.well-known/openid-configuration").Body.Bytes(), &doc))
		local := func(u string) string { return strings.TrimPrefix(u, "https://id.example.com") }

		assert.Equal(t, http.StatusOK, f.do(t, http.MethodGet, local(doc.Issuer)+"/.well-known/openid-configuration").Code)
		assert.Equal(t, http.StatusOK, f.do(t, http.MethodGet, "/.well-known/oauth-authorization-server"+local(doc.Issuer)).Code)
		assert.Equal(t, http.StatusOK, f.do(t, http.MethodGet, local(doc.JWKSURI)).Code)
		token := f.do(t, http.MethodPost, local(doc.TokenEndpoint))
		assert.Equal(t, http.StatusUnauthorized, token.Code)
		assert.JSONEq(t, `{"error":"invalid_client"}`, token.Body.String())
		userinfo := f.do(t, http.MethodGet, local(doc.UserinfoEndpoint))
		assert.Equal(t, http.StatusUnauthorized, userinfo.Code)
		assert.Contains(t, userinfo.Header().Get("WWW-Authenticate"), "Bearer")
	}
}

func TestJWKS_ServesTheProjectKeysWithAnETag(t *testing.T) {
	f := newFixture(t, "")
	rec := f.do(t, http.MethodGet, "/p/acme-web/oauth2/jwks.json")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, jwksBody, rec.Body.String())
	assert.Equal(t, "application/jwk-set+json", rec.Header().Get("Content-Type"))
	assert.Equal(t, "public, max-age=300", rec.Header().Get("Cache-Control"))
	etag := rec.Header().Get("ETag")
	assert.Regexp(t, `^"[A-Za-z0-9_-]{43}"$`, etag, "a strong ETag of base64url sha256")
	assert.Equal(t, tenant.Context{OrgID: f.acme.OrgID, ProjectID: f.acme.ProjectID}, f.keys.scope, "keys are read under the project's scope")

	for _, inm := range []string{etag, `"x", ` + etag, "*", "W/" + etag} {
		notModified := f.do(t, http.MethodGet, "/p/acme-web/oauth2/jwks.json", "If-None-Match", inm)
		assert.Equal(t, http.StatusNotModified, notModified.Code, inm)
		assert.Empty(t, notModified.Body.String())
		assert.Equal(t, etag, notModified.Header().Get("ETag"))
	}
	assert.Equal(t, http.StatusOK, f.do(t, http.MethodGet, "/p/acme-web/oauth2/jwks.json", "If-None-Match", `"stale"`).Code)

	head := f.do(t, http.MethodHead, "/p/acme-web/oauth2/jwks.json")
	assert.Equal(t, http.StatusOK, head.Code)
	assert.Empty(t, head.Body.String())
	assert.Equal(t, etag, head.Header().Get("ETag"))

	post := f.do(t, http.MethodPost, "/p/acme-web/oauth2/jwks.json")
	assert.Equal(t, http.StatusMethodNotAllowed, post.Code)
	assert.Equal(t, "GET, HEAD, OPTIONS", post.Header().Get("Allow"))
	assert.Positive(t, f.corsCalls, "public documents go through the CORS middleware")
}

// K13, SECURITY: an unknown, reserved or malformed handle answers one identical 404, so probing learns nothing.
func TestUnknownHandlesAnswerOneIdentical404(t *testing.T) {
	f := newFixture(t, "")
	var first *httptest.ResponseRecorder
	for _, handle := range []string{"nobody-here", "system", "Acme-Web", "ACME-WEB", "a_b", "ab", strings.Repeat("a", 41), "acme-web-"} {
		for _, path := range []string{
			"/p/" + handle + "/oauth2/jwks.json",
			"/p/" + handle + "/.well-known/openid-configuration",
			"/.well-known/oauth-authorization-server/p/" + handle,
		} {
			rec := f.do(t, http.MethodGet, path)
			assert.Equal(t, http.StatusNotFound, rec.Code, path)
			if first == nil {
				first = rec
				continue
			}
			assert.Equal(t, first.Body.String(), rec.Body.String(), path)
			assert.Equal(t, first.Header().Get("Content-Type"), rec.Header().Get("Content-Type"), path)
			assert.Equal(t, first.Header().Get("Cache-Control"), rec.Header().Get("Cache-Control"), path)
			assert.Empty(t, rec.Header().Get("Location"), "never a redirect to the lowercase handle")
		}
		token := f.do(t, http.MethodPost, "/p/"+handle+"/oauth2/token")
		assert.Equal(t, http.StatusNotFound, token.Code)
		assert.Equal(t, first.Body.String(), token.Body.String())
	}
	assert.JSONEq(t, `{"error":"not_found"}`, first.Body.String())
	assert.Equal(t, 4, f.issuers.calls, "only the well-formed unknown handle reaches the database, once per request")
}

func TestTokenStubRefusesEveryClient(t *testing.T) {
	f := newFixture(t, "")
	rec := f.do(t, http.MethodPost, "/p/acme-web/oauth2/token")
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "RFC 6749 §5.2: invalid_client is a 401")
	assert.Equal(t, `Basic realm="https://id.example.com/p/acme-web"`, rec.Header().Get("WWW-Authenticate"))
	assert.JSONEq(t, `{"error":"invalid_client"}`, rec.Body.String())
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "no-cache", rec.Header().Get("Pragma"))

	get := f.do(t, http.MethodGet, "/p/acme-web/oauth2/token")
	assert.Equal(t, http.StatusMethodNotAllowed, get.Code)
	assert.Equal(t, "POST, OPTIONS", get.Header().Get("Allow"))
}

func TestIssuerCache(t *testing.T) {
	f := newFixture(t, "")
	now := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	f.resolver.SetClock(func() time.Time { return now })

	for range 3 {
		f.do(t, http.MethodGet, "/p/acme-web/oauth2/jwks.json")
		f.do(t, http.MethodGet, "/p/acme-web/.well-known/openid-configuration")
	}
	assert.Equal(t, 1, f.issuers.calls, "one lookup serves both documents for the TTL")
	assert.Equal(t, 1, f.keys.calls)

	now = now.Add(31 * time.Second)
	f.do(t, http.MethodGet, "/p/acme-web/oauth2/jwks.json")
	assert.Equal(t, 2, f.issuers.calls, "the entry expires after 30 seconds")

	f.keys.body = `{"keys":[]}`
	f.resolver.Forget(f.acme.ProjectID)
	rec := f.do(t, http.MethodGet, "/p/acme-web/oauth2/jwks.json")
	assert.JSONEq(t, `{"keys":[]}`, rec.Body.String(), "a rotation drops the cached keys at once")
	f.resolver.Forget(uuid.New())
	assert.Equal(t, 3, f.issuers.calls)
}

// R-10: a renamed handle stops answering at once, not when its cache entry expires.
func TestIssuerCache_AHandleChangeDropsTheOldHandle(t *testing.T) {
	f := newFixture(t, "")
	require.Equal(t, http.StatusOK, f.do(t, http.MethodGet, "/p/acme-web/.well-known/openid-configuration").Code)
	renamed := *f.acme
	renamed.Handle = "acme-shop"
	f.issuers.mu.Lock()
	f.issuers.byHandle = map[string]*project.Issuer{"acme-shop": &renamed}
	f.issuers.mu.Unlock()
	assert.Equal(t, http.StatusOK, f.do(t, http.MethodGet, "/p/acme-web/.well-known/openid-configuration").Code, "still cached")

	f.resolver.Forget(f.acme.ProjectID)
	assert.Equal(t, http.StatusNotFound, f.do(t, http.MethodGet, "/p/acme-web/.well-known/openid-configuration").Code)
	rec := f.do(t, http.MethodGet, "/p/acme-shop/.well-known/openid-configuration")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"issuer":"https://id.example.com/p/acme-shop"`)
}

func TestLookupsAreRateLimitedPerClient(t *testing.T) {
	f := newFixture(t, "")
	lim := &limiter{allow: false}
	f.resolver.WithLimiter(lim, 0)

	rec := f.do(t, http.MethodGet, "/p/acme-web/oauth2/jwks.json")
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "2", rec.Header().Get("Retry-After"))
	assert.JSONEq(t, `{"error":"too_many_requests"}`, rec.Body.String())
	assert.Zero(t, f.issuers.calls, "a refused lookup never reaches the database")
	require.Len(t, lim.keys, 1)
	assert.Equal(t, "issuer-lookup-ip:ip:192.0.2.1", lim.keys[0])

	lim.allow = true
	require.Equal(t, http.StatusOK, f.do(t, http.MethodGet, "/p/acme-web/oauth2/jwks.json").Code)
	lim.allow = false
	assert.Equal(t, http.StatusOK, f.do(t, http.MethodGet, "/p/acme-web/oauth2/jwks.json").Code, "a cached issuer is never limited")

	lim.err = errors.New("redis down")
	f.resolver.Forget(f.acme.ProjectID)
	assert.Equal(t, http.StatusOK, f.do(t, http.MethodGet, "/p/acme-web/oauth2/jwks.json").Code, "a broken limiter never takes the keys offline")
	assert.Contains(t, f.reported, "oidc: rate limit")
}

func TestFailuresAnswer503AndAreReported(t *testing.T) {
	f := newFixture(t, "")
	f.keys.err = errors.New("db down")
	rec := f.do(t, http.MethodGet, "/p/acme-web/oauth2/jwks.json")
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.JSONEq(t, `{"error":"temporarily_unavailable"}`, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.NotContains(t, rec.Body.String(), "db down")

	f.keys.err = nil
	f.issuers.err = errors.New("db down")
	assert.Equal(t, http.StatusServiceUnavailable, f.do(t, http.MethodGet, "/p/acme-web/.well-known/openid-configuration").Code)
	assert.Len(t, f.reported, 2)
}

// The hosted pages share the resolver: typed errors they render as HTML, and the cache the documents fill.
func TestResolver_ResolveForPages(t *testing.T) {
	f := newFixture(t, "")
	req := func(handle string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/p/x/login", nil)
		r.SetPathValue("handle", handle)
		return r
	}
	i, err := f.resolver.Resolve(req("acme-web"))
	require.NoError(t, err)
	assert.Equal(t, f.acme.ProjectID, i.ProjectID)
	require.Equal(t, http.StatusOK, f.do(t, http.MethodGet, "/p/acme-web/oauth2/jwks.json").Code)
	assert.Equal(t, 1, f.issuers.calls, "the documents reuse the issuer the page loaded")

	for _, handle := range []string{"nope-nope", "Bad Handle", "p"} {
		_, err := f.resolver.Resolve(req(handle))
		assert.ErrorIs(t, err, oidc.ErrUnknownIssuer, handle)
	}

	f.resolver.Forget(f.acme.ProjectID)
	f.resolver.WithLimiter(&limiter{allow: false}, 0)
	_, err = f.resolver.Resolve(req("acme-web"))
	limited, ok := errors.AsType[*oidc.LookupLimitedError](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, 2, limited.RetryAfter)
	assert.NotEmpty(t, limited.Error())

	f.resolver.WithLimiter(&limiter{allow: true}, 0)
	f.issuers.err = errors.New("db down")
	_, err = f.resolver.Resolve(req("acme-web"))
	require.Error(t, err)
	assert.NotErrorIs(t, err, oidc.ErrUnknownIssuer)
}

type cimdProjects struct {
	accept bool
	err    error
	scope  tenant.Context
}

func (c *cimdProjects) AcceptsCIMD(ctx context.Context, _, _ uuid.UUID) (bool, error) {
	c.scope, _ = tenant.From(ctx)
	return c.accept, c.err
}

// pm P3, CIMD §6: discovery says true only while the project admits CIMD clients, read under the project's scope.
func TestDiscovery_CIMDFlagFollowsTheProject(t *testing.T) {
	f := newFixture(t, "")
	admissions := &cimdProjects{accept: true}
	f.resolver.WithCIMD(admissions)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(f.do(t, http.MethodGet, "/p/acme-web/.well-known/openid-configuration").Body.Bytes(), &doc))
	assert.Equal(t, true, doc["client_id_metadata_document_supported"])
	assert.Equal(t, f.acme.ProjectID, admissions.scope.ProjectID)

	f.resolver.Forget(f.acme.ProjectID)
	admissions.err = errors.New("db down")
	rec := f.do(t, http.MethodGet, "/p/acme-web/.well-known/openid-configuration")
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
