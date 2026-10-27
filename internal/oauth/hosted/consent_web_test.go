package hosted

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/internal/consent"
	"github.com/altalune-id/othra/internal/web/layout"
)

func natURL(extra ...string) string {
	return authorizeURL(append([]string{
		"client_id", "nat", "redirect_uri", cliRedirect, "resource", mcpResource, "scope", "openid sample:read offline_access",
	}, extra...)...)
}

func reqURL(extra ...string) string {
	return authorizeURL(append([]string{"client_id", "req", "resource", mcpResource, "scope", "openid sample:read"}, extra...)...)
}

func (f *webFixture) consentKey(email, clientID string) consent.Key {
	rs := rowID(mcpResource)
	return consent.Key{EndUserID: f.accountOf(email).EndUserID, ClientID: rowID(clientID), ResourceServerID: &rs}
}

func (f *webFixture) toConsent(c *http.Client, authorize string) hostedPage {
	f.t.Helper()
	first := f.get(c, authorize)
	require.Equal(f.t, http.StatusSeeOther, first.status, first.body)
	require.True(f.t, strings.HasPrefix(first.location(), "/p/acme/consent?"), first.location())
	page := f.get(c, first.location())
	require.Equal(f.t, http.StatusOK, page.status, page.body)
	return page
}

// the first consent shows the full page; Allow stores the scopes and the code follows.
func TestConsent_FirstTimeThenRemembered(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	page := f.toConsent(f.client, natURL())
	for _, want := range []string{"Native App", "Sample MCP", mcpResource, "Read your samples", "sample:read", "offline_access", "127.0.0.1"} {
		assert.Contains(t, page.body, want)
	}
	allowed := f.submit(f.client, page, "/consent", nil)
	require.Equal(t, http.StatusOK, allowed.status, "the continue page: %s", allowed.body)
	q := redirectParams(t, f.follow(f.client, allowed))
	require.NotEmpty(t, q.Get("code"), q)
	last := f.grants.codes[len(f.grants.codes)-1]
	require.NotNil(t, last.ResourceServerID)
	assert.Equal(t, rowID(mcpResource), *last.ResourceServerID)
	stored := f.consents.stored(f.consentKey("ana@example.com", "nat"))
	require.NotNil(t, stored)
	assert.Equal(t, []string{"openid", "profile", "email", "offline_access", "sample:read", "sample:write"}, f.consents.grants[0].Allowed,
		"the OIDC scopes and the server's catalog")
	assert.Equal(t, []string{"openid", "sample:read", "offline_access"}, stored.Scopes)

	again := f.toConsent(f.client, natURL())
	assert.Contains(t, again.body, "Continue to Native App", "remembered on a public client is one click")
	assert.NotContains(t, again.body, "Read your samples", "no scope list")
	done := f.submit(f.client, again, "/consent", nil)
	assert.NotEmpty(t, redirectParams(t, f.follow(f.client, done)).Get("code"))

	more := f.toConsent(f.client, natURL("scope", "openid sample:read sample:write"))
	assert.Contains(t, more.body, "sample:write", "a new scope shows the full page")
	assert.Contains(t, more.body, "Allowed before")
	f.submit(f.client, more, "/consent", nil)
	assert.Equal(t, []string{"openid", "sample:read", "offline_access", "sample:write"},
		f.consents.stored(f.consentKey("ana@example.com", "nat")).Scopes, "the union")
}

// a confidential client with a remembered consent goes straight through.
func TestConsent_RememberedConfidential(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	page := f.toConsent(f.client, reqURL())
	assert.NotContains(t, page.body, "127.0.0.1")
	f.follow(f.client, f.submit(f.client, page, "/consent", nil))
	p := f.get(f.client, reqURL())
	assert.Equal(t, http.StatusFound, p.status)
	assert.NotEmpty(t, redirectParams(t, p).Get("code"))
}

// prompt=consent always shows the full page, and approving it does not loop.
func TestConsent_PromptConsent(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	for _, client := range []string{"req", "web"} {
		u := authorizeURL("client_id", client, "resource", mcpResource, "scope", "openid sample:read", "prompt", "consent")
		page := f.toConsent(f.client, u)
		assert.Contains(t, page.body, "Read your samples", client)
		done := f.submit(f.client, page, "/consent", nil)
		assert.NotEmpty(t, redirectParams(t, f.follow(f.client, done)).Get("code"), client)
		page = f.toConsent(f.client, u)
		assert.NotContains(t, page.body, "Continue to", "%s: the full page even when remembered", client)
	}
}

// Deny answers access_denied and stores nothing; prompt=none without consent is consent_required.
func TestConsent_DenyAndPromptNone(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	q := redirectParams(t, f.get(f.client, reqURL("prompt", "none")))
	assert.Equal(t, "consent_required", q.Get("error"))
	assert.Equal(t, "st", q.Get("state"))

	page := f.toConsent(f.client, reqURL())
	denied := f.submit(f.client, page, "/consent", url.Values{"deny": {"1"}})
	require.Equal(t, http.StatusOK, denied.status, "the continue page")
	m := refreshRE.FindStringSubmatch(denied.body)
	require.NotEmpty(t, m, denied.body)
	u, err := url.Parse(plain(m[1]))
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(u.String(), webRedirect+"?"), u.String())
	q = u.Query()
	assert.Equal(t, "access_denied", q.Get("error"))
	assert.Equal(t, "st", q.Get("state"))
	assert.Equal(t, f.srv.URL+"/p/acme", q.Get("iss"))
	assert.Empty(t, f.consents.grants)

	forged := f.do(f.client, http.MethodPost, "/p/acme/consent", url.Values{"return_to": {"/p/acme/oauth2/authorize?x=1"}})
	assert.Equal(t, http.StatusForbidden, forged.status, "CSRF")
	assert.Empty(t, f.consents.grants)
}

// an implicit client without prompt=consent is admin consent; nothing is stored or shown.
func TestConsent_ImplicitClient(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	p := f.get(f.client, authorizeURL("resource", mcpResource, "scope", "openid sample:read"))
	assert.NotEmpty(t, redirectParams(t, p).Get("code"))
	assert.Empty(t, f.consents.grants)
}

// prompt=login and consent together still end in a code: the consent page keeps the sign-in marker.
func TestConsent_AfterFreshSignIn(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	first := f.get(f.client, reqURL("prompt", "login"))
	require.Equal(t, http.StatusSeeOther, first.status)
	done := f.signInVia(f.client, f.get(f.client, first.location()), "ana@example.com")
	toConsent := f.follow(f.client, done)
	require.Equal(t, http.StatusSeeOther, toConsent.status, toConsent.body)
	require.True(t, strings.HasPrefix(toConsent.location(), "/p/acme/consent?"), toConsent.location())
	page := f.get(f.client, toConsent.location())
	final := f.follow(f.client, f.submit(f.client, page, "/consent", nil))
	assert.NotEmpty(t, redirectParams(t, final).Get("code"), "no loop back to sign-in")
}

// a metadata document client shows its host, the unverified note and the loopback warning.
func TestConsent_MetadataClient(t *testing.T) {
	f := newWebFixture(t)
	f.signIn(f.client, "acme", "ana@example.com")
	u := authorizeURL("client_id", cimdClient, "redirect_uri", "http://localhost:53999/callback", "resource", mcpResource, "scope", "sample:read")
	page := f.toConsent(f.client, u)
	for _, want := range []string{"Claude Code", "claude.ai", "Unverified app", "localhost"} {
		assert.Contains(t, page.body, want)
	}
	done := f.follow(f.client, f.submit(f.client, page, "/consent", nil))
	require.Equal(t, http.StatusFound, done.status, done.body)
	assert.True(t, strings.HasPrefix(done.location(), "http://localhost:53999/callback?"), done.location())
}

// a throttled metadata document fetch is the 429 page with Retry-After, never a 500.
func TestAuthorize_MetadataFetchThrottled(t *testing.T) {
	f := newWebFixture(t)
	f.admissions.fail = &layout.RateLimitedError{RetryAfter: 30 * time.Second}
	p := f.get(f.client, authorizeURL("client_id", cimdClient, "redirect_uri", "http://localhost/callback", "resource", mcpResource))
	assert.Equal(t, http.StatusTooManyRequests, p.status, p.body)
	assert.Equal(t, "30", p.header.Get("Retry-After"))
	assert.Empty(t, p.location())
	assert.Empty(t, f.reports, "not an unexpected error")
	assert.NotContains(t, p.body, "Back to sign in", "product-design §6: an authorize error page has no way into sign-in")

	f.admissions.fail = errFake
	failed := f.get(f.client, authorizeURL("client_id", cimdClient, "redirect_uri", "http://localhost/callback", "resource", mcpResource))
	assert.Equal(t, http.StatusInternalServerError, failed.status)
	assert.NotContains(t, failed.body, "Back to sign in")
}

func TestConsent_PageEdges(t *testing.T) {
	f := newWebFixture(t)
	notSignedIn := f.get(f.newClient(), "/p/acme/consent?return_to="+url.QueryEscape(reqURL()))
	assert.Equal(t, http.StatusSeeOther, notSignedIn.status)
	assert.Contains(t, notSignedIn.location(), "/p/acme/login?")

	f.signIn(f.client, "acme", "ana@example.com")
	notAuthorize := f.get(f.client, "/p/acme/consent?return_to="+url.QueryEscape("/p/acme/account/"))
	assert.Equal(t, http.StatusSeeOther, notAuthorize.status)
	assert.Equal(t, "/p/acme/account/", notAuthorize.location())

	implicit := f.get(f.client, "/p/acme/consent?return_to="+url.QueryEscape(authorizeURL()))
	assert.Equal(t, http.StatusSeeOther, implicit.status, "nothing to consent to: back to authorize")

	bad := f.get(f.client, "/p/acme/consent?return_to="+url.QueryEscape("/p/acme/oauth2/authorize?client_id=nope"))
	assert.Equal(t, http.StatusBadRequest, bad.status)

	f.consents.failOn = "consent"
	assert.Equal(t, http.StatusInternalServerError, f.get(f.client, reqURL()).status)
	f.consents.failOn = ""
	page := f.toConsent(f.client, reqURL())
	f.consents.failOn = "grant"
	assert.Equal(t, http.StatusInternalServerError, f.submit(f.client, page, "/consent", nil).status)
}
