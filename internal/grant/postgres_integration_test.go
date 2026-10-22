//go:build integration

package grant_test

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/grant"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/testutil/pgtest"
)

const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

type pgFixture struct {
	db   *sql.DB
	svc  *grant.Service
	now  time.Time
	logs *bytes.Buffer
	cfg  grant.Config
}

type seeded struct {
	p                      grant.Project
	client                 grant.Client
	other                  grant.Client
	sub, userSession       string
	endUser, userSessionID uuid.UUID
}

func newPGFixture(t *testing.T) *pgFixture {
	t.Helper()
	pool, sqlDB, schemaName := pgtest.MigratedRLS(t)
	f := &pgFixture{
		db: sqlDB, now: time.Now().UTC().Truncate(time.Microsecond), logs: &bytes.Buffer{},
		cfg: grant.Config{CodeTTL: time.Minute, AccessTokenTTL: 15 * time.Minute, RefreshIdle: 24 * time.Hour, RefreshAbsolute: 72 * time.Hour},
	}
	log := slog.New(slog.NewJSONHandler(f.logs, nil))
	reporter := apperror.NewReporter(log, false)
	f.svc = grant.NewService(grant.NewStore(pool, schemaName, ""), db.NewTransactor(pool), f.cfg, reporter.Unexpected).
		WithLogger(log).WithClock(func() time.Time { return f.now })
	return f
}

func (f *pgFixture) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	_, err := f.db.ExecContext(t.Context(), query, args...)
	require.NoError(t, err)
}

func (f *pgFixture) seed(t *testing.T) seeded {
	t.Helper()
	s := seeded{
		p:      grant.Project{OrgID: uuid.New(), ID: uuid.New()},
		client: grant.Client{ID: uuid.New(), PublicID: "c" + uuid.NewString()[:12], Refresh: true},
		other:  grant.Client{ID: uuid.New(), PublicID: "c" + uuid.NewString()[:12], Refresh: true},
		sub:    "e" + uuid.NewString()[:12], userSession: "s" + uuid.NewString()[:12], endUser: uuid.New(), userSessionID: uuid.New(),
	}
	browser := uuid.New()
	f.exec(t, `INSERT INTO orgs (id, public_id, slug, name, created_at, updated_at) VALUES ($1, $2, $2, 'Org', now(), now())`,
		s.p.OrgID, "o"+s.p.OrgID.String()[:12])
	f.exec(t, `INSERT INTO projects (id, public_id, org_id, slug, name, issuer_handle, created_at, updated_at) VALUES ($1, $2, $3, $2, 'P', $4, now(), now())`,
		s.p.ID, "p"+s.p.ID.String()[:12], s.p.OrgID, "h"+s.p.ID.String()[:8])
	for _, c := range []grant.Client{s.client, s.other} {
		f.exec(t, `INSERT INTO oauth_clients (id, public_id, org_id, project_id, name, client_type, application_type, token_endpoint_auth_method, grant_types, allowed_scopes, redirect_uris, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'Web', 'confidential', 'web', 'client_secret_basic', '{authorization_code,refresh_token}', '{openid,offline_access}', '{https://a.example/cb}', now(), now())`,
			c.ID, c.PublicID, s.p.OrgID, s.p.ID)
	}
	f.exec(t, `INSERT INTO end_users (id, public_id, org_id, project_id, email, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, now(), now())`,
		s.endUser, s.sub, s.p.OrgID, s.p.ID, "eu"+uuid.NewString()[:8]+"@example.com")
	f.exec(t, `INSERT INTO browser_sessions (id, org_id, project_id, secret_hash, created_at, last_seen_at, absolute_expires_at)
		VALUES ($1, $2, $3, encode(sha256(convert_to($4, 'UTF8')), 'hex'), now(), now(), now() + interval '1 day')`,
		browser, s.p.OrgID, s.p.ID, uuid.NewString())
	f.exec(t, `INSERT INTO user_sessions (id, public_id, org_id, project_id, browser_session_id, end_user_id, method, auth_time, selected_at, created_at, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'magic_link', now(), now(), now(), now())`,
		s.userSessionID, s.userSession, s.p.OrgID, s.p.ID, browser, s.endUser)
	return s
}

func challenge(v string) string {
	sum := sha256.Sum256([]byte(v))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (s seeded) codeRequest(scopes ...string) grant.CodeRequest {
	return grant.CodeRequest{
		Client: s.client.PublicID, RedirectURI: "https://a.example/cb", Scopes: scopes, CodeChallenge: challenge(verifier),
		Nonce: "n-1", UserSession: s.userSession, EndUser: s.sub, AuthTime: time.Now().Add(-time.Minute),
	}
}

func (f *pgFixture) code(t *testing.T, s seeded, scopes ...string) string {
	t.Helper()
	code, err := f.svc.IssueCode(t.Context(), s.p, s.codeRequest(scopes...))
	require.NoError(t, err)
	return code
}

func (f *pgFixture) exchange(t *testing.T, s seeded, scopes ...string) *grant.Tokens {
	t.Helper()
	got, err := f.svc.Exchange(t.Context(), s.p, grant.ExchangeRequest{
		Client: s.client, Code: f.code(t, s, scopes...), RedirectURI: "https://a.example/cb", CodeVerifier: verifier,
	})
	require.NoError(t, err)
	return got
}

func (f *pgFixture) reason(t *testing.T, sid string) string {
	t.Helper()
	var reason sql.NullString
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT revoked_reason FROM grant_families WHERE public_id = $1`, sid).Scan(&reason))
	return reason.String
}

// SECURITY: OAuth 2.1 §4.1.3, a code works once, for its client, redirect URI and PKCE verifier; a replay revokes what it started.
func TestPostgres_CodeExchangeAndReplay(t *testing.T) {
	f := newPGFixture(t)
	s := f.seed(t)
	ctx := t.Context()
	code := f.code(t, s, "openid", "offline_access")
	var stored string
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT code_hash FROM authorization_codes WHERE project_id = $1`, s.p.ID).Scan(&stored))
	assert.NotContains(t, stored, code, "only the hash is stored")

	for name, req := range map[string]grant.ExchangeRequest{
		"another client":       {Client: s.other, Code: code, CodeVerifier: verifier},
		"another redirect uri": {Client: s.client, Code: code, RedirectURI: "https://a.example/other", CodeVerifier: verifier},
		"no verifier":          {Client: s.client, Code: code},
		"a wrong verifier":     {Client: s.client, Code: code, CodeVerifier: strings.Repeat("a", 43)},
		"an unknown code":      {Client: s.client, Code: "nope", CodeVerifier: verifier},
	} {
		_, err := f.svc.Exchange(ctx, s.p, req)
		assert.True(t, grant.IsInvalidGrantError(err), "%s: %v", name, err)
	}

	got, err := f.svc.Exchange(ctx, s.p, grant.ExchangeRequest{Client: s.client, Code: code, CodeVerifier: verifier})
	require.NoError(t, err, "redirect_uri may be left out (S8), and a refused attempt did not spend the code")
	assert.Equal(t, "n-1", got.Nonce)
	assert.Equal(t, []string{"openid", "offline_access"}, got.Scopes)
	assert.NotEmpty(t, got.RefreshToken, "offline_access with the refresh_token grant")
	assert.Equal(t, s.userSession, got.Family.UserSession)
	assert.Len(t, got.Family.PublicID, 21)
	assert.Equal(t, f.now.Add(15*time.Minute), got.ExpiresAt)

	a, err := f.svc.Access(ctx, s.p, got.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, s.endUser, a.Family.EndUserID)

	for name, req := range map[string]grant.ExchangeRequest{
		"another client":   {Client: s.other, Code: code, CodeVerifier: verifier},
		"a wrong verifier": {Client: s.client, Code: code, CodeVerifier: strings.Repeat("a", 43)},
	} {
		_, err = f.svc.Exchange(ctx, s.p, req)
		assert.True(t, grant.IsInvalidGrantError(err), name)
		assert.Empty(t, f.reason(t, got.Family.PublicID), "m5-spec T7: %s replaying a stolen code can't revoke the grant", name)
	}
	_, err = f.svc.Exchange(ctx, s.p, grant.ExchangeRequest{Client: s.client, Code: code, CodeVerifier: verifier})
	assert.True(t, grant.IsInvalidGrantError(err))
	assert.Equal(t, grant.ReasonCodeReplay, f.reason(t, got.Family.PublicID), "the code's own client replaying it revokes the family")
	_, err = f.svc.Access(ctx, s.p, got.AccessToken)
	assert.True(t, grant.IsInvalidTokenError(err), "a replay ends the tokens the code issued")
	_, err = f.svc.Refresh(ctx, s.p, grant.RefreshRequest{Client: s.client, RefreshToken: got.RefreshToken})
	assert.True(t, grant.IsInvalidGrantError(err))
	assert.Contains(t, f.logs.String(), got.Family.PublicID)
	assert.NotContains(t, f.logs.String(), code)
	assert.NotContains(t, f.logs.String(), got.AccessToken)
}

func TestPostgres_CodeLimits(t *testing.T) {
	f := newPGFixture(t)
	s := f.seed(t)
	ctx := t.Context()

	code := f.code(t, s, "openid")
	f.now = f.now.Add(time.Minute)
	_, err := f.svc.Exchange(ctx, s.p, grant.ExchangeRequest{Client: s.client, Code: code, CodeVerifier: verifier})
	assert.True(t, grant.IsInvalidGrantError(err), "a code lives for codeTTL")

	req := s.codeRequest("openid")
	req.CodeChallenge = ""
	code, err = f.svc.IssueCode(ctx, s.p, req)
	require.NoError(t, err, "a confidential client may turn PKCE off")
	_, err = f.svc.Exchange(ctx, s.p, grant.ExchangeRequest{Client: s.client, Code: code, CodeVerifier: verifier})
	assert.True(t, grant.IsInvalidRequestError(err), "RFC 9700 §2.1.1: a verifier for a code without a challenge is invalid_request")
	got, err := f.svc.Exchange(ctx, s.p, grant.ExchangeRequest{Client: s.client, Code: code})
	require.NoError(t, err)
	assert.Empty(t, got.RefreshToken, "no offline_access, no refresh token")

	noRefresh := s.client
	noRefresh.Refresh = false
	got, err = f.svc.Exchange(ctx, s.p, grant.ExchangeRequest{Client: noRefresh, Code: f.code(t, s, "openid", "offline_access"), CodeVerifier: verifier})
	require.NoError(t, err)
	assert.Empty(t, got.RefreshToken, "a client without the refresh_token grant gets none")

	for name, mutate := range map[string]func(r *grant.CodeRequest){
		"an unknown client":  func(r *grant.CodeRequest) { r.Client = "nope" },
		"another end user":   func(r *grant.CodeRequest) { r.EndUser = "nope" },
		"an unknown session": func(r *grant.CodeRequest) { r.UserSession = "nope" },
	} {
		r := s.codeRequest("openid")
		mutate(&r)
		_, err := f.svc.IssueCode(ctx, s.p, r)
		assert.True(t, grant.IsSessionGoneError(err), "%s: %v", name, err)
	}
	f.exec(t, `UPDATE user_sessions SET ended_at = now() WHERE id = $1`, s.userSessionID)
	_, err = f.svc.IssueCode(ctx, s.p, s.codeRequest("openid"))
	assert.True(t, grant.IsSessionGoneError(err), "a signed-out session gets no code")
}

// SECURITY: RFC 9700 §4.14.2, refresh tokens rotate on every use and a reused one revokes the family.
func TestPostgres_RefreshRotation(t *testing.T) {
	f := newPGFixture(t)
	s := f.seed(t)
	ctx := t.Context()
	first := f.exchange(t, s, "openid", "offline_access")

	f.now = f.now.Add(time.Hour)
	second, err := f.svc.Refresh(ctx, s.p, grant.RefreshRequest{Client: s.client, RefreshToken: first.RefreshToken, Scopes: []string{"openid"}})
	require.NoError(t, err)
	assert.NotEqual(t, first.RefreshToken, second.RefreshToken)
	assert.Equal(t, []string{"openid"}, second.Scopes, "a narrowed scope narrows this access token")
	assert.Equal(t, []string{"openid", "offline_access"}, second.Family.Scopes, "the grant keeps its scope (RFC 6749 §6)")
	assert.Empty(t, second.Nonce)
	assert.Equal(t, first.Family.PublicID, second.Family.PublicID)

	_, err = f.svc.Refresh(ctx, s.p, grant.RefreshRequest{Client: s.client, RefreshToken: second.RefreshToken, Scopes: []string{"email"}})
	assert.True(t, grant.IsInvalidScopeError(err))
	third, err := f.svc.Refresh(ctx, s.p, grant.RefreshRequest{Client: s.client, RefreshToken: second.RefreshToken})
	require.NoError(t, err, "a refused scope spent nothing")
	assert.Equal(t, []string{"openid", "offline_access"}, third.Scopes)

	_, err = f.svc.Refresh(ctx, s.p, grant.RefreshRequest{Client: s.client, RefreshToken: first.RefreshToken})
	assert.True(t, grant.IsInvalidGrantError(err))
	assert.Equal(t, grant.ReasonReuse, f.reason(t, first.Family.PublicID))
	_, err = f.svc.Refresh(ctx, s.p, grant.RefreshRequest{Client: s.client, RefreshToken: third.RefreshToken})
	assert.True(t, grant.IsInvalidGrantError(err), "the whole family is gone")
	_, err = f.svc.Access(ctx, s.p, third.AccessToken)
	assert.True(t, grant.IsInvalidTokenError(err))
	_, err = f.svc.Refresh(ctx, s.p, grant.RefreshRequest{Client: s.client, RefreshToken: first.RefreshToken})
	assert.True(t, grant.IsInvalidGrantError(err), "a second reuse of a revoked family stays refused")
}

func TestPostgres_RefreshLimits(t *testing.T) {
	f := newPGFixture(t)
	s := f.seed(t)
	ctx := t.Context()

	stolen := f.exchange(t, s, "openid", "offline_access")
	_, err := f.svc.Refresh(ctx, s.p, grant.RefreshRequest{Client: s.other, RefreshToken: stolen.RefreshToken})
	assert.True(t, grant.IsInvalidGrantError(err))
	assert.Equal(t, grant.ReasonClientMismatch, f.reason(t, stolen.Family.PublicID), "another client presenting it revokes the family")

	idle := f.exchange(t, s, "openid", "offline_access")
	f.now = f.now.Add(f.cfg.RefreshIdle)
	_, err = f.svc.Refresh(ctx, s.p, grant.RefreshRequest{Client: s.client, RefreshToken: idle.RefreshToken})
	assert.True(t, grant.IsInvalidGrantError(err), "a refresh token unused for the idle timeout expires")

	live := f.exchange(t, s, "openid", "offline_access")
	token := live.RefreshToken
	for range 4 {
		f.now = f.now.Add(f.cfg.RefreshIdle - time.Hour)
		next, err := f.svc.Refresh(ctx, s.p, grant.RefreshRequest{Client: s.client, RefreshToken: token})
		if err != nil {
			assert.True(t, grant.IsInvalidGrantError(err), "the absolute lifetime ends the family: %v", err)
			return
		}
		token = next.RefreshToken
	}
	t.Fatal("a family outlived its absolute lifetime")
}

func TestPostgres_RevocationAndLookup(t *testing.T) {
	f := newPGFixture(t)
	s := f.seed(t)
	ctx := t.Context()
	got := f.exchange(t, s, "openid", "offline_access")

	fam, err := f.svc.Family(ctx, s.p, got.Family.PublicID)
	require.NoError(t, err)
	require.NotNil(t, fam)
	assert.Equal(t, s.userSession, fam.UserSession)
	missing, err := f.svc.Family(ctx, s.p, "nope")
	require.NoError(t, err)
	assert.Nil(t, missing)

	anyFam, err := f.svc.AnyFamily(ctx, s.p, got.Family.PublicID)
	require.NoError(t, err)
	require.NotNil(t, anyFam)
	revoked, err := f.svc.RevokeFamily(ctx, s.p, got.Family.PublicID, grant.ReasonEndSession)
	require.NoError(t, err)
	require.NotNil(t, revoked)
	assert.Equal(t, s.userSession, revoked.UserSession)
	assert.Equal(t, grant.ReasonEndSession, f.reason(t, got.Family.PublicID))
	again, err := f.svc.RevokeFamily(ctx, s.p, got.Family.PublicID, grant.ReasonSignOut)
	require.NoError(t, err)
	assert.Nil(t, again, "revoking twice is no error and keeps the first reason")
	assert.Equal(t, grant.ReasonEndSession, f.reason(t, got.Family.PublicID))
	fam, err = f.svc.Family(ctx, s.p, got.Family.PublicID)
	require.NoError(t, err)
	assert.Nil(t, fam, "a revoked family is not live")
	anyFam, err = f.svc.AnyFamily(ctx, s.p, got.Family.PublicID)
	require.NoError(t, err)
	require.NotNil(t, anyFam, "end-session still finds a revoked family's user session")
	assert.Equal(t, s.userSession, anyFam.UserSession)
	assert.NotNil(t, anyFam.RevokedAt)
	anyFam, err = f.svc.AnyFamily(ctx, s.p, "nope")
	require.NoError(t, err)
	assert.Nil(t, anyFam)

	kept := f.exchange(t, s, "openid")
	offline := f.exchange(t, s, "openid", "offline_access")
	pending := f.code(t, s, "openid")
	require.NoError(t, f.svc.RevokeUserSession(ctx, s.p, s.userSession, grant.ReasonSignOut))
	assert.Equal(t, grant.ReasonSignOut, f.reason(t, kept.Family.PublicID))
	assert.Empty(t, f.reason(t, offline.Family.PublicID), "OIDC Core §11: an offline_access grant outlives the sign-in session")
	rotated, err := f.svc.Refresh(ctx, s.p, grant.RefreshRequest{Client: s.client, RefreshToken: offline.RefreshToken})
	require.NoError(t, err, "its refresh token still works after sign-out")
	_, err = f.svc.RevokeFamily(ctx, s.p, offline.Family.PublicID, grant.ReasonEndSession)
	require.NoError(t, err)
	assert.Equal(t, grant.ReasonEndSession, f.reason(t, offline.Family.PublicID), "end-session revokes the hinted family whatever its scope")
	_, err = f.svc.Refresh(ctx, s.p, grant.RefreshRequest{Client: s.client, RefreshToken: rotated.RefreshToken})
	assert.True(t, grant.IsInvalidGrantError(err))
	_, err = f.svc.Exchange(ctx, s.p, grant.ExchangeRequest{Client: s.client, Code: pending, CodeVerifier: verifier})
	assert.True(t, grant.IsInvalidGrantError(err), "signing out drops the session's unused codes")

	other := f.seed(t)
	_, err = f.svc.Access(ctx, other.p, kept.AccessToken)
	assert.True(t, grant.IsInvalidTokenError(err), "a token is only found in its own project")
}

func TestPostgres_Prune(t *testing.T) {
	f := newPGFixture(t)
	s := f.seed(t)
	ctx := t.Context()
	count := func(table string) int {
		var n int
		require.NoError(t, f.db.QueryRowContext(ctx, `SELECT count(*) FROM `+table+` WHERE project_id = $1`, s.p.ID).Scan(&n))
		return n
	}
	f.code(t, s, "openid")
	used := f.exchange(t, s, "openid", "offline_access")
	revoked := f.exchange(t, s, "openid")
	_, err := f.svc.RevokeFamily(ctx, s.p, revoked.Family.PublicID, grant.ReasonEndSession)
	require.NoError(t, err)

	n, err := f.svc.Prune(ctx, s.p.OrgID, s.p.ID)
	require.NoError(t, err)
	assert.Zero(t, n, "nothing is due yet")

	f.now = f.now.Add(f.cfg.AccessTokenTTL)
	n, err = f.svc.Prune(ctx, s.p.OrgID, s.p.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, n, "the expired unused code and both access tokens")
	assert.Equal(t, 2, count("authorization_codes"), "used codes stay for replay detection")

	f.now = f.now.Add(24 * time.Hour)
	_, err = f.svc.Prune(ctx, s.p.OrgID, s.p.ID)
	require.NoError(t, err)
	assert.Zero(t, count("authorization_codes"), "used codes go after a day")
	assert.Zero(t, count("refresh_tokens"), "the idle refresh token expired")
	assert.Zero(t, count("grant_families"), "a family unused for the idle timeout goes, revoked or not")
	_, err = f.svc.Refresh(ctx, s.p, grant.RefreshRequest{Client: s.client, RefreshToken: used.RefreshToken})
	assert.True(t, grant.IsInvalidGrantError(err))
}

func race(t *testing.T, n int, fn func() error) []error {
	t.Helper()
	errs := make([]error, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			<-start
			errs[i] = fn()
		})
	}
	close(start)
	wg.Wait()
	return errs
}

func wins(t *testing.T, errs []error) int {
	t.Helper()
	n := 0
	for _, err := range errs {
		if err == nil {
			n++
			continue
		}
		assert.True(t, grant.IsInvalidGrantError(err), "a loser gets invalid_grant: %v", err)
	}
	return n
}

// SECURITY: two exchanges of one code at once: exactly one wins, and the loser's replay revokes the family it started.
func TestPostgres_ParallelExchangesOfOneCode(t *testing.T) {
	f := newPGFixture(t)
	s := f.seed(t)
	code := f.code(t, s, "openid", "offline_access")
	var sid sync.Map
	errs := race(t, 2, func() error {
		got, err := f.svc.Exchange(t.Context(), s.p, grant.ExchangeRequest{Client: s.client, Code: code, CodeVerifier: verifier})
		if err == nil {
			sid.Store("sid", got.Family.PublicID)
		}
		return err
	})
	require.Equal(t, 1, wins(t, errs))
	won, _ := sid.Load("sid")
	assert.Equal(t, grant.ReasonCodeReplay, f.reason(t, won.(string)))
}

// SECURITY: two refreshes of one token at once: exactly one rotates, and the other's reuse revokes the family.
func TestPostgres_ParallelRefreshesOfOneToken(t *testing.T) {
	f := newPGFixture(t)
	s := f.seed(t)
	first := f.exchange(t, s, "openid", "offline_access")
	errs := race(t, 2, func() error {
		_, err := f.svc.Refresh(t.Context(), s.p, grant.RefreshRequest{Client: s.client, RefreshToken: first.RefreshToken})
		return err
	})
	require.Equal(t, 1, wins(t, errs))
	assert.Equal(t, grant.ReasonReuse, f.reason(t, first.Family.PublicID))
}
