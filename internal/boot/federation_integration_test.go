//go:build integration

package boot

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/config"
	"altalune.id/othra/internal/federation"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/upstream"
	"altalune.id/othra/oidcrp/oidcrptest"
)

// M7 T2: sign in with Google end to end: the login button, the sealed start, the Continue hop, the callback and the account.
func TestFederation_SignInWithGoogle(t *testing.T) {
	idp := oidcrptest.New(t)
	f := newSignInFixture(t, func(c *config.Config) {
		c.Upstreams.Google.IssuerOverride = idp.Issuer()
		c.Upstreams.AllowPrivateHosts = true
	})
	require.NotNil(t, f.svcs.Federation)
	f.svcs.Federation.WithConfig(federation.Config{GoogleIssuerOverride: idp.Issuer(), HTTPClient: idp.Client()})
	p := f.project("fed", "Fed")
	ctx := tenant.Into(t.Context(), tenant.Context{OrgID: p.OrgID, ProjectID: p.ID})
	require.NoError(t, f.svcs.Upstreams.Seed(ctx, p.OrgID, p.ID, upstream.KindGoogle, "cid.apps", "s3cret"))
	base := "/p/" + p.IssuerHandle

	signIn := func(c *http.Client) signInPage {
		login := f.do(c, http.MethodGet, base+"/login?add=1", nil)
		require.Equal(t, http.StatusOK, login.status, login.body)
		started := f.submit(c, login, "/federation/google/start", nil)
		require.Equal(t, http.StatusSeeOther, started.status, started.body)
		hop := f.do(c, http.MethodGet, started.header.Get("Location"), nil)
		require.Equal(t, http.StatusOK, hop.status, hop.body)
		i := strings.Index(hop.body, idp.Issuer()+"/authorize?")
		require.GreaterOrEqual(t, i, 0, hop.body)
		raw := hop.body[i:]
		raw = strings.ReplaceAll(raw[:strings.IndexAny(raw, `"'`)], "&amp;", "&")
		authURL, err := url.Parse(raw)
		require.NoError(t, err)
		assert.Equal(t, f.srv.URL+base+"/federation/google/callback", authURL.Query().Get("redirect_uri"), "P11")
		return f.do(c, http.MethodGet, base+"/federation/google/callback?"+idp.Authorize(raw).Encode(), nil)
	}

	done := signIn(f.c)
	require.Equal(t, http.StatusSeeOther, done.status, done.body)
	assert.Equal(t, base+"/account/", done.header.Get("Location"))
	status, me := f.profile(f.c, base)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "ana@gmail.com", me.Profile.Email, "signed in as the Google account's end user")

	var endUser, issuer, method string
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT i.end_user_id, i.issuer, s.method FROM end_user_identities i
		JOIN user_sessions s ON s.end_user_id = i.end_user_id WHERE i.project_id = $1`, p.ID).Scan(&endUser, &issuer, &method))
	assert.Equal(t, idp.Issuer(), issuer)
	assert.Equal(t, "google", method)

	again := signIn(f.client())
	require.Equal(t, http.StatusSeeOther, again.status, again.body)
	var users int
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM end_users WHERE project_id = $1`, p.ID).Scan(&users))
	assert.Equal(t, 1, users, "the same (iss, sub) on another device is the same end user")
}
