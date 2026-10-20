//go:build integration

package client_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/client"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/testutil/pgtest"
)

type failAfterLock struct {
	projects *project.Service
	err      error
}

func (l *failAfterLock) LockIssuerHandle(ctx context.Context, projectID uuid.UUID) error {
	if err := l.projects.LockIssuerHandle(ctx, projectID); err != nil {
		return err
	}
	return l.err
}

type pgFixture struct {
	db     *sql.DB
	pool   db.Pool
	schema string
	svc    *client.Service
	issuer *failAfterLock
}

func newPGFixture(t *testing.T) *pgFixture {
	t.Helper()
	pool, sqlDB, schemaName := pgtest.MigratedRLS(t)
	unexpected := apperror.NewReporter(nil, false).Unexpected
	issuer := &failAfterLock{projects: project.NewService(project.NewStore(pool, schemaName, ""), unexpected)}
	svc := client.NewService(client.NewStore(pool, schemaName, ""), db.NewTransactor(pool), issuer, unexpected)
	return &pgFixture{db: sqlDB, pool: pool, schema: schemaName, svc: svc, issuer: issuer}
}

func (f *pgFixture) seed(t *testing.T) (org, prj uuid.UUID) {
	t.Helper()
	org, prj = uuid.New(), uuid.New()
	now := time.Now().UTC()
	_, err := f.db.ExecContext(t.Context(), `INSERT INTO orgs (id, public_id, slug, name, created_at, updated_at) VALUES ($1, $2, $2, 'Org', $3, $3)`,
		org, "o"+org.String()[:12], now)
	require.NoError(t, err)
	_, err = f.db.ExecContext(t.Context(), `INSERT INTO projects (id, public_id, org_id, slug, name, issuer_handle, created_at, updated_at) VALUES ($1, $2, $3, $2, 'P', $4, $5, $5)`,
		prj, "p"+prj.String()[:12], org, "h"+prj.String()[:8], now)
	require.NoError(t, err)
	return org, prj
}

func (f *pgFixture) locked(t *testing.T, prj uuid.UUID) bool {
	t.Helper()
	var at *time.Time
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT issuer_handle_locked_at FROM projects WHERE id = $1`, prj).Scan(&at))
	return at != nil
}

func (f *pgFixture) count(t *testing.T, table string, prj uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM `+table+` WHERE project_id = $1`, prj).Scan(&n))
	return n
}

func TestPostgres_ClientLifecycle(t *testing.T) {
	f := newPGFixture(t)
	ctx := t.Context()
	org, prj := f.seed(t)
	assert.False(t, f.locked(t, prj))

	settings := client.Settings{
		Name: "Web", RedirectURIs: []string{"https://app.example/cb", `https://app.example/cb?q="a,b"`},
		PostLogoutRedirectURIs: []string{"https://app.example/"}, ConsentPolicy: client.ConsentImplicit,
	}
	c, err := f.svc.Create(ctx, org, prj, client.KindWeb, settings)
	require.NoError(t, err)
	assert.True(t, f.locked(t, prj), "the first client locks the issuer handle")

	got, err := f.svc.Get(ctx, org, prj, c.PublicID)
	require.NoError(t, err)
	assert.Equal(t, settings.RedirectURIs, got.RedirectURIs)
	assert.Equal(t, settings.PostLogoutRedirectURIs, got.PostLogoutRedirectURIs)
	assert.Equal(t, client.ConsentImplicit, got.ConsentPolicy)
	assert.Equal(t, client.KindWeb, got.Kind())
	assert.False(t, got.RefreshTokens())

	cli, err := f.svc.Create(ctx, org, prj, client.KindCLI, client.Settings{Name: "CLI", RedirectURIs: []string{"http://127.0.0.1/cb"}, RefreshTokens: true})
	require.NoError(t, err)
	got, err = f.svc.Get(ctx, org, prj, cli.PublicID)
	require.NoError(t, err)
	assert.Equal(t, client.KindCLI, got.Kind())
	assert.True(t, got.RefreshTokens())
	assert.Equal(t, []string{}, got.PostLogoutRedirectURIs)

	settings.RefreshTokens, settings.RedirectURIs = true, []string{"https://app.example/new"}
	upd, err := f.svc.Update(ctx, org, prj, c.PublicID, settings)
	require.NoError(t, err)
	got, err = f.svc.Get(ctx, org, prj, c.PublicID)
	require.NoError(t, err)
	assert.Equal(t, upd.RedirectURIs, got.RedirectURIs)
	assert.True(t, got.RefreshTokens())

	list, err := f.svc.List(ctx, org, prj)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, c.PublicID, list[0].PublicID, "oldest first")

	_, other := f.seed(t)
	_, err = f.svc.Get(ctx, org, other, c.PublicID)
	assert.True(t, client.IsNotFoundError(err), "another project does not see it: %v", err)

	require.NoError(t, f.svc.Delete(ctx, org, prj, cli.PublicID))
	assert.True(t, client.IsNotFoundError(f.svc.Delete(ctx, org, prj, cli.PublicID)))
	assert.Equal(t, 1, f.count(t, "oauth_clients", prj))
}

func TestPostgres_SecretRotation(t *testing.T) {
	f := newPGFixture(t)
	ctx := t.Context()
	org, prj := f.seed(t)
	c, err := f.svc.Create(ctx, org, prj, client.KindWeb, client.Settings{Name: "Web", RedirectURIs: []string{"https://app.example/cb"}})
	require.NoError(t, err)

	got, first, err := f.svc.GenerateSecret(ctx, org, prj, c.PublicID)
	require.NoError(t, err)
	require.Len(t, got.Secrets, 1)
	var hash string
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT secret_hash FROM oauth_client_secrets WHERE public_id = $1`, got.Secrets[0].PublicID).Scan(&hash))
	assert.True(t, client.SecretMatches(hash, first))
	assert.NotContains(t, hash, first)

	_, _, err = f.svc.GenerateSecret(ctx, org, prj, c.PublicID)
	assert.True(t, client.IsSecretExistsError(err), "%v", err)

	got, _, err = f.svc.RotateSecret(ctx, org, prj, c.PublicID, time.Hour)
	require.NoError(t, err)
	require.Len(t, got.Secrets, 2)
	assert.True(t, got.Secrets[0].Current())
	assert.NotNil(t, got.Secrets[1].ExpiresAt)

	got, _, err = f.svc.RotateSecret(ctx, org, prj, c.PublicID, time.Hour)
	require.NoError(t, err)
	assert.Len(t, got.Secrets, 2, "at most two live secrets")
	assert.Equal(t, 2, f.count(t, "oauth_client_secrets", prj))

	got, err = f.svc.RevokeSecret(ctx, org, prj, c.PublicID, got.Secrets[1].PublicID)
	require.NoError(t, err)
	assert.Len(t, got.Secrets, 1)

	got, _, err = f.svc.RotateSecret(ctx, org, prj, c.PublicID, 0)
	require.NoError(t, err)
	assert.Len(t, got.Secrets, 1)
	assert.Equal(t, 1, f.count(t, "oauth_client_secrets", prj))

	require.NoError(t, f.svc.Delete(ctx, org, prj, c.PublicID))
	assert.Zero(t, f.count(t, "oauth_client_secrets", prj), "secrets go with their client")
}

// Secret writes name the project as well as the client, so a wrong project touches nothing.
func TestPostgres_SecretWritesStayInTheirProject(t *testing.T) {
	f := newPGFixture(t)
	org, prj := f.seed(t)
	ctx := t.Context()
	c, err := f.svc.Create(ctx, org, prj, client.KindWeb, client.Settings{Name: "Web", RedirectURIs: []string{"https://app.example/cb"}})
	require.NoError(t, err)
	got, _, err := f.svc.GenerateSecret(ctx, org, prj, c.PublicID)
	require.NoError(t, err)
	store := client.NewStore(f.pool, f.schema, "")
	scoped := tenant.Into(ctx, tenant.Context{OrgID: org, ProjectID: prj})
	now := time.Now().UTC()
	require.NoError(t, db.NewTransactor(f.pool).RunInTx(scoped, func(ctx context.Context) error {
		if err := store.ExpireSecret(ctx, uuid.New(), c.ID, got.Secrets[0].ID, now); err != nil {
			return err
		}
		return store.DeleteSecrets(ctx, uuid.New(), c.ID, []uuid.UUID{got.Secrets[0].ID}, now)
	}))
	again, err := f.svc.Get(ctx, org, prj, c.PublicID)
	require.NoError(t, err)
	require.Len(t, again.Secrets, 1)
	assert.True(t, again.Secrets[0].Current(), "the secret is untouched")
}

// The handle lock commits with the first client, and a failure after the lock rolls both back.
func TestPostgres_HandleLockSharesTheUnitOfWork(t *testing.T) {
	f := newPGFixture(t)
	ctx := t.Context()
	org, prj := f.seed(t)
	f.issuer.err = errors.New("boom")
	_, err := f.svc.Create(ctx, org, prj, client.KindWeb, client.Settings{Name: "Web", RedirectURIs: []string{"https://app.example/cb"}})
	require.Error(t, err)
	assert.False(t, f.locked(t, prj), "the lock rolled back")
	assert.Zero(t, f.count(t, "oauth_clients", prj), "the client rolled back")

	f.issuer.err = nil
	_, err = f.svc.Create(ctx, org, prj, client.KindWeb, client.Settings{Name: "Web", RedirectURIs: []string{"https://app.example/cb"}})
	require.NoError(t, err)
	assert.True(t, f.locked(t, prj))
}
