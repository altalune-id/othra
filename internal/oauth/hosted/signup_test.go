package hosted

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
)

func newUnprovenFixture(t *testing.T) (*webFixture, *fakeFederation) {
	t.Helper()
	f, fed := newFederationFixture(t)
	fed.unproven, fed.email = true, "budi@acme.test"
	return f, fed
}

// a first Google sign-in for budi@acme.test (no hd) creates nobody, mails a sign-up link and lands on a reload-safe GET page.
func TestUnproven_MailsALinkAndShowsTheInboxPage(t *testing.T) {
	f, _ := newUnprovenFixture(t)
	page := f.collide(f.client)
	require.Equal(t, http.StatusOK, page.status, page.body)
	assert.Contains(t, page.body, "Check your inbox to finish signing up with Google")
	assert.Contains(t, page.body, "budi@acme.test")
	assert.Zero(t, f.users.redeems, "no end user yet")
	require.Len(t, f.users.issued, 1)
	assert.Contains(t, string(f.users.issued[0].Pending), `"sign_up":true`)
	require.Len(t, f.users.mails, 1)
	assert.Equal(t, "Finish signing up to Acme with Google", f.users.mails[0].Subject)
	assert.Contains(t, f.users.mails[0].TextBody, "with Google as budi@acme.test")

	reload := f.get(f.client, "/p/acme/federation/linking?return_to=%2Fp%2Facme%2Faccount%2F")
	assert.Equal(t, page.body, reload.body)
	assert.Len(t, f.users.mails, 1, "a reload sends nothing")
}

func TestUnproven_SameBrowserCreatesAndLinks(t *testing.T) {
	for _, emailLink := range []bool{true, false} {
		f, fed := newUnprovenFixture(t)
		f.acme.SignInEmailLink = emailLink
		require.Equal(t, http.StatusOK, f.collide(f.client).status)
		verify := f.openLink(f.client)
		assert.NotContains(t, verify.body, `name="skip"`, "a sign-up has nothing to skip")
		assert.Contains(t, verify.body, "Finish signing up to Acme with Google", "the confirm page names the Google sign-up it finishes")
		done := f.submit(f.client, verify, "/login/verify", nil)
		require.Equal(t, http.StatusSeeOther, done.status, done.body)
		assert.Len(t, fed.linked, 1, "email link on=%v", emailLink)
		assert.Equal(t, 1, f.users.redeems)
	}
}

func TestUnproven_AnotherBrowser(t *testing.T) {
	t.Run("email link on: a plain sign-up", func(t *testing.T) {
		f, fed := newUnprovenFixture(t)
		require.Equal(t, http.StatusOK, f.collide(f.client).status)
		other := f.newClient()
		done := f.submit(other, f.openLink(other), "/login/verify", nil)
		require.Equal(t, http.StatusSeeOther, done.status, done.body)
		assert.Empty(t, fed.linked)
		assert.Equal(t, 1, f.users.redeems)
	})
	t.Run("email link off: refused", func(t *testing.T) {
		f, fed := newUnprovenFixture(t)
		f.acme.SignInEmailLink = false
		require.Equal(t, http.StatusOK, f.collide(f.client).status)
		other := f.newClient()
		got := f.submit(other, f.openLink(other), "/login/verify", nil)
		assert.Equal(t, http.StatusForbidden, got.status, got.body)
		assert.Contains(t, got.body, string(apperror.CodeSignInEmailDisabled))
		assert.Empty(t, fed.linked)
		assert.Zero(t, f.users.redeems)
	})
}

// SECURITY: an end user who took the email before the redeem signs in by the link; the Google identity is never auto-linked.
func TestUnproven_EmailTakenByRedeemTimeIsNeverLinked(t *testing.T) {
	f, fed := newUnprovenFixture(t)
	require.Equal(t, http.StatusOK, f.collide(f.client).status)
	f.users.users[key(Project{ID: f.acme.ProjectID}, "budi@acme.test")] = "sub-budi"
	done := f.submit(f.client, f.openLink(f.client), "/login/verify", url.Values{})
	require.Equal(t, http.StatusSeeOther, done.status, done.body)
	assert.Empty(t, fed.linked)
}

// a Google sign-up proven by its inbox keeps Google's name; a collision link never renames an existing account.
func TestUnproven_KeepsTheGoogleName(t *testing.T) {
	f, fed := newUnprovenFixture(t)
	require.Equal(t, http.StatusOK, f.collide(f.client).status)
	done := f.submit(f.client, f.openLink(f.client), "/login/verify", nil)
	require.Equal(t, http.StatusSeeOther, done.status, done.body)
	require.Len(t, fed.linked, 1)
	assert.Equal(t, "Budi", f.users.names[fed.linked[0]])

	c, cfed := newFederationFixture(t)
	cfed.collide = true
	require.Equal(t, http.StatusOK, c.collide(c.client).status)
	require.Equal(t, http.StatusSeeOther, c.submit(c.client, c.openLink(c.client), "/login/verify", nil).status)
	require.Len(t, cfed.linked, 1)
	assert.Empty(t, c.users.names, "a collision link leaves the name alone")

	e, _ := newUnprovenFixture(t)
	require.Equal(t, http.StatusOK, e.collide(e.client).status)
	e.users.failOn = "name"
	assert.Equal(t, http.StatusInternalServerError, e.submit(e.client, e.openLink(e.client), "/login/verify", nil).status)
	assert.Zero(t, e.users.redeems, "the redeem rolls back")
}
