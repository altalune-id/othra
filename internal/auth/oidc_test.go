package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/altalune-id/oidcrp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/internal/kernel/session"
	"github.com/altalune-id/othra/internal/web"
	"github.com/altalune-id/othra/internal/web/layout"
	"github.com/altalune-id/othra/internal/web/middleware"
)

type fakeRP struct {
	started  oidcrp.AuthRequest
	token    *oidcrp.IDToken
	err      error
	endHint  string
	ended    []string
	endErr   error
	noEnd    bool
	startErr error
}

func (f *fakeRP) AuthCodeURL(_ context.Context, req oidcrp.AuthRequest) (string, error) {
	f.started = req
	return "http://othra.test/p/system/oauth2/authorize?state=" + req.State, f.startErr
}

func (f *fakeRP) Exchange(_ context.Context, cb oidcrp.Callback, req oidcrp.AuthRequest) (*oidcrp.IDToken, error) {
	switch {
	case f.err != nil:
		return nil, f.err
	case cb.State != req.State:
		return nil, &oidcrp.Error{Reason: oidcrp.ReasonState, Err: errors.New("state")}
	}
	return f.token, nil
}

func (f *fakeRP) EndSystemSession(_ context.Context, idToken string) error {
	f.ended = append(f.ended, idToken)
	return f.endErr
}

func (f *fakeRP) EndSessionURL(_ context.Context, hint, postLogout, _ string) (string, error) {
	f.endHint = hint
	if f.noEnd {
		return "", nil
	}
	return "http://othra.test/p/system/oauth2/end-session?post_logout_redirect_uri=" + url.QueryEscape(postLogout), nil
}

type consoleFixture struct {
	*loginFixture
	rp  *fakeRP
	web *Web
}

func newConsoleFixture(t *testing.T, passwordOff bool) *consoleFixture {
	t.Helper()
	f := &consoleFixture{loginFixture: newLoginFixture(t, ""), rp: &fakeRP{token: &oidcrp.IDToken{
		Subject: "sub-1", Email: "ada@example.com", EmailVerified: true, Name: "Ada", Raw: "the.id.token",
	}}}
	sessions := testSessions(f.store)
	sessions.Cookie.Path = "/"
	f.web = NewWeb(layout.Kit{Unexpected: f.report}, f.auth, sessions, f.ids).WithConsoleSignIn(ConsoleSignIn{
		RP: f.rp, SignedOut: "http://othra.test/console/", PasswordOff: passwordOff, RememberLastUser: true,
	})
	f.srv = web.NewServer(web.ServerOpts{
		Console: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("console")) }),
		Session: middleware.Session(middleware.SessionConfig{Store: f.store, Secret: testSecret, Auth: f.auth}),
		Routes:  []web.Registrar{f.web},
	})
	return f
}

func (f *consoleFixture) start(t *testing.T, query string) (*http.Cookie, string) {
	t.Helper()
	rec := f.do(httptest.NewRequest(http.MethodGet, "/login/oidc?"+query, nil))
	require.Equal(t, http.StatusFound, rec.Code, rec.Body.String())
	c := cookieNamed(rec, signInCookieName)
	require.NotNil(t, c)
	return c, f.rp.started.State
}

func (f *consoleFixture) callback(query string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/oauth/callback?"+query, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	return f.do(req)
}

// SECURITY: RFC 9700 §2.1.1, each start mints its own state, nonce and verifier, kept only in a signed HttpOnly cookie on the callback.
func TestConsoleSignIn_Start(t *testing.T) {
	f := newConsoleFixture(t, false)
	rec := f.do(httptest.NewRequest(http.MethodGet, "/login/oidc?return_to=/console/x&prompt=select_account&login_hint=Ada%40Example.com", nil))
	require.Equal(t, http.StatusFound, rec.Code)
	assert.True(t, strings.HasPrefix(rec.Header().Get("Location"), "http://othra.test/p/system/oauth2/authorize?state="))
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "no-referrer", rec.Header().Get("Referrer-Policy"))
	c := cookieNamed(rec, signInCookieName)
	require.NotNil(t, c)
	assert.True(t, c.HttpOnly)
	assert.Equal(t, "/oauth/callback", c.Path)
	assert.NotContains(t, c.Value, f.rp.started.CodeVerifier, "the cookie is encoded, never the bare verifier")
	assert.Equal(t, "select_account", f.rp.started.Prompt)
	assert.Equal(t, "ada@example.com", f.rp.started.LoginHint)

	_ = f.do(httptest.NewRequest(http.MethodGet, "/login/oidc?prompt=none&login_hint=ada@example.com", nil))
	assert.Empty(t, f.rp.started.Prompt, "only select_account and login pass through")

	f.rp.startErr = errors.New("discovery down")
	rec = f.do(httptest.NewRequest(http.MethodGet, "/login/oidc", nil))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestConsoleSignIn_Callback(t *testing.T) {
	f := newConsoleFixture(t, false)
	f.auth.principal.Source, f.auth.principal.IDToken = session.SourceOIDC, "the.id.token"
	c, state := f.start(t, "return_to=/console/x&invite=inv-1")

	rec := f.callback("code=c1&state="+state, c)
	require.Equal(t, http.StatusSeeOther, rec.Code, rec.Body.String())
	assert.Equal(t, "/console/x", rec.Header().Get("Location"))
	require.NotNil(t, cookieNamed(rec, session.CookieName), "signed in")
	require.Len(t, f.auth.oidc, 1)
	got := f.auth.oidc[0]
	assert.Equal(t, OIDCSignIn{Subject: "sub-1", Email: "ada@example.com", EmailVerified: true, Name: "Ada", IDToken: "the.id.token", Invite: "inv-1"}, got)
	cleared := cookieNamed(rec, signInCookieName)
	require.NotNil(t, cleared)
	assert.Negative(t, cleared.MaxAge, "the state is single use")
}

func TestConsoleSignIn_CallbackRefusals(t *testing.T) {
	f := newConsoleFixture(t, false)
	rec := f.callback("code=c1&state=x")
	assert.Equal(t, http.StatusBadRequest, rec.Code, "no state cookie")
	assert.Contains(t, rec.Body.String(), "This sign-in expired")
	assert.Nil(t, cookieNamed(rec, session.CookieName))

	c, _ := f.start(t, "")
	rec = f.callback("code=c1&state=forged", c)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "a forged state")
	assert.Empty(t, f.auth.oidc)

	c, state := f.start(t, "")
	f.rp.err = &oidcrp.Error{Reason: oidcrp.ReasonCancelled, Err: errors.New("access_denied")}
	rec = f.callback("error=access_denied&state="+state, c)
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/login", rec.Header().Get("Location"), "cancelled at Othra: back to sign in, quietly")

	for _, code := range []string{"invalid_grant", "invalid_request"} {
		c, state = f.start(t, "")
		f.rp.err = &oidcrp.Error{Reason: oidcrp.ReasonTokenExchange, OAuthError: code, Err: errors.New("status 400")}
		rec = f.callback("code=c1&state="+state, c)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "a replayed or unknown code is %s: start again", code)
		assert.Contains(t, rec.Body.String(), "This sign-in expired")
	}
	assert.Empty(t, f.reported, "a refused code is no server failure")

	for _, code := range []string{"", "invalid_client", "other"} {
		c, state = f.start(t, "")
		f.rp.err = &oidcrp.Error{Reason: oidcrp.ReasonTokenExchange, OAuthError: code, Err: errors.New("token endpoint down")}
		rec = f.callback("code=c1&state="+state, c)
		assert.Equal(t, http.StatusInternalServerError, rec.Code, "transport, 5xx or our own client: %q", code)
	}

	f.rp.err = nil
	f.auth.oidcErr = &NoConsoleAccountError{Reason: NoConsoleNotInvited}
	c, state = f.start(t, "return_to=/console/x")
	rec = f.callback("code=c1&state="+state, c)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "/login/oidc?prompt=select_account")
	assert.Nil(t, cookieNamed(rec, session.CookieName))

	f.auth.oidcErr = &IdentityConflictError{}
	c, state = f.start(t, "")
	rec = f.callback("code=c1&state="+state, c)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "or with your password")

	off := newConsoleFixture(t, true)
	off.auth.oidcErr = &IdentityConflictError{}
	c, state = off.start(t, "")
	rec = off.callback("code=c1&state="+state, c)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.NotContains(t, rec.Body.String(), "password", "no password to fall back on when passwords are off")
}

// NOTE: RP-Initiated Logout §2: a console session from /p/system ends there too, with its ID token as the hint.
func TestConsoleSignIn_Logout(t *testing.T) {
	f := newConsoleFixture(t, false)
	f.auth.principal.Source, f.auth.principal.IDToken = session.SourceOIDC, "the.id.token"
	c, state := f.start(t, "")
	rec := f.callback("code=c1&state="+state, c)
	sid := cookieNamed(rec, session.CookieName)
	require.NotNil(t, sid)

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.AddCookie(sid)
	rec = f.do(req)
	require.Equal(t, http.StatusSeeOther, rec.Code)
	assert.True(t, strings.HasPrefix(rec.Header().Get("Location"), "http://othra.test/p/system/oauth2/end-session?"))
	assert.Contains(t, rec.Header().Get("Location"), url.QueryEscape("http://othra.test/console/"))
	assert.Equal(t, "the.id.token", f.rp.endHint)
	assert.Equal(t, []string{"the.id.token"}, f.rp.ended, "the /p/system session ends in process too, whatever the browser does next")
	assert.Len(t, f.auth.loggedOut, 1, "the console session ends first")

	f.rp.noEnd = true
	c, state = f.start(t, "")
	sid = cookieNamed(f.callback("code=c1&state="+state, c), session.CookieName)
	require.NotNil(t, sid)
	req = httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.AddCookie(sid)
	rec = f.do(req)
	require.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/login", rec.Header().Get("Location"), "no end_session_endpoint: back to sign-in, never an empty redirect")
	assert.Empty(t, f.reported, "an issuer with no end-session endpoint is not a failure")
}

func TestConsoleSignIn_LoginPage(t *testing.T) {
	f := newConsoleFixture(t, false)
	rec := f.do(httptest.NewRequest(http.MethodGet, "/login?return_to=/console/x", nil))
	assert.Contains(t, rec.Body.String(), `/login/oidc?return_to=%2Fconsole%2Fx`)
	assert.Contains(t, rec.Body.String(), `name="password"`)

	off := newConsoleFixture(t, true)
	rec = off.do(httptest.NewRequest(http.MethodGet, "/login", nil))
	assert.NotContains(t, rec.Body.String(), `name="password"`)
	rec = off.do(loginPost("", url.Values{"email": {"ada@example.com"}, "password": {"correct horse battery"}}))
	assert.NotEqual(t, http.StatusSeeOther, rec.Code, "no password sign-in when passwords are off")
	assert.Nil(t, cookieNamed(rec, session.CookieName))
}

// SECURITY: the state's own issue time bounds it to ten minutes, whatever the browser does with the cookie's Max-Age.
func TestConsoleSignIn_StateExpires(t *testing.T) {
	f := newConsoleFixture(t, false)
	now := time.Now()
	f.web.signInCookie.Now = func() time.Time { return now }
	c, state := f.start(t, "")

	now = now.Add(signInTTL + time.Second)
	rec := f.callback("code=c1&state="+state, c)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "This sign-in expired")
	assert.Empty(t, f.auth.oidc)

	now = now.Add(-2 * signInTTL)
	rec = f.callback("code=c1&state="+state, c)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "issued in the future")

	now = now.Add(signInTTL)
	rec = f.callback("code=c1&state="+state, c)
	assert.Equal(t, http.StatusSeeOther, rec.Code, "within ten minutes: %s", rec.Body.String())
}

// A sign-in from an invite for another email lands on the invite, which shows Mismatch; one for this email goes on.
func TestConsoleSignIn_InviteForAnotherEmail(t *testing.T) {
	f := newConsoleFixture(t, false)
	f.auth.otherInvite = "inv-dan"
	c, state := f.start(t, "return_to=/console/x&invite=inv-dan")
	rec := f.callback("code=c1&state="+state, c)
	require.Equal(t, http.StatusSeeOther, rec.Code, rec.Body.String())
	assert.Equal(t, "/invite", rec.Header().Get("Location"))
	assert.NotNil(t, cookieNamed(rec, session.CookieName), "signed in, so the landing shows who is signed in")

	c, state = f.start(t, "return_to=/console/x&invite=inv-ada")
	rec = f.callback("code=c1&state="+state, c)
	assert.Equal(t, "/console/x", rec.Header().Get("Location"))
}

// The callback and the start share /login's IP limit; past it the exchange never runs.
func TestConsoleSignIn_LimitedPerIP(t *testing.T) {
	f := newConsoleFixture(t, false)
	limits, _ := memoryLimits(t)
	f.web.WithLimits(limits)
	from := func(path, ip string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = ip + ":4711"
		return req
	}
	for i := range LoginIPRule().Burst {
		require.Equal(t, http.StatusBadRequest, f.do(from("/oauth/callback?code=c&state=s", "198.51.100.7")).Code, "attempt %d", i+1)
	}
	f.rp.err = errors.New("must not be called")
	rec := f.do(from("/oauth/callback?code=c&state=s", "198.51.100.7"))
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
	assert.Contains(t, rec.Body.String(), "Too many attempts")
	assert.Equal(t, http.StatusTooManyRequests, f.do(from("/login/oidc", "198.51.100.7")).Code, "the start shares the bucket")
	assert.Equal(t, http.StatusBadRequest, f.do(from("/oauth/callback?code=c&state=s", "198.51.100.8")).Code, "another IP goes on")
}

func lastUserOf(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	return cookieNamed(rec, lastUserCookieName)
}

// After a sign-in the verified email is remembered: HttpOnly, Lax, 90 days; a hint only, never signed.
func TestConsoleSignIn_RemembersTheLastUser(t *testing.T) {
	f := newConsoleFixture(t, false)
	f.rp.token.Email = "Ada@Example.com"
	c, state := f.start(t, "")
	rec := f.callback("code=c1&state="+state, c)
	require.Equal(t, http.StatusSeeOther, rec.Code, rec.Body.String())
	last := lastUserOf(t, rec)
	require.NotNil(t, last)
	assert.Equal(t, "ada@example.com", last.Value)
	assert.True(t, last.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, last.SameSite)
	assert.Equal(t, int(lastUserTTL.Seconds()), last.MaxAge)
	assert.Equal(t, "/", last.Path)

	off := newConsoleFixture(t, false)
	off.web.console.RememberLastUser = false
	c, state = off.start(t, "")
	assert.Nil(t, lastUserOf(t, off.callback("code=c1&state="+state, c)), "auth.rememberLastUser off")
}

// SECURITY: OIDC Core §3.1.2.1: no account in mind means the picker, never a silent sign-in to whatever session is live.
func TestConsoleSignIn_PromptAndHint(t *testing.T) {
	f := newConsoleFixture(t, false)
	start := func(query string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/login/oidc?"+query, nil)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rec := f.do(req)
		require.Equal(t, http.StatusFound, rec.Code, rec.Body.String())
		return rec
	}

	start("")
	assert.Equal(t, "select_account", f.rp.started.Prompt, "no hint at all")
	assert.Empty(t, f.rp.started.LoginHint)

	start("login_hint=ada@example.com")
	assert.Equal(t, "ada@example.com", f.rp.started.LoginHint)
	assert.Empty(t, f.rp.started.Prompt, "a hint and no prompt")

	remembered := &http.Cookie{Name: lastUserCookieName, Value: "ada@example.com"}
	start("", remembered)
	assert.Equal(t, "ada@example.com", f.rp.started.LoginHint, "the remembered email is the hint")
	assert.Empty(t, f.rp.started.Prompt)

	start("", &http.Cookie{Name: lastUserCookieName, Value: "not an email"})
	assert.Empty(t, f.rp.started.LoginHint, "a malformed cookie is ignored")
	assert.Equal(t, "select_account", f.rp.started.Prompt)

	rec := start("prompt=select_account", remembered)
	assert.Equal(t, "select_account", f.rp.started.Prompt)
	assert.Empty(t, f.rp.started.LoginHint, "use another account drops the remembered hint")
	cleared := lastUserOf(t, rec)
	require.NotNil(t, cleared)
	assert.Negative(t, cleared.MaxAge, "and forgets it")
}

func TestConsoleSignIn_LoginPageContinueAs(t *testing.T) {
	f := newConsoleFixture(t, false)
	req := httptest.NewRequest(http.MethodGet, "/login?return_to=/console/x", nil)
	req.AddCookie(&http.Cookie{Name: lastUserCookieName, Value: "ada@example.com"})
	body := f.do(req).Body.String()
	assert.Contains(t, body, "Continue as")
	assert.Contains(t, body, "ada@example.com")
	assert.Contains(t, body, `/login/oidc?login_hint=ada%40example.com&amp;return_to=%2Fconsole%2Fx`)
	assert.Contains(t, body, `/login/oidc?prompt=select_account&amp;return_to=%2Fconsole%2Fx`)
	assert.Contains(t, body, "Use another account")

	body = f.do(httptest.NewRequest(http.MethodGet, "/login", nil)).Body.String()
	assert.Contains(t, body, "Continue with Othra")
	assert.NotContains(t, body, "Continue as")
}

// Logout ends the session, not the memory of the account.
func TestConsoleSignIn_LogoutKeepsTheLastUser(t *testing.T) {
	f := newConsoleFixture(t, false)
	f.auth.principal.Source, f.auth.principal.IDToken = session.SourceOIDC, "the.id.token"
	c, state := f.start(t, "")
	sid := cookieNamed(f.callback("code=c1&state="+state, c), session.CookieName)
	require.NotNil(t, sid)
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.AddCookie(sid)
	assert.Nil(t, lastUserOf(t, f.do(req)))
}
