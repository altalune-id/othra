package hosted

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeGrants struct {
	mu      sync.Mutex
	users   *fakeUsers
	codes   []CodeRequest
	hints   map[string]Hint
	ended   []string
	revoked []string
	failOn  string
}

func newFakeGrants(users *fakeUsers) *fakeGrants {
	return &fakeGrants{users: users, hints: map[string]Hint{}}
}

func (g *fakeGrants) IssueCode(_ context.Context, req CodeRequest) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch g.failOn {
	case "code":
		return "", errFake
	case "gone":
		return "", gone()
	}
	g.codes = append(g.codes, req)
	return "code-" + req.UserSession, nil
}

func (g *fakeGrants) VerifyHint(_ context.Context, _ Project, raw string) (Hint, bool, error) {
	if g.failOn == "hint" {
		return Hint{}, false, errFake
	}
	h, ok := g.hints[raw]
	return h, ok, nil
}

func (g *fakeGrants) SessionOf(_ context.Context, _ Project, sid string) (string, bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.failOn == "session" {
		return "", false, errFake
	}
	if !strings.HasPrefix(sid, "sid-") {
		return "", false, nil
	}
	return strings.TrimPrefix(sid, "sid-"), !slices.Contains(g.ended, sid) && !slices.Contains(g.revoked, sid), nil
}

func (g *fakeGrants) EndSession(_ context.Context, p Project, sid, us string) error {
	g.mu.Lock()
	if g.failOn == "end" {
		g.mu.Unlock()
		return errFake
	}
	g.ended = append(g.ended, sid)
	g.mu.Unlock()
	g.users.mu.Lock()
	defer g.users.mu.Unlock()
	for _, b := range g.users.browsers {
		for i, a := range b.accounts {
			if b.project.ID == p.ID && a.UserSession == us {
				b.accounts = append(b.accounts[:i], b.accounts[i+1:]...)
				return nil
			}
		}
	}
	return nil
}

func authorizeURL(extra ...string) string {
	q := valid(extra...)
	return "/p/acme/oauth2/authorize?" + q.Encode()
}

func redirectParams(t *testing.T, p hostedPage) url.Values {
	t.Helper()
	loc := p.location()
	require.True(t, strings.HasPrefix(loc, webRedirect+"?") || strings.HasPrefix(loc, cliRedirect), "%d %s %s", p.status, loc, p.body)
	u, err := url.Parse(loc)
	require.NoError(t, err)
	return u.Query()
}

var refreshRE = regexp.MustCompile(`http-equiv="refresh" content="0;url=([^"]+)"`)

func (f *webFixture) follow(c *http.Client, p hostedPage) hostedPage {
	f.t.Helper()
	if p.status == http.StatusOK {
		m := refreshRE.FindStringSubmatch(p.body)
		require.NotEmpty(f.t, m, "a continue page: %s", p.body)
		target := plain(m[1])
		return f.get(c, strings.TrimPrefix(target, f.srv.URL))
	}
	require.Contains(f.t, []int{http.StatusFound, http.StatusSeeOther}, p.status, p.body)
	return f.get(c, strings.TrimPrefix(p.location(), f.srv.URL))
}

func (f *webFixture) signInVia(c *http.Client, login hostedPage, email string) hostedPage {
	f.t.Helper()
	f.users.mu.Lock()
	f.users.last = map[string]time.Time{}
	f.users.mu.Unlock()
	sent := f.submit(c, login, "/login", url.Values{"email": {email}})
	require.Equal(f.t, http.StatusSeeOther, sent.status, sent.body)
	link := linkIn(f.t, f.users.mails[len(f.users.mails)-1].TextBody)
	moved := f.get(c, strings.TrimPrefix(link, f.srv.URL))
	verify := f.get(c, moved.location())
	return f.submit(c, verify, "/login/verify", nil)
}

// before the redirect URI is trusted, a branded 400 page, never a redirect, and no way to the sign-in page.
func TestAuthorize_PageErrors(t *testing.T) {
	f := newWebFixture(t)
	for path, code := range map[string]string{
		authorizeURL("client_id", "nope"):                              "SIG005",
		authorizeURL("redirect_uri", "https://evil.example/cb"):        "SIG006",
		authorizeURL("response_mode", "form_post"):                     "SIG007",
		"/p/acme/oauth2/authorize?client_id=web&client_id=cli&scope=x": "SIG007",
	} {
		p := f.get(f.client, path)
		assert.Equal(t, http.StatusBadRequest, p.status, path)
		assert.Empty(t, p.location(), path)
		assert.Contains(t, p.body, code, path)
		assert.Contains(t, p.body, "Can&#39;t sign in to this app", path)
		assert.NotContains(t, p.body, "Back to sign in", path)
		assert.NotContains(t, p.body, "evil.example", "the request is never echoed")
	}
}

// after the redirect is trusted, the error, state and iss go to the client.
func TestAuthorize_RedirectErrorsCarryStateAndIss(t *testing.T) {
	f := newWebFixture(t)
	p := f.get(f.client, authorizeURL("response_type", "token"))
	assert.Equal(t, http.StatusFound, p.status)
	q := redirectParams(t, p)
	assert.Equal(t, "unsupported_response_type", q.Get("error"))
	assert.Equal(t, "st", q.Get("state"))
	assert.Equal(t, f.srv.URL+"/p/acme", q.Get("iss"))
	assert.Empty(t, q.Get("code"))
}

// The flow: not signed in → sign in with the client's name shown → the continue page → a code with state and iss.
func TestAuthorize_SignInThenCode(t *testing.T) {
	f := newWebFixture(t)
	first := f.get(f.client, authorizeURL("nonce", "n-1", "login_hint", "ana@example.com"))
	require.Equal(t, http.StatusSeeOther, first.status)
	login := f.get(f.client, first.location())
	assert.Contains(t, login.body, "Sign in to continue to Openwa Web.")
	assert.Contains(t, login.body, `value="ana@example.com"`, "login_hint pre-fills the email")

	done := f.signInVia(f.client, login, "ana@example.com")
	require.Equal(t, http.StatusOK, done.status, "a POST whose next stop is /authorize answers with the continue page")
	assert.Contains(t, done.body, "Continuing to Openwa Web")
	final := f.follow(f.client, done)
	assert.Equal(t, http.StatusFound, final.status, "the last hop to the client is a GET 302")
	q := redirectParams(t, final)
	assert.NotEmpty(t, q.Get("code"))
	assert.Equal(t, "st", q.Get("state"))
	assert.Equal(t, f.srv.URL+"/p/acme", q.Get("iss"))

	require.Len(t, f.grants.codes, 1)
	c := f.grants.codes[0]
	assert.Equal(t, "web", c.ClientID)
	assert.Equal(t, webRedirect, c.RedirectURI)
	assert.Equal(t, []string{"openid", "email"}, c.Scopes)
	assert.Equal(t, challenge, c.CodeChallenge)
	assert.Equal(t, "n-1", c.Nonce)
	assert.NotEmpty(t, c.EndUser)
	assert.False(t, c.AuthTime.IsZero())

	silent := f.get(f.client, authorizeURL())
	assert.Equal(t, http.StatusFound, silent.status, "silent SSO for a signed-in browser")
	assert.NotEmpty(t, redirectParams(t, silent).Get("code"))
}

// prompt=none answers without any page.
func TestAuthorize_PromptNone(t *testing.T) {
	f := newWebFixture(t)
	q := redirectParams(t, f.get(f.client, authorizeURL("prompt", "none")))
	assert.Equal(t, "login_required", q.Get("error"))
	assert.Equal(t, f.srv.URL+"/p/acme", q.Get("iss"))

	f.signIn(f.client, "acme", "ana@example.com")
	q = redirectParams(t, f.get(f.client, authorizeURL("prompt", "none")))
	assert.NotEmpty(t, q.Get("code"), "signed in: a silent code")

	f.signIn(f.client, "acme", "budi@example.com")
	q = redirectParams(t, f.get(f.client, authorizeURL("prompt", "none")))
	assert.Equal(t, "account_selection_required", q.Get("error"))

	q = redirectParams(t, f.get(f.client, authorizeURL("prompt", "none", "max_age", "0")))
	assert.Equal(t, "login_required", q.Get("error"))
}

// prompt=login and max_age=0 need a fresh sign-in, and the marker stops the loop once it happened.
func TestAuthorize_PromptLoginDoesNotLoop(t *testing.T) {
	for _, extra := range [][]string{{"prompt", "login"}, {"max_age", "0"}} {
		f := newWebFixture(t)
		f.signIn(f.client, "acme", "ana@example.com")
		first := f.get(f.client, authorizeURL(extra...))
		require.Equal(t, http.StatusSeeOther, first.status, extra)
		assert.Contains(t, first.location(), "add=1", "a fresh sign-in skips the picker")
		assert.Contains(t, first.location(), MarkerParam)
		login := f.get(f.client, first.location())
		assert.Contains(t, login.body, "Openwa Web asks you to sign in again.")

		loc, err := url.Parse(first.location())
		require.NoError(t, err)
		stale := f.get(f.client, loc.Query().Get("return_to"))
		assert.Equal(t, http.StatusSeeOther, stale.status, "the old sign-in is older than the marker: sign in again")

		done := f.signInVia(f.client, login, "ana@example.com")
		final := f.follow(f.client, done)
		assert.Equal(t, http.StatusFound, final.status, "%v: after a fresh sign-in the same URL gives a code", extra)
		assert.NotEmpty(t, redirectParams(t, final).Get("code"))
	}
}

// a long max_age is satisfied by the existing sign-in.
func TestAuthorize_MaxAgeLong(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	assert.NotEmpty(t, redirectParams(t, f.get(f.client, authorizeURL("max_age", "10000"))).Get("code"))
}

// select_account, two accounts and a login_hint for another account show the picker; choosing gives a code.
func TestAuthorize_Picker(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	first := f.get(f.client, authorizeURL("prompt", "select_account"))
	require.Equal(t, http.StatusSeeOther, first.status)
	assert.NotContains(t, first.location(), "add=1")
	picker := f.get(f.client, first.location())
	assert.Contains(t, picker.body, "to continue to Openwa Web")
	chosen := f.submit(f.client, picker, "/login/select", nil)
	require.Equal(t, http.StatusOK, chosen.status, "the continue page")
	assert.NotEmpty(t, redirectParams(t, f.follow(f.client, chosen)).Get("code"), "the choice satisfies select_account")

	hinted := f.get(f.client, authorizeURL("login_hint", "budi@example.com"))
	assert.Equal(t, http.StatusSeeOther, hinted.status, "a hint for another account shows the picker")

	f.signIn(f.client, "acme", "budi@example.com")
	two := f.get(f.client, authorizeURL())
	assert.Equal(t, http.StatusSeeOther, two.status, "two accounts: the picker")
}

// owed terms send to /terms; prompt=none answers interaction_required; Decline answers access_denied.
func TestAuthorize_Terms(t *testing.T) {
	f := newWebFixture(t)
	f.acme.TermsVersion, f.acme.TermsURL = 1, "https://acme.example/terms"
	f.signIn(f.client, "acme", "ana@example.com")
	q := redirectParams(t, f.get(f.client, authorizeURL("prompt", "none")))
	assert.Equal(t, "interaction_required", q.Get("error"))

	gate := f.get(f.client, authorizeURL())
	require.Equal(t, http.StatusSeeOther, gate.status)
	assert.True(t, strings.HasPrefix(gate.location(), "/p/acme/terms?"), gate.location())
	terms := f.get(f.client, gate.location())
	declined := f.submit(f.client, terms, "/terms", url.Values{"decline": {"1"}})
	require.Equal(t, http.StatusOK, declined.status, "the continue page")
	m := refreshRE.FindStringSubmatch(declined.body)
	require.NotEmpty(t, m)
	u, err := url.Parse(plain(m[1]))
	require.NoError(t, err)
	assert.Equal(t, "access_denied", u.Query().Get("error"))
	assert.Equal(t, "st", u.Query().Get("state"))
	assert.Equal(t, f.srv.URL+"/p/acme", u.Query().Get("iss"))

	accepted := f.submit(f.client, terms, "/terms", url.Values{"accept_terms": {"1"}})
	require.Equal(t, http.StatusOK, accepted.status)
	assert.NotEmpty(t, redirectParams(t, f.follow(f.client, accepted)).Get("code"))
}

func TestTerms_DeclineWithoutAuthorize(t *testing.T) {
	f := newWebFixture(t)
	f.acme.TermsVersion, f.acme.TermsURL = 1, "https://acme.example/terms"
	f.acme.SupportEmail = "help@acme.example"
	f.signIn(f.client, "acme", "ana@example.com")
	terms := f.get(f.client, "/p/acme/terms")
	declined := f.submit(f.client, terms, "/terms", url.Values{"decline": {"1"}})
	assert.Equal(t, http.StatusOK, declined.status)
	assert.Contains(t, declined.body, "You didn&#39;t accept the terms")
	assert.Contains(t, declined.body, "Need help? Email", "the project's support email")
	assert.Contains(t, declined.body, `href="mailto:help@acme.example"`)
}

// prompt=consent asks every client; a require client asks first; prompt=none can only answer consent_required.
func TestAuthorize_Consent(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	for _, u := range []string{authorizeURL("prompt", "consent"), authorizeURL("client_id", "req")} {
		p := f.get(f.client, u)
		assert.Equal(t, http.StatusSeeOther, p.status, u)
		assert.True(t, strings.HasPrefix(p.location(), "/p/acme/consent?"), p.location())
	}
	q := redirectParams(t, f.get(f.client, authorizeURL("client_id", "req", "prompt", "none")))
	assert.Equal(t, "consent_required", q.Get("error"))
}

// a hint for another account: prompt=none says login_required, otherwise the picker; a forged hint is invalid_request.
func TestAuthorize_IDTokenHint(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	f.grants.hints["other"] = Hint{Subject: "someone-else"}
	q := redirectParams(t, f.get(f.client, authorizeURL("prompt", "none", "id_token_hint", "other")))
	assert.Equal(t, "login_required", q.Get("error"))
	assert.Equal(t, http.StatusSeeOther, f.get(f.client, authorizeURL("id_token_hint", "other")).status)
	q = redirectParams(t, f.get(f.client, authorizeURL("id_token_hint", "forged")))
	assert.Equal(t, "invalid_request", q.Get("error"))

	var sub string
	for _, b := range f.users.browsers {
		sub = b.accounts[0].EndUser
	}
	f.grants.hints["mine"] = Hint{Subject: sub}
	assert.NotEmpty(t, redirectParams(t, f.get(f.client, authorizeURL("prompt", "none", "id_token_hint", "mine"))).Get("code"))
}

// the authorize endpoint takes a form POST too.
func TestAuthorize_Post(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	p := f.do(f.client, http.MethodPost, "/p/acme/oauth2/authorize", valid())
	assert.Equal(t, http.StatusFound, p.status)
	assert.NotEmpty(t, redirectParams(t, p).Get("code"))
}

func TestAuthorize_Failures(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	f.grants.failOn = "code"
	assert.Equal(t, http.StatusInternalServerError, f.get(f.client, authorizeURL()).status)
	f.grants.failOn = "hint"
	assert.Equal(t, http.StatusInternalServerError, f.get(f.client, authorizeURL("id_token_hint", "x")).status)
	f.grants.failOn = ""
	f.users.failOn = "browser"
	assert.Equal(t, http.StatusInternalServerError, f.get(f.client, authorizeURL()).status)
	f.users.failOn = "terms"
	assert.Equal(t, http.StatusInternalServerError, f.get(f.client, authorizeURL()).status)
}

// A session that ended between the browser lookup and the code goes back to sign-in, or login_required under prompt=none.
func TestAuthorize_SessionGoneWhileIssuing(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	f.grants.failOn = "gone"
	p := f.get(f.client, authorizeURL())
	assert.Equal(t, http.StatusSeeOther, p.status)
	assert.Contains(t, p.location(), "/p/acme/login?")
	assert.Equal(t, "login_required", redirectParams(t, f.get(f.client, authorizeURL("prompt", "none"))).Get("error"))
}

// the marker lives as long as a magic link; a slow but fresh sign-in passes, a marker older than the link is asked again.
func TestAuthorize_MarkerLivesAsLongAsTheLink(t *testing.T) {
	for wait, wantCode := range map[time.Duration]bool{2 * time.Minute: true, 16 * time.Minute: false} {
		f := newWebFixture(t)
		f.signIn(f.client, "acme", "ana@example.com")
		first := f.get(f.client, authorizeURL("prompt", "login"))
		require.Equal(t, http.StatusSeeOther, first.status)
		login := f.get(f.client, first.location())
		f.users.stamp.advance(wait)
		final := f.follow(f.client, f.signInVia(f.client, login, "ana@example.com"))
		if wantCode {
			assert.NotEmpty(t, redirectParams(t, final).Get("code"), wait)
			continue
		}
		assert.Equal(t, http.StatusSeeOther, final.status, wait)
		assert.Contains(t, final.location(), "/p/acme/login?", "%v: asked to sign in again", wait)
	}
}

// the id_token_hint never reaches a return_to (login, picker, terms, magic link); its subject crosses signed.
func TestAuthorize_HintNeverInReturnTo(t *testing.T) {
	f := newWebFixture(t)
	f.acme.TermsVersion, f.acme.TermsURL = 1, "https://acme.example/terms"
	f.signIn(f.client, "acme", "ana@example.com")
	raw := "hint-" + strings.Repeat("h", 7000)
	f.grants.hints[raw] = Hint{Subject: "someone-else"}
	toPicker := f.get(f.client, authorizeURL("id_token_hint", raw))
	require.Equal(t, http.StatusSeeOther, toPicker.status, toPicker.body)
	assert.NotContains(t, toPicker.location(), "id_token_hint")
	assert.NotContains(t, toPicker.location(), "hhhh")
	assert.Contains(t, toPicker.location(), HintParam)

	var sub string
	for _, b := range f.users.browsers {
		sub = b.accounts[0].EndUser
	}
	f.grants.hints[raw] = Hint{Subject: sub}
	toTerms := f.get(f.client, authorizeURL("id_token_hint", raw))
	require.Equal(t, http.StatusSeeOther, toTerms.status, toTerms.body)
	assert.True(t, strings.HasPrefix(toTerms.location(), "/p/acme/terms?"), toTerms.location())
	assert.NotContains(t, toTerms.location(), "id_token_hint")

	phone := f.newClient()
	fresh := f.get(phone, authorizeURL("id_token_hint", raw, "claims", strings.Repeat("c", 400)))
	require.Equal(t, http.StatusSeeOther, fresh.status)
	assert.NotContains(t, fresh.location(), "id_token_hint")
	assert.NotContains(t, fresh.location(), "claims", "only the request's own parameters go back")
	f.signInVia(phone, f.get(phone, fresh.location()), "ana@example.com")
	issued := f.users.issued[len(f.users.issued)-1]
	assert.Contains(t, issued.ReturnTo, HintParam)
	assert.NotContains(t, issued.ReturnTo, "id_token_hint", "the magic link's return_to")
}

// a request whose way back can't fit a magic link's return_to is refused, never a 500.
func TestAuthorize_TooLongToComeBack(t *testing.T) {
	f := newWebFixture(t)
	long := strings.Repeat("/", maxParamLen)
	q := redirectParams(t, f.get(f.client, authorizeURL("state", long, "nonce", long, "login_hint", long)))
	assert.Equal(t, "invalid_request", q.Get("error"))
	assert.Equal(t, long, q.Get("state"))
}

// OIDC §3.1.2.2: after the picker, an account other than the hinted one answers login_required.
func TestAuthorize_HintedAccountNotChosen(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	f.signIn(f.client, "acme", "cici@example.com")
	var ana string
	for _, b := range f.users.browsers {
		for _, a := range b.accounts {
			if a.Email == "ana@example.com" {
				ana = a.EndUser
			}
		}
	}
	f.grants.hints["ana"] = Hint{Subject: ana}
	first := f.get(f.client, authorizeURL("id_token_hint", "ana"))
	require.Equal(t, http.StatusSeeOther, first.status)
	picker := f.get(f.client, first.location())
	chosen := f.submit(f.client, picker, "/login/select", url.Values{"user_session": {f.accountOf("cici@example.com").UserSession}})
	q := redirectParams(t, f.follow(f.client, chosen))
	assert.Equal(t, "login_required", q.Get("error"))
	assert.Equal(t, "st", q.Get("state"))

	picker = f.get(f.client, first.location())
	chosen = f.submit(f.client, picker, "/login/select", url.Values{"user_session": {f.accountOf("ana@example.com").UserSession}})
	assert.NotEmpty(t, redirectParams(t, f.follow(f.client, chosen)).Get("code"), "the hinted account gets its code")
}

func (f *webFixture) accountOf(email string) Account {
	f.t.Helper()
	for _, b := range f.users.browsers {
		for _, a := range b.accounts {
			if a.Email == email {
				return a
			}
		}
	}
	f.t.Fatalf("no account for %s", email)
	return Account{}
}

// a signed hint subject that expired or was forged answers login_required, so waiting can't skip the account check.
func TestAuthorize_HintSubjectExpired(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	f.grants.hints["other"] = Hint{Subject: "someone-else"}
	first := f.get(f.client, authorizeURL("id_token_hint", "other"))
	require.Equal(t, http.StatusSeeOther, first.status)
	loc, err := url.Parse(first.location())
	require.NoError(t, err)
	back := loc.Query().Get("return_to")
	require.Contains(t, back, HintParam)

	f.users.stamp.advance(16 * time.Minute)
	q := redirectParams(t, f.get(f.client, back))
	assert.Equal(t, "login_required", q.Get("error"))
	assert.Equal(t, "st", q.Get("state"))
	assert.Equal(t, f.srv.URL+"/p/acme", q.Get("iss"))

	forged := redirectParams(t, f.get(f.client, authorizeURL(HintParam, "AAAA")))
	assert.Equal(t, "login_required", forged.Get("error"))
}

// ui_locales goes back to /authorize with the rest of the request.
func TestAuthorize_KeepsUILocales(t *testing.T) {
	f := newWebFixture(t)
	first := f.get(f.client, authorizeURL("ui_locales", "id-ID en"))
	require.Equal(t, http.StatusSeeOther, first.status)
	loc, err := url.Parse(first.location())
	require.NoError(t, err)
	back, err := url.Parse(loc.Query().Get("return_to"))
	require.NoError(t, err)
	assert.Equal(t, "id-ID en", back.Query().Get("ui_locales"))
}

func TestMarkerExpiryIsConfigured(t *testing.T) {
	m := NewMarker([]byte("k"), time.Minute)
	assert.Equal(t, time.Minute, m.maxAge)
}
