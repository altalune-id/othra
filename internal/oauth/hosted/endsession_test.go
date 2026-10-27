package hosted

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/internal/web/i18n"
)

func endSessionURL(q url.Values) string {
	return "/p/acme" + endSessionPath + "?" + q.Encode()
}

func (f *webFixture) hintFor(c *http.Client, email string, aud ...string) string {
	f.t.Helper()
	f.signIn(c, "acme", email)
	if len(aud) == 0 {
		aud = []string{"web"}
	}
	a := f.accountOf(email)
	raw := "hint-" + a.UserSession
	f.grants.hints[raw] = Hint{Subject: a.EndUser, SID: "sid-" + a.UserSession, Audience: aud}
	return raw
}

// RPL §2, §4: a missing, forged or foreign hint and an unregistered post-logout URI are a page, never a redirect.
func TestEndSession_PageErrors(t *testing.T) {
	f := newWebFixture(t)
	hint := f.hintFor(f.client, "ana@example.com")
	for name, tc := range map[string]struct {
		q    url.Values
		code apperror.Code
	}{
		"no hint":         {url.Values{}, apperror.CodeSignOutHintInvalid},
		"forged hint":     {url.Values{"id_token_hint": {"forged"}}, apperror.CodeSignOutHintInvalid},
		"other client":    {url.Values{"id_token_hint": {hint}, "client_id": {"cli"}}, apperror.CodeSignOutHintInvalid},
		"unregistered":    {url.Values{"id_token_hint": {hint}, "post_logout_redirect_uri": {"https://evil.example/bye"}}, apperror.CodeSignOutRedirectNotRegistered},
		"redirect prefix": {url.Values{"id_token_hint": {hint}, "post_logout_redirect_uri": {byeRedirect + "/x"}}, apperror.CodeSignOutRedirectNotRegistered},
	} {
		p := f.get(f.client, endSessionURL(tc.q))
		assert.Equal(t, http.StatusBadRequest, p.status, name)
		assert.Empty(t, p.location(), name)
		assert.Contains(t, p.body, tc.code, name)
		assert.NotContains(t, p.body, "evil.example", name)
	}
	f.grants.hints["no-client"] = Hint{SID: "sid-x", Audience: []string{"gone"}}
	assert.Equal(t, http.StatusBadRequest, f.get(f.client, endSessionURL(url.Values{"id_token_hint": {"no-client"}})).status)
	assert.Empty(t, f.grants.ended)
}

// RPL §3: the hinted session on this browser ends at once and the browser goes back with state.
func TestEndSession_SameBrowser(t *testing.T) {
	f := newWebFixture(t)
	hint := f.hintFor(f.client, "ana@example.com")
	p := f.get(f.client, endSessionURL(url.Values{
		"id_token_hint": {hint}, "client_id": {"web"}, "post_logout_redirect_uri": {byeRedirect}, "state": {"s-1"},
	}))
	require.Equal(t, http.StatusFound, p.status, p.body)
	assert.Equal(t, byeRedirect+"?state=s-1", p.location())
	assert.Len(t, f.grants.ended, 1)

	again := f.get(f.client, endSessionURL(url.Values{"id_token_hint": {hint}}))
	assert.Equal(t, http.StatusSeeOther, again.status, "an ended session is done, not an error")
	assert.Equal(t, "/p/acme/signed-out", again.location())
	assert.Len(t, f.grants.ended, 1)
}

// The RP may POST the request; the next hop is the continue page.
func TestEndSession_Post(t *testing.T) {
	f := newWebFixture(t)
	hint := f.hintFor(f.client, "ana@example.com")
	p := f.do(f.client, http.MethodPost, "/p/acme"+endSessionPath, url.Values{
		"id_token_hint": {hint}, "post_logout_redirect_uri": {byeRedirect},
	})
	require.Equal(t, http.StatusOK, p.status, p.body)
	assert.Contains(t, p.body, `content="0;url=`+byeRedirect+`"`)
	assert.Len(t, f.grants.ended, 1)
}

// A session this browser doesn't hold needs a confirm with a CSRF-checked form.
func TestEndSession_OtherBrowserConfirms(t *testing.T) {
	f := newWebFixture(t)
	hint := f.hintFor(f.newClient(), "ana@example.com")
	q := url.Values{"id_token_hint": {hint}, "post_logout_redirect_uri": {byeRedirect}, "state": {"s-2"}}
	page := f.get(f.client, endSessionURL(q))
	require.Equal(t, http.StatusOK, page.status, page.body)
	assert.Contains(t, page.body, "Openwa Web")
	assert.Empty(t, f.grants.ended, "nothing ends without the confirm")

	forged := f.do(f.client, http.MethodPost, "/p/acme"+endSessionPath, url.Values{
		"id_token_hint": {hint}, "confirm": {"1"}, "csrf": {"nope"},
	})
	assert.NotEqual(t, http.StatusOK, forged.status)
	assert.Empty(t, f.grants.ended)

	done := f.submit(f.client, page, endSessionPath, nil)
	require.Equal(t, http.StatusOK, done.status, done.body)
	assert.Contains(t, done.body, `content="0;url=`+byeRedirect+`?state=s-2"`)
	assert.Len(t, f.grants.ended, 1)
}

func TestEndSession_Failures(t *testing.T) {
	f := newWebFixture(t)
	hint := f.hintFor(f.client, "ana@example.com")
	f.grants.failOn = "hint"
	assert.Equal(t, http.StatusInternalServerError, f.get(f.client, endSessionURL(url.Values{"id_token_hint": {hint}})).status)
	for _, on := range []string{"session", "end"} {
		f.grants.failOn = on
		assert.Equal(t, http.StatusInternalServerError, f.get(f.client, endSessionURL(url.Values{"id_token_hint": {hint}})).status, on)
	}
	assert.Empty(t, f.grants.ended)
}

// Every code a refused request shows has a message in every locale.
func TestRefused_Translated(t *testing.T) {
	for _, code := range []apperror.Code{
		apperror.CodeAuthorizeClientUnknown, apperror.CodeAuthorizeRedirectNotRegistered, apperror.CodeAuthorizeRequestInvalid,
		apperror.CodeSignOutHintInvalid, apperror.CodeSignOutRedirectNotRegistered,
	} {
		for _, l := range i18n.Locales() {
			assert.NotEqual(t, "apperror."+string(code), i18n.Error(i18n.WithLocale(context.Background(), l), code), "%s %s", code, l)
		}
		app := (&RefusedError{Code: code}).ToAppError()
		assert.Equal(t, code, app.Code())
	}
}

// a revoked, expired or pruned family still signs out the hinted user session, or this browser's account with the hint's sub.
func TestEndSession_FamilyGone(t *testing.T) {
	f := newWebFixture(t)
	hint := f.hintFor(f.client, "ana@example.com")
	a := f.accountOf("ana@example.com")
	f.grants.revoked = append(f.grants.revoked, "sid-"+a.UserSession)
	p := f.get(f.client, endSessionURL(url.Values{"id_token_hint": {hint}}))
	assert.Equal(t, http.StatusSeeOther, p.status, p.body)
	_, signedIn := f.browserOf(f.client).Find(a.UserSession)
	assert.False(t, signedIn, "a revoked family's session on this browser is signed out")

	f.users.last = map[string]time.Time{}
	f.signIn(f.client, "acme", "ana@example.com")
	f.grants.hints["pruned"] = Hint{Subject: f.accountOf("ana@example.com").EndUser, SID: "pruned-sid", Audience: []string{"web"}}
	p = f.get(f.client, endSessionURL(url.Values{"id_token_hint": {"pruned"}}))
	assert.Equal(t, http.StatusSeeOther, p.status, p.body)
	assert.Empty(t, f.browserOf(f.client).Accounts, "a pruned family falls back to the account with the hint's sub")

	other := f.newClient()
	f.signIn(other, "acme", "cici@example.com")
	f.grants.hints["elsewhere"] = Hint{Subject: f.accountOf("cici@example.com").EndUser, SID: "pruned-2", Audience: []string{"web"}}
	p = f.get(f.client, endSessionURL(url.Values{"id_token_hint": {"elsewhere"}}))
	assert.Equal(t, http.StatusSeeOther, p.status, "nothing to end on this browser: done, no confirm")
	assert.NotEmpty(t, f.browserOf(other).Accounts, "another browser is never signed out without a live family and a confirm")
}

func (f *webFixture) browserOf(c *http.Client) *Browser {
	f.t.Helper()
	u, err := url.Parse(f.srv.URL + "/p/acme/")
	require.NoError(f.t, err)
	req := httptest.NewRequest(http.MethodGet, "/p/acme/", nil)
	for _, ck := range c.Jar.Cookies(u) {
		req.AddCookie(ck)
	}
	b, err := f.users.Browser(f.t.Context(), Project{ID: f.acme.ProjectID}, f.web.cookies.browser(req))
	require.NoError(f.t, err)
	return b
}

// RPL §2: a hint that no longer verifies is left out; client_id names the app and the person confirms before this browser signs out.
func TestEndSession_WithoutHint(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	for name, q := range map[string]url.Values{
		"unknown client": {"client_id": {"nope"}},
		"unregistered":   {"client_id": {"web"}, "post_logout_redirect_uri": {"https://evil.example/bye"}},
	} {
		p := f.get(f.client, endSessionURL(q))
		assert.Equal(t, http.StatusBadRequest, p.status, name)
		assert.Empty(t, p.location(), name)
	}

	page := f.get(f.client, endSessionURL(url.Values{"client_id": {"web"}, "post_logout_redirect_uri": {byeRedirect}, "state": {"s-3"}}))
	require.Equal(t, http.StatusOK, page.status, page.body)
	assert.Contains(t, page.body, `name="client_id" value="web"`)
	assert.NotContains(t, page.body, `name="id_token_hint"`)
	assert.Empty(t, f.grants.ended, "nothing ends without the confirm")
	assert.NotEmpty(t, f.browserOf(f.client).Accounts)

	forged := f.do(f.client, http.MethodPost, "/p/acme"+endSessionPath, url.Values{"client_id": {"web"}, "confirm": {"1"}, "csrf": {"nope"}})
	assert.NotEqual(t, http.StatusOK, forged.status)
	assert.NotEmpty(t, f.browserOf(f.client).Accounts)

	done := f.submit(f.client, page, endSessionPath, nil)
	require.Equal(t, http.StatusOK, done.status, done.body)
	assert.Contains(t, done.body, `content="0;url=`+byeRedirect+`?state=s-3"`)
	assert.Empty(t, f.browserOf(f.client).Accounts, "this browser's account is signed out")

	again := f.submit(f.client, f.get(f.client, endSessionURL(url.Values{"client_id": {"web"}})), endSessionPath, nil)
	assert.Equal(t, http.StatusSeeOther, again.status, "nothing signed in here: done, not an error")
}
