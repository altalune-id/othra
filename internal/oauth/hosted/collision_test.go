package hosted

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/internal/federation"
)

func (f *webFixture) collideAtCallback(c *http.Client) hostedPage {
	f.t.Helper()
	_, authURL := f.startGoogle(c, "/p/acme/login?add=1")
	return f.callback(c, url.Values{"code": {"c"}, "state": {authURL.Query().Get("state")}})
}

func (f *webFixture) collide(c *http.Client) hostedPage {
	f.t.Helper()
	moved := f.collideAtCallback(c)
	require.Equal(f.t, http.StatusSeeOther, moved.status, moved.body)
	require.Equal(f.t, "/p/acme/federation/linking?return_to=%2Fp%2Facme%2Faccount%2F", moved.location())
	return f.get(c, moved.location())
}

func (f *webFixture) openLink(c *http.Client) hostedPage {
	f.t.Helper()
	moved := f.get(c, strings.TrimPrefix(linkIn(f.t, f.users.mails[len(f.users.mails)-1].TextBody), f.srv.URL))
	require.Equal(f.t, http.StatusSeeOther, moved.status, moved.body)
	verify := f.get(c, moved.location())
	require.Equal(f.t, http.StatusOK, verify.status, verify.body)
	return verify
}

// a collision mails the existing account a link; on the same browser, Connect links Google inside the redeem.
func TestCollision_LinksOnTheSameBrowser(t *testing.T) {
	f, fed := newFederationFixture(t)
	fed.collide = true
	page := f.collide(f.client)
	require.Equal(t, http.StatusOK, page.status, page.body)
	assert.Contains(t, page.body, "ana@gmail.com")
	require.Len(t, f.users.issued, 1)
	assert.Equal(t, "ana@gmail.com", f.users.issued[0].Email)
	assert.Contains(t, string(f.users.issued[0].Pending), `"Subject"`)
	require.Len(t, f.users.mails, 1)
	mail := f.users.mails[0]
	assert.Equal(t, "Add Google to your Acme account", mail.Subject, "the subject names adding Google to the account")
	assert.Contains(t, mail.TextBody, "with Google as ana@gmail.com")
	assert.Contains(t, mail.HTMLBody, ">Add Google</a>")

	verify := f.openLink(f.client)
	assert.Contains(t, verify.body, `name="skip"`)
	done := f.submit(f.client, verify, "/login/verify", nil)
	require.Equal(t, http.StatusSeeOther, done.status, done.body)
	assert.Len(t, fed.linked, 1)
	for _, c := range f.client.Jar.Cookies(mustURL(t, f.srv.URL+"/p/acme/login/verify")) {
		assert.NotEqual(t, bindingCookie, c.Name, "the binding is spent with the link")
	}
}

func TestCollision_NotNowSignsInWithoutLinking(t *testing.T) {
	f, fed := newFederationFixture(t)
	fed.collide = true
	require.Equal(t, http.StatusOK, f.collide(f.client).status)
	done := f.submit(f.client, f.openLink(f.client), "/login/verify", url.Values{"skip": {"1"}})
	require.Equal(t, http.StatusSeeOther, done.status, done.body)
	assert.Empty(t, fed.linked)
	assert.Equal(t, 1, f.users.redeems)
}

// SECURITY: the victim opening their own link on another device must not attach the Google account that caused the collision.
func TestCollision_AnotherDeviceSignsInPlainly(t *testing.T) {
	f, fed := newFederationFixture(t)
	fed.collide = true
	require.Equal(t, http.StatusOK, f.collide(f.client).status)
	other := f.newClient()
	verify := f.openLink(other)
	assert.NotContains(t, verify.body, `name="skip"`)
	done := f.submit(other, verify, "/login/verify", nil)
	require.Equal(t, http.StatusSeeOther, done.status, done.body)
	assert.Empty(t, fed.linked)
}

func TestCollision_LinkedElsewhereRollsTheRedeemBack(t *testing.T) {
	f, fed := newFederationFixture(t)
	fed.collide, fed.linkErr = true, &federation.LinkedElsewhereError{}
	require.Equal(t, http.StatusOK, f.collide(f.client).status)
	got := f.submit(f.client, f.openLink(f.client), "/login/verify", nil)
	assert.Equal(t, http.StatusForbidden, got.status, got.body)
	assert.Contains(t, got.body, string(apperror.CodeSignInUpstreamLinkedElsewhere))
	assert.Zero(t, f.users.redeems, "nothing signed in")
}

func TestCollision_EmailLinkOff(t *testing.T) {
	f, fed := newFederationFixture(t)
	fed.collide = true
	f.acme.SignInEmailLink = false
	got := f.collide(f.client)
	assert.Equal(t, http.StatusForbidden, got.status, got.body)
	assert.Contains(t, got.body, string(apperror.CodeSignInUpstreamEmailTaken))
	assert.Empty(t, f.users.issued)
}

func TestCollision_IssueFailure(t *testing.T) {
	f, fed := newFederationFixture(t)
	fed.collide = true
	f.users.failOn = "issue"
	assert.Equal(t, http.StatusInternalServerError, f.collideAtCallback(f.client).status)
}

func TestRedeem_EmailLinkTurnedOffAfterIssue(t *testing.T) {
	f := newWebFixture(t)
	login := f.get(f.client, "/p/acme/login")
	require.Equal(t, http.StatusSeeOther, f.submit(f.client, login, "/login", url.Values{"email": {"new@example.com"}}).status)
	f.acme.SignInEmailLink = false
	got := f.submit(f.client, f.openLink(f.client), "/login/verify", nil)
	assert.Equal(t, http.StatusForbidden, got.status, got.body)
	assert.Contains(t, got.body, string(apperror.CodeSignInEmailDisabled))
	assert.Zero(t, f.users.redeems)
}

// the collision page is a GET after a 303, so a reload neither replays the code nor mails again.
func TestCollision_PageSurvivesAReload(t *testing.T) {
	f, fed := newFederationFixture(t)
	fed.collide = true
	first := f.collide(f.client)
	require.Equal(t, http.StatusOK, first.status, first.body)
	assert.Contains(t, first.body, "ana@gmail.com")
	reload := f.get(f.client, "/p/acme/federation/linking?return_to=%2Fp%2Facme%2Faccount%2F")
	assert.Equal(t, http.StatusOK, reload.status, reload.body)
	assert.Equal(t, first.body, reload.body)
	assert.Len(t, f.users.mails, 1, "a reload sends nothing")

	cold := f.get(f.newClient(), "/p/acme/federation/linking")
	assert.Equal(t, http.StatusSeeOther, cold.status, "no collision on this browser")
	assert.Equal(t, "/p/acme/login?return_to=%2Fp%2Facme%2Faccount%2F", cold.location())
}
