//go:build integration

package boot

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/config"
	"altalune.id/othra/internal/kernel"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/testutil/pgtest"
)

func newIssuerServer(t *testing.T, basePath string) (*httptest.Server, *Services) {
	t.Helper()
	cfg := devLocalConfig()
	cfg.HTTP.BaseURL = "https://id.example.test"
	cfg.HTTP.BasePath = basePath
	cfg.HTTP.StateSecret = strings.Repeat("s", 40)
	cfg.Secrets.EncryptionKey = testKeyHex
	cfg.Auth.Genesis = config.GenesisConfig{Email: "root@example.com", Password: "genesis-password"}
	pool, _, schemaName := pgtest.MigratedRLS(t)
	cfg.Postgres.Schema = schemaName
	svcs := buildServices(kernel.Kernel{Pool: pool, Reporter: apperror.NewReporter(nil, false)}, cfg)
	require.NoError(t, buildAuthSecrets(cfg, svcs, testLogger()))
	svcs.Caps.EnvGenesis = true
	require.NoError(t, firstBoot(t.Context(), cfg, svcs, testLogger(), false))
	required, err := installRequired(t.Context(), svcs)
	require.NoError(t, err)

	public, pages := buildIssuerRoutes(cfg, svcs, nil, nil)
	ts := httptest.NewServer(buildHTTPHandler(httpSurfaces{
		basePath:        basePath,
		routes:          append(buildRoutes(cfg, svcs, nil, nil), pages...),
		public:          public,
		session:         buildSessionMiddleware(svcs, nil),
		installRequired: required,
	}))
	t.Cleanup(ts.Close)
	return ts, svcs
}

type fetched struct {
	StatusCode int
	Header     http.Header
}

func fetch(t *testing.T, method, url string) (fetched, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, url, nil)
	require.NoError(t, err)
	req.Header.Set("Origin", "https://app.example")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, resp.Body.Close())
	require.NoError(t, err)
	return fetched{StatusCode: resp.StatusCode, Header: resp.Header}, string(body)
}

// K9, K12, K14 through boot: each issuer serves its documents and stubs, under basePath, with no cookie and a public cache.
func TestIssuerRoutesThroughBoot(t *testing.T) {
	for _, basePath := range []string{"", "/auth"} {
		t.Run("basePath="+basePath, func(t *testing.T) {
			ts, svcs := newIssuerServer(t, basePath)
			sys, err := svcs.Orgs.System(t.Context())
			require.NoError(t, err)
			p, err := svcs.Projects.Create(t.Context(), project.CreateRequest{OrgID: sys.ID, Slug: "web", Name: "Web", IssuerHandle: "acme-web"})
			require.NoError(t, err)
			issuer := "https://id.example.test" + basePath + "/p/acme-web"
			local := func(u string) string { return ts.URL + strings.TrimPrefix(u, "https://id.example.test") }

			resp, oidcBody := fetch(t, http.MethodGet, ts.URL+basePath+"/p/acme-web/.well-known/openid-configuration")
			require.Equal(t, http.StatusOK, resp.StatusCode, oidcBody)
			assert.Equal(t, "public, max-age=300", resp.Header.Get("Cache-Control"))
			assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
			assert.Empty(t, resp.Header.Values("Set-Cookie"))
			_, rfcBody := fetch(t, http.MethodGet, ts.URL+"/.well-known/oauth-authorization-server"+basePath+"/p/acme-web")
			assert.Equal(t, oidcBody, rfcBody, "the RFC 8414 path at the host root serves the same document")

			var doc map[string]any
			require.NoError(t, json.Unmarshal([]byte(oidcBody), &doc))
			assert.Equal(t, issuer, doc["issuer"])
			assert.Equal(t, issuer+"/oauth2/jwks.json", doc["jwks_uri"])
			assert.Equal(t, []any{"query"}, doc["response_modes_supported"])

			resp, jwks := fetch(t, http.MethodGet, local(doc["jwks_uri"].(string)))
			require.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "application/jwk-set+json", resp.Header.Get("Content-Type"))
			var set struct {
				Keys []map[string]any `json:"keys"`
			}
			require.NoError(t, json.Unmarshal([]byte(jwks), &set))
			require.NotEmpty(t, set.Keys, "the project got its key in the same unit of work as its creation")
			for _, k := range set.Keys {
				assert.Equal(t, "RSA", k["kty"])
				assert.Equal(t, "RS256", k["alg"])
				for _, private := range []string{"d", "p", "q", "dp", "dq", "qi"} {
					assert.NotContains(t, k, private, "SECURITY: no private member is ever published")
				}
			}
			before := resp.Header.Get("ETag")

			resp, body := fetch(t, http.MethodPost, local(doc["token_endpoint"].(string)))
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			assert.Equal(t, `Basic realm="`+issuer+`"`, resp.Header.Get("WWW-Authenticate"))
			assert.JSONEq(t, `{"error":"invalid_client"}`, body)
			resp, body = fetch(t, http.MethodGet, local(doc["authorization_endpoint"].(string)))
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
			assert.Contains(t, body, "This app isn&#39;t registered")
			assert.Empty(t, resp.Header.Get("Location"))

			for _, path := range []string{"/p/nobody-home/oauth2/jwks.json", "/p/ACME-WEB/.well-known/openid-configuration", "/p/system/oauth2/jwks.json"} {
				resp, body := fetch(t, http.MethodGet, ts.URL+basePath+path)
				assert.Equal(t, http.StatusNotFound, resp.StatusCode, path)
				assert.JSONEq(t, `{"error":"not_found"}`, body, path)
			}

			_, err = svcs.Keys.Rotate(t.Context(), sys.ID, p.ID, false)
			require.NoError(t, err)
			resp, jwks = fetch(t, http.MethodGet, local(doc["jwks_uri"].(string)))
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.NoError(t, json.Unmarshal([]byte(jwks), &set))
			assert.Len(t, set.Keys, 2, "a rotation publishes the next key at once, past the 30s cache")
			assert.NotEqual(t, before, resp.Header.Get("ETag"))
		})
	}
}
