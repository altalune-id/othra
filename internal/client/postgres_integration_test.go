//go:build integration

package client_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/client"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/kernel/secret"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/testutil/fakes"
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
	issuer := &failAfterLock{projects: project.NewService(project.NewStore(pool, schemaName, ""), fakes.Transactor{}, fakes.NoMemberships{}, nil, "", unexpected)}
	svc := client.NewService(client.NewStore(pool, schemaName, ""), db.NewTransactor(pool), issuer, docFetcher{}, client.MetadataConfig{}, unexpected)
	return &pgFixture{db: sqlDB, pool: pool, schema: schemaName, svc: svc, issuer: issuer}
}

func (f *pgFixture) withMetadata(fetcher client.MetadataFetcher, cfg client.MetadataConfig) *client.Service {
	return client.NewService(client.NewStore(f.pool, f.schema, ""), db.NewTransactor(f.pool), f.issuer, fetcher, cfg,
		apperror.NewReporter(nil, false).Unexpected).WithLinks(noLinks{})
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
	assert.True(t, secret.Matches(hash, first))
	authn, err := f.svc.Authenticate(ctx, org, prj, c.PublicID)
	require.NoError(t, err)
	assert.True(t, authn.VerifySecret(first, time.Now()), "the authentication read carries the hashes")
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

type noLinks struct{}

func (noLinks) LinkMetadataClient(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, []string) error {
	return nil
}

type docFetcher struct{ name string }

func (d docFetcher) FetchMetadata(context.Context, string) (*client.FetchedMetadata, error) {
	body := `{"client_id":"https://claude.ai/oauth/claude-code-client-metadata","client_name":"` + d.name +
		`","redirect_uris":["http://localhost/callback"],"grant_types":["authorization_code","refresh_token"],"token_endpoint_auth_method":"none"}`
	return &client.FetchedMetadata{Body: []byte(body), ContentType: "application/json", CacheControl: "max-age=1"}, nil
}

// a shadow client is one row per project and URL, read by its URL, hidden from the list, and passes the 026 checks.
func TestPostgres_ShadowClient(t *testing.T) {
	f := newPGFixture(t)
	org, prj := f.seed(t)
	ctx := t.Context()
	const metadataURL = "https://claude.ai/oauth/claude-code-client-metadata"
	cfg := client.MetadataConfig{Cache: client.MetadataCache{Min: 0, Max: time.Hour, Default: time.Hour}, IssuerHost: "othra.example.com"}
	first, err := f.withMetadata(docFetcher{name: "Claude Code"}, cfg).Admit(ctx, org, prj, client.AdmitRequest{URL: metadataURL, ResourceServerID: uuid.New()})
	require.NoError(t, err)
	time.Sleep(1100 * time.Millisecond)
	second, err := f.withMetadata(docFetcher{name: "Claude Code 2"}, cfg).Admit(ctx, org, prj, client.AdmitRequest{URL: metadataURL, ResourceServerID: uuid.New()})
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID)
	assert.Equal(t, first.PublicID, second.PublicID)
	assert.Equal(t, 1, f.count(t, "oauth_clients", prj))

	got, err := f.svc.Get(ctx, org, prj, metadataURL)
	require.NoError(t, err)
	assert.Equal(t, "Claude Code 2", got.Name)
	assert.Equal(t, metadataURL, got.OAuthID())
	require.NotNil(t, got.MetadataExpiresAt)
	byID, err := f.svc.Authenticate(ctx, org, prj, got.PublicID)
	require.NoError(t, err)
	assert.Equal(t, got.ID, byID.ID)
	list, err := f.svc.List(ctx, org, prj)
	require.NoError(t, err)
	assert.Empty(t, list)
	var kind string
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT registration_kind FROM oauth_clients WHERE id = $1`, got.ID).Scan(&kind))
	assert.Equal(t, "cimd", kind)
}

// an expired shadow with no consent and no live grant is pruned; one with a consent stays.
func TestPostgres_PruneShadows(t *testing.T) {
	f := newPGFixture(t)
	org, prj := f.seed(t)
	ctx := t.Context()
	insert := func(url string, expires time.Time) uuid.UUID {
		id := uuid.New()
		_, err := f.db.ExecContext(ctx, `INSERT INTO oauth_clients (id, public_id, org_id, project_id, name, client_type, application_type,
			token_endpoint_auth_method, grant_types, allowed_scopes, redirect_uris, consent_policy, registration_kind, metadata_url,
			metadata_fetched_at, metadata_expires_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'C', 'public', 'native', 'none', '{authorization_code}', '{openid}', '{http://localhost/callback}',
			'require', 'cimd', $5, $6, $6, now(), now())`, id, "c"+strings.ReplaceAll(uuid.NewString(), "-", "")[:20], org, prj, url, expires)
		require.NoError(t, err)
		return id
	}
	old := time.Now().Add(-8 * 24 * time.Hour)
	unused := insert("https://a.example.com/c.json", old)
	consented := insert("https://b.example.com/c.json", old)
	fresh := insert("https://c.example.com/c.json", time.Now().Add(time.Hour))
	endUser := uuid.New()
	_, err := f.db.ExecContext(ctx, `INSERT INTO end_users (id, public_id, org_id, project_id, email, created_at, updated_at) VALUES ($1, $2, $3, $4, 'e@example.com', now(), now())`,
		endUser, "e"+endUser.String()[:12], org, prj)
	require.NoError(t, err)
	_, err = f.db.ExecContext(ctx, `INSERT INTO scope_consents (id, public_id, org_id, project_id, end_user_id, client_id, scopes, granted_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, '{openid}', now(), now())`, uuid.New(), "k"+uuid.NewString()[:12], org, prj, endUser, consented)
	require.NoError(t, err)

	n, err := f.svc.PruneShadows(ctx, org, prj)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	var left []uuid.UUID
	rows, err := f.db.QueryContext(ctx, `SELECT id FROM oauth_clients WHERE project_id = $1 ORDER BY metadata_url`, prj)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id uuid.UUID
		require.NoError(t, rows.Scan(&id))
		left = append(left, id)
	}
	assert.Equal(t, []uuid.UUID{consented, fresh}, left)
	assert.NotContains(t, left, unused)
}
