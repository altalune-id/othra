//go:build integration

package enduser_test

import (
	"bytes"
	"context"
	"database/sql"
	"log/slog"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/enduser"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/testutil/pgtest"
	"altalune.id/othra/mailer"
)

type mailbox struct {
	mu   sync.Mutex
	sent []mailer.Message
}

func (m *mailbox) Enqueue(_ context.Context, msg mailer.Message) (uuid.UUID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, msg)
	return uuid.New(), nil
}

type pgFixture struct {
	db   *sql.DB
	svc  *enduser.Service
	mail *mailbox
	now  time.Time
	logs *bytes.Buffer
}

func newPGFixture(t *testing.T) *pgFixture {
	t.Helper()
	pool, sqlDB, schemaName := pgtest.MigratedRLS(t)
	f := &pgFixture{db: sqlDB, mail: &mailbox{}, now: time.Now().UTC().Truncate(time.Microsecond), logs: &bytes.Buffer{}}
	reporter := apperror.NewReporter(slog.New(slog.NewJSONHandler(f.logs, nil)), false)
	cfg := enduser.Config{LinkTTL: 15 * time.Minute, ResendAfter: time.Minute, Idle: 30 * 24 * time.Hour, Absolute: 90 * 24 * time.Hour, MaxAccounts: 2}
	f.svc = enduser.NewService(enduser.NewStore(pool, schemaName, ""), db.NewTransactor(pool), f.mail, cfg, reporter.Unexpected).
		WithClock(func() time.Time { return f.now })
	return f
}

func (f *pgFixture) project(t *testing.T) enduser.Project {
	t.Helper()
	org, prj := uuid.New(), uuid.New()
	_, err := f.db.ExecContext(t.Context(), `INSERT INTO orgs (id, public_id, slug, name, created_at, updated_at) VALUES ($1, $2, $2, 'Org', now(), now())`,
		org, "o"+org.String()[:12])
	require.NoError(t, err)
	_, err = f.db.ExecContext(t.Context(), `INSERT INTO projects (id, public_id, org_id, slug, name, issuer_handle, created_at, updated_at) VALUES ($1, $2, $3, $2, 'P', $4, now(), now())`,
		prj, "p"+prj.String()[:12], org, "h"+prj.String()[:8])
	require.NoError(t, err)
	return enduser.Project{OrgID: org, ID: prj, IssuerURL: "https://id.example.com/p/h", EmailLink: true}
}

func compose(_ context.Context, l enduser.IssuedLink) (mailer.Message, error) {
	return mailer.Message{To: l.Email, Subject: "Sign in", TextBody: l.URL}, nil
}

func (f *pgFixture) link(t *testing.T, p enduser.Project, email string) (token string, l *enduser.Link) {
	t.Helper()
	issued, err := f.svc.SendLink(t.Context(), p, enduser.LinkRequest{Email: email, ReturnTo: "/p/h/account/", IP: "2001:db8::/64"}, compose)
	require.NoError(t, err)
	require.True(t, issued.Issued)
	u, err := url.Parse(issued.URL)
	require.NoError(t, err)
	token = u.Query().Get("t")
	l, err = f.svc.LinkByToken(t.Context(), p, token)
	require.NoError(t, err)
	return token, l
}

func (f *pgFixture) count(t *testing.T, table string, p enduser.Project) int {
	t.Helper()
	var n int
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM `+table+` WHERE project_id = $1`, p.ID).Scan(&n))
	return n
}

func TestPostgres_SignInLifecycle(t *testing.T) {
	f := newPGFixture(t)
	ctx := t.Context()
	p := f.project(t)

	token, l := f.link(t, p, "Ada@Example.com")
	require.Len(t, f.mail.sent, 1)
	again, err := f.svc.SendLink(ctx, p, enduser.LinkRequest{Email: "ada@example.com", ReturnTo: "/p/h/account/"}, compose)
	require.NoError(t, err)
	assert.False(t, again.Issued, "the cooldown answers but sends nothing")
	assert.Len(t, f.mail.sent, 1)
	var hash, ip string
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT token_hash, host(request_ip) FROM magic_links WHERE public_id = $1`, l.PublicID).Scan(&hash, &ip))
	assert.NotContains(t, hash, token)
	assert.Equal(t, "2001:db8::", ip)

	r, err := f.svc.Redeem(ctx, p, enduser.RedeemRequest{Token: token, UserAgent: "test-agent"})
	require.NoError(t, err)
	assert.Equal(t, "ada@example.com", r.Account.Email)
	assert.Equal(t, "/p/h/account/", r.ReturnTo)
	assert.Len(t, r.Account.Subject, 21)
	var verified *time.Time
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT email_verified_at FROM end_users WHERE public_id = $1`, r.Account.Subject).Scan(&verified))
	require.NotNil(t, verified)

	repeat, err := f.svc.Redeem(ctx, p, enduser.RedeemRequest{Token: token, BrowserSecret: r.Secret})
	require.NoError(t, err, "a double click from the same browser succeeds")
	assert.Equal(t, r.Account.PublicID, repeat.Account.PublicID)
	_, err = f.svc.Redeem(ctx, p, enduser.RedeemRequest{Token: token})
	assert.True(t, enduser.IsLinkNotUsableError(err), "another browser can't reuse it: %v", err)

	f.now = f.now.Add(time.Second)
	bobToken, _ := f.link(t, p, "bob@example.com")
	bob, err := f.svc.Redeem(ctx, p, enduser.RedeemRequest{Token: bobToken, BrowserSecret: r.Secret})
	require.NoError(t, err)
	sess, err := f.svc.Session(ctx, p, bob.Secret)
	require.NoError(t, err)
	require.Len(t, sess.Accounts, 2)
	assert.Equal(t, "bob@example.com", sess.Active().Email)

	f.now = f.now.Add(time.Second)
	a, err := f.svc.Select(ctx, p, bob.Secret, r.Account.PublicID)
	require.NoError(t, err)
	assert.Equal(t, "ada@example.com", a.Email)
	sess, err = f.svc.Session(ctx, p, bob.Secret)
	require.NoError(t, err)
	assert.Equal(t, "ada@example.com", sess.Active().Email)

	f.now = f.now.Add(time.Second)
	carolToken, _ := f.link(t, p, "carol@example.com")
	carol, err := f.svc.Redeem(ctx, p, enduser.RedeemRequest{Token: carolToken, BrowserSecret: bob.Secret})
	require.NoError(t, err)
	sess, err = f.svc.Session(ctx, p, carol.Secret)
	require.NoError(t, err)
	require.Len(t, sess.Accounts, 2, "MaxAccounts 2 keeps the two most recently selected")
	_, ok := sess.Account(bob.Account.PublicID)
	assert.False(t, ok)

	p.TermsVersion, p.TermsURL = 1, "https://acme.test/terms"
	need, _, err := f.svc.NeedsTerms(ctx, p, *sess.Active())
	require.NoError(t, err)
	assert.True(t, need)
	_, err = f.svc.AcceptTerms(ctx, p, carol.Secret, carol.Account.PublicID, 1, "192.0.2.4", "ua")
	require.NoError(t, err)
	_, err = f.svc.AcceptTerms(ctx, p, carol.Secret, carol.Account.PublicID, 1, "not-an-ip", "ua")
	require.NoError(t, err, "accepting again changes nothing")
	need, _, err = f.svc.NeedsTerms(ctx, p, *sess.Active())
	require.NoError(t, err)
	assert.False(t, need)
	assert.Equal(t, 1, f.count(t, "end_user_terms", p))

	require.NoError(t, f.svc.SignOut(ctx, p, carol.Secret, carol.Account.PublicID))
	sess, err = f.svc.Session(ctx, p, carol.Secret)
	require.NoError(t, err)
	require.Len(t, sess.Accounts, 1)
	assert.Equal(t, "ada@example.com", sess.Active().Email)

	f.now = f.now.Add(31 * 24 * time.Hour)
	n, err := f.svc.Prune(ctx, p.OrgID, p.ID)
	require.NoError(t, err)
	assert.Positive(t, n)
	for _, table := range []string{"magic_links", "browser_sessions", "user_sessions"} {
		assert.Zero(t, f.count(t, table, p), table)
	}
	assert.Equal(t, 1, f.count(t, "end_user_terms", p), "consent outlives the session that gave it")
	var sessionID *uuid.UUID
	var sessionPublic string
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT user_session_id, user_session_public_id FROM end_user_terms WHERE project_id = $1`, p.ID).
		Scan(&sessionID, &sessionPublic))
	assert.Nil(t, sessionID, "the pruned session is gone")
	assert.Equal(t, carol.Account.PublicID, sessionPublic, "and the evidence still names it")
	assert.NotContains(t, f.logs.String(), token)
}

// SECURITY: H3, two redeems of one link at once sign in exactly once.
func TestPostgres_ConcurrentRedeemsWinOnce(t *testing.T) {
	f := newPGFixture(t)
	p := f.project(t)
	token, _ := f.link(t, p, "ada@example.com")
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Go(func() {
			_, errs[i] = f.svc.Redeem(t.Context(), p, enduser.RedeemRequest{Token: token})
		})
	}
	wg.Wait()
	wins := 0
	for _, err := range errs {
		if err == nil {
			wins++
			continue
		}
		assert.True(t, enduser.IsLinkNotUsableError(err), "%v", err)
	}
	assert.Equal(t, 1, wins)
	assert.Equal(t, 1, f.count(t, "user_sessions", p))
	assert.Equal(t, 1, f.count(t, "browser_sessions", p), "the losers' browser sessions rolled back")
}

// F7: two requests for one email at once issue one link; the loser answers like the cooldown.
func TestPostgres_ConcurrentSendsIssueOneLink(t *testing.T) {
	f := newPGFixture(t)
	p := f.project(t)
	var wg sync.WaitGroup
	issued := make([]bool, 4)
	errs := make([]error, 4)
	for i := range issued {
		wg.Go(func() {
			l, err := f.svc.SendLink(t.Context(), p, enduser.LinkRequest{Email: "ada@example.com", ReturnTo: "/p/h/account/"}, compose)
			issued[i], errs[i] = l.Issued, err
		})
	}
	wg.Wait()
	n := 0
	for i := range issued {
		require.NoError(t, errs[i])
		if issued[i] {
			n++
		}
	}
	assert.Equal(t, 1, n)
	assert.Equal(t, 1, f.count(t, "magic_links", p))
	assert.Len(t, f.mail.sent, 1)
}

func TestPostgres_ExpiryAndProjects(t *testing.T) {
	f := newPGFixture(t)
	ctx := t.Context()
	p, other := f.project(t), f.project(t)
	token, _ := f.link(t, p, "ada@example.com")

	_, err := f.svc.LinkByToken(ctx, other, token)
	assert.True(t, enduser.IsLinkNotUsableError(err), "another project never sees the link")
	_, err = f.svc.Redeem(ctx, other, enduser.RedeemRequest{Token: token})
	assert.True(t, enduser.IsLinkNotUsableError(err))

	f.now = f.now.Add(15 * time.Minute)
	_, err = f.svc.Redeem(ctx, p, enduser.RedeemRequest{Token: token})
	assert.True(t, enduser.IsLinkNotUsableError(err), "an expired link")
	assert.Zero(t, f.count(t, "end_users", p), "a refused redeem creates no end user")

	freshToken, _ := f.link(t, p, "ada@example.com")
	r, err := f.svc.Redeem(ctx, p, enduser.RedeemRequest{Token: freshToken})
	require.NoError(t, err)
	sess, err := f.svc.Session(ctx, other, r.Secret)
	require.NoError(t, err)
	assert.Nil(t, sess, "a cookie names no browser in another project")
	_, err = f.svc.Select(ctx, other, r.Secret, r.Account.PublicID)
	assert.True(t, enduser.IsSessionGoneError(err))

	oToken, _ := f.link(t, other, "ada@example.com")
	o, err := f.svc.Redeem(ctx, other, enduser.RedeemRequest{Token: oToken})
	require.NoError(t, err)
	assert.NotEqual(t, r.Account.Subject, o.Account.Subject, "the same email is another end user in another project")
}

func TestPostgres_EndUserAndEndSession(t *testing.T) {
	f := newPGFixture(t)
	ctx := t.Context()
	p := f.project(t)
	adaToken, _ := f.link(t, p, "ada@example.com")
	ada, err := f.svc.Redeem(ctx, p, enduser.RedeemRequest{Token: adaToken})
	require.NoError(t, err)
	f.now = f.now.Add(time.Second)
	bobToken, _ := f.link(t, p, "bob@example.com")
	bob, err := f.svc.Redeem(ctx, p, enduser.RedeemRequest{Token: bobToken, BrowserSecret: ada.Secret})
	require.NoError(t, err)

	u, err := f.svc.EndUser(ctx, p, ada.Account.EndUserID)
	require.NoError(t, err)
	require.NotNil(t, u)
	assert.Equal(t, ada.Account.Subject, u.Subject)
	assert.Equal(t, "ada@example.com", u.Email)
	assert.True(t, u.EmailVerified)
	other := f.project(t)
	gone, err := f.svc.EndUser(ctx, other, ada.Account.EndUserID)
	require.NoError(t, err)
	assert.Nil(t, gone, "an end user is only found in its project")

	require.NoError(t, f.svc.EndSession(ctx, p, ada.Account.PublicID))
	sess, err := f.svc.Session(ctx, p, bob.Secret)
	require.NoError(t, err)
	require.Len(t, sess.Accounts, 1, "only the named session ends")
	assert.Equal(t, "bob@example.com", sess.Active().Email)
}
