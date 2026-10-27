//go:build integration

package keys_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/internal/kernel/tenant"
	"github.com/altalune-id/othra/internal/keys"
	"github.com/altalune-id/othra/internal/testutil/pgtest"
	"github.com/altalune-id/othra/sealer"
)

type pgFixture struct {
	db   *sql.DB
	pool db.Pool
	svc  *keys.Service
}

func newPGFixture(t *testing.T) *pgFixture {
	t.Helper()
	pool, sqlDB, schemaName := pgtest.MigratedRLS(t)
	key, err := sealer.GenerateKey()
	require.NoError(t, err)
	sl, err := sealer.New(key)
	require.NoError(t, err)
	svc := keys.NewService(keys.NewStore(pool, schemaName, ""), db.NewTransactor(pool), sl,
		keys.Config{RSABits: 2048, PrePublish: 10 * time.Minute, Overlap: time.Hour}, apperror.NewReporter(nil, false).Unexpected)
	return &pgFixture{db: sqlDB, pool: pool, svc: svc}
}

func (f *pgFixture) seed(t *testing.T) (org, project uuid.UUID) {
	t.Helper()
	org, project = uuid.New(), uuid.New()
	now := time.Now().UTC()
	_, err := f.db.ExecContext(t.Context(), `INSERT INTO orgs (id, public_id, slug, name, created_at, updated_at) VALUES ($1, $2, $2, 'Org', $3, $3)`,
		org, "o"+org.String()[:12], now)
	require.NoError(t, err)
	_, err = f.db.ExecContext(t.Context(), `INSERT INTO projects (id, public_id, org_id, slug, name, issuer_handle, created_at, updated_at) VALUES ($1, $2, $3, $2, 'P', $4, $5, $5)`,
		project, "p"+project.String()[:12], org, "h"+project.String()[:8], now)
	require.NoError(t, err)
	return org, project
}

func scoped(t *testing.T, org, project uuid.UUID) context.Context {
	return tenant.Into(t.Context(), tenant.Context{OrgID: org, ProjectID: project})
}

func TestPostgres_KeyLifecycle(t *testing.T) {
	f := newPGFixture(t)
	org, project := f.seed(t)
	ctx := t.Context()

	first, err := f.svc.Generate(tenant.Into(ctx, tenant.Context{OrgID: org}), org, project)
	require.NoError(t, err)
	next, err := f.svc.Rotate(ctx, org, project, false)
	require.NoError(t, err)
	_, err = f.svc.Rotate(ctx, org, project, false)
	assert.True(t, keys.IsRotationPendingError(err), "%v", err)

	listed, err := f.svc.List(ctx, org, project)
	require.NoError(t, err)
	require.Len(t, listed, 2)
	assert.Equal(t, next.ID, listed[0].ID, "newest first")
	assert.Equal(t, first.JWK, listed[1].JWK, "the public JWK round-trips through jsonb")

	body, err := f.svc.JWKS(scoped(t, org, project), project)
	require.NoError(t, err)
	assert.Contains(t, string(body), first.KID)
	assert.Contains(t, string(body), next.KID)

	sk, err := f.svc.ActiveKey(scoped(t, org, project), project)
	require.NoError(t, err)
	assert.Equal(t, first.KID, sk.KID, "the active key opens with its row's AAD")

	promoted, err := f.svc.Rotate(ctx, org, project, true)
	require.NoError(t, err)
	assert.Equal(t, next.ID, promoted.ID)

	var sealed []byte
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT private_key_enc FROM signing_keys WHERE id = $1`, first.ID).Scan(&sealed))
	assert.NotContains(t, string(sealed), "PRIVATE")
	assert.Greater(t, len(sealed), 1000, "a sealed PKCS#8 RSA key")

	n, err := f.svc.Prune(ctx, org, project)
	require.NoError(t, err)
	assert.Zero(t, n, "within the overlap")
}

// SECURITY: once a key is retired its sealed private key is gone; only a retired row may lack one.
func TestPostgres_ARetiredKeyKeepsNoPrivateKey(t *testing.T) {
	f := newPGFixture(t)
	org, project := f.seed(t)
	first, err := f.svc.Generate(t.Context(), org, project)
	require.NoError(t, err)
	_, err = f.svc.Rotate(t.Context(), org, project, true)
	require.NoError(t, err)
	_, err = f.db.ExecContext(t.Context(), `UPDATE signing_keys SET retire_after = now() - interval '1 second' WHERE id = $1`, first.ID)
	require.NoError(t, err)
	n, err := f.svc.Prune(t.Context(), org, project)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	var status string
	var sealed []byte
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT status, private_key_enc FROM signing_keys WHERE id = $1`, first.ID).Scan(&status, &sealed))
	assert.Equal(t, "retired", status)
	assert.Nil(t, sealed)

	_, err = f.db.ExecContext(t.Context(), `UPDATE signing_keys SET private_key_enc = NULL WHERE project_id = $1 AND status = 'active'`, project)
	assert.Error(t, err, "a live key always keeps its sealed private key")
}

// SECURITY: a sealed key moved to another row of the database does not open there.
func TestPostgres_ASealedKeyMovedToAnotherProjectDoesNotOpen(t *testing.T) {
	f := newPGFixture(t)
	orgA, projectA := f.seed(t)
	orgB, projectB := f.seed(t)
	a, err := f.svc.Generate(t.Context(), orgA, projectA)
	require.NoError(t, err)
	b, err := f.svc.Generate(t.Context(), orgB, projectB)
	require.NoError(t, err)
	_, err = f.db.ExecContext(t.Context(), `UPDATE signing_keys SET private_key_enc = (SELECT private_key_enc FROM signing_keys WHERE id = $1) WHERE id = $2`, a.ID, b.ID)
	require.NoError(t, err)
	_, err = f.svc.ActiveKey(scoped(t, orgB, projectB), projectB)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "PRIVATE")
}

func TestPostgres_OneActiveAndOneNextPerProject(t *testing.T) {
	f := newPGFixture(t)
	org, project := f.seed(t)
	_, err := f.svc.Generate(t.Context(), org, project)
	require.NoError(t, err)
	_, err = f.db.ExecContext(t.Context(),
		`INSERT INTO signing_keys (id, public_id, org_id, project_id, kid, alg, public_jwk, private_key_enc, status, created_at) VALUES ($1, $2, $3, $4, $5, 'RS256', '{}', '\x01', 'active', now())`,
		uuid.New(), uuid.NewString()[:21], org, project, uuid.NewString())
	assert.True(t, db.IsUniqueViolation(err), "a second active key: %v", err)
	_, err = f.db.ExecContext(t.Context(),
		`INSERT INTO signing_keys (id, public_id, org_id, project_id, kid, alg, public_jwk, private_key_enc, status, created_at) VALUES ($1, $2, $3, $4, $5, 'RS256', '{}', '\x01', 'next', now())`,
		uuid.New(), uuid.NewString()[:21], org, project, uuid.NewString())
	assert.Error(t, err, "a next key needs activates_at")
}

func TestPostgres_KeysAreProjectScoped(t *testing.T) {
	f := newPGFixture(t)
	org, project := f.seed(t)
	_, err := f.svc.Generate(t.Context(), org, project)
	require.NoError(t, err)
	other := uuid.New()
	_, err = f.db.ExecContext(t.Context(), `INSERT INTO projects (id, public_id, org_id, slug, name, issuer_handle, created_at, updated_at) VALUES ($1, $2, $3, $2, 'P', $4, now(), now())`,
		other, "p"+other.String()[:12], org, "h"+other.String()[:8])
	require.NoError(t, err)

	body, err := f.svc.JWKS(scoped(t, org, other), project)
	require.NoError(t, err)
	assert.JSONEq(t, `{"keys":[]}`, string(body), "another project's scope reads none of these keys")
	_, err = f.svc.ActiveKey(scoped(t, org, other), project)
	assert.True(t, keys.IsNoActiveKeyError(err), "%v", err)
}

func TestPostgres_DeletingAProjectDeletesItsKeys(t *testing.T) {
	f := newPGFixture(t)
	org, project := f.seed(t)
	_, err := f.svc.Generate(t.Context(), org, project)
	require.NoError(t, err)
	_, err = f.db.ExecContext(t.Context(), `DELETE FROM projects WHERE id = $1`, project)
	require.NoError(t, err)
	var n int
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM signing_keys WHERE project_id = $1`, project).Scan(&n))
	assert.Zero(t, n)
}
