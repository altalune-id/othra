//go:build integration

package consent_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/consent"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/testutil/pgtest"
)

type seeded struct {
	p                       consent.Project
	endUser, client, server uuid.UUID
}

func seed(t *testing.T, db *sql.DB) seeded {
	t.Helper()
	s := seeded{
		p: consent.Project{OrgID: uuid.New(), ID: uuid.New(), PublicID: "p" + uuid.NewString()[:12]}, endUser: uuid.New(),
		client: uuid.New(), server: uuid.New(),
	}
	exec := func(query string, args ...any) {
		_, err := db.ExecContext(t.Context(), query, args...)
		require.NoError(t, err)
	}
	exec(`INSERT INTO orgs (id, public_id, slug, name, created_at, updated_at) VALUES ($1, $2, $2, 'Org', now(), now())`,
		s.p.OrgID, "o"+s.p.OrgID.String()[:12])
	exec(`INSERT INTO projects (id, public_id, org_id, slug, name, issuer_handle, created_at, updated_at) VALUES ($1, $2, $3, $2, 'P', $4, now(), now())`,
		s.p.ID, s.p.PublicID, s.p.OrgID, "h"+s.p.ID.String()[:8])
	exec(`INSERT INTO oauth_clients (id, public_id, org_id, project_id, name, client_type, application_type, token_endpoint_auth_method, grant_types, allowed_scopes, redirect_uris, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'Web', 'confidential', 'web', 'client_secret_basic', '{authorization_code}', '{openid}', '{https://a.example/cb}', now(), now())`,
		s.client, "c"+s.client.String()[:12], s.p.OrgID, s.p.ID)
	exec(`INSERT INTO end_users (id, public_id, org_id, project_id, email, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, now(), now())`,
		s.endUser, "e"+s.endUser.String()[:12], s.p.OrgID, s.p.ID, "eu"+uuid.NewString()[:8]+"@example.com")
	exec(`INSERT INTO resource_servers (id, public_id, org_id, project_id, name, identifier, scopes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'MCP', $5, '{sample:read,sample:write}', now(), now())`,
		s.server, "r"+s.server.String()[:12], s.p.OrgID, s.p.ID, "https://mcp"+s.server.String()[:8]+".test/mcp")
	return s
}

// one row per end user, client and resource server; approving again adds scopes; another project sees nothing.
func TestPostgres_GrantFindRevoke(t *testing.T) {
	pool, sqlDB, schemaName := pgtest.MigratedRLS(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	svc := consent.NewService(consent.NewStore(pool, schemaName, ""), db.NewTransactor(pool), noGrants{}, apperror.NewReporter(nil, false).Unexpected).
		WithClock(func() time.Time { return now })
	s := seed(t, sqlDB)
	ctx := t.Context()
	rsKey := consent.Key{EndUserID: s.endUser, ClientID: s.client, ResourceServerID: &s.server}
	oidcKey := consent.Key{EndUserID: s.endUser, ClientID: s.client}

	got, err := svc.Find(ctx, s.p, rsKey)
	require.NoError(t, err)
	assert.Nil(t, got)

	first, err := svc.Grant(ctx, s.p, consent.GrantRequest{Key: rsKey, Scopes: []string{"openid", "sample:read"}, IP: "203.0.113.9", UserAgent: "UA"})
	require.NoError(t, err)
	assert.Equal(t, []string{"openid", "sample:read"}, first.Scopes)

	now = now.Add(time.Minute)
	second, err := svc.Grant(ctx, s.p, consent.GrantRequest{Key: rsKey, Scopes: []string{"sample:write", "openid"}, IP: "not an ip"})
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID, "the same row")
	assert.Equal(t, first.PublicID, second.PublicID)
	assert.Equal(t, []string{"openid", "sample:read", "sample:write"}, second.Scopes, "the union, stored order kept")
	assert.Equal(t, now, second.UpdatedAt)
	assert.Equal(t, first.GrantedAt, second.GrantedAt)

	_, err = svc.Grant(ctx, s.p, consent.GrantRequest{Key: oidcKey, Scopes: []string{"openid"}})
	require.NoError(t, err)
	_, err = svc.Grant(ctx, s.p, consent.GrantRequest{Key: oidcKey, Scopes: []string{"email"}})
	require.NoError(t, err)
	_, err = svc.Grant(ctx, s.p, consent.GrantRequest{Key: rsKey, Scopes: []string{"sample:new"}, Allowed: []string{"openid", "sample:new"}})
	require.NoError(t, err)
	pruned, err := svc.Find(ctx, s.p, rsKey)
	require.NoError(t, err)
	assert.Equal(t, []string{"openid", "sample:new"}, pruned.Scopes, "scopes gone from the catalog are dropped")
	oidc, err := svc.Find(ctx, s.p, oidcKey)
	require.NoError(t, err)
	assert.Equal(t, []string{"openid", "email"}, oidc.Scopes, "NULLS NOT DISTINCT: one OIDC-only row")
	var rows int
	require.NoError(t, sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM scope_consents WHERE project_id = $1`, s.p.ID).Scan(&rows))
	assert.Equal(t, 2, rows)

	other := seed(t, sqlDB)
	got, err = svc.Find(ctx, other.p, rsKey)
	require.NoError(t, err)
	assert.Nil(t, got, "RLS: another project's scope sees none of these rows")

	apps, err := svc.ConnectedApps(ctx, s.p, s.endUser)
	require.NoError(t, err)
	require.Len(t, apps, 1)
	assert.Len(t, apps[0].Resources, 2, "the resource server and the OIDC-only consent")
	none, err := svc.ConnectedApps(ctx, other.p, s.endUser)
	require.NoError(t, err)
	assert.Empty(t, none, "RLS: another project lists nothing")

	require.NoError(t, svc.ForgetClient(ctx, other.p, s.client), "another project's scope deletes nothing")
	got, err = svc.Find(ctx, s.p, oidcKey)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NoError(t, svc.ForgetClient(ctx, s.p, s.client))
	got, err = svc.Find(ctx, s.p, oidcKey)
	require.NoError(t, err)
	assert.Nil(t, got)
	_, err = svc.Grant(ctx, s.p, consent.GrantRequest{Key: oidcKey, Scopes: []string{"openid"}})
	require.NoError(t, err)

	_, err = sqlDB.ExecContext(ctx, `DELETE FROM oauth_clients WHERE id = $1`, s.client)
	require.NoError(t, err)
	got, err = svc.Find(ctx, s.p, oidcKey)
	require.NoError(t, err)
	assert.Nil(t, got, "a deleted client takes its consents")
}

type noGrants struct{}

func (noGrants) LiveFamilies(context.Context, consent.Project, uuid.UUID) ([]consent.Family, error) {
	return nil, nil
}

func (noGrants) RevokeClient(context.Context, consent.Project, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (noGrants) ResourceScopes(scopes []string) []string { return scopes }

// Disconnect deletes every consent of the client across resource servers in its unit of work, and a second one is ACC003.
func TestPostgres_Disconnect(t *testing.T) {
	pool, sqlDB, schemaName := pgtest.MigratedRLS(t)
	svc := consent.NewService(consent.NewStore(pool, schemaName, ""), db.NewTransactor(pool), noGrants{}, apperror.NewReporter(nil, false).Unexpected)
	s := seed(t, sqlDB)
	ctx := t.Context()
	for _, k := range []consent.Key{
		{EndUserID: s.endUser, ClientID: s.client, ResourceServerID: &s.server}, {EndUserID: s.endUser, ClientID: s.client},
	} {
		_, err := svc.Grant(ctx, s.p, consent.GrantRequest{Key: k, Scopes: []string{"openid"}})
		require.NoError(t, err)
	}
	require.NoError(t, svc.Disconnect(ctx, s.p, consent.Connection{EndUserID: s.endUser, ClientID: s.client}))
	var rows int
	require.NoError(t, sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM scope_consents WHERE project_id = $1`, s.p.ID).Scan(&rows))
	assert.Zero(t, rows)
	assert.True(t, consent.IsAppNotConnectedError(svc.Disconnect(ctx, s.p, consent.Connection{EndUserID: s.endUser, ClientID: s.client})))
}
