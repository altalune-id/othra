package hosted

import (
	"context"
	"html"
	"io"
	"maps"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/kernel/ratelimit"
	"altalune.id/othra/internal/kernel/ratelimit/memory"
	"altalune.id/othra/internal/kernel/ratelimit/ratelimittest"
	"altalune.id/othra/internal/oauth/oidc"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/web/i18n"
	"altalune.id/othra/internal/web/layout"
	"altalune.id/othra/internal/web/middleware"
)

type webFixture struct {
	t          *testing.T
	srv        *httptest.Server
	web        *Web
	users      *fakeUsers
	issuers    *fakeIssuers
	acme       *project.Issuer
	beta       *project.Issuer
	client     *http.Client
	reports    []string
	grants     *fakeGrants
	admissions *fakeAdmissions
	consents   *fakeConsents
}

func newIssuer(handle, name string) *project.Issuer {
	return &project.Issuer{
		ProjectID: uuid.New(), OrgID: uuid.New(), PublicID: "p" + strings.Repeat(handle[:1], 20), Handle: handle, Name: name,
		DisplayName: name, PrimaryColor: "#1e3a8a", SupportEmail: "help@" + handle + ".example", SignInEmailLink: true,
	}
}

func newWebFixture(t *testing.T) *webFixture {
	t.Helper()
	f := &webFixture{t: t, users: newFakeUsers()}
	f.acme, f.beta = newIssuer("acme", "Acme"), newIssuer("beta", "Beta")
	f.issuers = &fakeIssuers{byHandle: map[string]*project.Issuer{"acme": f.acme, "beta": f.beta}}
	mux := http.NewServeMux()
	var handler http.Handler = mux
	f.srv = httptest.NewUnstartedServer(nil)
	kit := layout.Kit{Unexpected: func(_ context.Context, msg string, err error, _ ...any) *apperror.AppError {
		f.reports = append(f.reports, msg)
		return apperror.New(apperror.CodeUnexpected, "unexpected").WithCause(err)
	}}
	f.srv.Config.Handler = middleware.NoStore(middleware.NoReferrer(middleware.HostedCSP("https://cdn.example")(
		http.NewCrossOriginProtection().Handler(i18n.Middleware(handler)))))
	f.srv.StartTLS()
	t.Cleanup(f.srv.Close)
	f.web = NewWeb(f.users, f.issuers, kit, Config{
		PublicBase: f.srv.URL, MailFrom: "Othra <no-reply@othra.example>", CSRFKey: []byte("csrf-key-csrf-key-csrf-key-csrf-k"),
		CookieKey: []byte("cookie-key-cookie-key-cookie-key"), CSRFMaxAge: time.Hour, LinkTTL: 15 * time.Minute,
		LogoURL: func(k string) string { return "https://cdn.example/" + k },
	})
	f.grants = newFakeGrants(f.users)
	marker := NewMarker([]byte("marker-key-marker-key-marker-key"), 15*time.Minute)
	marker.now = f.users.stamp.now
	f.admissions, f.consents = &fakeAdmissions{}, newFakeConsents(f.users.stamp.now)
	f.web.WithOAuth(OAuth{
		Clients: testClients(), Resources: testResources(), Admissions: f.admissions, Consents: f.consents, Grants: f.grants, Marker: marker,
	})
	f.web.Register(mux)
	f.client = f.newClient()
	return f
}

func (f *webFixture) newClient() *http.Client {
	jar, err := cookiejar.New(nil)
	require.NoError(f.t, err)
	c := *f.srv.Client()
	c.Jar = jar
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &c
}

type hostedPage struct {
	status int
	body   string
	header http.Header
}

func (p hostedPage) location() string { return p.header.Get("Location") }

func (f *webFixture) do(c *http.Client, method, path string, form url.Values, header ...string) hostedPage {
	f.t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(f.t.Context(), method, f.srv.URL+path, body)
	require.NoError(f.t, err)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	resp, err := c.Do(req)
	require.NoError(f.t, err)
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	require.NoError(f.t, err)
	return hostedPage{status: resp.StatusCode, body: string(b), header: resp.Header}
}

func (f *webFixture) get(c *http.Client, path string, header ...string) hostedPage {
	f.t.Helper()
	return f.do(c, http.MethodGet, path, nil, header...)
}

var (
	formRE  = regexp.MustCompile(`(?s)<form[^>]*action="([^"]+)"(.*?)</form>`)
	csrfRE  = regexp.MustCompile(`name="csrf" value="([^"]+)"`)
	inputRE = regexp.MustCompile(`<input type="hidden" name="([a-z_]+)" value="([^"]*)"`)
)

func (f *webFixture) submit(c *http.Client, p hostedPage, action string, extra url.Values, header ...string) hostedPage {
	f.t.Helper()
	for _, m := range formRE.FindAllStringSubmatch(p.body, -1) {
		if !strings.HasSuffix(html.UnescapeString(m[1]), action) {
			continue
		}
		form := url.Values{}
		for _, in := range inputRE.FindAllStringSubmatch(m[2], -1) {
			form.Set(in[1], html.UnescapeString(in[2]))
		}
		maps.Copy(form, extra)
		target, err := url.Parse(html.UnescapeString(m[1]))
		require.NoError(f.t, err)
		return f.do(c, http.MethodPost, target.Path, form, header...)
	}
	f.t.Fatalf("no form posting to %s in:\n%s", action, p.body)
	return hostedPage{}
}

func (f *webFixture) signIn(c *http.Client, handle, email string) hostedPage {
	f.t.Helper()
	login := f.get(c, "/p/"+handle+"/login?add=1")
	require.Equal(f.t, http.StatusOK, login.status, login.body)
	sent := f.submit(c, login, "/login", url.Values{"email": {email}})
	require.Equal(f.t, http.StatusSeeOther, sent.status, sent.body)
	link := f.users.mails[len(f.users.mails)-1]
	verifyURL := linkIn(f.t, link.TextBody)
	moved := f.get(c, strings.TrimPrefix(verifyURL, f.srv.URL))
	require.Equal(f.t, http.StatusSeeOther, moved.status)
	verify := f.get(c, moved.location())
	require.Equal(f.t, http.StatusOK, verify.status, verify.body)
	return f.submit(c, verify, "/login/verify", nil)
}

var tagRE = regexp.MustCompile(`<[^>]*>`)

func plain(body string) string { return html.UnescapeString(tagRE.ReplaceAllString(body, "")) }

var linkRE = regexp.MustCompile(`https://\S+/login/verify\?t=\S+`)

func linkIn(t *testing.T, text string) string {
	t.Helper()
	m := linkRE.FindString(text)
	require.NotEmpty(t, m, text)
	return m
}

// J1: a first sign-in by magic link lands on the account page.
func TestWeb_FirstSignIn(t *testing.T) {
	f := newWebFixture(t)
	login := f.get(f.client, "/p/acme/login")
	assert.Equal(t, http.StatusOK, login.status)
	assert.Contains(t, login.body, "Sign in to continue to Acme.")
	assert.Equal(t, "no-store", login.header.Get("Cache-Control"))
	assert.Equal(t, "no-referrer", login.header.Get("Referrer-Policy"))

	sent := f.submit(f.client, login, "/login", url.Values{"email": {"Ana@Example.com"}})
	require.Equal(t, http.StatusSeeOther, sent.status, sent.body)
	assert.Equal(t, "/p/acme/login/sent?return_to=%2Fp%2Facme%2Faccount%2F", sent.location())
	require.Len(t, f.users.issued, 1)
	assert.Equal(t, "ana@example.com", f.users.issued[0].Email)
	assert.Equal(t, f.srv.URL+"/p/acme", f.users.issued[0].Project.IssuerURL)

	page := f.get(f.client, sent.location())
	assert.Contains(t, page.body, "ana@example.com")
	assert.Contains(t, page.body, "15 minutes")
	assert.NotContains(t, sent.location(), "ana", "the email never reaches the URL")

	verifyURL := linkIn(t, f.users.mails[0].TextBody)
	moved := f.get(f.client, strings.TrimPrefix(verifyURL, f.srv.URL))
	require.Equal(t, http.StatusSeeOther, moved.status)
	assert.Equal(t, "/p/acme/login/verify", moved.location(), "H3: the token leaves the URL")
	verify := f.get(f.client, moved.location())
	assert.Contains(t, verify.body, "ana@example.com")
	assert.Zero(t, f.users.redeems, "C5: GET never redeems")
	again := f.get(f.client, moved.location())
	assert.Equal(t, http.StatusOK, again.status, "a second GET still only shows the page")

	done := f.submit(f.client, verify, "/login/verify", nil)
	require.Equal(t, http.StatusSeeOther, done.status, done.body)
	assert.Equal(t, "/p/acme/account/", done.location())
	assert.Equal(t, 1, f.users.redeems, "GET twice and POST once give one session")
	repeat := f.submit(f.client, verify, "/login/verify", nil)
	assert.Equal(t, http.StatusSeeOther, repeat.status, "H23: a second Continue lands signed in")
	assert.Equal(t, "/p/acme/account/", repeat.location())
	assert.Equal(t, 1, f.users.redeems)

	account := f.get(f.client, "/p/acme/account/")
	assert.Equal(t, http.StatusOK, account.status)
	assert.Contains(t, account.body, "ana@example.com")
}

func TestWeb_Cookies(t *testing.T) {
	f := newWebFixture(t)
	login := f.get(f.client, "/p/acme/login")
	sent := f.submit(f.client, login, "/login", url.Values{"email": {"ana@example.com"}})
	verifyURL := linkIn(t, f.users.mails[0].TextBody)
	moved := f.get(f.client, strings.TrimPrefix(verifyURL, f.srv.URL))
	verify := f.get(f.client, moved.location())
	done := f.submit(f.client, verify, "/login/verify", nil)
	attrs := map[string]string{}
	for _, p := range []hostedPage{login, sent, moved, done} {
		for _, line := range p.header.Values("Set-Cookie") {
			name, _, _ := strings.Cut(line, "=")
			attrs[name] += line + "\n"
		}
	}
	for name, path := range map[string]string{
		CSRFCookie: "Path=/p/acme/", sentCookie: "Path=/p/acme/login", linkCookie: "Path=/p/acme/login/verify", BrowserCookie: "Path=/p/acme/",
	} {
		lines := attrs[name]
		require.NotEmpty(t, lines, name)
		assert.Contains(t, lines, path, name)
		assert.Contains(t, lines, "Secure", name)
		assert.Contains(t, lines, "HttpOnly", name)
		assert.Contains(t, lines, "SameSite=Lax", name)
	}
	assert.Contains(t, attrs[linkCookie], "Max-Age=0", "the link cookie is cleared after the redeem")
	assert.NotContains(t, attrs[BrowserCookie], f.users.issued[0].Email)
}

// H4, D3a: sign-in renews the CSRF cookie, so a token rendered before it stops working.
func TestWeb_SignInRenewsTheCSRFCookie(t *testing.T) {
	f := newWebFixture(t)
	before := f.get(f.client, "/p/acme/login?add=1")
	f.signIn(f.client, "acme", "ana@example.com")
	stale := f.submit(f.client, before, "/login", url.Values{"email": {"ana@example.com"}})
	assert.Equal(t, http.StatusForbidden, stale.status)
	assert.Contains(t, stale.body, "This page expired")
	assert.Contains(t, stale.body, "SIG004")
	assert.Contains(t, plain(stale.body), "Go back, reload the page and try again.", "D10: the body does not repeat the title")
}

// J2, J3, J5: the picker lists both accounts, selects one, and signs one out with a notice.
func TestWeb_PickerSelectAndSignOut(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	f.signIn(f.client, "acme", "budi@example.com")
	picker := f.get(f.client, "/p/acme/login")
	assert.Contains(t, picker.body, "Choose an account")
	assert.Contains(t, picker.body, "ana@example.com")
	assert.Contains(t, picker.body, "budi@example.com")

	chosen := f.submit(f.client, picker, "/login/select", nil)
	require.Equal(t, http.StatusSeeOther, chosen.status, chosen.body)
	assert.Equal(t, "/p/acme/account/", chosen.location())

	out := f.submit(f.client, picker, "/login/signout", nil)
	require.Equal(t, http.StatusSeeOther, out.status, out.body)
	assert.Equal(t, "/p/acme/login?return_to=%2Fp%2Facme%2Faccount%2F", out.location())
	after := f.get(f.client, out.location())
	assert.Equal(t, 1, strings.Count(plain(after.body), "is signed out on this browser."), "the notice is translated once")
	assert.Contains(t, plain(after.body), "budi@example.com is signed out on this browser.")
	assert.Equal(t, 1, strings.Count(after.body, "/login/select\""), "one tile left")
	assert.NotContains(t, f.get(f.client, out.location()).body, "is signed out", "the notice shows once")

	last := f.submit(f.client, after, "/login/signout", nil)
	assert.Equal(t, http.StatusSeeOther, last.status)
	empty := f.get(f.client, last.location())
	assert.Contains(t, empty.body, "Continue with email", "no tile left: the email form")
}

func TestWeb_AccountSignOutAndSignedOut(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	account := f.get(f.client, "/p/acme/account/")
	out := f.submit(f.client, account, "/login/signout", url.Values{"next": {"signed_out"}})
	require.Equal(t, http.StatusSeeOther, out.status)
	assert.Equal(t, "/p/acme/signed-out", out.location())
	assert.Contains(t, f.get(f.client, out.location()).body, "You signed out of Acme")
	gone := f.get(f.client, "/p/acme/account/")
	assert.Equal(t, http.StatusSeeOther, gone.status)
	assert.Equal(t, "/p/acme/login?return_to=%2Fp%2Facme%2Faccount%2F", gone.location())
}

// SIG003: a tile whose session ended elsewhere goes back to the picker with the notice.
func TestWeb_StaleTile(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	f.signIn(f.client, "acme", "budi@example.com")
	picker := f.get(f.client, "/p/acme/login")
	for _, b := range f.users.browsers {
		b.accounts = b.accounts[1:]
	}
	stale := f.submit(f.client, picker, "/login/select", nil)
	assert.Equal(t, http.StatusOK, stale.status)
	assert.Contains(t, stale.body, "isn&#39;t signed in on this browser anymore")
}

// J4, S8: the terms gate shows for a new version, records the account named in the form, then continues.
func TestWeb_TermsGate(t *testing.T) {
	f := newWebFixture(t)
	f.acme.TermsVersion, f.acme.TermsURL = 1, "https://acme.example/terms"
	done := f.signIn(f.client, "acme", "ana@example.com")
	require.Equal(t, http.StatusSeeOther, done.status)
	assert.Equal(t, "/p/acme/terms?return_to=%2Fp%2Facme%2Faccount%2F", done.location())
	terms := f.get(f.client, done.location())
	assert.Contains(t, terms.body, "One quick step before we create your Acme account.")
	assert.Contains(t, terms.body, "https://acme.example/terms")
	assert.NotContains(t, terms.body, "Privacy Policy", "H20: no privacy URL, no privacy link")

	unticked := f.submit(f.client, terms, "/terms", nil)
	assert.Equal(t, http.StatusUnprocessableEntity, unticked.status)
	assert.Contains(t, unticked.body, "Tick the box")

	accepted := f.submit(f.client, terms, "/terms", url.Values{"accept_terms": {"1"}})
	require.Equal(t, http.StatusSeeOther, accepted.status, accepted.body)
	assert.Equal(t, "/p/acme/account/", accepted.location())
	assert.Equal(t, http.StatusOK, f.get(f.client, "/p/acme/account/").status)

	f.acme.TermsVersion = 2
	assert.Equal(t, http.StatusSeeOther, f.get(f.client, "/p/acme/account/").status, "a version bump asks again")
	updated := f.get(f.client, "/p/acme/terms?return_to=%2Fp%2Facme%2Faccount%2F")
	assert.Contains(t, updated.body, "Acme has updated its terms.")

	f.acme.TermsVersion = 3
	stale := f.submit(f.client, updated, "/terms", url.Values{"accept_terms": {"1"}})
	assert.Equal(t, http.StatusSeeOther, stale.status, "terms bumped while the page was open: shown again")
	assert.Equal(t, "/p/acme/terms?return_to=%2Fp%2Facme%2Faccount%2F", stale.location())
	updated = f.get(f.client, stale.location())
	forged := f.submit(f.client, updated, "/terms", url.Values{"accept_terms": {"1"}, "user_session": {"someone-elses"}})
	assert.Equal(t, http.StatusOK, forged.status)
	assert.Contains(t, forged.body, "Choose an account", "T4: a session not on this browser is refused")
}

// D1, S6: two projects in one browser keep separate sessions; the cookie of one never signs in to the other.
func TestWeb_TwoProjectsStayApart(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	assert.Equal(t, http.StatusOK, f.get(f.client, "/p/acme/account/").status)
	assert.Equal(t, http.StatusSeeOther, f.get(f.client, "/p/beta/account/").status, "beta has no session")
	assert.Contains(t, f.get(f.client, "/p/beta/login").body, "Continue with email")

	acmeCookie := ""
	u, err := url.Parse(f.srv.URL + "/p/acme/")
	require.NoError(t, err)
	for _, c := range f.client.Jar.Cookies(u) {
		if c.Name == BrowserCookie {
			acmeCookie = c.Value
		}
	}
	require.NotEmpty(t, acmeCookie)
	other := f.newClient()
	page := f.get(other, "/p/beta/account/", "Cookie", BrowserCookie+"="+acmeCookie)
	assert.Equal(t, http.StatusSeeOther, page.status, "acme's browser secret means nothing to beta")
}

// D1: the link opened in another browser signs that browser in, not the one that asked.
func TestWeb_CrossDeviceLink(t *testing.T) {
	f := newWebFixture(t)
	laptop, phone := f.client, f.newClient()
	login := f.get(laptop, "/p/acme/login")
	f.submit(laptop, login, "/login", url.Values{"email": {"ana@example.com"}})
	verifyURL := linkIn(t, f.users.mails[0].TextBody)
	moved := f.get(phone, strings.TrimPrefix(verifyURL, f.srv.URL))
	verify := f.get(phone, moved.location())
	assert.Contains(t, plain(verify.body), "You're signing in as ana@example.com on this device.")
	done := f.submit(phone, verify, "/login/verify", nil)
	assert.Equal(t, "/p/acme/account/", done.location())
	assert.Equal(t, http.StatusOK, f.get(phone, "/p/acme/account/").status)
	assert.Equal(t, http.StatusSeeOther, f.get(laptop, "/p/acme/account/").status, "the laptop is not signed in")
}

// J7: an unknown, used or missing link shows the same 400 page.
func TestWeb_LinkNotUsable(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	used := linkIn(t, f.users.mails[0].TextBody)
	other := f.newClient()
	for name, path := range map[string]string{"used": used, "unknown": f.srv.URL + "/p/acme/login/verify?t=nope"} {
		moved := f.get(other, strings.TrimPrefix(path, f.srv.URL))
		page := f.get(other, moved.location())
		assert.Equal(t, http.StatusBadRequest, page.status, name)
		assert.Contains(t, page.body, "This link can&#39;t be used", name)
	}
	assert.Equal(t, http.StatusBadRequest, f.get(f.newClient(), "/p/acme/login/verify").status, "no cookie at all")
	beta := f.newClient()
	moved := f.get(beta, "/p/beta/login/verify?t=whatever")
	assert.Equal(t, http.StatusBadRequest, f.get(beta, moved.location()).status)
}

// RFC 9700 §4.11: an off-issuer return_to falls back to the account hostedPage on every hop.
func TestWeb_ReturnToFallsBack(t *testing.T) {
	f := newWebFixture(t)
	login := f.get(f.client, "/p/acme/login?return_to=https%3A%2F%2Fevil.example%2F")
	assert.Contains(t, login.body, `value="/p/acme/account/"`)
	sent := f.submit(f.client, login, "/login", url.Values{"email": {"ana@example.com"}, "return_to": {"//evil.example"}})
	assert.Equal(t, "/p/acme/login/sent?return_to=%2Fp%2Facme%2Faccount%2F", sent.location())
	assert.Equal(t, "/p/acme/account/", f.users.issued[0].ReturnTo)

	ok := f.get(f.client, "/p/acme/login?return_to=%2Fp%2Facme%2Foauth2%2Fauthorize%3Fclient_id%3Dabc")
	assert.Contains(t, ok.body, `value="/p/acme/oauth2/authorize?client_id=abc"`)
}

func TestWeb_InvalidEmailAndDisabledEmail(t *testing.T) {
	f := newWebFixture(t)
	login := f.get(f.client, "/p/acme/login")
	bad := f.submit(f.client, login, "/login", url.Values{"email": {"not an email"}})
	assert.Equal(t, http.StatusUnprocessableEntity, bad.status)
	assert.Contains(t, bad.body, "Enter a valid email address.")
	assert.Contains(t, bad.body, `value="not an email"`)
	assert.Empty(t, f.users.issued)

	f.acme.SignInEmailLink = false
	off := f.get(f.client, "/p/acme/login")
	assert.Equal(t, http.StatusForbidden, off.status)
	assert.Contains(t, off.body, "Signing in by email is turned off")
}

// D2: the pages follow Accept-Language.
func TestWeb_Indonesian(t *testing.T) {
	f := newWebFixture(t)
	login := f.get(f.client, "/p/acme/login", "Accept-Language", "id-ID,id;q=0.9")
	assert.Contains(t, login.body, `lang="id-ID"`)
	assert.Contains(t, login.body, "Masuk untuk melanjutkan ke Acme.")
	assert.Contains(t, login.body, "Lanjutkan dengan email")
}

// D4, H13: the mail is from the project's name at mail.from, with Reply-To the support email, in the request's language.
func TestWeb_Mail(t *testing.T) {
	f := newWebFixture(t)
	login := f.get(f.client, "/p/acme/login", "Accept-Language", "id")
	f.submit(f.client, login, "/login", url.Values{"email": {"ana@example.com"}}, "Accept-Language", "id")
	require.Len(t, f.users.mails, 1)
	m := f.users.mails[0]
	assert.Equal(t, `"Acme" <no-reply@othra.example>`, m.From)
	assert.Equal(t, "help@acme.example", m.ReplyTo)
	assert.Equal(t, "ana@example.com", m.To)
	assert.Equal(t, "Selesaikan pendaftaran di Acme", m.Subject, "an unknown email gets the sign-up variant")
	assert.Contains(t, m.TextBody, "Ada pertanyaan? Balas email ini.")
	assert.Contains(t, m.HTMLBody, linkIn(t, m.TextBody))
	assert.NoError(t, m.Validate())

	f.acme.SupportEmail, f.acme.DisplayName = "", `Ev"il <x@y>`
	login = f.get(f.client, "/p/acme/login?add=1")
	f.submit(f.client, login, "/login", url.Values{"email": {"budi@example.com"}})
	m = f.users.mails[1]
	assert.Empty(t, m.ReplyTo)
	assert.NotContains(t, m.TextBody, "Reply to this email")
	assert.NoError(t, m.Validate(), "a hostile project name stays one header")
	assert.Contains(t, m.From, "<no-reply@othra.example>")
}

// J8, S11, RL1, RL2: the per-email limits and the cooldown are silent; the per-IP limits answer 429.
func TestWeb_RateLimits(t *testing.T) {
	clock := ratelimittest.NewClock()
	f := newWebFixture(t)
	f.users.now = clock.Now
	f.web.WithLimits(layout.Limits{Limiter: memory.New(1000, memory.WithClock(clock.Now))})
	send := func(handle, email string) hostedPage {
		login := f.get(f.client, "/p/"+handle+"/login?add=1")
		return f.submit(f.client, login, "/login", url.Values{"email": {email}})
	}
	for range 4 {
		assert.Equal(t, http.StatusSeeOther, send("acme", "ana@example.com").status, "the same answer, sent or not")
		clock.Advance(61 * time.Second)
	}
	assert.Len(t, f.users.issued, 3, "3 in 15 minutes: the 4th is dropped silently")

	assert.Equal(t, http.StatusSeeOther, send("acme", "budi@example.com").status)
	assert.Equal(t, http.StatusSeeOther, send("acme", "budi@example.com").status, "within the cooldown: the same answer")
	assert.Len(t, f.users.issued, 4)

	for range 8 {
		send("acme", "x"+uuid.NewString()+"@example.com")
	}
	limited := send("acme", "y@example.com")
	assert.Equal(t, http.StatusTooManyRequests, limited.status, "11th send in a minute from one IP")
	assert.NotEmpty(t, limited.header.Get("Retry-After"))
	assert.Contains(t, limited.body, "Too many attempts")
	assert.Contains(t, limited.body, "Try again in "+limited.header.Get("Retry-After")+" seconds.", "B3: the title is not repeated in the body")
	assert.NotContains(t, limited.body, "Too many attempts. Try again", "the shared AUT003 sentence is not used")
	assert.Contains(t, limited.body, "AUT003")
	assert.Contains(t, limited.body, "Back to sign in", "the sign-in flow's own 429 keeps its way back")
	assert.Equal(t, http.StatusSeeOther, send("beta", "y@example.com").status, "limits are per project")
}

// qa: a request the cooldown blocks spends no per-email budget, so a send after the cooldown still mails.
func TestWeb_CooldownSpendsNoEmailBudget(t *testing.T) {
	clock := ratelimittest.NewClock()
	f := newWebFixture(t)
	f.users.now = clock.Now
	f.web.WithLimits(layout.Limits{Limiter: memory.New(1000, memory.WithClock(clock.Now))})
	login := f.get(f.client, "/p/acme/login")
	sent := f.submit(f.client, login, "/login", url.Values{"email": {"ana@example.com"}})
	page := f.get(f.client, sent.location())
	for range 2 {
		clock.Advance(10 * time.Second)
		f.submit(f.client, page, "/login/resend", nil)
	}
	require.Len(t, f.users.issued, 1, "both resends fall in the cooldown")
	clock.Advance(61 * time.Second)
	login = f.get(f.client, "/p/acme/login?add=1")
	f.submit(f.client, login, "/login", url.Values{"email": {"ana@example.com"}})
	assert.Len(t, f.users.issued, 2, "the cooled-down resends spent nothing: the send after 61 s mails")
}

// RL2: the per-email day limit holds after the 15-minute window refills.
func TestWeb_EmailDayLimit(t *testing.T) {
	clock := ratelimittest.NewClock()
	f := newWebFixture(t)
	f.users.now = clock.Now
	f.web.WithLimits(layout.Limits{Limiter: memory.New(1000, memory.WithClock(clock.Now))})
	issued := func() int { return len(f.users.issued) }
	for range 12 {
		login := f.get(f.client, "/p/acme/login?add=1")
		f.submit(f.client, login, "/login", url.Values{"email": {"ana@example.com"}})
		clock.Advance(5 * time.Minute)
	}
	assert.Equal(t, 10, issued(), "10 a day, though the 15-minute rule allows one every 5 minutes")
	clock.Advance(24 * time.Hour)
	login := f.get(f.client, "/p/acme/login?add=1")
	f.submit(f.client, login, "/login", url.Values{"email": {"ana@example.com"}})
	assert.Equal(t, 11, issued(), "the next day works again")
}

func TestWeb_BrokenLimiterFailsClosed(t *testing.T) {
	f := newWebFixture(t)
	f.web.WithLimits(layout.Limits{Limiter: brokenLimiter{}})
	login := f.get(f.client, "/p/acme/login")
	page := f.submit(f.client, login, "/login", url.Values{"email": {"ana@example.com"}})
	assert.Equal(t, http.StatusServiceUnavailable, page.status)
	assert.Empty(t, f.users.issued)
}

type brokenLimiter struct{}

func (brokenLimiter) Allow(context.Context, string, ratelimit.Rule) (ratelimit.Result, error) {
	return ratelimit.Result{}, errFake
}

func (brokenLimiter) Reset(context.Context, string) error { return nil }

// C1, C3: every hosted POST needs its CSRF token, and a cross-site POST is refused even with one.
func TestWeb_EveryPOSTNeedsItsToken(t *testing.T) {
	f := newWebFixture(t)
	for _, path := range []string{"/login", "/login/resend", "/login/verify", "/login/select", "/login/signout", "/terms"} {
		p := f.do(f.client, http.MethodPost, "/p/acme"+path, url.Values{"email": {"ana@example.com"}})
		assert.Equal(t, http.StatusForbidden, p.status, path)
		assert.Contains(t, p.body, "SIG004", path)
	}
	login := f.get(f.client, "/p/acme/login")
	token := csrfRE.FindStringSubmatch(login.body)[1]
	cross := f.do(f.client, http.MethodPost, "/p/acme/login", url.Values{"email": {"a@example.com"}, layout.CSRFField: {token}},
		"Sec-Fetch-Site", "cross-site")
	assert.Equal(t, http.StatusForbidden, cross.status)
	beta := f.do(f.client, http.MethodPost, "/p/beta/login", url.Values{"email": {"a@example.com"}, layout.CSRFField: {token}})
	assert.Equal(t, http.StatusForbidden, beta.status, "acme's token is not beta's")
	assert.Empty(t, f.users.issued)
}

func TestWeb_SentAndResend(t *testing.T) {
	f := newWebFixture(t)
	direct := f.get(f.client, "/p/acme/login/sent")
	assert.Equal(t, http.StatusSeeOther, direct.status, "no flash cookie: back to the form")
	login := f.get(f.client, "/p/acme/login")
	sent := f.submit(f.client, login, "/login", url.Values{"email": {"ana@example.com"}})
	page := f.get(f.client, sent.location())
	again := f.submit(f.client, page, "/login/resend", url.Values{"email": {"evil@example.com"}})
	require.Equal(t, http.StatusSeeOther, again.status, again.body)
	assert.Contains(t, again.location(), "resent=1")
	require.Len(t, f.users.requests, 2)
	assert.Equal(t, "ana@example.com", f.users.requests[1].Email, "the email comes from the signed cookie, never the form")
	assert.Contains(t, f.get(f.client, again.location()).body, "Done. If it isn")
	noFlash := f.newClient()
	resend := f.do(noFlash, http.MethodPost, "/p/acme/login/resend", nil)
	assert.Equal(t, http.StatusForbidden, resend.status)
}

func TestWeb_UnknownAndCatchAll(t *testing.T) {
	f := newWebFixture(t)
	unknown := f.get(f.client, "/p/nope/login")
	assert.Equal(t, http.StatusNotFound, unknown.status)
	assert.Contains(t, unknown.body, "We couldn&#39;t find this sign-in page")
	assert.NotContains(t, unknown.body, "Acme")
	assert.NotContains(t, unknown.body, "/console")

	missing := f.get(f.client, "/p/acme/nothing-here")
	assert.Equal(t, http.StatusNotFound, missing.status)
	assert.Contains(t, missing.body, "Acme", "a known project's 404 is branded")
	assert.Contains(t, missing.body, "help@acme.example")
	assert.NotContains(t, missing.body, "/console")
}

func TestWeb_ResolverFailures(t *testing.T) {
	f := newWebFixture(t)
	f.issuers.err = &oidc.LookupLimitedError{RetryAfter: 7}
	limited := f.get(f.client, "/p/acme/login")
	assert.Equal(t, http.StatusTooManyRequests, limited.status)
	assert.Equal(t, "7", limited.header.Get("Retry-After"))
	f.issuers.err = errFake
	broken := f.get(f.client, "/p/acme/login")
	assert.Equal(t, http.StatusInternalServerError, broken.status)
	assert.Contains(t, broken.body, "GEN900")
	assert.NotContains(t, broken.body, errFake.Error())
	assert.Contains(t, f.reports, "hosted: resolve issuer")
}

func TestWeb_ServiceFailures(t *testing.T) {
	f := newWebFixture(t)
	f.users.failOn = "browser"
	assert.Equal(t, http.StatusInternalServerError, f.get(f.client, "/p/acme/login").status)
	f.users.failOn = ""
	login := f.get(f.client, "/p/acme/login")
	f.users.failOn = "issue"
	assert.Equal(t, http.StatusInternalServerError, f.submit(f.client, login, "/login", url.Values{"email": {"a@example.com"}}).status)
	f.users.failOn = ""
	f.signIn(f.client, "acme", "ana@example.com")
	f.users.failOn = "terms"
	assert.Equal(t, http.StatusInternalServerError, f.get(f.client, "/p/acme/account/").status)
	assert.Equal(t, http.StatusInternalServerError, f.get(f.client, "/p/acme/terms").status)
	f.users.failOn = ""
	account := f.get(f.client, "/p/acme/account/")
	f.users.failOn = "signout"
	assert.Equal(t, http.StatusInternalServerError, f.submit(f.client, account, "/login/signout", nil).status)
	assert.NotEmpty(t, f.reports)
}

func TestUserAgent(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("User-Agent", strings.Repeat("é", 400))
	ua := userAgent(r)
	assert.LessOrEqual(t, len(ua), maxUserAgentLen)
	assert.True(t, strings.HasPrefix(strings.Repeat("é", 400), ua))
}

func TestWeb_MoreFailures(t *testing.T) {
	f := newWebFixture(t)
	login := f.get(f.client, "/p/acme/login")
	sent := f.submit(f.client, login, "/login", url.Values{"email": {"ana@example.com"}})
	page := f.get(f.client, sent.location())
	f.users.failOn = "issue"
	assert.Equal(t, http.StatusInternalServerError, f.submit(f.client, page, "/login/resend", nil).status)
	f.users.failOn = ""

	verifyURL := linkIn(t, f.users.mails[0].TextBody)
	moved := f.get(f.client, strings.TrimPrefix(verifyURL, f.srv.URL))
	verify := f.get(f.client, moved.location())
	f.users.failOn = "redeem"
	assert.Equal(t, http.StatusInternalServerError, f.submit(f.client, verify, "/login/verify", nil).status)
	f.users.failOn = ""

	f.signIn(f.client, "acme", "ana@example.com")
	picker := f.get(f.client, "/p/acme/login")
	f.users.failOn = "terms"
	assert.Equal(t, http.StatusInternalServerError, f.submit(f.client, picker, "/login/select", nil).status)
	f.users.failOn = "browser"
	assert.Equal(t, http.StatusInternalServerError, f.submit(f.client, picker, "/login/signout", nil).status)
	f.users.failOn = ""
	f.acme.TermsVersion = 1
	terms := f.get(f.client, "/p/acme/terms")
	f.users.failOn = "browser"
	assert.Equal(t, http.StatusInternalServerError, f.submit(f.client, terms, "/terms", nil).status)
	f.users.failOn = "terms"
	assert.Equal(t, http.StatusInternalServerError, f.submit(f.client, terms, "/terms", nil).status)
	f.users.failOn = ""
	assert.Equal(t, http.StatusSeeOther, f.get(f.newClient(), "/p/acme/terms").status, "no account: back to sign in")

	assert.Empty(t, codeOf(errFake))
	assert.Equal(t, "/p/acme/account/", ReturnTo(f.srv.URL+"/p/acme", ""))
}

func TestWeb_TermsAlreadyAccepted(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	done := f.get(f.client, "/p/acme/terms?return_to=%2Fp%2Facme%2Faccount%2F")
	assert.Equal(t, http.StatusSeeOther, done.status, "nothing to accept: straight on")
	assert.Equal(t, "/p/acme/account/", done.location())
}

// H8: the return_to redirect helper runs ReturnTo itself, so an off-issuer value falls back.
func TestWeb_ToReturnToFallsBack(t *testing.T) {
	f := newWebFixture(t)
	for _, raw := range []string{"https://evil.example/", "//evil.example", "/p/beta/account/", ""} {
		rec := httptest.NewRecorder()
		f.web.toReturnTo(rec, httptest.NewRequest(http.MethodGet, "/", nil), f.web.site(f.acme), raw)
		assert.Equal(t, "/p/acme/account/", rec.Header().Get("Location"), raw)
	}
	rec := httptest.NewRecorder()
	f.web.toPage(rec, httptest.NewRequest(http.MethodGet, "/", nil), f.web.site(f.acme), pageLogin, backTo("//evil"))
	assert.Equal(t, "/p/acme/login?return_to=%2F%2Fevil", rec.Header().Get("Location"), "a page redirect only encodes values")
}

// RL1: the per-IP hour cap trips even when every send keeps under the minute cap.
func TestWeb_IPHourLimit(t *testing.T) {
	clock := ratelimittest.NewClock()
	f := newWebFixture(t)
	f.web.WithLimits(layout.Limits{Limiter: memory.New(1000, memory.WithClock(clock.Now))})
	refused := 0
	for i := range 60 {
		login := f.get(f.client, "/p/acme/login?add=1")
		page := f.submit(f.client, login, "/login", url.Values{"email": {"u" + strconv.Itoa(i) + "@example.com"}})
		if page.status == http.StatusTooManyRequests {
			refused++
		}
		clock.Advance(6 * time.Second)
	}
	assert.Positive(t, refused, "the hour cap refused some of 60 sends in 6 minutes")
	assert.LessOrEqual(t, len(f.users.issued), 50+int(6*time.Minute/(time.Hour/50)), "about 50 an hour")
	assert.Equal(t, 60, len(f.users.issued)+refused)
}

// N5, RL3: 30 redeems a minute per IP and project, counted on the POST only; the verify GET never counts.
func TestWeb_RedeemLimit(t *testing.T) {
	clock := ratelimittest.NewClock()
	f := newWebFixture(t)
	f.web.WithLimits(layout.Limits{Limiter: memory.New(1000, memory.WithClock(clock.Now))})
	verify := f.get(f.client, "/p/acme/login/verify?t=nope")
	for range 40 {
		assert.NotEqual(t, http.StatusTooManyRequests, f.get(f.client, verify.location()).status, "a GET never counts")
	}
	statuses := map[int]int{}
	for range 31 {
		login := f.get(f.client, "/p/acme/login?add=1")
		form := url.Values{layout.CSRFField: {csrfTokenFor(t, f, login, "verify")}}
		statuses[f.do(f.client, http.MethodPost, "/p/acme/login/verify", form).status]++
	}
	assert.Equal(t, 1, statuses[http.StatusTooManyRequests], "the 31st redeem in a minute")
	clock.Advance(time.Minute)
	form := url.Values{layout.CSRFField: {csrfTokenFor(t, f, f.get(f.client, "/p/acme/login?add=1"), "verify")}}
	assert.NotEqual(t, http.StatusTooManyRequests, f.do(f.client, http.MethodPost, "/p/acme/login/verify", form).status, "a minute later it works")
}

func csrfTokenFor(t *testing.T, f *webFixture, p hostedPage, form string) string {
	t.Helper()
	u, err := url.Parse(f.srv.URL + "/p/acme/")
	require.NoError(t, err)
	for _, c := range f.client.Jar.Cookies(u) {
		if c.Name == CSRFCookie {
			return f.web.csrf.Token(c.Value, f.acme.PublicID, form)
		}
	}
	t.Fatalf("no CSRF cookie after %d", p.status)
	return ""
}

// qa F3: the sign-out notice is the email, translated once by the page that shows it.
func TestWeb_SignOutNoticeIndonesian(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	f.signIn(f.client, "acme", "cici@example.com")
	out := f.submit(f.client, f.get(f.client, "/p/acme/login"), "/login/signout", nil)
	id := f.get(f.client, out.location(), "Accept-Language", "id-ID")
	assert.Equal(t, 1, strings.Count(plain(id.body), "sudah keluar di browser ini."), "translated once")
	assert.Contains(t, plain(id.body), "cici@example.com sudah keluar di browser ini.")
}

// B1: an invalid email in add-account mode keeps the way back to the accounts.
func TestWeb_AddModeKeepsBackOnError(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	add := f.get(f.client, "/p/acme/login?add=1")
	require.Contains(t, add.body, "Back to accounts")
	bad := f.submit(f.client, add, "/login", url.Values{"email": {"nope"}})
	assert.Equal(t, http.StatusUnprocessableEntity, bad.status)
	assert.Contains(t, bad.body, "Enter a valid email address.")
	assert.Contains(t, bad.body, "Back to accounts", "the error keeps add mode")

	fresh := f.newClient()
	login := f.get(fresh, "/p/acme/login")
	plain := f.submit(fresh, login, "/login", url.Values{"email": {"nope"}})
	assert.NotContains(t, plain.body, "Back to accounts", "no accounts, no way back")
}

// B5: signing out the last tile lands on the email form with the notice, translated once.
func TestWeb_LastTileSignOutShowsTheNotice(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	out := f.submit(f.client, f.get(f.client, "/p/acme/login"), "/login/signout", nil)
	require.Equal(t, http.StatusSeeOther, out.status)
	form := f.get(f.client, out.location())
	assert.Contains(t, form.body, "Continue with email", "no tile left: the email form")
	assert.Equal(t, 1, strings.Count(plain(form.body), "ana@example.com is signed out on this browser."))
	assert.NotContains(t, f.get(f.client, out.location()).body, "is signed out", "once")
}

func TestWeb_CodeErrorNamesTheProject(t *testing.T) {
	f := newWebFixture(t)
	s := f.web.site(f.acme)
	rec := httptest.NewRecorder()
	f.web.codeError(rec, httptest.NewRequest(http.MethodGet, "/", nil), s, http.StatusForbidden, apperror.CodeSignInUpstreamDisabled)
	body := rec.Body.String()
	assert.Contains(t, body, s.views().Project)
	assert.NotContains(t, body, "{project}")
}
