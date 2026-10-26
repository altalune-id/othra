package hosted

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/web/layout"
)

const (
	testPath  = "/p/acme/"
	testScope = "prjprjprjprjprjprjprj"
)

type csrfFixture struct {
	csrf   *CSRF
	now    time.Time
	cookie string
}

func newCSRFFixture(t *testing.T) *csrfFixture {
	t.Helper()
	f := &csrfFixture{now: time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)}
	f.csrf = NewCSRF([]byte("0123456789abcdef0123456789abcdef"), time.Hour)
	f.csrf.now = func() time.Time { return f.now }
	rec := httptest.NewRecorder()
	f.cookie = f.csrf.Cookie(rec, httptest.NewRequest(http.MethodGet, "/p/acme/login", nil), testPath)
	return f
}

func post(cookie, token string, header ...string) *http.Request {
	form := url.Values{}
	if token != "" {
		form.Set(layout.CSRFField, token)
	}
	r := httptest.NewRequest(http.MethodPost, "https://id.example.com/p/acme/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: CSRFCookie, Value: cookie})
	}
	for i := 0; i+1 < len(header); i += 2 {
		r.Header.Set(header[i], header[i+1])
	}
	return r
}

func reason(t *testing.T, err error) FormReason {
	t.Helper()
	e, ok := errors.AsType[*FormExpiredError](err)
	require.True(t, ok, "%v", err)
	return e.Reason
}

func TestCSRF_Cookie(t *testing.T) {
	f := newCSRFFixture(t)
	rec := httptest.NewRecorder()
	got := f.csrf.Cookie(rec, httptest.NewRequest(http.MethodGet, "/p/acme/login", nil), testPath)
	set := rec.Result().Cookies()
	require.Len(t, set, 1)
	c := set[0]
	assert.Equal(t, CSRFCookie, c.Name)
	assert.Equal(t, got, c.Value)
	assert.Equal(t, testPath, c.Path)
	assert.True(t, c.Secure)
	assert.True(t, c.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
	assert.Zero(t, c.MaxAge, "a browser-session cookie")
	assert.GreaterOrEqual(t, len(got), 26, "128 random bits")

	req := httptest.NewRequest(http.MethodGet, "/p/acme/login", nil)
	req.AddCookie(&http.Cookie{Name: CSRFCookie, Value: f.cookie})
	rec = httptest.NewRecorder()
	assert.Equal(t, f.cookie, f.csrf.Cookie(rec, req, testPath), "an existing cookie is kept")
	assert.Empty(t, rec.Result().Cookies())
}

func TestCSRF_AValidTokenPasses(t *testing.T) {
	f := newCSRFFixture(t)
	token := f.csrf.Token(f.cookie, testScope, "login")
	f.now = f.now.Add(59 * time.Minute)
	assert.NoError(t, f.csrf.Check(post(f.cookie, token), testScope, "login"))
}

// D3, D3a: no token, a wrong token, an expired one, another form's, another project's or another browser's are all refused.
func TestCSRF_Refusals(t *testing.T) {
	f := newCSRFFixture(t)
	token := f.csrf.Token(f.cookie, testScope, "login")
	other := NewCSRF([]byte("another key, another deployment."), time.Hour)
	for name, tc := range map[string]struct {
		req   *http.Request
		scope string
		form  string
		want  FormReason
	}{
		"no token":             {post(f.cookie, ""), testScope, "login", FormNoToken},
		"no cookie":            {post("", token), testScope, "login", FormNoCookie},
		"garbage":              {post(f.cookie, "not-a-token"), testScope, "login", FormBadToken},
		"truncated":            {post(f.cookie, token[:20]), testScope, "login", FormBadToken},
		"another form":         {post(f.cookie, token), testScope, "terms", FormBadToken},
		"another project":      {post(f.cookie, token), "otherotherotherother1", "login", FormBadToken},
		"another browser":      {post("someone-elses-cookie", token), testScope, "login", FormBadToken},
		"another key":          {post(f.cookie, other.Token(f.cookie, testScope, "login")), testScope, "login", FormBadToken},
		"a tampered timestamp": {post(f.cookie, retime(t, token, f.now.Add(time.Minute))), testScope, "login", FormBadToken},
	} {
		assert.Equal(t, tc.want, reason(t, f.csrf.Check(tc.req, tc.scope, tc.form)), name)
	}
}

func retime(t *testing.T, token string, at time.Time) string {
	t.Helper()
	b, err := decodeToken(token)
	require.NoError(t, err)
	fresh, err := decodeToken((&CSRF{key: []byte("x")}).token("c", "s", "f", at.Unix()))
	require.NoError(t, err)
	copy(b[:8], fresh[:8])
	return encodeToken(b)
}

func TestCSRF_Expiry(t *testing.T) {
	f := newCSRFFixture(t)
	token := f.csrf.Token(f.cookie, testScope, "login")
	f.now = f.now.Add(time.Hour + time.Second)
	assert.Equal(t, FormExpired, reason(t, f.csrf.Check(post(f.cookie, token), testScope, "login")))

	f.now = time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	future := f.csrf.token(f.cookie, testScope, "login", f.now.Add(2*time.Minute).Unix())
	assert.Equal(t, FormExpired, reason(t, f.csrf.Check(post(f.cookie, future), testScope, "login")), "a token from the future")
}

// D3a: sign-in renews the cookie, so a token issued before it stops verifying.
func TestCSRF_RenewKillsOldTokens(t *testing.T) {
	f := newCSRFFixture(t)
	token := f.csrf.Token(f.cookie, testScope, "login")
	renewed := f.csrf.Renew(httptest.NewRecorder(), testPath)
	assert.NotEqual(t, f.cookie, renewed)
	assert.Equal(t, FormBadToken, reason(t, f.csrf.Check(post(renewed, token), testScope, "login")))
	assert.NoError(t, f.csrf.Check(post(renewed, f.csrf.Token(renewed, testScope, "login")), testScope, "login"))
}

// D3, C3: http.CrossOriginProtection lets an old browser with neither Sec-Fetch-Site nor Origin through; the token still refuses it.
func TestCSRF_BehindCrossOriginProtection(t *testing.T) {
	f := newCSRFFixture(t)
	h := http.NewCrossOriginProtection().Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := f.csrf.Check(r, testScope, "login"); err != nil {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	token := f.csrf.Token(f.cookie, testScope, "login")
	for name, tc := range map[string]struct {
		req  *http.Request
		want int
	}{
		"old browser, no token":    {post(f.cookie, ""), http.StatusForbidden},
		"old browser, good token":  {post(f.cookie, token), http.StatusOK},
		"cross-site, good token":   {post(f.cookie, token, "Sec-Fetch-Site", "cross-site"), http.StatusForbidden},
		"same-origin, no token":    {post(f.cookie, "", "Sec-Fetch-Site", "same-origin"), http.StatusForbidden},
		"same-origin, good token":  {post(f.cookie, token, "Sec-Fetch-Site", "same-origin"), http.StatusOK},
		"foreign origin, no fetch": {post(f.cookie, token, "Origin", "https://evil.example"), http.StatusForbidden},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, tc.req)
		assert.Equal(t, tc.want, rec.Code, name)
	}
}

func TestFormExpiredError(t *testing.T) {
	err := &FormExpiredError{Reason: FormExpired}
	assert.True(t, IsFormExpiredError(err))
	assert.False(t, IsFormExpiredError(assert.AnError))
	assert.Contains(t, err.Error(), "expired")
	app := err.ToAppError()
	assert.Equal(t, apperror.CodeSignInFormExpired, app.Code())
	assert.Empty(t, app.Meta(), "the reason never reaches the page")
}

func TestCSRF_OneCookiePerResponse(t *testing.T) {
	f := newCSRFFixture(t)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/p/acme/login", nil)
	first, second := f.csrf.Cookie(rec, r, testPath), f.csrf.Cookie(rec, r, testPath)
	assert.Equal(t, first, second, "two forms on one fresh page share the cookie their tokens are bound to")
	assert.Len(t, rec.Result().Cookies(), 1)
}
