//go:build integration

package pat_test

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/internal/kernel/secret"
	"github.com/altalune-id/othra/internal/kernel/tenant"
	"github.com/altalune-id/othra/internal/kernel/usage"
	"github.com/altalune-id/othra/internal/pat"
	"github.com/altalune-id/othra/internal/testutil/pgtest"
)

type servers struct{ server pat.Server }

func (s servers) Accepting(context.Context, pat.Project) ([]pat.Server, error) {
	return []pat.Server{s.server}, nil
}

func (s servers) ByPublicID(_ context.Context, _ pat.Project, publicID string) (pat.Server, bool, error) {
	return s.server, publicID == s.server.PublicID, nil
}

func (s servers) ByID(_ context.Context, _ pat.Project, id uuid.UUID) (pat.Server, bool, error) {
	return s.server, id == s.server.ID, nil
}

type subjects struct{}

func (subjects) Subject(_ context.Context, _ pat.Project, endUserID uuid.UUID) (string, error) {
	return "sub-" + endUserID.String()[:8], nil
}

type pgFixture struct {
	db      *sql.DB
	svc     *pat.Service
	usage   *usage.Worker
	now     time.Time
	p       pat.Project
	endUser uuid.UUID
	server  pat.Server
}

func pub(prefix string) string { return prefix + strings.ReplaceAll(uuid.NewString(), "-", "")[:20] }

func newPGFixture(t *testing.T) *pgFixture {
	t.Helper()
	pool, sqlDB, schemaName := pgtest.MigratedRLS(t)
	f := &pgFixture{
		db: sqlDB, now: time.Now().UTC().Truncate(time.Microsecond), endUser: uuid.New(),
		p: pat.Project{OrgID: uuid.New(), ID: uuid.New(), Name: "Acme"},
	}
	f.p.PublicID = pub("p")
	f.server = pat.Server{
		ID: uuid.New(), PublicID: pub("r"), Name: "MCP", Identifier: "https://mcp.example.test/mcp", Scopes: []string{"read", "write"},
		Accepting: true,
	}
	f.seed(t, f.p, f.endUser, f.server.ID)
	store := pat.NewStore(pool, schemaName, "")
	f.usage = usage.New("pat-usage", store, time.Hour, nil)
	f.svc = pat.NewService(store, db.NewTransactor(pool), servers{server: f.server}, subjects{}, f.usage, apperror.NewReporter(nil, false).Unexpected).
		WithClock(func() time.Time { return f.now })
	return f
}

func (f *pgFixture) tenant() tenant.Context {
	return tenant.Context{OrgID: f.p.OrgID, ProjectID: f.p.ID}
}

func (f *pgFixture) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	_, err := f.db.ExecContext(t.Context(), query, args...)
	require.NoError(t, err)
}

func (f *pgFixture) seed(t *testing.T, p pat.Project, endUser, server uuid.UUID) {
	t.Helper()
	f.exec(t, `INSERT INTO orgs (id, public_id, slug, name, created_at, updated_at) VALUES ($1, $2, $2, 'Org', now(), now())`,
		p.OrgID, "o"+p.OrgID.String()[:12])
	f.exec(t, `INSERT INTO projects (id, public_id, org_id, slug, name, issuer_handle, created_at, updated_at) VALUES ($1, $2, $3, $2, 'P', $4, now(), now())`,
		p.ID, p.PublicID, p.OrgID, "h"+p.ID.String()[:8])
	f.exec(t, `INSERT INTO end_users (id, public_id, org_id, project_id, email, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, now(), now())`,
		endUser, pub("e"), p.OrgID, p.ID, endUser.String()[:8]+"@example.com")
	f.exec(t, `INSERT INTO resource_servers (id, public_id, org_id, project_id, name, identifier, scopes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'MCP', $5, '{read,write}', now(), now())`, server, pub("r"), p.OrgID, p.ID, "https://"+server.String()[:8]+".example.test/mcp")
}

func (f *pgFixture) create(t *testing.T) *pat.Created {
	t.Helper()
	c, err := f.svc.Create(t.Context(), f.p, pat.CreateRequest{
		EndUserID: f.endUser, Name: "CLI", ServerPublicID: f.server.PublicID, Scopes: []string{"read"}, Expiry: pat.Expiry7Days,
	})
	require.NoError(t, err)
	return c
}

func (f *pgFixture) introspect(t *testing.T, token string) bool {
	t.Helper()
	_, ok, err := f.svc.Introspect(t.Context(), f.p, f.server.ID, token)
	require.NoError(t, err)
	return ok
}

// the hash at rest, active until revoked, inactive on the very next read.
func TestPostgres_CreateIntrospectRevoke(t *testing.T) {
	f := newPGFixture(t)
	c := f.create(t)
	assert.True(t, pat.Scheme.Valid(c.Secret))

	var hash, hint string
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT token_hash, hint FROM personal_tokens WHERE public_id = $1`, c.Token.PublicID).
		Scan(&hash, &hint))
	assert.Equal(t, secret.Versioned(c.Secret), hash)
	assert.Equal(t, c.Secret[len(c.Secret)-4:], hint)

	in, ok, err := f.svc.Introspect(t.Context(), f.p, f.server.ID, c.Secret)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, []string{"read"}, in.Token.Scopes)
	assert.Equal(t, f.now, in.Token.CreatedAt)
	assert.False(t, f.introspect(t, c.Secret+"x"))

	listed, err := f.svc.List(t.Context(), f.p, f.endUser)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, c.Token.PublicID, listed[0].Token.PublicID)

	require.NoError(t, f.svc.Revoke(t.Context(), f.p, f.endUser, c.Token.PublicID))
	assert.False(t, f.introspect(t, c.Secret), "revoked: inactive at once")
	assert.True(t, pat.IsNotFoundError(f.svc.Revoke(t.Context(), f.p, f.endUser, c.Token.PublicID)))
	assert.True(t, pat.IsNotFoundError(f.svc.Revoke(t.Context(), f.p, uuid.New(), c.Token.PublicID)), "another end user's token")
}

// the live-token cap holds when creates race, because each locks the end user's row.
func TestPostgres_LiveCapUnderConcurrency(t *testing.T) {
	f := newPGFixture(t)
	var wg sync.WaitGroup
	var mu sync.Mutex
	created, limited := 0, 0
	for range pat.MaxLive + 5 {
		wg.Go(func() {
			_, err := f.svc.Create(t.Context(), f.p, pat.CreateRequest{
				EndUserID: f.endUser, Name: "CLI", ServerPublicID: f.server.PublicID, Scopes: []string{"read"}, Expiry: pat.Expiry7Days,
			})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				created++
			case pat.IsLimitError(err):
				limited++
			default:
				t.Errorf("create: %v", err)
			}
		})
	}
	wg.Wait()
	assert.Equal(t, pat.MaxLive, created)
	assert.Equal(t, 5, limited)
}

// last use goes through the batcher, flushed per tenant, and never moves back.
func TestPostgres_LastUsed(t *testing.T) {
	f := newPGFixture(t)
	c := f.create(t)
	require.True(t, f.introspect(t, c.Secret))
	flush := func() {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_ = f.usage.Run(ctx)
	}
	flush()
	lastUsed := func() *time.Time {
		var at *time.Time
		require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT last_used_at FROM personal_tokens WHERE id = $1`, c.Token.ID).Scan(&at))
		return at
	}
	require.NotNil(t, lastUsed())
	assert.True(t, f.now.Equal(*lastUsed()))

	f.usage.Record(f.tenant(), c.Token.ID, f.now.Add(-time.Hour))
	flush()
	assert.True(t, f.now.Equal(*lastUsed()), "never moved back")
}

// tokens revoked or expired over 30 days ago are pruned, the rest kept.
func TestPostgres_Prune(t *testing.T) {
	f := newPGFixture(t)
	revoked, kept := f.create(t), f.create(t)
	require.NoError(t, f.svc.Revoke(t.Context(), f.p, f.endUser, revoked.Token.PublicID))
	f.now = f.now.Add(31 * 24 * time.Hour)
	n, err := f.svc.Prune(t.Context(), f.p.OrgID, f.p.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "revoked 31 days ago; the other expired 24 days ago")
	f.now = f.now.Add(7 * 24 * time.Hour)
	n, err = f.svc.Prune(t.Context(), f.p.OrgID, f.p.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	listed, err := f.svc.List(t.Context(), f.p, f.endUser)
	require.NoError(t, err)
	assert.Empty(t, listed, kept.Token.PublicID)
}

// another project's scope sees none of the tokens, and its servers can't introspect them.
func TestPostgres_ProjectsStayApart(t *testing.T) {
	f := newPGFixture(t)
	c := f.create(t)
	other := pat.Project{OrgID: uuid.New(), ID: uuid.New(), PublicID: pub("p")}
	f.seed(t, other, uuid.New(), uuid.New())

	listed, err := f.svc.List(t.Context(), other, f.endUser)
	require.NoError(t, err)
	assert.Empty(t, listed)
	_, ok, err := f.svc.Introspect(t.Context(), other, f.server.ID, c.Secret)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.True(t, pat.IsNotFoundError(f.svc.Revoke(t.Context(), other, f.endUser, c.Token.PublicID)))
}
