//go:build integration

package project_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/keys"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/testutil/fakes"
	"altalune.id/othra/sealer"
)

func schemaOf(t *testing.T, f *projectFixture) string {
	t.Helper()
	var name string
	require.NoError(t, f.sqlDB.QueryRowContext(t.Context(), `SELECT current_schema()`).Scan(&name))
	return name
}

func keysService(t *testing.T, f *projectFixture, sl sealer.Sealer) *keys.Service {
	t.Helper()
	return keys.NewService(keys.NewStore(f.pool, schemaOf(t, f), ""), db.NewTransactor(f.pool), sl,
		keys.Config{RSABits: 2048, PrePublish: keys.CacheTTL, Overlap: keys.CacheTTL}, apperror.NewReporter(nil, false).Unexpected)
}

func projectsWithKeys(f *projectFixture, ks *keys.Service) *project.Service {
	return project.NewService(f.projects, db.NewTransactor(f.pool), fakes.NoMemberships{}, func(ctx context.Context, p *project.Project) error {
		_, err := ks.Generate(ctx, p.OrgID, p.ID)
		return err
	}, "", apperror.NewReporter(nil, false).Unexpected).WithOrgSlugs(fakes.OrgSlugs("")).WithStorage(&fakes.FileStorage{})
}

func countRows(t *testing.T, f *projectFixture, table string, orgID uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, f.sqlDB.QueryRowContext(t.Context(), `SELECT count(*) FROM `+table+` WHERE org_id = $1`, orgID).Scan(&n))
	return n
}

// a project and its first signing key commit together, even when the caller's scope names another project.
func TestCreate_GivesTheProjectAnActiveKeyInOneUnitOfWork(t *testing.T) {
	f := newProjectFixture(t)
	orgID := f.seedOrg(t, "acme")
	key, err := sealer.GenerateKey()
	require.NoError(t, err)
	sl, err := sealer.New(key)
	require.NoError(t, err)
	svc := projectsWithKeys(f, keysService(t, f, sl))

	ctx := tenant.Into(t.Context(), tenant.Context{OrgID: orgID, ProjectID: uuid.New(), UserID: uuid.New()})
	p, err := svc.Create(ctx, project.CreateRequest{OrgID: orgID, Name: "Web"})
	require.NoError(t, err)
	assert.Equal(t, 1, countRows(t, f, "projects", orgID))

	var status string
	require.NoError(t, f.sqlDB.QueryRowContext(t.Context(),
		`SELECT status FROM signing_keys WHERE project_id = $1`, p.ID).Scan(&status))
	assert.Equal(t, "active", status)
}

// a key that cannot be created rolls its project back.
func TestCreate_AFailingKeyRollsTheProjectBack(t *testing.T) {
	f := newProjectFixture(t)
	orgID := f.seedOrg(t, "acme")
	svc := projectsWithKeys(f, keysService(t, f, sealer.Disabled()))

	_, err := svc.Create(tenant.Into(t.Context(), tenant.Context{OrgID: orgID}), project.CreateRequest{OrgID: orgID, Name: "Web"})
	require.Error(t, err)
	assert.Zero(t, countRows(t, f, "projects", orgID), "no project without its key")
	assert.Zero(t, countRows(t, f, "signing_keys", orgID))
}
