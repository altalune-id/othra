//go:build integration

package boot

import (
	"database/sql"
	"encoding/json"
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

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/config"
	"github.com/altalune-id/othra/internal/kernel"
	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/internal/kernel/tenant"
	"github.com/altalune-id/othra/internal/project"
	"github.com/altalune-id/othra/internal/testutil/pgtest"
	"github.com/altalune-id/othra/internal/web/layout"
)

type signInFixture struct {
	t    *testing.T
	srv  *httptest.Server
	svcs *Services
	c    *http.Client
	db   *sql.DB
	pool db.Pool
	cfg  config.Config
}

func newSignInFixture(t *testing.T, opts ...func(*config.Config)) *signInFixture {
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
	for _, opt := range opts {
		opt(&cfg)
	}
	pool, sqlDB, schemaName := pgtest.MigratedRLS(t)
	cfg.Postgres.Schema = schemaName
	svcs := buildServices(kernel.Kernel{Pool: pool, Reporter: apperror.NewReporter(nil, false)}, cfg)
	require.NoError(t, buildAuthSecrets(cfg, svcs, testLogger()))
	svcs.Caps.EnvGenesis = true
	require.NoError(t, firstBoot(t.Context(), cfg, svcs, testLogger(), false))
	required, err := installRequired(t.Context(), svcs)
	require.NoError(t, err)
	inProcess := http.NewServeMux()
	rp := buildConsoleRP(cfg, svcs, inProcess)
	public, hosted := buildIssuerRoutes(cfg, svcs, inProcess, nil, nil)
	srv.Config.Handler = buildHTTPHandler(httpSurfaces{
		routes: buildRoutes(cfg, svcs, rp, nil, nil), public: public, hosted: hosted,
		session: buildSessionMiddleware(svcs, nil), installRequired: required,
	})
	f := &signInFixture{t: t, srv: srv, svcs: svcs, db: sqlDB, pool: pool, cfg: cfg}
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

type portalProfile struct {
	Profile struct {
		Email string `json:"email"`
	} `json:"profile"`
	SignOutToken     string `json:"signOutToken"`
	CurrentSessionID string `json:"currentSessionId"`
}

// NOTE: the portal's own calls: who account.v1 says is signed in on c, and its hosted sign-out with the token GetProfile hands out.
func (f *signInFixture) profile(c *http.Client, base string) (int, portalProfile) {
	f.t.Helper()
	req, err := http.NewRequestWithContext(f.t.Context(), http.MethodPost, f.srv.URL+base+"/api/account.v1.AccountService/GetProfile", strings.NewReader("{}"))
	require.NoError(f.t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	require.NoError(f.t, err)
	defer func() { _ = resp.Body.Close() }()
	var out portalProfile
	if resp.StatusCode == http.StatusOK {
		require.NoError(f.t, json.NewDecoder(resp.Body).Decode(&out))
	}
	return resp.StatusCode, out
}

func (f *signInFixture) portalSignOut(c *http.Client, base string) signInPage {
	f.t.Helper()
	status, me := f.profile(c, base)
	require.Equal(f.t, http.StatusOK, status)
	return f.do(c, http.MethodPost, base+"/login/signout", url.Values{
		layout.CSRFField: {me.SignOutToken}, "user_session": {me.CurrentSessionID}, "next": {"signed_out"},
	})
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

// a magic link mailed through the queue signs the end user in on the device that opens it, once.
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
	assert.Equal(t, http.StatusSeeOther, twice.status, "a second Continue from the same browser still succeeds")

	account := f.do(phone, http.MethodGet, base+"/account/", nil)
	assert.NotEqual(t, http.StatusSeeOther, account.status, "the portal is served to the signed-in phone, not sent to sign in")
	status, me := f.profile(phone, base)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "ana@example.com", me.Profile.Email, "account.v1 acts as the account the link signed in")
	assert.Equal(t, http.StatusSeeOther, f.do(f.c, http.MethodGet, base+"/account/", nil).status, "the asking laptop is not signed in")
	laptop, _ := f.profile(f.c, base)
	assert.Equal(t, http.StatusUnauthorized, laptop, "and account.v1 agrees")

	other := f.client()
	again := f.do(other, http.MethodGet, strings.TrimPrefix(link, f.srv.URL), nil)
	used := f.do(other, http.MethodGet, again.header.Get("Location"), nil)
	assert.Equal(t, http.StatusBadRequest, used.status, "a used link can't be used by another browser")

	blog := f.project("blog", "Blog")
	assert.Equal(t, http.StatusSeeOther, f.do(phone, http.MethodGet, "/p/"+blog.IssuerHandle+"/account/", nil).status,
		"another project's pages see no session")

	out := f.portalSignOut(phone, base)
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
