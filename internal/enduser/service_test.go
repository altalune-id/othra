package enduser

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/testutil/fakes"
	"altalune.id/othra/mailer"
	"altalune.id/othra/nanoid"
)

type harness struct {
	store   *fakeStore
	mail    *fakeMail
	svc     *Service
	now     time.Time
	project Project
}

func testConfig() Config {
	return Config{LinkTTL: 15 * time.Minute, ResendAfter: time.Minute, Idle: 30 * 24 * time.Hour, Absolute: 90 * 24 * time.Hour, MaxAccounts: 3}
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{
		store: newFakeStore(), mail: &fakeMail{}, now: time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC),
		project: Project{OrgID: uuid.New(), ID: uuid.New(), IssuerURL: "https://id.example.com/p/acme", EmailLink: true},
	}
	h.svc = NewService(h.store, fakes.Transactor{}, h.mail, testConfig(), apperror.NewReporter(nil, false).Unexpected).
		WithClock(func() time.Time { return h.now })
	return h
}

func compose(_ context.Context, l IssuedLink) (mailer.Message, error) {
	return mailer.Message{To: l.Email, Subject: "Sign in", TextBody: l.URL}, nil
}

func tokenOf(t *testing.T, link string) string {
	t.Helper()
	u, err := url.Parse(link)
	require.NoError(t, err)
	return u.Query().Get("t")
}

func (h *harness) send(t *testing.T, email string) IssuedLink {
	t.Helper()
	l, err := h.svc.SendLink(context.Background(), h.project, LinkRequest{Email: email, ReturnTo: "/p/acme/account/", IP: "192.0.2.1"}, compose)
	require.NoError(t, err)
	return l
}

func (h *harness) signIn(t *testing.T, email, browserSecret string) *Redeemed {
	t.Helper()
	l := h.send(t, email)
	r, err := h.svc.Redeem(context.Background(), h.project, RedeemRequest{Token: tokenOf(t, l.URL), BrowserSecret: browserSecret, UserAgent: "test"})
	require.NoError(t, err)
	return r
}

func codeOf(err error) apperror.Code {
	if ae, ok := apperror.AsAppError(err); ok {
		return ae.Code()
	}
	return ""
}

func TestSendLink(t *testing.T) {
	h := newHarness(t)
	l := h.send(t, "  Ada@Example.COM ")
	assert.True(t, l.Issued)
	assert.True(t, l.SignUp, "no end user has this email yet")
	assert.Equal(t, "ada@example.com", l.Email)
	assert.True(t, strings.HasPrefix(l.URL, "https://id.example.com/p/acme/login/verify?t="))
	assert.Equal(t, h.now.Add(15*time.Minute), l.ExpiresAt)
	require.Len(t, h.mail.sent, 1)
	assert.Equal(t, "ada@example.com", h.mail.sent[0].To)

	token := tokenOf(t, l.URL)
	assert.Len(t, token, 43, "32 random bytes, base64url")
	require.Len(t, h.store.links, 1)
	assert.Equal(t, nanoid.HashToken(token), h.store.links[0].hash, "only the hash is stored")
	assert.NotContains(t, h.store.links[0].hash, token)
	assert.Equal(t, "192.0.2.1", h.store.links[0].ip)

	again := h.send(t, "ada@example.com")
	assert.False(t, again.Issued, "a resend inside the cooldown issues nothing")
	assert.Empty(t, again.URL)
	assert.Len(t, h.mail.sent, 1, "and sends nothing")

	h.now = h.now.Add(time.Minute)
	fresh := h.send(t, "ada@example.com")
	assert.True(t, fresh.Issued)
	assert.Len(t, h.store.links, 1, "one live link per email: the older unused one is gone")
	_, err := h.svc.LinkByToken(context.Background(), h.project, token)
	assert.Equal(t, apperror.CodeSignInLinkNotUsable, codeOf(err), "the replaced link no longer works")
}

func TestIssueLink_SignUpOnlyForANewEmail(t *testing.T) {
	h := newHarness(t)
	h.signIn(t, "ada@example.com", "")
	h.now = h.now.Add(time.Hour)
	l, err := h.svc.IssueLink(context.Background(), h.project, LinkRequest{Email: "ada@example.com"})
	require.NoError(t, err)
	assert.False(t, l.SignUp)
	h.store.failOn = "EndUserExists"
	h.now = h.now.Add(time.Hour)
	_, err = h.svc.IssueLink(context.Background(), h.project, LinkRequest{Email: "ada@example.com"})
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

func TestIssueLink_ReturnsTheURLWithoutMail(t *testing.T) {
	h := newHarness(t)
	l, err := h.svc.IssueLink(context.Background(), h.project, LinkRequest{Email: "ada@example.com", ReturnTo: "/p/acme/account/"})
	require.NoError(t, err)
	assert.NotEmpty(t, l.URL)
	assert.Empty(t, h.mail.sent)
}

func TestSendLink_Refusals(t *testing.T) {
	h := newHarness(t)
	_, err := h.svc.SendLink(context.Background(), h.project, LinkRequest{Email: "not an email"}, compose)
	assert.True(t, IsInvalidEmailError(err))
	assert.Equal(t, apperror.CodeValidation, codeOf(err))

	off := h.project
	off.EmailLink = false
	_, err = h.svc.SendLink(context.Background(), off, LinkRequest{Email: "ada@example.com"}, compose)
	assert.Equal(t, apperror.CodeSignInEmailDisabled, codeOf(err))

	h.mail.err = errFake
	_, err = h.svc.SendLink(context.Background(), h.project, LinkRequest{Email: "ada@example.com"}, compose)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err), "a failed enqueue is unexpected")

	h = newHarness(t)
	_, err = h.svc.SendLink(context.Background(), h.project, LinkRequest{Email: "ada@example.com"},
		func(context.Context, IssuedLink) (mailer.Message, error) { return mailer.Message{}, errFake })
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))

	h = newHarness(t)
	h.store.failOn = "InsertLink"
	_, err = h.svc.SendLink(context.Background(), h.project, LinkRequest{Email: "ada@example.com"}, compose)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	assert.NotContains(t, err.Error(), "verify?t=")
}

func TestRedeem_SignsInOnceAndRotatesTheBrowserSecret(t *testing.T) {
	h := newHarness(t)
	l := h.send(t, "ada@example.com")
	link, err := h.svc.LinkByToken(context.Background(), h.project, tokenOf(t, l.URL))
	require.NoError(t, err)
	assert.Equal(t, "/p/acme/account/", link.ReturnTo)
	byID, err := h.svc.LinkByPublicID(context.Background(), h.project, link.PublicID)
	require.NoError(t, err)
	assert.Equal(t, link.Email, byID.Email)

	r, err := h.svc.Redeem(context.Background(), h.project, RedeemRequest{Token: tokenOf(t, l.URL)})
	require.NoError(t, err)
	assert.Equal(t, "/p/acme/account/", r.ReturnTo)
	assert.Equal(t, "ada@example.com", r.Account.Email)
	assert.Equal(t, h.now, r.Account.AuthTime)
	assert.Len(t, r.Secret, 43)
	assert.Equal(t, h.now.Add(30*24*time.Hour), r.ExpiresAt)
	require.Len(t, h.store.users, 1)
	assert.Equal(t, h.now, *h.store.users[0].verifiedAt, "the end user is created verified at redeem")

	_, err = h.svc.Redeem(context.Background(), h.project, RedeemRequest{Token: tokenOf(t, l.URL)})
	assert.Equal(t, apperror.CodeSignInLinkNotUsable, codeOf(err), "single use from another browser")
	_, err = h.svc.LinkByToken(context.Background(), h.project, tokenOf(t, l.URL))
	assert.Equal(t, apperror.CodeSignInLinkNotUsable, codeOf(err))

	repeat, err := h.svc.Redeem(context.Background(), h.project, RedeemRequest{Token: tokenOf(t, l.URL), BrowserSecret: r.Secret})
	require.NoError(t, err, "H23: a second Continue from the same browser succeeds")
	assert.Equal(t, r.Secret, repeat.Secret, "and signs nobody new in")
	assert.Equal(t, r.Account.PublicID, repeat.Account.PublicID)

	h.now = h.now.Add(time.Second)
	second := h.signIn(t, "bob@example.com", r.Secret)
	assert.NotEqual(t, r.Secret, second.Secret, "every sign-in rotates the browser secret")
	sess, err := h.svc.Session(context.Background(), h.project, r.Secret)
	require.NoError(t, err)
	assert.Nil(t, sess, "the old secret names no browser any more")
	sess, err = h.svc.Session(context.Background(), h.project, second.Secret)
	require.NoError(t, err)
	require.Len(t, sess.Accounts, 2)
	assert.Equal(t, "bob@example.com", sess.Active().Email, "the newest sign-in is active")
}

func TestRedeem_ExpiredAndUnknownLinks(t *testing.T) {
	h := newHarness(t)
	l := h.send(t, "ada@example.com")
	link, err := h.svc.LinkByToken(context.Background(), h.project, tokenOf(t, l.URL))
	require.NoError(t, err)
	h.now = h.now.Add(15 * time.Minute)
	_, err = h.svc.LinkByPublicID(context.Background(), h.project, link.PublicID)
	assert.Equal(t, apperror.CodeSignInLinkNotUsable, codeOf(err))
	_, err = h.svc.Redeem(context.Background(), h.project, RedeemRequest{Token: tokenOf(t, l.URL)})
	assert.Equal(t, apperror.CodeSignInLinkNotUsable, codeOf(err))
	assert.Empty(t, h.store.users, "a refused redeem creates no end user")

	for _, token := range []string{"", "nope"} {
		_, err = h.svc.LinkByToken(context.Background(), h.project, token)
		assert.Equal(t, apperror.CodeSignInLinkNotUsable, codeOf(err), token)
	}
	other := h.project
	other.ID = uuid.New()
	h.now = h.now.Add(-time.Minute)
	_, err = h.svc.LinkByToken(context.Background(), other, tokenOf(t, l.URL))
	assert.Equal(t, apperror.CodeSignInLinkNotUsable, codeOf(err), "another project's link is unknown")

	h.store.failOn = "LinkByTokenHash"
	_, err = h.svc.LinkByToken(context.Background(), h.project, tokenOf(t, l.URL))
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

func TestSessions_SelectSignOutAndTheAccountCap(t *testing.T) {
	h := newHarness(t)
	ada := h.signIn(t, "ada@example.com", "")
	h.now = h.now.Add(time.Second)
	bob := h.signIn(t, "bob@example.com", ada.Secret)
	secret := bob.Secret

	a, err := h.svc.Select(context.Background(), h.project, secret, ada.Account.PublicID)
	require.NoError(t, err)
	assert.Equal(t, "ada@example.com", a.Email)
	sess, err := h.svc.Session(context.Background(), h.project, secret)
	require.NoError(t, err)
	assert.Equal(t, "ada@example.com", sess.Active().Email)

	_, err = h.svc.Select(context.Background(), h.project, secret, "nope")
	assert.Equal(t, apperror.CodeSignInSessionGone, codeOf(err))
	_, err = h.svc.Select(context.Background(), h.project, "", ada.Account.PublicID)
	assert.Equal(t, apperror.CodeSignInSessionGone, codeOf(err))

	require.NoError(t, h.svc.SignOut(context.Background(), h.project, secret, ada.Account.PublicID))
	sess, err = h.svc.Session(context.Background(), h.project, secret)
	require.NoError(t, err)
	require.Len(t, sess.Accounts, 1, "sign-out ends the current account only")
	assert.Equal(t, "bob@example.com", sess.Active().Email)
	_, err = h.svc.Select(context.Background(), h.project, secret, ada.Account.PublicID)
	assert.Equal(t, apperror.CodeSignInSessionGone, codeOf(err))
	require.NoError(t, h.svc.SignOut(context.Background(), h.project, "", "x"), "signing out of nothing is fine")

	for _, email := range []string{"c@example.com", "d@example.com", "e@example.com"} {
		h.now = h.now.Add(time.Second)
		secret = h.signIn(t, email, secret).Secret
	}
	sess, err = h.svc.Session(context.Background(), h.project, secret)
	require.NoError(t, err)
	require.Len(t, sess.Accounts, 3, "MaxAccounts keeps the most recently selected")
	assert.Equal(t, "e@example.com", sess.Active().Email)
	_, ok := sess.Account(bob.Account.PublicID)
	assert.False(t, ok, "the least recently selected account was signed out")
}

func TestSessions_IdleAndAbsoluteExpiry(t *testing.T) {
	h := newHarness(t)
	r := h.signIn(t, "ada@example.com", "")
	h.now = h.now.Add(29 * 24 * time.Hour)
	sess, err := h.svc.Session(context.Background(), h.project, r.Secret)
	require.NoError(t, err)
	require.NotNil(t, sess, "inside the idle window")
	assert.Equal(t, h.now.Add(30*24*time.Hour), sess.ExpiresAt, "a lookup extends the idle window")

	for range 3 {
		h.now = h.now.Add(29 * 24 * time.Hour)
		if sess, _ = h.svc.Session(context.Background(), h.project, r.Secret); sess == nil {
			break
		}
	}
	assert.Nil(t, sess, "the absolute window ends it even when used")

	h = newHarness(t)
	r = h.signIn(t, "ada@example.com", "")
	h.now = h.now.Add(31 * 24 * time.Hour)
	sess, err = h.svc.Session(context.Background(), h.project, r.Secret)
	require.NoError(t, err)
	assert.Nil(t, sess, "idle for longer than the idle window")
	other := h.project
	other.ID = uuid.New()
	sess, err = h.svc.Session(context.Background(), other, r.Secret)
	require.NoError(t, err)
	assert.Nil(t, sess, "a cookie names no browser in another project")
}

func TestTerms(t *testing.T) {
	h := newHarness(t)
	r := h.signIn(t, "ada@example.com", "")
	need, _, err := h.svc.NeedsTerms(context.Background(), h.project, r.Account)
	require.NoError(t, err)
	assert.False(t, need, "a project without terms asks nothing")
	a, err := h.svc.AcceptTerms(context.Background(), h.project, r.Secret, r.Account.PublicID, 0, "", "")
	require.NoError(t, err)
	assert.Equal(t, r.Account.PublicID, a.PublicID)
	assert.Empty(t, h.store.terms)

	h.project.TermsVersion, h.project.TermsURL, h.project.PrivacyURL = 2, "https://acme.test/terms", "https://acme.test/privacy"
	need, _, err = h.svc.NeedsTerms(context.Background(), h.project, r.Account)
	require.NoError(t, err)
	assert.True(t, need)

	h.now = h.now.Add(time.Second)
	bob := h.signIn(t, "bob@example.com", r.Secret)
	_, err = h.svc.AcceptTerms(context.Background(), h.project, bob.Secret, r.Account.PublicID, 2, "192.0.2.7", strings.Repeat("u", 600))
	require.NoError(t, err)
	got := h.store.terms[r.Account.EndUserID][2]
	assert.Equal(t, TermsAcceptance{
		EndUserID: r.Account.EndUserID, Version: 2, TermsURL: "https://acme.test/terms", PrivacyURL: "https://acme.test/privacy",
		AcceptedAt: h.now, UserSessionID: r.Account.UserSessionID, UserSessionPublicID: r.Account.PublicID, IP: "192.0.2.7",
		UserAgent: strings.Repeat("u", 512),
	}, got, "recorded for the account named in the form, not the active one")
	need, _, err = h.svc.NeedsTerms(context.Background(), h.project, r.Account)
	require.NoError(t, err)
	assert.False(t, need)
	need, _, err = h.svc.NeedsTerms(context.Background(), h.project, bob.Account)
	require.NoError(t, err)
	assert.True(t, need, "the active account still owes the terms")

	_, err = h.svc.AcceptTerms(context.Background(), h.project, bob.Secret, "nope", 2, "", "")
	assert.Equal(t, apperror.CodeSignInSessionGone, codeOf(err))
	h.store.failOn = "InsertTerms"
	_, err = h.svc.AcceptTerms(context.Background(), h.project, bob.Secret, bob.Account.PublicID, 2, "", "")
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	h.project.TermsVersion = 3
	need, updated, err := h.svc.NeedsTerms(context.Background(), h.project, r.Account)
	require.NoError(t, err)
	assert.True(t, need)
	assert.True(t, updated, "accepted 2, so version 3 is an update")
	_, updated, err = h.svc.NeedsTerms(context.Background(), h.project, bob.Account)
	require.NoError(t, err)
	assert.False(t, updated, "never accepted, so it is the first time")
	h.store.failOn = "LatestTerms"
	_, _, err = h.svc.NeedsTerms(context.Background(), h.project, bob.Account)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

// F4: an acceptance of the version the page showed only; a changed version shows the current terms again.
func TestAcceptTerms_TheVersionTheUserSaw(t *testing.T) {
	h := newHarness(t)
	r := h.signIn(t, "ada@example.com", "")
	h.project.TermsVersion, h.project.TermsURL = 3, "https://acme.test/terms"
	_, err := h.svc.AcceptTerms(context.Background(), h.project, r.Secret, r.Account.PublicID, 2, "", "")
	changed, ok := errors.AsType[*TermsChangedError](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, TermsChangedError{Seen: 2, Current: 3}, *changed)
	assert.True(t, IsTermsChangedError(err))
	assert.Contains(t, err.Error(), "2 to 3")
	assert.Empty(t, h.store.terms, "nothing is recorded")
}

// The per-email limit is spent only by a request the cooldown lets through, so blocked resends never drop the next real send.
func TestSendLink_TheLimitRunsAfterTheCooldown(t *testing.T) {
	h := newHarness(t)
	calls := 0
	allow := true
	req := LinkRequest{Email: "ada@example.com", Allow: func(context.Context) (bool, error) { calls++; return allow, nil }}
	l, err := h.svc.SendLink(context.Background(), h.project, req, compose)
	require.NoError(t, err)
	require.True(t, l.Issued)
	require.Equal(t, 1, calls)

	for range 3 {
		l, err = h.svc.SendLink(context.Background(), h.project, req, compose)
		require.NoError(t, err)
		assert.False(t, l.Issued)
	}
	assert.Equal(t, 1, calls, "cooldown-blocked resends never call Allow")

	h.now = h.now.Add(time.Minute)
	l, err = h.svc.SendLink(context.Background(), h.project, req, compose)
	require.NoError(t, err)
	assert.True(t, l.Issued)
	assert.Equal(t, 2, calls, "the next issue does")

	h.now = h.now.Add(time.Minute)
	allow = false
	l, err = h.svc.SendLink(context.Background(), h.project, req, compose)
	require.NoError(t, err)
	assert.False(t, l.Issued, "a refused limit issues nothing, silently")
	assert.Len(t, h.mail.sent, 2)
	assert.Len(t, h.store.links, 1, "and keeps the live link")

	h.now = h.now.Add(time.Minute)
	req.Allow = func(context.Context) (bool, error) { return false, errFake }
	_, err = h.svc.SendLink(context.Background(), h.project, req, compose)
	assert.ErrorIs(t, err, errFake, "a limiter error is returned as is")
}

// F7: a concurrent request that already issued the email's live link answers like the cooldown.
func TestSendLink_ARaceOnTheLiveLinkIsACooldown(t *testing.T) {
	h := newHarness(t)
	h.store.raceInsert = true
	l, err := h.svc.SendLink(context.Background(), h.project, LinkRequest{Email: "ada@example.com"}, compose)
	require.NoError(t, err)
	assert.False(t, l.Issued)
	assert.Equal(t, "ada@example.com", l.Email)
	assert.Empty(t, h.mail.sent)
}

// F3: one line per sign-in and terms acceptance, with public ids only.
func TestRedeemAndAcceptTerms_LogOneLineEach(t *testing.T) {
	h := newHarness(t)
	var buf bytes.Buffer
	h.svc.WithLogger(slog.New(slog.NewJSONHandler(&buf, nil)))
	h.project.PublicID = "prj_public_id_000000"
	l := h.send(t, "ada@example.com")
	r, err := h.svc.Redeem(context.Background(), h.project, RedeemRequest{Token: tokenOf(t, l.URL)})
	require.NoError(t, err)
	h.project.TermsVersion, h.project.TermsURL = 1, "https://acme.test/terms"
	_, err = h.svc.AcceptTerms(context.Background(), h.project, r.Secret, r.Account.PublicID, 1, "192.0.2.1", "ua")
	require.NoError(t, err)

	logs := buf.String()
	assert.Equal(t, 2, strings.Count(logs, "\n"), logs)
	for _, want := range []string{`"msg":"enduser: signed in"`, `"msg":"enduser: accepted terms"`, `"project":"prj_public_id_000000"`,
		`"end_user":"` + r.Account.Subject + `"`, `"user_session":"` + r.Account.PublicID + `"`, `"version":1`} {
		assert.Contains(t, logs, want)
	}
	for _, secret := range []string{"ada@example.com", tokenOf(t, l.URL), r.Secret, "192.0.2.1"} {
		assert.NotContains(t, logs, secret)
	}
}

func TestPrune(t *testing.T) {
	h := newHarness(t)
	r := h.signIn(t, "ada@example.com", "")
	require.NoError(t, h.svc.SignOut(context.Background(), h.project, r.Secret, r.Account.PublicID))
	h.send(t, "bob@example.com")
	h.now = h.now.Add(31 * 24 * time.Hour)
	n, err := h.svc.Prune(context.Background(), h.project.OrgID, h.project.ID)
	require.NoError(t, err)
	assert.Equal(t, 4, n, "two links, the ended user session and the idle browser")
	h.store.failOn = "Prune"
	_, err = h.svc.Prune(context.Background(), h.project.OrgID, h.project.ID)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

func TestSessionAccessors(t *testing.T) {
	var s *Session
	assert.Nil(t, s.Active())
	_, ok := s.Account("x")
	assert.False(t, ok)
	assert.Nil(t, (&Session{}).Active())
	assert.Equal(t, "héllo", clip("héllo", 10))
	assert.Equal(t, "h", clip("héllo", 2), "never splits a rune")
	assert.False(t, Project{TermsVersion: 1}.TermsRequired(), "a version without a terms link asks nothing")
}

func TestStoreFailuresAreUnexpected(t *testing.T) {
	h := newHarness(t)
	r := h.signIn(t, "ada@example.com", "")
	for _, op := range []string{"BrowserBySecretHash", "TouchBrowser", "Accounts"} {
		h.store.failOn = op
		h.now = h.now.Add(2 * time.Minute)
		_, err := h.svc.Session(context.Background(), h.project, r.Secret)
		assert.Equal(t, apperror.CodeUnexpected, codeOf(err), op)
	}
	h.store.failOn = "Accounts"
	_, err := h.svc.Select(context.Background(), h.project, r.Secret, r.Account.PublicID)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	h.store.failOn = "EndUserSession"
	assert.Equal(t, apperror.CodeUnexpected, codeOf(h.svc.SignOut(context.Background(), h.project, r.Secret, r.Account.PublicID)))
}
