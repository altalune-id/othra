//go:build integration

package boot

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/internal/client"
	"github.com/altalune-id/othra/internal/grant"
	"github.com/altalune-id/othra/internal/kernel/tenant"
	"github.com/altalune-id/othra/internal/keys"
	"github.com/altalune-id/othra/internal/resourceserver"
)

const (
	oauthCallback  = "https://shop.example/cb"
	oauthBye       = "https://shop.example/bye"
	oauthVerifier  = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	oauthChallenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
)

var continueRE = regexp.MustCompile(`http-equiv="refresh" content="0;url=([^"]+)"`)

func (f *signInFixture) signInForCode(base string, q url.Values, email, clientName string) url.Values {
	f.t.Helper()
	first := f.do(f.c, http.MethodGet, base+"/oauth2/authorize?"+q.Encode(), nil)
	require.Equal(f.t, http.StatusSeeOther, first.status, first.body)
	login := f.do(f.c, http.MethodGet, first.header.Get("Location"), nil)
	require.Equal(f.t, http.StatusOK, login.status, login.body)
	assert.Contains(f.t, login.body, clientName)
	sent := f.submit(f.c, login, "/login", url.Values{"email": {email}})
	require.Equal(f.t, http.StatusSeeOther, sent.status, sent.body)
	link := signInLinkRE.FindString(claimMail(f.t, f.svcs)[0].TextBody)
	moved := f.do(f.c, http.MethodGet, strings.TrimPrefix(link, f.srv.URL), nil)
	verify := f.do(f.c, http.MethodGet, moved.header.Get("Location"), nil)
	done := f.submit(f.c, verify, "/login/verify", nil)
	require.Equal(f.t, http.StatusOK, done.status, "the continue page: %s", done.body)
	m := continueRE.FindStringSubmatch(done.body)
	require.NotEmpty(f.t, m, done.body)
	return f.callback(f.do(f.c, http.MethodGet, strings.TrimPrefix(strings.ReplaceAll(m[1], "&amp;", "&"), f.srv.URL), nil))
}

func (f *signInFixture) callback(p signInPage) url.Values {
	f.t.Helper()
	require.Equal(f.t, http.StatusFound, p.status, p.body)
	loc := p.header.Get("Location")
	require.NotContains(f.t, loc, "error=", loc)
	back, err := url.Parse(loc)
	require.NoError(f.t, err)
	require.Equal(f.t, oauthCallback, back.Scheme+"://"+back.Host+back.Path)
	require.NotEmpty(f.t, back.Query().Get("code"))
	return back.Query()
}

// authorize signs the end user in, returns a code with state and iss, and end-session ends the session the hint names.
func TestHosted_AuthorizeCodeAndEndSession(t *testing.T) {
	f := newSignInFixture(t)
	p := f.project("shop", "Shop")
	base := "/p/" + p.IssuerHandle
	issuer := f.srv.URL + base
	ctx := tenant.Into(t.Context(), tenant.Context{OrgID: p.OrgID, ProjectID: p.ID})
	oc, err := f.svcs.Clients.Create(ctx, p.OrgID, p.ID, client.KindWeb, client.Settings{
		Name: "Shop Web", RedirectURIs: []string{oauthCallback}, PostLogoutRedirectURIs: []string{oauthBye},
		ConsentPolicy: client.ConsentImplicit,
	})
	require.NoError(t, err)

	q := url.Values{
		"client_id": {oc.PublicID}, "redirect_uri": {oauthCallback}, "response_type": {"code"}, "scope": {"openid email"},
		"state": {"st-1"}, "nonce": {"n-1"}, "code_challenge": {oauthChallenge}, "code_challenge_method": {"S256"},
	}
	back := f.signInForCode(base, q, "ana@example.com", "Shop Web")
	assert.Equal(t, "st-1", back.Get("state"))
	assert.Equal(t, issuer, back.Get("iss"))
	code := back.Get("code")
	require.NotEmpty(t, code)

	gp := grant.Project{OrgID: p.OrgID, ID: p.ID}
	tokens, err := f.svcs.Grants.Exchange(ctx, gp, grant.ExchangeRequest{
		Client: grant.Client{ID: oc.ID, OAuthID: oc.PublicID}, Code: code, RedirectURI: oauthCallback, CodeVerifier: oauthVerifier,
	})
	require.NoError(t, err)
	sid := tokens.Family.PublicID
	now := time.Now()
	idToken, err := keys.NewSigner(f.svcs.Keys).MintIDToken(ctx, p.ID, &keys.MintIDTokenInput{
		Issuer: issuer, Subject: "subject-from-token", Audience: []string{oc.PublicID}, ClientID: oc.PublicID, SessionID: sid,
		AuthTime: now, IssuedAt: now.Add(-2 * time.Hour), ExpiresAt: now.Add(-time.Hour),
	})
	require.NoError(t, err)

	forged := f.do(f.c, http.MethodGet, base+"/oauth2/end-session?id_token_hint="+url.QueryEscape(idToken.Compact+"x"), nil)
	assert.Equal(t, http.StatusBadRequest, forged.status)
	assert.Contains(t, forged.body, string(apperror.CodeSignOutHintInvalid))

	_, err = f.svcs.Grants.RevokeFamily(ctx, gp, sid, grant.ReasonSignOut)
	require.NoError(t, err)
	out := url.Values{"id_token_hint": {idToken.Compact}, "post_logout_redirect_uri": {oauthBye}, "state": {"bye-1"}}
	ended := f.do(f.c, http.MethodGet, base+"/oauth2/end-session?"+out.Encode(), nil)
	require.Equal(t, http.StatusFound, ended.status, "an expired hint of a revoked family still signs out: %s", ended.body)
	assert.Equal(t, oauthBye+"?state=bye-1", ended.header.Get("Location"))
	family, err := f.svcs.Grants.Family(ctx, gp, sid)
	require.NoError(t, err)
	assert.Nil(t, family, "the grant family is revoked")

	q.Set("prompt", "none")
	silent := f.do(f.c, http.MethodGet, base+"/oauth2/authorize?"+q.Encode(), nil)
	require.Equal(t, http.StatusFound, silent.status, silent.body)
	assert.Contains(t, silent.header.Get("Location"), "error=login_required", "the browser is signed out too")

	q.Del("prompt")
	q.Set("id_token_hint", idToken.Compact)
	hinted := f.do(f.c, http.MethodGet, base+"/oauth2/authorize?"+q.Encode(), nil)
	require.Equal(t, http.StatusSeeOther, hinted.status, hinted.body)
	assert.NotContains(t, hinted.header.Get("Location"), "id_token_hint", "the hint never reaches return_to")
	relogin := f.do(f.c, http.MethodGet, hinted.header.Get("Location"), nil)
	resent := f.submit(f.c, relogin, "/login", url.Values{"email": {"ana@example.com"}})
	assert.Equal(t, http.StatusSeeOther, resent.status, "the magic link's return_to fits: %s", resent.body)
	for _, m := range claimMail(t, f.svcs) {
		assert.NotContains(t, m.TextBody, "id_token_hint")
	}
}

// OIDC §11: an offline_access grant outlives end-session and sign-out of its user session; the hinted grant and plain ones don't.
func TestHosted_OfflineAccessSurvivesSignOut(t *testing.T) {
	f := newSignInFixture(t)
	p := f.project("shop", "Shop")
	base, issuer := "/p/"+p.IssuerHandle, f.srv.URL+"/p/"+p.IssuerHandle
	ctx := tenant.Into(t.Context(), tenant.Context{OrgID: p.OrgID, ProjectID: p.ID})
	oc, err := f.svcs.Clients.Create(ctx, p.OrgID, p.ID, client.KindWeb, client.Settings{
		Name: "Shop Web", RedirectURIs: []string{oauthCallback}, PostLogoutRedirectURIs: []string{oauthBye},
		ConsentPolicy: client.ConsentImplicit, RefreshTokens: true,
	})
	require.NoError(t, err)
	gp := grant.Project{OrgID: p.OrgID, ID: p.ID}
	gc := grant.Client{ID: oc.ID, OAuthID: oc.PublicID, Refresh: true}
	request := func(scope string) url.Values {
		return url.Values{
			"client_id": {oc.PublicID}, "redirect_uri": {oauthCallback}, "response_type": {"code"}, "scope": {scope},
			"state": {"st"}, "nonce": {"n"}, "code_challenge": {oauthChallenge}, "code_challenge_method": {"S256"},
		}
	}
	exchange := func(back url.Values) string {
		tokens, err := f.svcs.Grants.Exchange(ctx, gp, grant.ExchangeRequest{
			Client: gc, Code: back.Get("code"), RedirectURI: oauthCallback, CodeVerifier: oauthVerifier,
		})
		require.NoError(t, err)
		return tokens.Family.PublicID
	}
	silent := func(scope string) string {
		return exchange(f.callback(f.do(f.c, http.MethodGet, base+"/oauth2/authorize?"+request(scope).Encode(), nil)))
	}
	live := func(sid string) bool {
		fam, err := f.svcs.Grants.Family(ctx, gp, sid)
		require.NoError(t, err)
		return fam != nil
	}

	hinted := exchange(f.signInForCode(base, request("openid email"), "ana@example.com", "Shop Web"))
	offline := silent("openid offline_access")
	plain := silent("openid email")
	now := time.Now()
	idToken, err := keys.NewSigner(f.svcs.Keys).MintIDToken(ctx, p.ID, &keys.MintIDTokenInput{
		Issuer: issuer, Subject: "sub", Audience: []string{oc.PublicID}, ClientID: oc.PublicID, SessionID: hinted,
		AuthTime: now, IssuedAt: now, ExpiresAt: now.Add(time.Hour),
	})
	require.NoError(t, err)
	ended := f.do(f.c, http.MethodGet, base+"/oauth2/end-session?id_token_hint="+url.QueryEscape(idToken.Compact), nil)
	require.Equal(t, http.StatusSeeOther, ended.status, ended.body)
	assert.False(t, live(hinted), "end-session revokes the hinted grant")
	assert.False(t, live(plain), "and the session's plain grants")
	assert.True(t, live(offline), "an offline_access grant survives end-session")

	offline = exchange(f.signInForCode(base, request("openid offline_access"), "ana@example.com", "Shop Web"))
	plain = silent("openid email")
	out := f.portalSignOut(f.c, base)
	require.Equal(t, http.StatusSeeOther, out.status, out.body)
	assert.False(t, live(plain), "sign-out revokes the session's plain grants")
	assert.True(t, live(offline), "an offline_access grant survives sign-out")
}

// resource, consent page, Allow, a code bound to the server; a public client's second sign-in is one click.
func TestHosted_ResourceAndConsent(t *testing.T) {
	f := newSignInFixture(t)
	p := f.project("shop", "Shop")
	base := "/p/" + p.IssuerHandle
	ctx := tenant.Into(t.Context(), tenant.Context{OrgID: p.OrgID, ProjectID: p.ID})
	const callback = "http://127.0.0.1/callback"
	oc, err := f.svcs.Clients.Create(ctx, p.OrgID, p.ID, client.KindCLI, client.Settings{
		Name: "Shop CLI", RedirectURIs: []string{callback}, RefreshTokens: true,
	})
	require.NoError(t, err)
	rs, err := f.svcs.ResourceServers.Create(ctx, resourceserver.CreateRequest{
		OrgID: p.OrgID, ProjectID: p.ID, Name: "Shop MCP", Identifier: "https://mcp.shop.test/mcp", Scopes: []string{"read", "write"},
		Trusted: true, Person: uuid.New(), Descriptions: map[string]string{"read": "Read your orders"},
	})
	require.NoError(t, err)
	_, err = f.svcs.ResourceServers.LinkClient(ctx, p.OrgID, p.ID, oc.PublicID, rs.PublicID, []string{"read"})
	require.NoError(t, err)

	q := url.Values{
		"client_id": {oc.PublicID}, "redirect_uri": {"http://127.0.0.1:53999/callback"}, "response_type": {"code"},
		"scope": {"openid read write offline_access"}, "resource": {rs.Identifier}, "state": {"st"},
		"code_challenge": {oauthChallenge}, "code_challenge_method": {"S256"},
	}
	first := f.do(f.c, http.MethodGet, base+"/oauth2/authorize?"+q.Encode(), nil)
	require.Equal(t, http.StatusSeeOther, first.status, first.body)
	login := f.do(f.c, http.MethodGet, first.header.Get("Location"), nil)
	sent := f.submit(f.c, login, "/login", url.Values{"email": {"ana@example.com"}})
	require.Equal(t, http.StatusSeeOther, sent.status, sent.body)
	link := signInLinkRE.FindString(claimMail(t, f.svcs)[0].TextBody)
	moved := f.do(f.c, http.MethodGet, strings.TrimPrefix(link, f.srv.URL), nil)
	verify := f.do(f.c, http.MethodGet, moved.header.Get("Location"), nil)
	done := f.submit(f.c, verify, "/login/verify", nil)
	m := continueRE.FindStringSubmatch(done.body)
	require.NotEmpty(t, m, done.body)
	toConsent := f.do(f.c, http.MethodGet, strings.TrimPrefix(strings.ReplaceAll(m[1], "&amp;", "&"), f.srv.URL), nil)
	require.Equal(t, http.StatusSeeOther, toConsent.status, toConsent.body)
	page := f.do(f.c, http.MethodGet, toConsent.header.Get("Location"), nil)
	require.Equal(t, http.StatusOK, page.status, page.body)
	for _, want := range []string{"Shop CLI", "Shop MCP", "Read your orders", "127.0.0.1"} {
		assert.Contains(t, page.body, want)
	}
	assert.NotContains(t, page.body, ">write<", "write is outside the pin")

	allowed := f.submit(f.c, page, "/consent", nil)
	m = continueRE.FindStringSubmatch(allowed.body)
	require.NotEmpty(t, m, allowed.body)
	back := f.do(f.c, http.MethodGet, strings.TrimPrefix(strings.ReplaceAll(m[1], "&amp;", "&"), f.srv.URL), nil)
	require.Equal(t, http.StatusFound, back.status, back.body)
	loc, err := url.Parse(back.header.Get("Location"))
	require.NoError(t, err)
	require.NotEmpty(t, loc.Query().Get("code"), loc.String())

	var bound uuid.UUID
	var scopes int
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT resource_server_id FROM authorization_codes ORDER BY created_at DESC LIMIT 1`).Scan(&bound))
	assert.Equal(t, rs.ID, bound, "RFC 8707: the code is bound to the server")
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT cardinality(scopes) FROM scope_consents WHERE client_id = $1`, oc.ID).Scan(&scopes))
	assert.Equal(t, 3, scopes, "openid, read, offline_access")

	_, err = f.svcs.ResourceServers.Update(ctx, p.OrgID, p.ID, rs.PublicID, "Shop MCP", []string{"read", "orders"}, nil)
	require.NoError(t, err)
	_, err = f.db.ExecContext(ctx, `UPDATE scope_consents SET scopes = scopes || '{gone}' WHERE client_id = $1`, oc.ID)
	require.NoError(t, err)
	q.Set("prompt", "consent")
	again := f.do(f.c, http.MethodGet, base+"/oauth2/authorize?"+q.Encode(), nil)
	require.Equal(t, http.StatusSeeOther, again.status, again.body)
	full := f.do(f.c, http.MethodGet, again.header.Get("Location"), nil)
	f.submit(f.c, full, "/consent", nil)
	var kept []byte
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT array_to_json(scopes) FROM scope_consents WHERE client_id = $1`, oc.ID).Scan(&kept))
	assert.JSONEq(t, `["openid","read","offline_access"]`, string(kept), "a scope gone from the catalog is dropped on approve")
	q.Del("prompt")

	again = f.do(f.c, http.MethodGet, base+"/oauth2/authorize?"+q.Encode(), nil)
	require.Equal(t, http.StatusSeeOther, again.status, again.body)
	remembered := f.do(f.c, http.MethodGet, again.header.Get("Location"), nil)
	assert.Contains(t, remembered.body, "Continue to Shop CLI", "one click for a public client")
	assert.NotContains(t, remembered.body, "Read your orders")
}
