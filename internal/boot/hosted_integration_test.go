//go:build integration

package boot

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/config"
	"altalune.id/othra/internal/auth"
	"altalune.id/othra/internal/kernel/session"
	"altalune.id/othra/internal/kernel/tenant"
)

// SECURITY: O5, R1, an admin who owes Othra's terms is sent to /welcome from admin pages, but a hosted /p/<handle> page never redirects and sets no admin cookie.
func TestHostedPages_IgnoreTheAdminSessionAndWelcomeGate(t *testing.T) {
	cfg := devLocalConfig()
	cfg.HTTP.StateSecret = strings.Repeat("s", 40)
	cfg.Auth.SessionTTL = 24 * time.Hour
	cfg.Auth.Genesis = config.GenesisConfig{Email: "root@example.com", Password: "genesis-password"}
	cfg.Compliance.RequireAcceptance = true
	svcs, cfg := migratedServices(t, cfg)
	svcs.Caps.EnvGenesis = true
	require.NoError(t, firstBoot(t.Context(), cfg, svcs, testLogger(), false))
	public, hosted := buildIssuerRoutes(cfg, svcs, testLogger(), nil)
	h := buildHTTPHandler(httpSurfaces{
		routes: buildRoutes(cfg, svcs, nil, nil), public: public, hosted: hosted, session: buildSessionMiddleware(svcs, nil),
		requireAcceptance: true, termsUpdated: svcs.Legal.TermsUpdatedAt(),
	})

	ctx := t.Context()
	p, err := svcs.Auth.LoginLocal(ctx, auth.Credentials{Email: "root@example.com", Password: "genesis-password"})
	require.NoError(t, err)
	p.TermsAcceptedAt = time.Time{}
	require.NoError(t, svcs.Sessions.Save(ctx, "admin-sid", p, time.Now().Add(time.Hour)))
	admin := &http.Cookie{Name: session.CookieName, Value: session.Sign(svcs.StateSecret, "admin-sid")}

	sys, err := svcs.Orgs.System(ctx)
	require.NoError(t, err)
	projects, err := svcs.Projects.List(tenant.Into(ctx, tenant.Context{OrgID: sys.ID}), sys.ID)
	require.NoError(t, err)
	require.NotEmpty(t, projects)
	handle := projects[0].IssuerHandle

	gated := serve(h, httptest.NewRequest(http.MethodGet, "/somewhere", nil), admin)
	require.Equal(t, http.StatusSeeOther, gated.Code, "the admin owes the terms, so admin pages redirect")
	require.Contains(t, gated.Header().Get("Location"), "/welcome")

	rec := serve(h, httptest.NewRequest(http.MethodGet, "/p/"+handle+"/oauth2/authorize", nil), admin)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "the hosted page answers itself, not /welcome")
	assert.Contains(t, rec.Body.String(), "isn")
	assert.Empty(t, rec.Header().Get("Location"))
	assert.Empty(t, rec.Header().Values("Set-Cookie"), "no admin session is touched or refreshed")
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "no-referrer", rec.Header().Get("Referrer-Policy"))

	assert.Equal(t, http.StatusNotFound, serve(h, httptest.NewRequest(http.MethodGet, "/p/no-such-handle/oauth2/authorize", nil), admin).Code)
	docs := serve(h, httptest.NewRequest(http.MethodGet, "/p/"+handle+"/.well-known/openid-configuration", nil), admin)
	assert.Equal(t, http.StatusOK, docs.Code)
	assert.Equal(t, "public, max-age=300", docs.Header().Get("Cache-Control"), "the issuer documents stay public")
}
