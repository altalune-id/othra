//go:build integration

package boot

import (
	"html"
	"io"
	"maps"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/config"
	"altalune.id/othra/internal/kernel"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/testutil/pgtest"
)

type signInFixture struct {
	t    *testing.T
	srv  *httptest.Server
	svcs *Services
	c    *http.Client
}

func newSignInFixture(t *testing.T) *signInFixture {
	t.Helper()
	srv := httptest.NewUnstartedServer(nil)
	srv.StartTLS()
	t.Cleanup(srv.Close)
	cfg := devLocalConfig()
	cfg.HTTP.BaseURL = srv.URL
	cfg.HTTP.StateSecret = strings.Repeat("s", 40)
	cfg.Secrets.EncryptionKey = testKeyHex
	cfg.Mail.From = "Othra <no-reply@othra.example>"
	cfg.EndUsers = config.EndUsersConfig{
		MagicLink: config.EndUsersMagicLinkConfig{TTL: 15 * time.Minute, ResendAfter: time.Minute},
		Session:   config.EndUsersSessionConfig{Idle: 30 * 24 * time.Hour, Absolute: 90 * 24 * time.Hour, MaxAccounts: 20},
		CSRF:      config.EndUsersCSRFConfig{MaxAge: time.Hour},
	}
	cfg.Auth.Genesis = config.GenesisConfig{Email: "root@example.com", Password: "genesis-password"}
	pool, _, schemaName := pgtest.MigratedRLS(t)
	cfg.Postgres.Schema = schemaName
	svcs := buildServices(kernel.Kernel{Pool: pool, Reporter: apperror.NewReporter(nil, false)}, cfg)
	require.NoError(t, buildAuthSecrets(cfg, svcs, testLogger()))
	svcs.Caps.EnvGenesis = true
	require.NoError(t, firstBoot(t.Context(), cfg, svcs, testLogger(), false))
	required, err := installRequired(t.Context(), svcs)
	require.NoError(t, err)
	public, hosted := buildIssuerRoutes(cfg, svcs, nil, nil)
	srv.Config.Handler = buildHTTPHandler(httpSurfaces{
		routes: buildRoutes(cfg, svcs, nil, nil), public: public, hosted: hosted,
		session: buildSessionMiddleware(svcs, nil), installRequired: required,
	})
	f := &signInFixture{t: t, srv: srv, svcs: svcs}
	f.c = f.client()
	return f
}

func (f *signInFixture) client() *http.Client {
	jar, err := cookiejar.New(nil)
	require.NoError(f.t, err)
	c := *f.srv.Client()
	c.Jar = jar
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &c
}

func (f *signInFixture) project(slug, name string) *project.Project {
	f.t.Helper()
	sys, err := f.svcs.Orgs.System(f.t.Context())
	require.NoError(f.t, err)
	ctx := tenant.Into(f.t.Context(), tenant.Context{OrgID: sys.ID})
	p, err := f.svcs.Projects.Create(ctx, project.CreateRequest{OrgID: sys.ID, Slug: slug, Name: name})
	require.NoError(f.t, err)
	return p
}

type signInPage struct {
	status int
	body   string
	header http.Header
}

func (f *signInFixture) do(c *http.Client, method, path string, form url.Values) signInPage {
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
	resp, err := c.Do(req)
	require.NoError(f.t, err)
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	require.NoError(f.t, err)
	return signInPage{status: resp.StatusCode, body: string(b), header: resp.Header}
}

var (
	signInFormRE   = regexp.MustCompile(`(?s)<form[^>]*action="([^"]+)"(.*?)</form>`)
	signInHiddenRE = regexp.MustCompile(`<input type="hidden" name="([a-z_]+)" value="([^"]*)"`)
	signInLinkRE   = regexp.MustCompile(`https://\S+/login/verify\?t=\S+`)
)

func (f *signInFixture) submit(c *http.Client, p signInPage, action string, extra url.Values) signInPage {
	f.t.Helper()
	for _, m := range signInFormRE.FindAllStringSubmatch(p.body, -1) {
		target := html.UnescapeString(m[1])
		if !strings.HasSuffix(target, action) {
			continue
		}
		form := url.Values{}
		for _, in := range signInHiddenRE.FindAllStringSubmatch(m[2], -1) {
			form.Set(in[1], html.UnescapeString(in[2]))
		}
		maps.Copy(form, extra)
		return f.do(c, http.MethodPost, target, form)
	}
	f.t.Fatalf("no form posting to %s:\n%s", action, p.body)
	return signInPage{}
}

// J1, D1, D4: a magic link mailed through the queue signs the end user in on the device that opens it, once.
func TestHosted_MagicLinkEndToEnd(t *testing.T) {
	f := newSignInFixture(t)
	p := f.project("shop", "Shop")
	base := "/p/" + p.IssuerHandle

	login := f.do(f.c, http.MethodGet, base+"/login", nil)
	require.Equal(t, http.StatusOK, login.status, login.body)
	sent := f.submit(f.c, login, "/login", url.Values{"email": {"Ana@Example.com"}})
	require.Equal(t, http.StatusSeeOther, sent.status, sent.body)

	mails := claimMail(t, f.svcs)
	require.Len(t, mails, 1)
	m := mails[0]
	assert.Equal(t, "ana@example.com", m.To)
	assert.Equal(t, `"Shop" <no-reply@othra.example>`, m.From)
	assert.Equal(t, "Finish signing up to Shop", m.Subject)
	link := signInLinkRE.FindString(m.TextBody)
	require.NotEmpty(t, link, m.TextBody)
	assert.True(t, strings.HasPrefix(link, f.srv.URL+base+"/login/verify?t="), link)

	phone := f.client()
	moved := f.do(phone, http.MethodGet, strings.TrimPrefix(link, f.srv.URL), nil)
	require.Equal(t, http.StatusSeeOther, moved.status, moved.body)
	verify := f.do(phone, http.MethodGet, moved.header.Get("Location"), nil)
	require.Equal(t, http.StatusOK, verify.status, verify.body)
	assert.Contains(t, verify.body, "ana@example.com")
	done := f.submit(phone, verify, "/login/verify", nil)
	require.Equal(t, http.StatusSeeOther, done.status, done.body)
	assert.Equal(t, base+"/account/", done.header.Get("Location"))
	twice := f.submit(phone, verify, "/login/verify", nil)
	assert.Equal(t, http.StatusSeeOther, twice.status, "H23: a second Continue from the same browser still succeeds")

	account := f.do(phone, http.MethodGet, base+"/account/", nil)
	require.Equal(t, http.StatusOK, account.status, account.body)
	assert.Contains(t, account.body, "ana@example.com")
	assert.Equal(t, http.StatusSeeOther, f.do(f.c, http.MethodGet, base+"/account/", nil).status, "D1: the asking laptop is not signed in")

	other := f.client()
	again := f.do(other, http.MethodGet, strings.TrimPrefix(link, f.srv.URL), nil)
	used := f.do(other, http.MethodGet, again.header.Get("Location"), nil)
	assert.Equal(t, http.StatusBadRequest, used.status, "a used link can't be used by another browser")

	blog := f.project("blog", "Blog")
	assert.Equal(t, http.StatusSeeOther, f.do(phone, http.MethodGet, "/p/"+blog.IssuerHandle+"/account/", nil).status,
		"S6: another project's pages see no session")

	out := f.submit(phone, account, "/login/signout", url.Values{"next": {"signed_out"}})
	assert.Equal(t, base+"/signed-out", out.header.Get("Location"))
	assert.Equal(t, http.StatusSeeOther, f.do(phone, http.MethodGet, base+"/account/", nil).status)
}

func TestHosted_UnknownHandleAndBrandedNotFound(t *testing.T) {
	f := newSignInFixture(t)
	p := f.project("shop", "Shop")
	unknown := f.do(f.c, http.MethodGet, "/p/no-such-handle/login", nil)
	assert.Equal(t, http.StatusNotFound, unknown.status)
	assert.NotContains(t, unknown.body, "/console")
	missing := f.do(f.c, http.MethodGet, "/p/"+p.IssuerHandle+"/nope", nil)
	assert.Equal(t, http.StatusNotFound, missing.status)
	assert.Contains(t, missing.body, "Shop")
	assert.Equal(t, "no-store", missing.header.Get("Cache-Control"))
}
