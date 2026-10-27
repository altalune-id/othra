//go:build integration

package federation_test

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/enduser"
	"altalune.id/othra/internal/federation"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/testutil/pgtest"
	"altalune.id/othra/internal/upstream"
)

type endUsers struct{ svc *enduser.Service }

func eup(p federation.Project) enduser.Project {
	return enduser.Project{OrgID: p.OrgID, ID: p.ID, PublicID: p.PublicID}
}

func (u endUsers) EndUserIDByEmail(ctx context.Context, p federation.Project, email string) (uuid.UUID, bool, error) {
	return u.svc.EndUserIDByEmail(ctx, eup(p), email)
}

func (u endUsers) Create(ctx context.Context, p federation.Project, email, name string) (uuid.UUID, error) {
	id, err := u.svc.CreateFederated(ctx, eup(p), email, name)
	if errors.Is(err, enduser.ErrEmailTaken) {
		return uuid.Nil, federation.ErrEmailTaken
	}
	return id, err
}

func (u endUsers) SignIn(ctx context.Context, p federation.Project, req federation.SessionRequest) (*federation.Session, error) {
	got, err := u.svc.SignInFederated(ctx, eup(p), enduser.FederatedSignIn{
		EndUserID: req.EndUserID, Method: req.Method, BrowserSecret: req.BrowserSecret, UserAgent: req.UserAgent,
	})
	if err != nil {
		return nil, err
	}
	return &federation.Session{BrowserSecret: got.Secret, EndUserID: got.Account.EndUserID, UserSession: got.Account.PublicID}, nil
}

type upstreams struct{ id uuid.UUID }

func (u upstreams) Offered(context.Context, uuid.UUID, uuid.UUID) ([]upstream.Kind, error) {
	return []upstream.Kind{upstream.KindGoogle}, nil
}

func (u upstreams) Credentials(context.Context, uuid.UUID, uuid.UUID, upstream.Kind) (*upstream.Credentials, error) {
	return &upstream.Credentials{Upstream: upstream.Upstream{ID: u.id, Kind: upstream.KindGoogle, ClientID: "cid", Enabled: true}}, nil
}

type fixture struct {
	db    *sql.DB
	svc   *federation.Service
	users *enduser.Service
	p     federation.Project
	up    uuid.UUID
	with  func(federation.EndUsers) *federation.Service
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	pool, sqlDB, schemaName := pgtest.MigratedRLS(t)
	f := &fixture{db: sqlDB}
	f.p, f.up = seedProject(t, sqlDB)
	unexpected := apperror.NewReporter(nil, false).Unexpected
	cfg := enduser.Config{LinkTTL: 15 * time.Minute, ResendAfter: time.Minute, Idle: 24 * time.Hour, Absolute: 48 * time.Hour, MaxAccounts: 5}
	uow := db.NewTransactor(pool)
	f.users = enduser.NewService(enduser.NewStore(pool, schemaName, ""), uow, nil, cfg, unexpected)
	projects := project.NewService(project.NewStore(pool, schemaName, ""), unexpected)
	f.with = func(users federation.EndUsers) *federation.Service {
		return federation.NewService(federation.NewStore(pool, schemaName, ""), uow, upstreams{id: f.up}, users, projects, federation.Config{},
			unexpected)
	}
	f.svc = f.with(endUsers{svc: f.users})
	return f
}

func seedProject(t *testing.T, sqlDB *sql.DB) (federation.Project, uuid.UUID) {
	t.Helper()
	p := federation.Project{OrgID: uuid.New(), ID: uuid.New()}
	p.PublicID = "p" + p.ID.String()[:12]
	up := uuid.New()
	exec := func(query string, args ...any) {
		_, err := sqlDB.ExecContext(t.Context(), query, args...)
		require.NoError(t, err)
	}
	exec(`INSERT INTO orgs (id, public_id, slug, name, created_at, updated_at) VALUES ($1, $2, $2, 'Org', now(), now())`,
		p.OrgID, "o"+p.OrgID.String()[:12])
	exec(`INSERT INTO projects (id, public_id, org_id, slug, name, issuer_handle, created_at, updated_at) VALUES ($1, $2, $3, $2, 'P', $4, now(), now())`,
		p.ID, p.PublicID, p.OrgID, "h"+p.ID.String()[:8])
	exec(`INSERT INTO upstream_providers (id, public_id, org_id, project_id, kind, client_id, client_secret_sealed, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'google', 'cid', decode(repeat('ab', 40), 'hex'), now(), now())`, up, "u"+up.String()[:12], p.OrgID, p.ID)
	return p, up
}

func (f *fixture) identity(sub, email string) federation.Identity {
	return federation.Identity{
		UpstreamID: f.up, Issuer: "https://accounts.google.com", Subject: sub, Email: email, EmailVerified: true, Name: "Ana",
		Kind: upstream.KindGoogle,
	}
}

func (f *fixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, f.db.QueryRowContext(t.Context(), query, args...).Scan(&n))
	return n
}

func TestPostgres_SignInCreatesThenFinds(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	first, err := f.svc.SignIn(ctx, f.p, f.identity("sub-1", "ana@gmail.com"), federation.SessionRequest{UserAgent: "UA"})
	require.NoError(t, err)
	assert.NotEmpty(t, first.BrowserSecret)
	assert.Equal(t, "google", func() string {
		var m string
		require.NoError(t, f.db.QueryRowContext(ctx, `SELECT method FROM user_sessions WHERE public_id = $1`, first.UserSession).Scan(&m))
		return m
	}())
	assert.Equal(t, 1, f.count(t, `SELECT count(*) FROM end_users WHERE project_id = $1 AND name = 'Ana'`, f.p.ID))

	again, err := f.svc.SignIn(ctx, f.p, f.identity("sub-1", "ana.renamed@gmail.com"), federation.SessionRequest{BrowserSecret: first.BrowserSecret})
	require.NoError(t, err)
	assert.Equal(t, first.EndUserID, again.EndUserID, "the (iss, sub) is the key, not the email")
	assert.Equal(t, 1, f.count(t, `SELECT count(*) FROM end_user_identities WHERE project_id = $1 AND email = 'ana.renamed@gmail.com'`, f.p.ID))

	ids, err := f.svc.Identities(ctx, f.p, first.EndUserID)
	require.NoError(t, err)
	require.Len(t, ids, 1)
	assert.Equal(t, upstream.KindGoogle, ids[0].Kind)
	assert.NotEmpty(t, ids[0].PublicID)
	assert.False(t, ids[0].LinkedAt.IsZero())
}

func TestPostgres_CollisionLinksNothing(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	_, err := f.users.CreateFederated(ctx, eup(f.p), "ana@gmail.com", "")
	require.NoError(t, err)
	_, err = f.svc.SignIn(ctx, f.p, f.identity("sub-1", "ana@gmail.com"), federation.SessionRequest{})
	assert.True(t, federation.IsCollisionError(err))
	assert.Equal(t, 0, f.count(t, `SELECT count(*) FROM end_user_identities WHERE project_id = $1`, f.p.ID))
	assert.Equal(t, 0, f.count(t, `SELECT count(*) FROM user_sessions WHERE project_id = $1`, f.p.ID))
}

func TestPostgres_LinkUnlinkAndGuard(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	ana, err := f.users.CreateFederated(ctx, eup(f.p), "ana@acme.test", "")
	require.NoError(t, err)
	budi, err := f.users.CreateFederated(ctx, eup(f.p), "budi@acme.test", "")
	require.NoError(t, err)

	all, err := f.svc.EveryEndUserLinked(ctx, f.p.OrgID, f.p.ID)
	require.NoError(t, err)
	assert.False(t, all)

	require.NoError(t, f.svc.Link(ctx, f.p, ana, f.identity("sub-a", "ana@gmail.com")))
	require.NoError(t, f.svc.Link(ctx, f.p, ana, f.identity("sub-a", "ana@gmail.com")))
	err = f.svc.Link(ctx, f.p, budi, f.identity("sub-a", "ana@gmail.com"))
	assert.Equal(t, apperror.CodeSignInUpstreamLinkedElsewhere, codeOf(err))
	err = f.svc.Link(ctx, f.p, ana, f.identity("sub-a2", "ana2@gmail.com"))
	assert.Equal(t, apperror.CodeSignInUpstreamLinkedElsewhere, codeOf(err), "the one-per-upstream constraint")

	require.NoError(t, f.svc.Link(ctx, f.p, budi, f.identity("sub-b", "budi@gmail.com")))
	all, err = f.svc.EveryEndUserLinked(ctx, f.p.OrgID, f.p.ID)
	require.NoError(t, err)
	assert.True(t, all)

	ids, err := f.svc.Identities(ctx, f.p, ana)
	require.NoError(t, err)
	require.Len(t, ids, 1)
	err = f.svc.Unlink(ctx, f.p, budi, ids[0].PublicID)
	assert.Equal(t, apperror.CodeNotFound, codeOf(err), "another end user's identity")
	require.NoError(t, f.svc.Unlink(ctx, f.p, ana, ids[0].PublicID), "the email link is on, so Google is not the last method")
	assert.Equal(t, 1, f.count(t, `SELECT count(*) FROM end_user_identities WHERE project_id = $1`, f.p.ID))
}

func TestPostgres_OtherProjectSeesNothing(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	got, err := f.svc.SignIn(ctx, f.p, f.identity("sub-1", "ana@gmail.com"), federation.SessionRequest{})
	require.NoError(t, err)
	other, _ := seedProject(t, f.db)
	ids, err := f.svc.Identities(ctx, other, got.EndUserID)
	require.NoError(t, err)
	assert.Empty(t, ids)
}

// SIG019: turning the email link off and unlinking the last Google can't both pass: Unlink waits on the project row.
func TestPostgres_UnlinkWaitsForTheEmailLinkSwitch(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	ana, err := f.users.CreateFederated(ctx, eup(f.p), "ana@acme.test", "")
	require.NoError(t, err)
	require.NoError(t, f.svc.Link(ctx, f.p, ana, f.identity("sub-a", "ana@gmail.com")))
	ids, err := f.svc.Identities(ctx, f.p, ana)
	require.NoError(t, err)

	off, err := f.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = off.Rollback() }()
	_, err = off.ExecContext(ctx, `SELECT 1 FROM projects WHERE id = $1 FOR UPDATE`, f.p.ID)
	require.NoError(t, err)
	_, err = off.ExecContext(ctx, `UPDATE projects SET signin_email_link = false WHERE id = $1`, f.p.ID)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() { done <- f.svc.Unlink(ctx, f.p, ana, ids[0].PublicID) }()
	f.waitForLock(t, done, "projects")
	require.NoError(t, off.Commit())
	err = <-done
	assert.Equal(t, apperror.CodeSignInLastMethod, codeOf(err), "it reads the switch the other transaction committed")
	assert.Equal(t, 1, f.count(t, `SELECT count(*) FROM end_user_identities WHERE project_id = $1`, f.p.ID))
}

func (f *fixture) waitForLock(t *testing.T, done chan error, table string) {
	t.Helper()
	require.Eventually(t, func() bool {
		select {
		case err := <-done:
			done <- err
			return true
		default:
		}
		var waiting int
		err := f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND query LIKE '%'||$1||'%'`,
			table).Scan(&waiting)
		return err == nil && waiting > 0
	}, 10*time.Second, 20*time.Millisecond)
	select {
	case err := <-done:
		t.Fatalf("finished without waiting on the other transaction: %v", err)
	default:
	}
}

// A21: two first sign-ins with one new Google account: the loser waits on the winner, then signs in as the same end user.
func TestPostgres_SameNewAccountRace(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	id := f.identity("sub-race", "race@gmail.com")
	winner, idn := uuid.New(), uuid.New()
	tx, err := f.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `INSERT INTO end_users (id, public_id, org_id, project_id, email, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, now(), now())`, winner, "e"+winner.String()[:12], f.p.OrgID, f.p.ID, id.Email)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO end_user_identities (id, public_id, org_id, project_id, end_user_id, upstream_provider_id,
		issuer, subject, email, email_verified, linked_at, last_used_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, true, now(), now())`,
		idn, "i"+idn.String()[:12], f.p.OrgID, f.p.ID, winner, f.up, id.Issuer, id.Subject, id.Email)
	require.NoError(t, err)

	type result struct {
		s   *federation.Session
		err error
	}
	got := make(chan result, 1)
	done := make(chan error, 1)
	go func() {
		s, err := f.svc.SignIn(ctx, f.p, id, federation.SessionRequest{})
		got <- result{s, err}
		done <- err
	}()
	f.waitForLock(t, done, "end_users")
	require.NoError(t, tx.Commit())
	r := <-got
	require.NoError(t, r.err, "never the collision page for the person's own new account")
	assert.Equal(t, winner, r.s.EndUserID)
	assert.Equal(t, 1, f.count(t, `SELECT count(*) FROM end_users WHERE project_id = $1`, f.p.ID))
	assert.Equal(t, 1, f.count(t, `SELECT count(*) FROM end_user_identities WHERE project_id = $1`, f.p.ID))
}

// A21: many first sign-ins with one new Google account at once: one end user, one identity, and every one signs in.
func TestPostgres_SameNewAccountConcurrent(t *testing.T) {
	f := newFixture(t)
	id := f.identity("sub-many", "many@gmail.com")
	start := make(chan struct{})
	results := make(chan *federation.Session, 4)
	errs := make(chan error, 4)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			<-start
			s, err := f.svc.SignIn(t.Context(), f.p, id, federation.SessionRequest{})
			errs <- err
			results <- s
		})
	}
	close(start)
	wg.Wait()
	close(errs)
	close(results)
	for err := range errs {
		require.NoError(t, err)
	}
	var first uuid.UUID
	for s := range results {
		if first == uuid.Nil {
			first = s.EndUserID
		}
		assert.Equal(t, first, s.EndUserID)
	}
	assert.Equal(t, 1, f.count(t, `SELECT count(*) FROM end_users WHERE project_id = $1`, f.p.ID))
	assert.Equal(t, 1, f.count(t, `SELECT count(*) FROM end_user_identities WHERE project_id = $1`, f.p.ID))
	assert.Equal(t, 4, f.count(t, `SELECT count(*) FROM user_sessions WHERE project_id = $1`, f.p.ID))
}

// A21, ADR-0014: a Google sign-in racing a magic-link sign-up of the same email never links: it is a collision.
func TestPostgres_SignInVersusMagicLinkSignUp(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	id := f.identity("sub-g", "dina@acme.test")
	id.HostedDomain = "acme.test"
	signUp := uuid.New()
	tx, err := f.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `INSERT INTO end_users (id, public_id, org_id, project_id, email, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, now(), now())`, signUp, "e"+signUp.String()[:12], f.p.OrgID, f.p.ID, id.Email)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		_, err := f.svc.SignIn(ctx, f.p, id, federation.SessionRequest{})
		done <- err
	}()
	f.waitForLock(t, done, "end_users")
	require.NoError(t, tx.Commit())
	err = <-done
	assert.True(t, federation.IsCollisionError(err), "%v", err)
	assert.Equal(t, 0, f.count(t, `SELECT count(*) FROM end_user_identities WHERE project_id = $1`, f.p.ID), "nothing linked")
	assert.Equal(t, 0, f.count(t, `SELECT count(*) FROM user_sessions WHERE project_id = $1`, f.p.ID), "nobody signed in")
	assert.Equal(t, 1, f.count(t, `SELECT count(*) FROM end_users WHERE project_id = $1`, f.p.ID))
}

// A sign-in on an instance whose clock is behind the one that linked the identity still signs in.
func TestPostgres_TouchNeverMovesLastUsedBack(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	at := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
	f.svc.WithClock(func() time.Time { return at })
	first, err := f.svc.SignIn(ctx, f.p, f.identity("sub-clock", "clock@gmail.com"), federation.SessionRequest{})
	require.NoError(t, err)
	f.svc.WithClock(func() time.Time { return at.Add(-time.Minute) })
	again, err := f.svc.SignIn(ctx, f.p, f.identity("sub-clock", "clock@gmail.com"), federation.SessionRequest{})
	require.NoError(t, err)
	assert.Equal(t, first.EndUserID, again.EndUserID)
	assert.Equal(t, 1, f.count(t, `SELECT count(*) FROM end_user_identities WHERE project_id = $1 AND last_used_at = linked_at`, f.p.ID))
}

type commitBeforeLookup struct {
	endUsers
	commit func()
}

func (u commitBeforeLookup) EndUserIDByEmail(ctx context.Context, p federation.Project, email string) (uuid.UUID, bool, error) {
	u.commit()
	return u.endUsers.EndUserIDByEmail(ctx, p, email)
}

// A21: the race winner commits after the loser's (iss, sub) read and before its email read: the loser still signs in as the winner.
func TestPostgres_SameNewAccountCommitsBetweenLookups(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	id := f.identity("sub-between", "between@gmail.com")
	winner, idn := uuid.New(), uuid.New()
	tx, err := f.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `INSERT INTO end_users (id, public_id, org_id, project_id, email, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, now(), now())`, winner, "e"+winner.String()[:12], f.p.OrgID, f.p.ID, id.Email)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO end_user_identities (id, public_id, org_id, project_id, end_user_id, upstream_provider_id,
		issuer, subject, email, email_verified, linked_at, last_used_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, true, now(), now())`,
		idn, "i"+idn.String()[:12], f.p.OrgID, f.p.ID, winner, f.up, id.Issuer, id.Subject, id.Email)
	require.NoError(t, err)

	svc := f.with(commitBeforeLookup{endUsers: endUsers{svc: f.users}, commit: func() { require.NoError(t, tx.Commit()) }})
	got, err := svc.SignIn(ctx, f.p, id, federation.SessionRequest{})
	require.NoError(t, err, "never a collision with the person's own account")
	assert.Equal(t, winner, got.EndUserID)
	assert.Equal(t, 1, f.count(t, `SELECT count(*) FROM end_users WHERE project_id = $1`, f.p.ID))
	assert.Equal(t, 1, f.count(t, `SELECT count(*) FROM end_user_identities WHERE project_id = $1`, f.p.ID))
}

func codeOf(err error) apperror.Code {
	if app, ok := apperror.AsAppError(err); ok {
		return app.Code()
	}
	return ""
}
