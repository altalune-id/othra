//go:build integration

package resourceserver_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/resourceserver"
	"altalune.id/othra/internal/testutil/fakes"
	"altalune.id/othra/internal/testutil/pgtest"
)

const issuerBase = "https://othra.example.com"

type issuers struct{ projects *project.Service }

func (i issuers) Project(ctx context.Context, id uuid.UUID) (resourceserver.ProjectRef, error) {
	p, err := i.projects.Get(ctx, id)
	if err != nil {
		return resourceserver.ProjectRef{}, err
	}
	return resourceserver.ProjectRef{PublicID: p.PublicID, Issuer: i.projects.IssuerURL(p.IssuerHandle)}, nil
}

type people struct{}

func (people) UserPublicID(context.Context, uuid.UUID) (string, error) {
	return "usrusrusrusrusrusrusr", nil
}

func (i issuers) LockIssuerHandle(ctx context.Context, id uuid.UUID) error {
	return i.projects.LockIssuerHandle(ctx, id)
}

type clients struct{ db *sql.DB }

func (c clients) Lookup(ctx context.Context, _, projectID uuid.UUID, publicID string) (resourceserver.ClientRef, bool, error) {
	ref := resourceserver.ClientRef{PublicID: publicID}
	err := c.db.QueryRowContext(ctx, `SELECT id, name FROM oauth_clients WHERE project_id = $1 AND public_id = $2`, projectID, publicID).
		Scan(&ref.ID, &ref.Name)
	if err == sql.ErrNoRows {
		return ref, false, nil
	}
	return ref, err == nil, err
}

func (c clients) Refs(ctx context.Context, _, projectID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]resourceserver.ClientRef, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT id, public_id, name, coalesce(metadata_url, '') FROM oauth_clients WHERE project_id = $1`, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[uuid.UUID]resourceserver.ClientRef{}
	for rows.Next() {
		var r resourceserver.ClientRef
		if err := rows.Scan(&r.ID, &r.PublicID, &r.Name, &r.MetadataURL); err != nil {
			return nil, err
		}
		for _, id := range ids {
			if id == r.ID {
				out[id] = r
			}
		}
	}
	return out, rows.Err()
}

type passing struct{ issuers []string }

func (p *passing) Check(_ context.Context, _, issuer string) *resourceserver.Failure {
	p.issuers = append(p.issuers, issuer)
	return nil
}

type pgFixture struct {
	schema  string
	db      *sql.DB
	pool    db.Pool
	svc     *resourceserver.Service
	checker *passing
}

func newPGFixture(t *testing.T) *pgFixture {
	t.Helper()
	pool, sqlDB, schemaName := pgtest.MigratedRLS(t)
	unexpected := apperror.NewReporter(nil, false).Unexpected
	projects := project.NewService(project.NewStore(pool, schemaName, ""), fakes.Transactor{}, fakes.NoMemberships{}, nil, issuerBase, unexpected)
	f := &pgFixture{schema: schemaName, db: sqlDB, pool: pool, checker: &passing{}}
	f.svc = resourceserver.NewService(resourceserver.Deps{
		Store: resourceserver.NewStore(pool, schemaName, ""), Transactor: db.NewTransactor(pool), Issuers: issuers{projects: projects}, People: people{},
		Clients: clients{db: sqlDB}, Checker: f.checker, Unexpected: unexpected,
	}, resourceserver.Config{Interval: time.Hour, StaleAfter: 24 * time.Hour, TrustAvailable: true})
	return f
}

func (f *pgFixture) seed(t *testing.T) (org, project uuid.UUID) {
	t.Helper()
	org, project = uuid.New(), uuid.New()
	now := time.Now().UTC()
	_, err := f.db.ExecContext(t.Context(), `INSERT INTO orgs (id, public_id, slug, name, created_at, updated_at) VALUES ($1, $2, $2, 'Org', $3, $3)`,
		org, "o"+org.String()[:12], now)
	require.NoError(t, err)
	f.project(t, org, project)
	return org, project
}

func (f *pgFixture) project(t *testing.T, org, project uuid.UUID) {
	t.Helper()
	_, err := f.db.ExecContext(t.Context(), `INSERT INTO projects (id, public_id, org_id, slug, name, issuer_handle, created_at, updated_at) VALUES ($1, $2, $3, $2, 'P', $4, now(), now())`,
		project, "p"+project.String()[:12], org, "h"+project.String()[:8])
	require.NoError(t, err)
}

func (f *pgFixture) client(t *testing.T, org, project uuid.UUID, name string) resourceserver.ClientRef {
	t.Helper()
	c := resourceserver.ClientRef{ID: uuid.New(), PublicID: "c" + strings.ReplaceAll(uuid.NewString(), "-", "")[:20], Name: name}
	_, err := f.db.ExecContext(t.Context(), `INSERT INTO oauth_clients (id, public_id, org_id, project_id, name, client_type, application_type,
		token_endpoint_auth_method, grant_types, allowed_scopes, redirect_uris, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'public', 'native', 'none', '{authorization_code}', '{openid}', '{http://127.0.0.1/callback}', now(), now())`,
		c.ID, c.PublicID, org, project, name)
	require.NoError(t, err)
	return c
}

func (f *pgFixture) lockedAt(t *testing.T, project uuid.UUID) *time.Time {
	t.Helper()
	var at *time.Time
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT issuer_handle_locked_at FROM projects WHERE id = $1`, project).Scan(&at))
	return at
}

func create(t *testing.T, f *pgFixture, org, project uuid.UUID, identifier string, scopes ...string) *resourceserver.ResourceServer {
	t.Helper()
	r, err := f.svc.Create(t.Context(), resourceserver.CreateRequest{
		OrgID: org, ProjectID: project, Name: "API", Identifier: identifier, Scopes: scopes, Person: uuid.New(),
	})
	require.NoError(t, err)
	return r
}

func TestPostgres_Lifecycle(t *testing.T) {
	f := newPGFixture(t)
	org, project := f.seed(t)
	ctx := t.Context()

	r := create(t, f, org, project, "https://api.example.com/mcp", "devices:read", "odd,{scope}")
	got, err := f.svc.Get(ctx, org, project, r.PublicID)
	require.NoError(t, err)
	assert.Equal(t, []string{"devices:read", "odd,{scope}"}, got.Scopes, "scopes round-trip through text[]")
	assert.Equal(t, resourceserver.StatePending, got.State)
	assert.Nil(t, got.LastError)
	assert.Equal(t, uuid.Nil, got.TrustedBy)

	_, err = f.svc.Create(ctx, resourceserver.CreateRequest{OrgID: org, ProjectID: project, Name: "Again", Identifier: r.Identifier, Scopes: []string{"x"}})
	assert.True(t, resourceserver.IsIdentifierTakenError(err), "%v", err)

	verified, failure, err := f.svc.Verify(ctx, org, project, r.PublicID)
	require.NoError(t, err)
	require.Nil(t, failure)
	assert.Equal(t, resourceserver.StateVerified, verified.State)
	assert.Equal(t, []string{issuerBase + "/p/h" + project.String()[:8]}, f.checker.issuers, "the check names the project's issuer")

	_, err = f.db.ExecContext(ctx, `UPDATE resource_servers SET last_checked_at = now() - interval '2 days' WHERE id = $1`, r.ID)
	require.NoError(t, err)
	f.checker.issuers = nil
	n, err := f.svc.Recheck(ctx, org, project)
	require.NoError(t, err)
	assert.Zero(t, n, "still verified")
	assert.Len(t, f.checker.issuers, 1, "a stale server is re-checked")

	trusted, err := f.svc.SetTrust(ctx, org, project, r.PublicID, true, uuid.New())
	require.NoError(t, err)
	reread, err := f.svc.Get(ctx, org, project, r.PublicID)
	require.NoError(t, err)
	assert.True(t, reread.Trusted)
	assert.Equal(t, trusted.TrustedBy, reread.TrustedBy)
	_, err = f.svc.SetTrust(ctx, org, project, r.PublicID, false, uuid.New())
	require.NoError(t, err)
	reread, err = f.svc.Get(ctx, org, project, r.PublicID)
	require.NoError(t, err)
	assert.False(t, reread.Trusted)
	assert.Equal(t, resourceserver.StatePending, reread.State)

	updated, err := f.svc.Update(ctx, org, project, r.PublicID, "Renamed", []string{"devices:read"}, nil)
	require.NoError(t, err)
	assert.Equal(t, "Renamed", updated.Name)

	list, err := f.svc.List(ctx, org, project)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.NoError(t, f.svc.Delete(ctx, org, project, r.PublicID))
	_, err = f.svc.Get(ctx, org, project, r.PublicID)
	assert.True(t, resourceserver.IsNotFoundError(err))
}

func TestPostgres_LastErrorRoundTrips(t *testing.T) {
	f := newPGFixture(t)
	org, project := f.seed(t)
	r := create(t, f, org, project, "https://api.example.com", "read")
	_, err := f.db.ExecContext(t.Context(), `UPDATE resource_servers SET verification_state = 'verified', verified_at = now(),
		consecutive_failures = 2, last_checked_at = now() - interval '2 days',
		last_error = '{"reason":"status","status":503,"checked_at":"2026-10-20T09:00:00Z"}' WHERE id = $1`, r.ID)
	require.NoError(t, err)
	got, err := f.svc.Get(t.Context(), org, project, r.PublicID)
	require.NoError(t, err)
	require.NotNil(t, got.LastError)
	assert.Equal(t, resourceserver.FailureStatus, got.LastError.Reason)
	assert.Equal(t, int32(503), got.LastError.Status)
	assert.Equal(t, time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC), got.LastError.CheckedAt)

	failing := resourceserver.NewService(resourceserver.Deps{
		Store: resourceserver.NewStore(f.pool, f.schema, ""), Transactor: db.NewTransactor(f.pool), Issuers: staticIssuer{}, People: people{},
		Clients: clients{db: f.db}, Checker: failingChecker{}, Unexpected: apperror.NewReporter(nil, false).Unexpected,
	}, resourceserver.Config{Interval: time.Hour, StaleAfter: 24 * time.Hour})
	n, err := failing.Recheck(t.Context(), org, project)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	got, err = f.svc.Get(t.Context(), org, project, r.PublicID)
	require.NoError(t, err)
	assert.Equal(t, resourceserver.StateFailed, got.State)
	assert.Equal(t, int32(3), got.ConsecutiveFailures)
	assert.Equal(t, resourceserver.FailureTimeout, got.LastError.Reason)
}

type staticIssuer struct{}

func (staticIssuer) Project(context.Context, uuid.UUID) (resourceserver.ProjectRef, error) {
	return resourceserver.ProjectRef{PublicID: "prjprjprjprjprjprjprj", Issuer: issuerBase}, nil
}
func (staticIssuer) LockIssuerHandle(context.Context, uuid.UUID) error { return nil }

type failingChecker struct{}

func (failingChecker) Check(context.Context, string, string) *resourceserver.Failure {
	return &resourceserver.Failure{Reason: resourceserver.FailureTimeout, CheckedAt: time.Now().UTC()}
}

// the first resource server locks the issuer handle in its own transaction; a failing insert leaves it unlocked.
func TestPostgres_FirstServerLocksTheIssuerHandle(t *testing.T) {
	f := newPGFixture(t)
	org, project := f.seed(t)
	_, err := f.svc.Create(t.Context(), resourceserver.CreateRequest{
		OrgID: org, ProjectID: project, Name: strings.Repeat("n", 101), Identifier: "https://api.example.com", Scopes: []string{"read"},
	})
	require.Error(t, err, "the name breaks the table's check")
	assert.Nil(t, f.lockedAt(t, project), "the failed insert rolled the lock back")

	create(t, f, org, project, "https://api.example.com", "read")
	first := f.lockedAt(t, project)
	require.NotNil(t, first)
	create(t, f, org, project, "https://b.example.com", "read")
	assert.Equal(t, *first, *f.lockedAt(t, project), "a locked handle keeps its first lock time")
}

func TestPostgres_Links(t *testing.T) {
	f := newPGFixture(t)
	org, project := f.seed(t)
	ctx := t.Context()
	a := create(t, f, org, project, "https://a.example.com", "read", "write")
	b := create(t, f, org, project, "https://b.example.com", "sync")
	web := f.client(t, org, project, "Web")
	cli := f.client(t, org, project, "CLI")

	_, err := f.svc.LinkClient(ctx, org, project, web.PublicID, a.PublicID, []string{"read"})
	require.NoError(t, err)
	l, err := f.svc.LinkClient(ctx, org, project, web.PublicID, a.PublicID, []string{"write"})
	require.NoError(t, err)
	assert.Equal(t, []string{"write"}, l.Scopes, "linking again replaces the scopes")
	assert.Equal(t, "Web", l.Client.Name)
	_, err = f.svc.LinkClient(ctx, org, project, web.PublicID, b.PublicID, []string{"sync"})
	require.NoError(t, err)
	_, err = f.svc.LinkClient(ctx, org, project, cli.PublicID, a.PublicID, []string{"write"})
	require.NoError(t, err)

	l, err = f.svc.UpdateLink(ctx, org, project, web.PublicID, a.PublicID, []string{"read", "write"})
	require.NoError(t, err)
	assert.Equal(t, []string{"read", "write"}, l.Scopes)
	_, err = f.svc.UpdateLink(ctx, org, project, cli.PublicID, b.PublicID, []string{"sync"})
	assert.True(t, resourceserver.IsNotFoundError(err), "%v", err)

	byClient, err := f.svc.ListLinks(ctx, org, project, web.PublicID, "")
	require.NoError(t, err)
	require.Len(t, byClient, 2)
	assert.Equal(t, a.PublicID, byClient[0].ResourceServer.PublicID)
	assert.Equal(t, b.PublicID, byClient[1].ResourceServer.PublicID)
	byServer, err := f.svc.ListLinks(ctx, org, project, "", a.PublicID)
	require.NoError(t, err)
	assert.Len(t, byServer, 2)

	_, err = f.svc.Update(ctx, org, project, a.PublicID, "API", []string{"read"}, nil)
	require.True(t, resourceserver.IsScopePinnedError(err), "%v", err)
	assert.Contains(t, apperrorMeta(err)["clients"], "Web")
	assert.Contains(t, apperrorMeta(err)["clients"], "CLI")
	err = f.svc.Delete(ctx, org, project, a.PublicID)
	require.True(t, resourceserver.IsLinkedError(err), "%v", err)

	_, err = f.db.ExecContext(ctx, `DELETE FROM resource_servers WHERE id = $1`, a.ID)
	assert.True(t, db.IsForeignKeyViolation(err), "the table refuses a linked delete too: %v", err)

	_, err = f.db.ExecContext(ctx, `DELETE FROM oauth_clients WHERE id = $1`, cli.ID)
	require.NoError(t, err)
	byServer, err = f.svc.ListLinks(ctx, org, project, "", a.PublicID)
	require.NoError(t, err)
	assert.Len(t, byServer, 1, "a deleted client takes its links with it")

	require.NoError(t, f.svc.UnlinkClient(ctx, org, project, web.PublicID, a.PublicID))
	require.NoError(t, f.svc.UnlinkClient(ctx, org, project, web.PublicID, b.PublicID))
	require.NoError(t, f.svc.Delete(ctx, org, project, a.PublicID))

	_, err = f.db.ExecContext(ctx, `DELETE FROM projects WHERE id = $1`, project)
	require.NoError(t, err, "a project delete cascades through servers and links")
}

func apperrorMeta(err error) map[string]string {
	if app, ok := apperror.AsAppError(err); ok {
		return app.Meta()
	}
	return nil
}

// SECURITY: a link between two projects is impossible in the database, and one project's scope sees no other project's rows.
func TestPostgres_ProjectsStayApart(t *testing.T) {
	f := newPGFixture(t)
	org, project := f.seed(t)
	other := uuid.New()
	f.project(t, org, other)
	a := create(t, f, org, project, "https://api.example.com", "read")
	b := create(t, f, org, other, "https://api.example.com", "read")
	assert.NotEqual(t, a.ID, b.ID, "the same identifier in two projects")
	otherClient := f.client(t, org, other, "Other")

	_, err := f.db.ExecContext(t.Context(), `INSERT INTO oauth_client_resources (org_id, project_id, client_id, resource_server_id, scopes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, '{read}', now(), now())`, org, project, otherClient.ID, a.ID)
	assert.True(t, db.IsForeignKeyViolation(err), "a client of another project: %v", err)

	_, err = f.svc.LinkClient(t.Context(), org, project, otherClient.PublicID, a.PublicID, []string{"read"})
	assert.True(t, resourceserver.IsClientNotFoundError(err), "%v", err)
	_, err = f.svc.Get(t.Context(), org, other, a.PublicID)
	assert.True(t, resourceserver.IsNotFoundError(err), "%v", err)

	scoped := tenant.Into(t.Context(), tenant.Context{OrgID: org, ProjectID: other})
	store := resourceserver.NewStore(f.pool, f.schema, "")
	got, err := store.List(scoped, project)
	require.NoError(t, err)
	assert.Empty(t, got, "another project's scope reads none of these rows")
	links, err := store.Links(scoped, project, uuid.Nil, a.ID)
	require.NoError(t, err)
	assert.Empty(t, links)
}

// the table holds an http identifier only while it is trusted.
func TestPostgres_AnHTTPIdentifierNeedsTrust(t *testing.T) {
	f := newPGFixture(t)
	org, project := f.seed(t)
	insert := `INSERT INTO resource_servers (id, public_id, org_id, project_id, name, identifier, scopes, trusted, trusted_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'API', $5, '{read}', $6, $7, now(), now())`
	pub := func() string { return "r" + strings.ReplaceAll(uuid.NewString(), "-", "")[:20] }
	var trustedAt *time.Time
	_, err := f.db.ExecContext(t.Context(), insert, uuid.New(), pub(), org, project, "http://127.0.0.1:8080", false, trustedAt)
	require.Error(t, err)
	name, ok := db.ViolatedConstraint(err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, "resource_servers_https_or_trusted", name)

	now := time.Now().UTC()
	_, err = f.db.ExecContext(t.Context(), insert, uuid.New(), pub(), org, project, "http://127.0.0.1:8080", true, &now)
	require.NoError(t, err)
	_, err = f.db.ExecContext(t.Context(), `UPDATE resource_servers SET trusted = false, trusted_at = NULL WHERE identifier = 'http://127.0.0.1:8080'`)
	assert.Error(t, err, "trust can't leave an http identifier")
}

type mismatchChecker struct{}

func (mismatchChecker) Check(context.Context, string, string) *resourceserver.Failure {
	return &resourceserver.Failure{Reason: resourceserver.FailureResourceMismatch, Resource: "https://elsewhere", CheckedAt: time.Now().UTC()}
}

// a pending server whose document names another resource is saved failed, with no verified_at.
func TestPostgres_AMismatchFailsAPendingServer(t *testing.T) {
	f := newPGFixture(t)
	org, project := f.seed(t)
	r := create(t, f, org, project, "https://api.example.com", "read")
	svc := resourceserver.NewService(resourceserver.Deps{
		Store: resourceserver.NewStore(f.pool, f.schema, ""), Transactor: db.NewTransactor(f.pool), Issuers: staticIssuer{}, People: people{},
		Clients: clients{db: f.db}, Checker: mismatchChecker{}, Unexpected: apperror.NewReporter(nil, false).Unexpected,
	}, resourceserver.Config{Interval: time.Hour, StaleAfter: 24 * time.Hour})
	n, err := svc.Recheck(t.Context(), org, project)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	got, err := f.svc.Get(t.Context(), org, project, r.PublicID)
	require.NoError(t, err)
	assert.Equal(t, resourceserver.StateFailed, got.State)
	assert.Nil(t, got.VerifiedAt)
	assert.Equal(t, "https://elsewhere", got.LastError.Resource)
}

// 026: admission and scope descriptions persist; Resolve and Admits read them; turning admission off is immediate.
func TestPostgres_AdmissionAndDescriptions(t *testing.T) {
	f := newPGFixture(t)
	org, project := f.seed(t)
	ctx := t.Context()
	r, err := f.svc.Create(ctx, resourceserver.CreateRequest{
		OrgID: org, ProjectID: project, Name: "MCP", Identifier: "https://mcp.example.com/mcp", Scopes: []string{"read", "write"},
		Descriptions: map[string]string{"read": "Read your messages"},
	})
	require.NoError(t, err)
	_, f2, err := f.svc.Verify(ctx, org, project, r.PublicID)
	require.NoError(t, err)
	require.Nil(t, f2)

	got, err := f.svc.SetAdmission(ctx, org, project, r.PublicID,
		resourceserver.Admission{Accept: true, Hosts: []string{"claude.ai"}, Scopes: []string{"read"}})
	require.NoError(t, err)
	reread, err := f.svc.Get(ctx, org, project, r.PublicID)
	require.NoError(t, err)
	assert.Equal(t, got.Admission, reread.Admission)
	assert.Equal(t, map[string]string{"read": "Read your messages"}, reread.Descriptions)

	c := f.client(t, org, project, "Claude Code")
	require.NoError(t, f.svc.LinkMetadataClient(ctx, org, project, c.ID, r.ID, reread.MetadataClientScopes()))
	aud, ok, err := f.svc.Admits(ctx, org, project, r.ID, c.ID, "claude.ai", []string{"read"})
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, r.Identifier, aud)
	_, pin, ok, err := f.svc.Resolve(ctx, org, project, c.ID, r.Identifier)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, []string{"read"}, pin)
	accepts, err := f.svc.AcceptsCIMD(ctx, org, project)
	require.NoError(t, err)
	assert.True(t, accepts)

	all, err := f.svc.SetAdmission(ctx, org, project, r.PublicID, resourceserver.Admission{})
	require.NoError(t, err)
	assert.Nil(t, all.Admission.Scopes)
	reread, err = f.svc.Get(ctx, org, project, r.PublicID)
	require.NoError(t, err)
	assert.Equal(t, resourceserver.Admission{}, reread.Admission, "NULL scopes read back as the whole catalog")
	_, ok, err = f.svc.Admits(ctx, org, project, r.ID, c.ID, "claude.ai", []string{"read"})
	require.NoError(t, err)
	assert.False(t, ok, "admission off: refresh refused")
}

// introspection secrets authenticate per server and project, rotate with grace, revoke, and go with their server.
func TestPostgres_IntrospectionSecrets(t *testing.T) {
	f := newPGFixture(t)
	org, project := f.seed(t)
	ctx := t.Context()
	r := create(t, f, org, project, "https://mcp.example.com/mcp", "read")
	auth := func(publicID, presented string) bool {
		t.Helper()
		_, ok, err := f.svc.Authenticate(ctx, org, project, publicID, presented)
		require.NoError(t, err)
		return ok
	}

	_, err := f.svc.SetPersonalTokenAccess(ctx, org, project, r.PublicID, true)
	assert.True(t, resourceserver.IsAdmissionUnverifiedError(err), "a pending server takes no personal tokens")
	_, _, err = f.svc.Verify(ctx, org, project, r.PublicID)
	require.NoError(t, err)
	_, err = f.svc.SetPersonalTokenAccess(ctx, org, project, r.PublicID, true)
	assert.True(t, resourceserver.IsIntrospectionRequiredError(err), "no credential yet")

	got, first, err := f.svc.GenerateIntrospectionSecret(ctx, org, project, r.PublicID)
	require.NoError(t, err)
	require.Len(t, got.IntrospectionSecrets, 1)
	assert.True(t, strings.HasPrefix(first, resourceserver.IntrospectionSecretPrefix))
	_, _, err = f.svc.GenerateIntrospectionSecret(ctx, org, project, r.PublicID)
	assert.True(t, resourceserver.IsIntrospectionSecretExistsError(err))
	assert.True(t, auth(r.PublicID, first))
	assert.False(t, auth(r.PublicID, first+"x"))
	assert.False(t, auth("r-unknown", first))

	got, err = f.svc.SetPersonalTokenAccess(ctx, org, project, r.PublicID, true)
	require.NoError(t, err)
	assert.True(t, got.TakesPersonalTokens(time.Now(), true))

	_, second, err := f.svc.RotateIntrospectionSecret(ctx, org, project, r.PublicID, time.Hour)
	require.NoError(t, err)
	assert.True(t, auth(r.PublicID, first), "the old secret works through its grace")
	assert.True(t, auth(r.PublicID, second))
	got, third, err := f.svc.RotateIntrospectionSecret(ctx, org, project, r.PublicID, 0)
	require.NoError(t, err)
	assert.False(t, auth(r.PublicID, first), "a secret already in its grace ends at once")
	assert.False(t, auth(r.PublicID, second), "no grace")
	assert.True(t, auth(r.PublicID, third))
	require.Len(t, got.IntrospectionSecrets, 1)

	_, err = f.svc.RevokeIntrospectionSecret(ctx, org, project, r.PublicID, got.IntrospectionSecrets[0].PublicID)
	require.NoError(t, err)
	assert.False(t, auth(r.PublicID, third))
	got, err = f.svc.Get(ctx, org, project, r.PublicID)
	require.NoError(t, err)
	assert.False(t, got.TakesPersonalTokens(time.Now(), true), "no live secret: closed to personal tokens")

	otherOrg, otherProject := f.seed(t)
	_, fourth, err := f.svc.GenerateIntrospectionSecret(ctx, org, project, r.PublicID)
	require.NoError(t, err)
	_, ok, err := f.svc.Authenticate(ctx, otherOrg, otherProject, r.PublicID, fourth)
	require.NoError(t, err)
	assert.False(t, ok, "another project's endpoint")

	require.NoError(t, f.svc.Delete(ctx, org, project, r.PublicID))
	var left int
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT count(*) FROM resource_server_secrets WHERE resource_server_id = $1`, r.ID).Scan(&left))
	assert.Zero(t, left, "deleting the server deletes its secrets")
}

// a server with an admitted metadata document client deletes cleanly, its link with it; never a raw SQL error.
func TestPostgres_DeleteWithACIMDLink(t *testing.T) {
	f := newPGFixture(t)
	org, project := f.seed(t)
	ctx := t.Context()
	r := create(t, f, org, project, "https://mcp.example.com/mcp", "read")
	cimd := uuid.New()
	_, err := f.db.ExecContext(ctx, `INSERT INTO oauth_clients (id, public_id, org_id, project_id, name, client_type, application_type,
		token_endpoint_auth_method, grant_types, allowed_scopes, redirect_uris, consent_policy, registration_kind, metadata_url,
		metadata_fetched_at, metadata_expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'Claude Code', 'public', 'native', 'none', '{authorization_code}', '{openid}', '{http://127.0.0.1/callback}',
		'require', 'cimd', 'https://claude.ai/oauth/claude-code-client-metadata', now(), now() + interval '1 day', now(), now())`,
		cimd, "c"+strings.ReplaceAll(uuid.NewString(), "-", "")[:20], org, project)
	require.NoError(t, err)
	require.NoError(t, f.svc.LinkMetadataClient(ctx, org, project, cimd, r.ID, []string{"read"}))

	require.NoError(t, f.svc.Delete(ctx, org, project, r.PublicID))
	var left int
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT count(*) FROM oauth_client_resources WHERE resource_server_id = $1`, r.ID).Scan(&left))
	assert.Zero(t, left)
	var clients int
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT count(*) FROM oauth_clients WHERE id = $1`, cimd).Scan(&clients))
	assert.Equal(t, 1, clients, "the client stays; the shadow prune owns it")
}
