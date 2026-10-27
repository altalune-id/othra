package hosted

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/federation"
	"altalune.id/othra/internal/kernel/ratelimit/memory"
	"altalune.id/othra/internal/kernel/ratelimit/ratelimittest"
	"altalune.id/othra/internal/upstream"
	"altalune.id/othra/internal/web/i18n"
	"altalune.id/othra/internal/web/layout"
	"altalune.id/othra/oidcrp"
	"altalune.id/othra/sealer"
)

const fakeAuthorize = "https://accounts.example/o/oauth2/auth"

type fakeFederation struct {
	users       *fakeUsers
	mu          sync.Mutex
	off         bool
	enabledErr  error
	startErr    error
	authErr     error
	completeErr error
	signInErr   error
	linkErr     error
	email       string
	hints       []string
	linked      []uuid.UUID
	collide     bool
	unproven    bool
	identities  []federation.Identity
	unlinkErr   error
	unlinked    []string
}

func (f *fakeFederation) Enabled(context.Context, federation.Project, upstream.Kind) (bool, error) {
	return !f.off, f.enabledErr
}

func (f *fakeFederation) Start(_ context.Context, _ federation.Project, _ upstream.Kind, hint string) (oidcrp.AuthRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hints = append(f.hints, hint)
	if f.startErr != nil {
		return oidcrp.AuthRequest{}, f.startErr
	}
	return oidcrp.AuthRequest{State: "st-" + uuid.NewString(), Nonce: "n", CodeVerifier: "v"}, nil
}

func (f *fakeFederation) AuthURL(_ context.Context, _ federation.Project, _ upstream.Kind, redirectURI string, req oidcrp.AuthRequest) (string, error) {
	if f.authErr != nil {
		return "", f.authErr
	}
	return fakeAuthorize + "?" + url.Values{"state": {req.State}, "redirect_uri": {redirectURI}}.Encode(), nil
}

func (f *fakeFederation) Complete(
	_ context.Context, _ federation.Project, kind upstream.Kind, _ string, cb oidcrp.Callback, req oidcrp.AuthRequest,
) (federation.Identity, error) {
	switch {
	case f.completeErr != nil:
		return federation.Identity{}, f.completeErr
	case cb.Error == "access_denied":
		return federation.Identity{}, &federation.UpstreamError{Reason: oidcrp.ReasonCancelled}
	case cb.State != req.State:
		return federation.Identity{}, &federation.UpstreamError{Reason: oidcrp.ReasonState}
	}
	return federation.Identity{Issuer: "https://accounts.google.com", Subject: "g-1", Email: f.email, EmailVerified: true, Name: "Budi", Kind: kind}, nil
}

func (f *fakeFederation) SignIn(_ context.Context, p federation.Project, id federation.Identity, req federation.SessionRequest) (*federation.Session, error) {
	switch {
	case f.signInErr != nil:
		return nil, f.signInErr
	case f.collide:
		return nil, &federation.CollisionError{Email: id.Email, Identity: id}
	case f.unproven:
		return nil, &federation.UnprovenEmailError{Email: id.Email, Identity: id}
	}
	r, err := f.users.signInAs(Project{OrgID: p.OrgID, ID: p.ID}, id.Email, req.BrowserSecret)
	if err != nil {
		return nil, err
	}
	a := r.Account
	return &federation.Session{
		BrowserSecret: r.BrowserSecret, ExpiresAt: r.ExpiresAt, EndUserID: a.EndUserID, UserSession: a.UserSession, Subject: a.EndUser,
		Email: a.Email, AuthTime: a.AuthTime, SelectedAt: a.SelectedAt,
	}, nil
}

func (f *fakeFederation) Link(_ context.Context, _ federation.Project, endUserID uuid.UUID, _ federation.Identity) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.linked = append(f.linked, endUserID)
	return f.linkErr
}

func (f *fakeFederation) Unlink(_ context.Context, _ federation.Project, _ uuid.UUID, identity string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unlinked = append(f.unlinked, identity)
	return f.unlinkErr
}

func (f *fakeFederation) Identities(context.Context, federation.Project, uuid.UUID) ([]federation.Identity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.identities, nil
}

func (f *fakeUsers) signInAs(p Project, email, secret string) (*Redeemed, error) {
	p.EmailLink = true
	f.mu.Lock()
	token := f.next("tok")
	f.links[token] = &fakeLink{email: email, project: p}
	f.mu.Unlock()
	return f.Redeem(context.Background(), RedeemRequest{Project: p, Token: token, BrowserSecret: secret})
}

func newFederationFixture(t *testing.T) (*webFixture, *fakeFederation) {
	t.Helper()
	f := newWebFixture(t)
	fed := &fakeFederation{users: f.users, email: "ana@gmail.com"}
	sl, err := sealer.New([]byte("federation-key-federation-key-32"))
	require.NoError(t, err)
	f.web.WithFederation(fed, sl)
	mux := http.NewServeMux()
	f.web.Register(mux)
	f.srv.Config.Handler = i18n.Middleware(mux)
	return f, fed
}

func (f *webFixture) startGoogle(c *http.Client, from string) (continuePage hostedPage, authURL *url.URL) {
	f.t.Helper()
	login := f.get(c, from)
	require.Equal(f.t, http.StatusOK, login.status, login.body)
	moved := f.submit(c, login, "/federation/google/start", nil)
	require.Equal(f.t, http.StatusSeeOther, moved.status, moved.body)
	require.Equal(f.t, "/p/acme/federation/google/continue", moved.location())
	page := f.get(c, moved.location())
	require.Equal(f.t, http.StatusOK, page.status, page.body)
	m := strings.Index(page.body, fakeAuthorize)
	require.GreaterOrEqual(f.t, m, 0, page.body)
	raw := page.body[m:]
	raw = raw[:strings.IndexAny(raw, `"'`)]
	u, err := url.Parse(strings.ReplaceAll(raw, "&amp;", "&"))
	require.NoError(f.t, err)
	return page, u
}

func (f *webFixture) callback(c *http.Client, q url.Values) hostedPage {
	f.t.Helper()
	return f.get(c, "/p/acme/federation/google/callback?"+q.Encode())
}

func TestFederation_LoginButton(t *testing.T) {
	f, fed := newFederationFixture(t)
	login := f.get(f.client, "/p/acme/login")
	assert.Contains(t, login.body, `action="/p/acme/federation/google/start"`)
	assert.Contains(t, login.body, `name="email"`)

	fed.off = true
	assert.NotContains(t, f.get(f.client, "/p/acme/login").body, "federation/google/start")
	f.acme.SignInEmailLink = false
	assert.Equal(t, http.StatusForbidden, f.get(f.client, "/p/acme/login").status, "no way to sign in at all")

	fed.off = false
	only := f.get(f.client, "/p/acme/login")
	assert.Equal(t, http.StatusOK, only.status)
	assert.Contains(t, only.body, "federation/google/start")
	assert.NotContains(t, only.body, `name="email"`, "Google only")

	fed.enabledErr = errFake
	assert.Equal(t, http.StatusForbidden, f.get(f.client, "/p/acme/login").status, "a failed lookup hides the button")
}

// N1, P10, P11, G4: POST start, a sealed state cookie, a GET hop to Google, and a callback that spends the cookie.
func TestFederation_SignIn(t *testing.T) {
	f, fed := newFederationFixture(t)
	assert.Equal(t, http.StatusMethodNotAllowed, f.get(f.client, "/p/acme/federation/google/start").status)

	page, authURL := f.startGoogle(f.client, "/p/acme/login")
	assert.Contains(t, page.body, `http-equiv="refresh"`)
	assert.Equal(t, f.srv.URL+"/p/acme/federation/google/callback", authURL.Query().Get("redirect_uri"))
	assert.Equal(t, []string{""}, fed.hints, "P7: no account, no hint")
	var sealed *http.Cookie
	for _, c := range f.client.Jar.Cookies(mustURL(t, f.srv.URL+"/p/acme/federation/google/callback")) {
		if c.Name == federationCookie {
			sealed = c
		}
	}
	require.NotNil(t, sealed)
	assert.NotContains(t, sealed.Value, authURL.Query().Get("state"), "the state is sealed, not plain")
	for _, c := range f.client.Jar.Cookies(mustURL(t, f.srv.URL+"/p/acme/login")) {
		assert.NotEqual(t, federationCookie, c.Name, "the state cookie stays on /federation/")
	}

	done := f.callback(f.client, url.Values{"code": {"c"}, "state": {authURL.Query().Get("state")}})
	require.Equal(t, http.StatusSeeOther, done.status, done.body)
	assert.Equal(t, "/p/acme/account/", done.location())
	assert.Equal(t, http.StatusOK, f.get(f.client, "/p/acme/account/").status, "signed in with Google")

	replay := f.callback(f.client, url.Values{"code": {"c"}, "state": {authURL.Query().Get("state")}})
	assert.Equal(t, http.StatusBadRequest, replay.status, "the cookie is spent")
	assert.Contains(t, replay.body, string(apperror.CodeSignInUpstreamResponseInvalid))
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

func TestFederation_CallbackRefusals(t *testing.T) {
	f, fed := newFederationFixture(t)
	cases := []struct {
		name   string
		q      func(state string) url.Values
		setup  func()
		status int
		code   apperror.Code
	}{
		{"wrong state", func(string) url.Values { return url.Values{"code": {"c"}, "state": {"forged"}} }, nil, http.StatusBadRequest,
			apperror.CodeSignInUpstreamResponseInvalid},
		{"cancelled", func(s string) url.Values { return url.Values{"error": {"access_denied"}, "state": {s}} }, nil, http.StatusForbidden,
			apperror.CodeSignInUpstreamCancelled},
		{"unverified", func(s string) url.Values { return url.Values{"code": {"c"}, "state": {s}} },
			func() { fed.collide, fed.signInErr = false, &federation.UnverifiedEmailError{} }, http.StatusForbidden,
			apperror.CodeSignInUpstreamEmailUnverified},
		{"upstream down", func(s string) url.Values { return url.Values{"code": {"c"}, "state": {s}} },
			func() { fed.signInErr, fed.completeErr = nil, &federation.UpstreamError{Reason: oidcrp.ReasonTimeout} }, http.StatusBadGateway,
			apperror.CodeSignInUpstreamUnavailable},
		{"reported", func(s string) url.Values { return url.Values{"code": {"c"}, "state": {s}} },
			func() { fed.completeErr = apperror.New(apperror.CodeUnexpected, "unexpected") }, http.StatusInternalServerError, apperror.CodeUnexpected},
		{"unexpected", func(s string) url.Values { return url.Values{"code": {"c"}, "state": {s}} },
			func() { fed.completeErr = errFake }, http.StatusInternalServerError, apperror.CodeUnexpected},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup()
			}
			_, authURL := f.startGoogle(f.client, "/p/acme/login?add=1")
			got := f.callback(f.client, tc.q(authURL.Query().Get("state")))
			assert.Equal(t, tc.status, got.status, got.body)
			assert.Contains(t, got.body, string(tc.code))
		})
	}
}

func TestFederation_StartRefusals(t *testing.T) {
	f, fed := newFederationFixture(t)
	login := f.get(f.client, "/p/acme/login")
	assert.Equal(t, http.StatusForbidden, f.do(f.client, http.MethodPost, "/p/acme/federation/google/start", url.Values{}).status, "no CSRF token")

	fed.startErr = &federation.UpstreamError{Reason: "disabled"}
	got := f.submit(f.client, login, "/federation/google/start", nil)
	assert.Equal(t, http.StatusForbidden, got.status)
	assert.Contains(t, got.body, string(apperror.CodeSignInUpstreamDisabled))

	fed.startErr = nil
	fed.authErr = &federation.UpstreamError{Reason: oidcrp.ReasonDiscovery}
	moved := f.submit(f.client, f.get(f.client, "/p/acme/login"), "/federation/google/start", nil)
	require.Equal(t, http.StatusSeeOther, moved.status)
	got = f.get(f.client, moved.location())
	assert.Equal(t, http.StatusBadGateway, got.status)
	assert.Contains(t, got.body, string(apperror.CodeSignInUpstreamUnavailable))

	assert.Equal(t, http.StatusBadRequest, f.get(f.newClient(), "/p/acme/federation/google/continue").status, "no state cookie")
}

func TestFederation_StateCookie(t *testing.T) {
	f, _ := newFederationFixture(t)
	_, authURL := f.startGoogle(f.client, "/p/acme/login")
	f.web.cookies.now = func() time.Time { return time.Now().Add(federationTTL + time.Minute) }
	got := f.callback(f.client, url.Values{"code": {"c"}, "state": {authURL.Query().Get("state")}})
	assert.Equal(t, http.StatusBadRequest, got.status, "an expired state is refused even if the browser kept the cookie")
	f.web.cookies.now = time.Now

	s := f.web.site(f.acme)
	other := f.web.site(f.beta)
	rec := &cookieRecorder{header: http.Header{}}
	require.NoError(t, f.web.setFederation(rec, other, federationState{Kind: upstream.KindGoogle, Issued: time.Now().Unix()}))
	r := &http.Request{Header: http.Header{"Cookie": {rec.header.Get("Set-Cookie")}}}
	_, ok := f.web.federation(r, s)
	assert.False(t, ok, "a state sealed for another project never opens")
	_, ok = f.web.federation(r, other)
	assert.True(t, ok)
	r.Header.Set("Cookie", federationCookie+"=!!!")
	_, ok = f.web.federation(r, s)
	assert.False(t, ok)
}

type cookieRecorder struct{ header http.Header }

func (c *cookieRecorder) Header() http.Header         { return c.header }
func (c *cookieRecorder) Write(b []byte) (int, error) { return len(b), nil }
func (c *cookieRecorder) WriteHeader(int)             {}

func TestFederation_SealFails(t *testing.T) {
	f, fed := newFederationFixture(t)
	f.web.WithFederation(fed, sealer.Disabled())
	got := f.submit(f.client, f.get(f.client, "/p/acme/login"), "/federation/google/start", nil)
	assert.Equal(t, http.StatusInternalServerError, got.status, "fails closed")
}

func TestFederation_Link(t *testing.T) {
	f, fed := newFederationFixture(t)
	f.signIn(f.client, "acme", "ana@acme.test")
	b, err := f.users.Browser(t.Context(), f.web.site(f.acme).project(), browserSecret(t, f))
	require.NoError(t, err)
	ana, _ := b.Active()

	link := func() hostedPage {
		form := url.Values{layout.CSRFField: {csrfTokenFor(t, f, hostedPage{}, federationForm)}, "intent": {"link"}}
		moved := f.do(f.client, http.MethodPost, "/p/acme/federation/google/start", form)
		require.Equal(t, http.StatusSeeOther, moved.status, moved.body)
		page := f.get(f.client, moved.location())
		state := mustURL(t, strings.ReplaceAll(page.body[strings.Index(page.body, fakeAuthorize):strings.Index(page.body, fakeAuthorize)+
			strings.IndexAny(page.body[strings.Index(page.body, fakeAuthorize):], `"'`)], "&amp;", "&")).Query().Get("state")
		return f.callback(f.client, url.Values{"code": {"c"}, "state": {state}})
	}
	done := link()
	require.Equal(t, http.StatusSeeOther, done.status, done.body)
	assert.Equal(t, "/p/acme/account/sign-in?linked=1", done.location(), "back to the portal's sign-in methods")
	assert.Equal(t, []uuid.UUID{ana.EndUserID}, fed.linked, "the identity goes to the account that asked")
	assert.Equal(t, []string{""}, fed.hints, "P7: linking sends no hint")

	fed.linkErr = &federation.LinkedElsewhereError{}
	got := link()
	assert.Equal(t, http.StatusForbidden, got.status)
	assert.Contains(t, got.body, string(apperror.CodeSignInUpstreamLinkedElsewhere))
}

func TestFederation_LinkNeedsTheAccount(t *testing.T) {
	f, fed := newFederationFixture(t)
	f.get(f.client, "/p/acme/login")
	form := url.Values{layout.CSRFField: {csrfTokenFor(t, f, hostedPage{}, federationForm)}, "intent": {"link"}}
	got := f.do(f.client, http.MethodPost, "/p/acme/federation/google/start", form)
	assert.Equal(t, http.StatusSeeOther, got.status)
	assert.True(t, strings.HasPrefix(got.location(), "/p/acme/login"), got.location())
	assert.Empty(t, fed.hints, "nothing started")
}

func browserSecret(t *testing.T, f *webFixture) string {
	t.Helper()
	for _, c := range f.client.Jar.Cookies(mustURL(t, f.srv.URL+"/p/acme/")) {
		if c.Name == BrowserCookie {
			r := &http.Request{Header: http.Header{"Cookie": {c.Name + "=" + c.Value}}}
			return f.web.cookies.browser(r)
		}
	}
	t.Fatal("no browser cookie")
	return ""
}

// R-4, F6, P7, A9–A11: a re-authentication hints the account's linked Google email, and that sign-in satisfies the same URL.
func TestFederation_ReauthHandOff(t *testing.T) {
	for _, extra := range [][]string{{"prompt", "login"}, {"max_age", "0"}} {
		f, fed := newFederationFixture(t)
		f.signIn(f.client, "acme", "ana@example.com")
		fed.identities = []federation.Identity{{Kind: upstream.KindGoogle, Email: "ana.g@gmail.com", PublicID: "idn1"}}
		f.startGoogle(f.client, "/p/acme/login?add=1")
		assert.Equal(t, []string{""}, fed.hints, "adding another account hints nothing")
		fed.hints = nil

		first := f.get(f.client, authorizeURL(append(extra, "login_hint", "app-hint@example.com")...))
		require.Equal(t, http.StatusSeeOther, first.status, extra)
		loc, err := url.Parse(first.location())
		require.NoError(t, err)
		back := loc.Query().Get("return_to")
		require.Contains(t, back, MarkerParam)

		_, authURL := f.startGoogle(f.client, first.location())
		assert.Equal(t, []string{"ana.g@gmail.com"}, fed.hints, "%v: the linked Google email, never the app's hint or the Othra email", extra)
		done := f.callback(f.client, url.Values{"code": {"c"}, "state": {authURL.Query().Get("state")}})
		require.Equal(t, http.StatusSeeOther, done.status, done.body)
		assert.Equal(t, back, done.location(), "%v: back to the authorize request", extra)

		final := f.follow(f.client, done)
		assert.Equal(t, http.StatusFound, final.status, "%v: the Google sign-in is newer than the marker", extra)
		assert.NotEmpty(t, redirectParams(t, final).Get("code"))
	}
}

// A25: starting a link is limited per signed-in account, apart from the per-IP form bucket.
func TestFederation_LinkStartLimitedPerSession(t *testing.T) {
	f, _ := newFederationFixture(t)
	clock := ratelimittest.NewClock()
	f.web.WithLimits(layout.Limits{Limiter: memory.New(1000, memory.WithClock(clock.Now))})
	f.signIn(f.client, "acme", "ana@acme.test")
	start := func(c *http.Client) int {
		form := url.Values{layout.CSRFField: {csrfTokenFor(t, f, hostedPage{}, federationForm)}, "intent": {"link"}}
		return f.do(c, http.MethodPost, "/p/acme/federation/google/start", form).status
	}
	for i := range LinkSessionRule().Burst {
		require.Equal(t, http.StatusSeeOther, start(f.client), "start %d", i+1)
	}
	assert.Equal(t, http.StatusTooManyRequests, start(f.client), "the account's 11th link start in an hour")

	other := f.newClient()
	f.client = other
	f.signIn(other, "acme", "budi@acme.test")
	assert.Equal(t, http.StatusSeeOther, start(other), "another account from the same IP still starts")
}

func (f *webFixture) linkGoogle(t *testing.T, callback url.Values, header ...string) hostedPage {
	t.Helper()
	form := url.Values{layout.CSRFField: {csrfTokenFor(t, f, hostedPage{}, federationForm)}, "intent": {"link"}}
	moved := f.do(f.client, http.MethodPost, "/p/acme/federation/google/start", form, header...)
	require.Equal(t, http.StatusSeeOther, moved.status, moved.body)
	page := f.get(f.client, moved.location(), header...)
	i := strings.Index(page.body, fakeAuthorize)
	require.GreaterOrEqual(t, i, 0, page.body)
	raw := page.body[i:]
	state := mustURL(t, strings.ReplaceAll(raw[:strings.IndexAny(raw, `"'`)], "&amp;", "&")).Query().Get("state")
	callback.Set("state", state)
	return f.get(f.client, "/p/acme/federation/google/callback?"+callback.Encode(), header...)
}

// MF7: a failed Connect speaks to the signed-in person: "Couldn't connect Google" and back to the account, never to sign in.
func TestFederation_ConnectFailureWording(t *testing.T) {
	f, fed := newFederationFixture(t)
	f.signIn(f.client, "acme", "ana@acme.test")

	cancelled := f.linkGoogle(t, url.Values{"error": {"access_denied"}})
	assert.Equal(t, http.StatusForbidden, cancelled.status)
	assert.Contains(t, cancelled.body, "Couldn&#39;t connect Google")
	assert.Contains(t, cancelled.body, "Connecting Google was cancelled. Nothing changed.", "BL-3")
	assert.Contains(t, cancelled.body, string(apperror.CodeSignInUpstreamCancelled))
	assert.Contains(t, cancelled.body, `href="/p/acme/account/">Back to your account</a>`)
	assert.NotContains(t, cancelled.body, "Back to sign in")
	assert.NotContains(t, cancelled.body, "Couldn&#39;t sign in with Google")

	fed.linkErr = &federation.LinkedElsewhereError{}
	elsewhere := f.linkGoogle(t, url.Values{"code": {"c"}}, "Accept-Language", "id-ID,id;q=0.9")
	assert.Equal(t, http.StatusForbidden, elsewhere.status)
	assert.Contains(t, elsewhere.body, "Tidak bisa menghubungkan Google")
	assert.Contains(t, elsewhere.body, string(apperror.CodeSignInUpstreamLinkedElsewhere))
	assert.Contains(t, elsewhere.body, `href="/p/acme/account/">Kembali ke akun Anda</a>`)

	_, authURL := f.startGoogle(f.client, "/p/acme/login?add=1")
	signIn := f.callback(f.client, url.Values{"error": {"access_denied"}, "state": {authURL.Query().Get("state")}})
	assert.Contains(t, signIn.body, "Couldn&#39;t sign in with Google", "a sign-in failure keeps the sign-in wording")
	assert.Contains(t, signIn.body, "Back to sign in")
}
