//go:build integration

package upstream_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/testutil/pgtest"
	"altalune.id/othra/internal/upstream"
	"altalune.id/othra/sealer"
)

type emailLinkOn struct{}

func (emailLinkOn) EmailLinkOn(context.Context, uuid.UUID) (bool, error) { return true, nil }

type issuerOf struct{}

func (issuerOf) IssuerURL(context.Context, uuid.UUID) (string, error) {
	return "https://id.example.com/p/acme", nil
}

func seedProject(t *testing.T, sqlDB *sql.DB) (org, project uuid.UUID) {
	t.Helper()
	org, project = uuid.New(), uuid.New()
	_, err := sqlDB.ExecContext(t.Context(), `INSERT INTO orgs (id, public_id, slug, name, created_at, updated_at) VALUES ($1, $2, $2, 'Org', now(), now())`,
		org, "o"+org.String()[:12])
	require.NoError(t, err)
	_, err = sqlDB.ExecContext(t.Context(), `INSERT INTO projects (id, public_id, org_id, slug, name, issuer_handle, created_at, updated_at) VALUES ($1, $2, $3, $2, 'P', $4, now(), now())`,
		project, "p"+project.String()[:12], org, "h"+project.String()[:8])
	require.NoError(t, err)
	return org, project
}

func codeOf(err error) apperror.Code {
	if app, ok := apperror.AsAppError(err); ok {
		return app.Code()
	}
	return ""
}

// the sealed secret round-trips, one Google per project, UPS004 while linked, and another project sees nothing.
func TestPostgres_Upstreams(t *testing.T) {
	pool, sqlDB, schemaName := pgtest.MigratedRLS(t)
	key, err := sealer.GenerateKey()
	require.NoError(t, err)
	seal, err := sealer.New(key)
	require.NoError(t, err)
	svc := upstream.NewService(upstream.NewStore(pool, schemaName, ""), db.NewTransactor(pool), seal, emailLinkOn{}, issuerOf{},
		apperror.NewReporter(nil, false).Unexpected)
	ctx := t.Context()
	org, project := seedProject(t, sqlDB)

	u, err := svc.Create(ctx, org, project, upstream.CreateRequest{Kind: upstream.KindGoogle, ClientID: "c1", ClientSecret: "s1", Enabled: true})
	require.NoError(t, err)
	_, err = svc.Create(ctx, org, project, upstream.CreateRequest{Kind: upstream.KindGoogle, ClientID: "c2", ClientSecret: "s2"})
	assert.Equal(t, apperror.CodeUpstreamExists, codeOf(err))
	var stored []byte
	require.NoError(t, sqlDB.QueryRowContext(ctx, `SELECT client_secret_sealed FROM upstream_providers WHERE id = $1`, u.ID).Scan(&stored))
	assert.NotContains(t, string(stored), "s1")

	got, err := svc.Update(ctx, org, project, u.PublicID, "c1b", "s1b")
	require.NoError(t, err)
	assert.Equal(t, "c1b", got.ClientID)
	c, err := svc.Credentials(ctx, org, project, upstream.KindGoogle)
	require.NoError(t, err)
	assert.Equal(t, "s1b", c.ClientSecret)
	off, err := svc.SetEnabled(ctx, org, project, u.PublicID, false)
	require.NoError(t, err)
	assert.False(t, off.Enabled)
	_, err = svc.Credentials(ctx, org, project, upstream.KindGoogle)
	assert.Equal(t, apperror.CodeUpstreamNotFound, codeOf(err))

	otherOrg, other := seedProject(t, sqlDB)
	list, err := svc.List(ctx, otherOrg, other)
	require.NoError(t, err)
	assert.Empty(t, list, "RLS: another project sees none")
	_, err = svc.Get(ctx, otherOrg, other, u.PublicID)
	assert.Equal(t, apperror.CodeUpstreamNotFound, codeOf(err))

	endUser := uuid.New()
	_, err = sqlDB.ExecContext(ctx, `INSERT INTO end_users (id, public_id, org_id, project_id, email, created_at, updated_at) VALUES ($1, $2, $3, $4, 'a@example.com', now(), now())`,
		endUser, "e"+endUser.String()[:12], org, project)
	require.NoError(t, err)
	_, err = sqlDB.ExecContext(ctx, `INSERT INTO end_user_identities (id, public_id, org_id, project_id, end_user_id, upstream_provider_id, issuer, subject, email, email_verified, linked_at, last_used_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'https://accounts.google.com', 'g-1', 'a@example.com', true, now(), now())`,
		uuid.New(), "i"+uuid.NewString()[:12], org, project, endUser, u.ID)
	require.NoError(t, err)
	got, err = svc.Get(ctx, org, project, u.PublicID)
	require.NoError(t, err)
	assert.True(t, got.IdentitiesLinked)
	assert.Equal(t, apperror.CodeUpstreamHasIdentities, codeOf(svc.Delete(ctx, org, project, u.PublicID)))
	_, err = sqlDB.ExecContext(ctx, `DELETE FROM end_user_identities WHERE upstream_provider_id = $1`, u.ID)
	require.NoError(t, err)
	require.NoError(t, svc.Delete(ctx, org, project, u.PublicID))

	require.NoError(t, svc.Seed(ctx, org, project, upstream.KindGoogle, "seeded", "seed-secret"))
	c, err = svc.Credentials(ctx, org, project, upstream.KindGoogle)
	require.NoError(t, err)
	assert.Equal(t, "seeded", c.Upstream.ClientID)
}
