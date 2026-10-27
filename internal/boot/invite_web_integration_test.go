//go:build integration

package boot

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/config"
	"github.com/altalune-id/othra/internal/auth"
	"github.com/altalune-id/othra/internal/kernel/session"
	"github.com/altalune-id/othra/internal/org/member"
	"github.com/altalune-id/othra/internal/user"
)

func inviteCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == "othra_invite" {
			return c
		}
	}
	t.Fatalf("no invite cookie in %v", rec.Result().Cookies())
	return nil
}

// SECURITY: the invite pages run before sign-in, as the row level security role, so the invite must resolve through its definer and the join or accept must run in the invite's org.
func TestInviteWeb_JoinAndAcceptAsTheServiceRole(t *testing.T) {
	cfg := devLocalConfig()
	cfg.HTTP.StateSecret = strings.Repeat("s", 40)
	cfg.HTTP.BaseURL = "https://othra.example.com"
	cfg.Secrets.EncryptionKey = testKeyHex
	cfg.Auth.SessionTTL = 24 * time.Hour
	cfg.Auth.Genesis = config.GenesisConfig{Email: "root@example.com", Password: "genesis-password"}
	svcs, cfg := migratedServices(t, cfg)
	svcs.Caps.EnvGenesis = true
	require.NoError(t, firstBoot(t.Context(), cfg, svcs, testLogger(), false))
	h := buildHTTPHandler(httpSurfaces{routes: buildRoutes(cfg, svcs, nil, nil, nil), session: buildSessionMiddleware(svcs, nil)})
	ctx := t.Context()
	sys, err := svcs.Orgs.System(ctx)
	require.NoError(t, err)
	open := func(email string) *http.Cookie {
		t.Helper()
		c, err := svcs.Invites.Create(ctx, member.Requester{Operator: true}, sys.ID, email, member.RoleMember)
		require.NoError(t, err)
		link := serve(h, httptest.NewRequest(http.MethodGet, strings.TrimPrefix(c.Link, cfg.HTTP.BaseURL), nil))
		require.Equal(t, http.StatusSeeOther, link.Code, link.Body.String())
		return inviteCookie(t, link)
	}

	t.Run("a new person joins", func(t *testing.T) {
		ck := open("newcomer@example.com")
		page := serve(h, httptest.NewRequest(http.MethodGet, "/invite", nil), ck)
		require.Equal(t, http.StatusOK, page.Code, page.Body.String())
		assert.Contains(t, page.Body.String(), `action="/invite/join"`)

		rec := serve(h, formPost("/invite/join", url.Values{"name": {"Newcomer"}, "password": {"a sturdy passphrase"}, "accept_terms": {"1"}}), ck)
		require.Equal(t, http.StatusSeeOther, rec.Code, rec.Body.String())
		u, err := svcs.Users.ByEmail(ctx, "newcomer@example.com")
		require.NoError(t, err)
		m, err := svcs.Members.Find(ctx, sys.ID, u.ID)
		require.NoError(t, err)
		assert.Equal(t, member.RoleMember, m.Role)
	})

	t.Run("an existing person accepts", func(t *testing.T) {
		u, err := svcs.Users.Create(ctx, user.CreateRequest{Email: "existing@example.com", Name: "Existing", Password: "quiet forest lamp nine"})
		require.NoError(t, err)
		ck := open("existing@example.com")
		p, err := svcs.Auth.LoginLocal(ctx, auth.Credentials{Email: "existing@example.com", Password: "quiet forest lamp nine"})
		require.NoError(t, err)
		require.NoError(t, svcs.Sessions.Save(ctx, "existing-sid", p, time.Now().Add(time.Hour)))
		s := &http.Cookie{Name: session.CookieName, Value: session.Sign(svcs.StateSecret, "existing-sid")}

		page := serve(h, httptest.NewRequest(http.MethodGet, "/invite", nil), ck, s)
		require.Equal(t, http.StatusOK, page.Code, page.Body.String())
		assert.Contains(t, page.Body.String(), `action="/invite/accept"`)

		rec := serve(h, formPost("/invite/accept", nil), ck, s)
		require.Equal(t, http.StatusSeeOther, rec.Code, rec.Body.String())
		m, err := svcs.Members.Find(ctx, sys.ID, u.ID)
		require.NoError(t, err)
		assert.Equal(t, member.RoleMember, m.Role)
	})
}
