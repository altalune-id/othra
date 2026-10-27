//go:build integration

package project_test

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/kernel/storage"
	"altalune.id/othra/internal/kernel/storage/memory"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/org"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/testutil/fakes"
	"altalune.id/othra/internal/testutil/pgtest"
	"altalune.id/othra/nanoid"
)

type projectFixture struct {
	sqlDB    *sql.DB
	pool     db.Pool
	orgs     org.Store
	projects project.Store
	files    storage.Store
}

func newProjectFixture(t *testing.T) *projectFixture {
	t.Helper()
	pool, sqlDB, schemaName := pgtest.MigratedRLS(t)
	return &projectFixture{
		sqlDB:    sqlDB,
		pool:     pool,
		orgs:     org.NewStore(pool, schemaName, ""),
		projects: project.NewStore(pool, schemaName, ""),
		files:    storage.NewStore(pool, schemaName, ""),
	}
}

func (f *projectFixture) seedOrg(t *testing.T, slug string) uuid.UUID {
	t.Helper()
	o, err := org.NewOrg(slug, "Org "+slug, uuid.Nil)
	require.NoError(t, err)
	require.NoError(t, f.orgs.Save(tenant.Into(t.Context(), tenant.Context{OrgID: o.ID}), o))
	return o.ID
}

func newProject(t *testing.T, orgID uuid.UUID, slug string, createdBy uuid.UUID) *project.Project {
	t.Helper()
	p, err := project.NewProject(orgID, slug, "Project "+slug, createdBy)
	require.NoError(t, err)
	p.CreatedAt = time.Now().UTC().Truncate(time.Microsecond)
	p.UpdatedAt = p.CreatedAt
	return p
}

func slugsOf(ps []*project.Project) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Slug)
	}
	return out
}

func TestProjectStore_RoundTrip(t *testing.T) {
	cases := []struct {
		name      string
		createdBy uuid.UUID
	}{
		{name: "created_by NULL", createdBy: uuid.Nil},
		{name: "created_by set", createdBy: uuid.New()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newProjectFixture(t)
			ctx := t.Context()
			orgID := f.seedOrg(t, "acme")
			p := newProject(t, orgID, "web", tc.createdBy)
			p.CreatedAt = time.Date(2024, 2, 29, 23, 59, 58, 123456000, time.FixedZone("WIB", 7*3600))

			before := time.Now()
			require.NoError(t, f.projects.Save(tenant.Into(ctx, tenant.Context{OrgID: p.OrgID}), p))

			byID, err := f.projects.ByID(tenant.Into(ctx, tenant.Context{OrgID: p.OrgID}), p.ID)
			require.NoError(t, err)
			bySlug, err := f.projects.BySlug(tenant.Into(ctx, tenant.Context{OrgID: orgID}), orgID, "web")
			require.NoError(t, err)
			assert.Equal(t, byID, bySlug)

			assert.Equal(t, p.ID, byID.ID)
			assert.Equal(t, p.PublicID, byID.PublicID)
			assert.Equal(t, orgID, byID.OrgID)
			assert.Equal(t, "web", byID.Slug)
			assert.Equal(t, "Project web", byID.Name)
			assert.Equal(t, p.CreatedBy, byID.CreatedBy)
			assert.Equal(t, tc.createdBy != uuid.Nil, byID.CreatedBy.Valid)
			assert.True(t, p.CreatedAt.Equal(byID.CreatedAt), "created_at %v != %v", byID.CreatedAt, p.CreatedAt)
			assert.WithinDuration(t, before, byID.UpdatedAt, 5*time.Second, "updated_at is stamped by the store")
		})
	}
}

func TestProjectStore_SaveUpdatesMutableColumnsOnly(t *testing.T) {
	f := newProjectFixture(t)
	ctx := t.Context()
	orgA, orgB := f.seedOrg(t, "acme"), f.seedOrg(t, "globex")
	creator := uuid.New()
	p := newProject(t, orgA, "web", creator)
	require.NoError(t, f.projects.Save(tenant.Into(ctx, tenant.Context{OrgID: p.OrgID}), p))
	first, err := f.projects.ByID(tenant.Into(ctx, tenant.Context{OrgID: p.OrgID}), p.ID)
	require.NoError(t, err)

	changed := *p
	changed.Slug = "web-renamed"
	changed.Name = "Web Renamed"
	changed.OrgID = orgB
	changed.PublicID = "different-public-id"
	changed.CreatedBy = uuid.NullUUID{UUID: uuid.New(), Valid: true}
	changed.CreatedAt = p.CreatedAt.Add(-48 * time.Hour)
	scopeA := tenant.Into(ctx, tenant.Context{OrgID: orgA})
	assert.Equal(t, "42501", pgCode(f.projects.Save(scopeA, &changed)), "row level security refuses moving a project to another org")
	changed.OrgID = orgA
	require.NoError(t, f.projects.Save(scopeA, &changed))

	got, err := f.projects.ByID(tenant.Into(ctx, tenant.Context{OrgID: p.OrgID}), p.ID)
	require.NoError(t, err)
	assert.Equal(t, "web-renamed", got.Slug)
	assert.Equal(t, "Web Renamed", got.Name)
	assert.Equal(t, orgA, got.OrgID, "org_id is immutable: a project never moves between orgs")
	assert.Equal(t, p.PublicID, got.PublicID, "public_id is immutable")
	assert.Equal(t, uuid.NullUUID{UUID: creator, Valid: true}, got.CreatedBy, "created_by is immutable")
	assert.True(t, first.CreatedAt.Equal(got.CreatedAt), "created_at is immutable")
	assert.False(t, got.UpdatedAt.Before(first.UpdatedAt), "updated_at must not go backwards")

	inB, err := f.projects.List(tenant.Into(ctx, tenant.Context{OrgID: orgB}), orgB)
	require.NoError(t, err)
	assert.Empty(t, inB)
	inA, err := f.projects.List(tenant.Into(ctx, tenant.Context{OrgID: orgA}), orgA)
	require.NoError(t, err)
	assert.Equal(t, []string{"web-renamed"}, slugsOf(inA), "upsert by id must not insert a second row")
}

func TestProjectStore_UniqueConstraints(t *testing.T) {
	t.Run("slug unique within an org", func(t *testing.T) {
		f := newProjectFixture(t)
		ctx := t.Context()
		orgID := f.seedOrg(t, "acme")
		require.NoError(t, f.projects.Save(tenant.Into(ctx, tenant.Context{OrgID: orgID}), newProject(t, orgID, "web", uuid.Nil)))

		dup := newProject(t, orgID, "web", uuid.Nil)
		err := f.projects.Save(tenant.Into(ctx, tenant.Context{OrgID: dup.OrgID}), dup)
		ae, ok := errors.AsType[*project.AlreadyExistsError](err)
		require.True(t, ok, "want AlreadyExistsError, got %T: %v", err, err)
		assert.Equal(t, "web", ae.Slug)
		_, err = f.projects.ByID(tenant.Into(ctx, tenant.Context{OrgID: dup.OrgID}), dup.ID)
		assert.True(t, project.IsNotFoundError(err), "a rejected insert must leave no row")
	})

	t.Run("same slug allowed in different orgs", func(t *testing.T) {
		f := newProjectFixture(t)
		ctx := t.Context()
		orgA, orgB := f.seedOrg(t, "acme"), f.seedOrg(t, "globex")
		require.NoError(t, f.projects.Save(tenant.Into(ctx, tenant.Context{OrgID: orgA}), newProject(t, orgA, project.DefaultSlug, uuid.Nil)))
		require.NoError(t, f.projects.Save(tenant.Into(ctx, tenant.Context{OrgID: orgB}), newProject(t, orgB, project.DefaultSlug, uuid.Nil)))
	})

	t.Run("public_id unique across orgs", func(t *testing.T) {
		f := newProjectFixture(t)
		ctx := t.Context()
		orgA, orgB := f.seedOrg(t, "acme"), f.seedOrg(t, "globex")
		a := newProject(t, orgA, "web", uuid.Nil)
		require.NoError(t, f.projects.Save(tenant.Into(ctx, tenant.Context{OrgID: a.OrgID}), a))

		b := newProject(t, orgB, "api", uuid.Nil)
		b.PublicID = a.PublicID
		err := f.projects.Save(tenant.Into(ctx, tenant.Context{OrgID: b.OrgID}), b)
		assert.True(t, project.IsAlreadyExistsError(err), "want AlreadyExistsError, got %T: %v", err, err)
	})

	t.Run("rename into a sibling's slug", func(t *testing.T) {
		f := newProjectFixture(t)
		ctx := t.Context()
		orgID := f.seedOrg(t, "acme")
		web, api := newProject(t, orgID, "web", uuid.Nil), newProject(t, orgID, "api", uuid.Nil)
		require.NoError(t, f.projects.Save(tenant.Into(ctx, tenant.Context{OrgID: web.OrgID}), web))
		require.NoError(t, f.projects.Save(tenant.Into(ctx, tenant.Context{OrgID: api.OrgID}), api))

		api.Slug = "web"
		err := f.projects.Save(tenant.Into(ctx, tenant.Context{OrgID: api.OrgID}), api)
		assert.True(t, project.IsAlreadyExistsError(err), "want AlreadyExistsError, got %T: %v", err, err)
		got, err := f.projects.ByID(tenant.Into(ctx, tenant.Context{OrgID: api.OrgID}), api.ID)
		require.NoError(t, err)
		assert.Equal(t, "api", got.Slug, "the failed update must not change the row")
	})
}

func TestProjectStore_UnknownOrgIsForeignKeyViolation(t *testing.T) {
	f := newProjectFixture(t)
	p := newProject(t, uuid.New(), "web", uuid.Nil)

	err := f.projects.Save(tenant.Into(t.Context(), tenant.Context{OrgID: p.OrgID}), p)
	require.Error(t, err)
	assert.False(t, project.IsAlreadyExistsError(err), "a missing parent org must not look like a slug collision")
	assert.False(t, project.IsExpectedError(err))
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	require.True(t, ok, "want the raw *pgconn.PgError wrapped, got %T: %v", err, err)
	assert.Equal(t, "23503", pgErr.Code)
}

func TestProjectStore_DeletingOrgCascades(t *testing.T) {
	f := newProjectFixture(t)
	ctx := t.Context()
	orgA, orgB := f.seedOrg(t, "acme"), f.seedOrg(t, "globex")
	a1, a2 := newProject(t, orgA, "web", uuid.Nil), newProject(t, orgA, "api", uuid.Nil)
	b1 := newProject(t, orgB, "web", uuid.Nil)
	for _, p := range []*project.Project{a1, a2, b1} {
		require.NoError(t, f.projects.Save(tenant.Into(ctx, tenant.Context{OrgID: p.OrgID}), p))
	}

	_, err := f.sqlDB.ExecContext(ctx, `DELETE FROM orgs WHERE id = $1`, orgA)
	require.NoError(t, err)

	for _, id := range []uuid.UUID{a1.ID, a2.ID} {
		_, err := f.projects.ByID(tenant.Into(ctx, tenant.Context{OrgID: orgA}), id)
		assert.True(t, project.IsNotFoundError(err), "project %s must be cascaded away, got %v", id, err)
	}
	got, err := f.projects.ByID(tenant.Into(ctx, tenant.Context{OrgID: b1.OrgID}), b1.ID)
	require.NoError(t, err, "another org's projects must survive")
	assert.Equal(t, orgB, got.OrgID)
}

func TestProjectStore_TenantScoping(t *testing.T) {
	f := newProjectFixture(t)
	ctx := t.Context()
	orgA, orgB := f.seedOrg(t, "acme"), f.seedOrg(t, "globex")
	require.NoError(t, f.projects.Save(tenant.Into(ctx, tenant.Context{OrgID: orgA}), newProject(t, orgA, "only-in-a", uuid.Nil)))
	require.NoError(t, f.projects.Save(tenant.Into(ctx, tenant.Context{OrgID: orgA}), newProject(t, orgA, "shared", uuid.Nil)))
	bShared := newProject(t, orgB, "shared", uuid.Nil)
	require.NoError(t, f.projects.Save(tenant.Into(ctx, tenant.Context{OrgID: bShared.OrgID}), bShared))

	_, err := f.projects.BySlug(tenant.Into(ctx, tenant.Context{OrgID: orgB}), orgB, "only-in-a")
	nf, ok := errors.AsType[*project.NotFoundError](err)
	require.True(t, ok, "want NotFoundError, got %T: %v", err, err)
	assert.Equal(t, "only-in-a", nf.Slug)

	got, err := f.projects.BySlug(tenant.Into(ctx, tenant.Context{OrgID: orgB}), orgB, "shared")
	require.NoError(t, err)
	assert.Equal(t, bShared.ID, got.ID, "BySlug must resolve inside the given org")

	inA, err := f.projects.List(tenant.Into(ctx, tenant.Context{OrgID: orgA}), orgA)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"only-in-a", "shared"}, slugsOf(inA))
	for _, p := range inA {
		assert.Equal(t, orgA, p.OrgID)
	}

	none, err := f.projects.List(tenant.Into(ctx, tenant.Context{OrgID: uuid.New()}), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, none)

	id := uuid.New()
	_, err = f.projects.ByID(ctx, id)
	nf, ok = errors.AsType[*project.NotFoundError](err)
	require.True(t, ok, "want NotFoundError, got %T: %v", err, err)
	assert.Equal(t, id.String(), nf.ID)
}

func TestProjectStore_ListOrdersByCreatedAtAscending(t *testing.T) {
	f := newProjectFixture(t)
	ctx := t.Context()
	orgID := f.seedOrg(t, "acme")

	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	offsets := map[string]time.Duration{"charlie": 3 * time.Hour, "alpha": time.Hour, "bravo": 2 * time.Hour}
	for _, slug := range []string{"charlie", "alpha", "bravo"} {
		p := newProject(t, orgID, slug, uuid.Nil)
		p.CreatedAt = base.Add(offsets[slug])
		require.NoError(t, f.projects.Save(tenant.Into(ctx, tenant.Context{OrgID: p.OrgID}), p))
	}

	got, err := f.projects.List(tenant.Into(ctx, tenant.Context{OrgID: orgID}), orgID)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "bravo", "charlie"}, slugsOf(got))
}

func TestProjectStore_HonorsUnitOfWork(t *testing.T) {
	errAbort := errors.New("abort")

	t.Run("rollback undoes org and project together", func(t *testing.T) {
		f := newProjectFixture(t)
		o, err := org.NewOrg("acme", "Acme", uuid.Nil)
		require.NoError(t, err)
		p := newProject(t, o.ID, project.DefaultSlug, uuid.Nil)

		err = db.RunInTx(tenant.Into(t.Context(), tenant.Context{OrgID: o.ID}), f.pool, func(ctx context.Context) error {
			require.NoError(t, f.orgs.Save(ctx, o))
			require.NoError(t, f.projects.Save(ctx, p), "project insert sees the uncommitted org through the same tx")
			got, err := f.projects.BySlug(ctx, o.ID, project.DefaultSlug)
			require.NoError(t, err)
			assert.Equal(t, p.ID, got.ID)

			_, err = f.projects.ByID(tenant.Into(t.Context(), tenant.Context{OrgID: p.OrgID}), p.ID)
			assert.True(t, project.IsNotFoundError(err), "uncommitted row must be invisible outside the tx")
			return errAbort
		})
		require.ErrorIs(t, err, errAbort)

		_, err = f.projects.ByID(tenant.Into(t.Context(), tenant.Context{OrgID: p.OrgID}), p.ID)
		assert.True(t, project.IsNotFoundError(err), "rolled-back project must not persist, got %v", err)
		_, err = f.orgs.ByID(tenant.Into(t.Context(), tenant.Context{OrgID: o.ID}), o.ID)
		assert.True(t, org.IsNotFoundError(err), "rolled-back org must not persist, got %v", err)
	})

	t.Run("commit persists", func(t *testing.T) {
		f := newProjectFixture(t)
		orgID := f.seedOrg(t, "acme")
		p := newProject(t, orgID, "web", uuid.Nil)
		require.NoError(t, db.RunInTx(tenant.Into(t.Context(), tenant.Context{OrgID: orgID}), f.pool, func(ctx context.Context) error {
			return f.projects.Save(ctx, p)
		}))
		got, err := f.projects.ByID(tenant.Into(t.Context(), tenant.Context{OrgID: p.OrgID}), p.ID)
		require.NoError(t, err)
		assert.Equal(t, "web", got.Slug)
	})
}

func TestProjectStore_ConcurrentSameSlugExactlyOneWins(t *testing.T) {
	f := newProjectFixture(t)
	ctx := t.Context()
	orgID := f.seedOrg(t, "acme")
	const workers = 8

	start := make(chan struct{})
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := range workers {
		p := newProject(t, orgID, "contested", uuid.Nil)
		wg.Go(func() {
			<-start
			errs[i] = f.projects.Save(tenant.Into(ctx, tenant.Context{OrgID: p.OrgID}), p)
		})
	}
	close(start)
	wg.Wait()

	var ok, dup int
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case project.IsAlreadyExistsError(err):
			dup++
		default:
			t.Errorf("unexpected error: %T: %v", err, err)
		}
	}
	assert.Equal(t, 1, ok)
	assert.Equal(t, workers-1, dup)

	all, err := f.projects.List(tenant.Into(ctx, tenant.Context{OrgID: orgID}), orgID)
	require.NoError(t, err)
	assert.Len(t, all, 1)
}

func TestProjectStore_ByPublicID(t *testing.T) {
	f := newProjectFixture(t)
	p := newProject(t, f.seedOrg(t, "acme"), "web", uuid.Nil)
	require.NoError(t, f.projects.Save(tenant.Into(t.Context(), tenant.Context{OrgID: p.OrgID}), p))

	got, err := f.projects.ByPublicID(tenant.Into(t.Context(), tenant.Context{OrgID: p.OrgID}), p.PublicID)
	require.NoError(t, err)
	assert.Equal(t, p.ID, got.ID)
	for _, bad := range []string{"", "nope", p.ID.String()} {
		_, err = f.projects.ByPublicID(t.Context(), bad)
		assert.True(t, project.IsNotFoundError(err), "%q is not a public id", bad)
	}
}

func pgCode(err error) string {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pgErr.Code
	}
	return ""
}

func (f *projectFixture) in(orgID uuid.UUID) context.Context {
	return tenant.Into(context.Background(), tenant.Context{OrgID: orgID})
}

// SECURITY: an issuer handle is one exact issuer URL, so it is unique across every org.
func TestProjectStore_IssuerHandleIsGlobal(t *testing.T) {
	f := newProjectFixture(t)
	orgA, orgB := f.seedOrg(t, "acme"), f.seedOrg(t, "globex")
	a := newProject(t, orgA, "web", uuid.Nil)
	a.IssuerHandle = "shared-web"
	require.NoError(t, f.projects.Save(f.in(orgA), a))
	b := newProject(t, orgB, "web", uuid.Nil)
	b.IssuerHandle = "shared-web"
	err := f.projects.Save(f.in(orgB), b)
	assert.True(t, project.IsIssuerHandleTakenError(err), "got %v", err)

	b.IssuerHandle = "globex-web"
	require.NoError(t, f.projects.Save(f.in(orgB), b))
	assert.True(t, project.IsIssuerHandleTakenError(f.projects.SetIssuerHandle(f.in(orgB), b.ID, "shared-web")))

	exists, err := f.projects.IssuerHandleExists(f.in(orgB), "shared-web")
	require.NoError(t, err)
	assert.True(t, exists, "another org's handle counts")
	exists, err = f.projects.IssuerHandleExists(context.Background(), "free-handle")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestProjectStore_SetAndLockIssuerHandle(t *testing.T) {
	f := newProjectFixture(t)
	orgID := f.seedOrg(t, "acme")
	p := newProject(t, orgID, "web", uuid.Nil)
	require.NoError(t, f.projects.Save(f.in(orgID), p))

	require.NoError(t, f.projects.SetIssuerHandle(f.in(orgID), p.ID, "acme-web"))
	first := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, f.projects.LockIssuerHandle(f.in(orgID), p.ID, first))
	require.NoError(t, f.projects.LockIssuerHandle(f.in(orgID), p.ID, first.Add(time.Hour)))
	got, err := f.projects.ByID(f.in(orgID), p.ID)
	require.NoError(t, err)
	assert.Equal(t, "acme-web", got.IssuerHandle)
	require.NotNil(t, got.IssuerHandleLockedAt)
	assert.True(t, got.IssuerHandleLockedAt.Equal(first), "a lock keeps its first time")

	assert.True(t, project.IsIssuerHandleLockedError(f.projects.SetIssuerHandle(f.in(orgID), p.ID, "acme-other")))
	assert.True(t, project.IsNotFoundError(f.projects.SetIssuerHandle(f.in(orgID), uuid.New(), "acme-other")))
	got.IssuerHandle = "acme-sneaky"
	require.NoError(t, f.projects.Save(f.in(orgID), got))
	again, err := f.projects.ByID(f.in(orgID), p.ID)
	require.NoError(t, err)
	assert.Equal(t, "acme-web", again.IssuerHandle, "Save never changes the handle")
}

func TestProjectStore_SettingsRoundTrip(t *testing.T) {
	f := newProjectFixture(t)
	orgID := f.seedOrg(t, "acme")
	p := newProject(t, orgID, "web", uuid.Nil)
	require.NoError(t, f.projects.Save(f.in(orgID), p))
	now := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, p.UpdateBranding("Acme Web", "#4F46E5", "help@acme.test"))
	require.NoError(t, p.UpdateTerms("https://acme.test/terms", "https://acme.test/privacy", now))
	require.NoError(t, f.projects.Save(f.in(orgID), p))

	got, err := f.projects.ByID(f.in(orgID), p.ID)
	require.NoError(t, err)
	assert.Equal(t, "Acme Web", got.DisplayName)
	assert.Equal(t, "#4F46E5", got.PrimaryColor)
	assert.Equal(t, "help@acme.test", got.SupportEmail)
	assert.Equal(t, "https://acme.test/terms", got.TermsURL)
	assert.Equal(t, "https://acme.test/privacy", got.PrivacyURL)
	assert.Equal(t, int32(1), got.TermsVersion)
	require.NotNil(t, got.TermsPublishedAt)
	assert.True(t, got.TermsPublishedAt.Equal(now))
	assert.True(t, got.SignInEmailLink)
	assert.Nil(t, got.LogoFileID)
	assert.Empty(t, got.LogoKey)
}

func (f *projectFixture) seedFile(t *testing.T, orgID uuid.UUID, p *project.Project, status storage.Status) *storage.File {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	publicID, err := nanoid.New(21)
	require.NoError(t, err)
	file := &storage.File{
		ID: uuid.New(), PublicID: publicID, OrgID: orgID,
		OwnerKind: storage.OwnerProjects, OwnerID: p.ID, Purpose: "logo", Visibility: storage.Public,
		ContentType: "image/png", SizeBytes: 10, Status: status, CreatedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	file.ObjectKey = "public/othra/projects/" + p.PublicID + "/logo/" + file.ID.String() + ".png"
	require.NoError(t, f.files.Insert(f.in(orgID), file))
	return file
}

func TestProjectStore_LogoFileIsScoped(t *testing.T) {
	f := newProjectFixture(t)
	orgA, orgB := f.seedOrg(t, "acme"), f.seedOrg(t, "globex")
	p := newProject(t, orgA, "web", uuid.Nil)
	require.NoError(t, f.projects.Save(f.in(orgA), p))
	first := f.seedFile(t, orgA, p, storage.StatusReady)
	second := f.seedFile(t, orgA, p, storage.StatusPending)

	previous, err := f.projects.SetLogoFile(f.in(orgA), p.ID, &first.ID)
	require.NoError(t, err)
	assert.Nil(t, previous, "the first logo replaces nothing")
	got, err := f.projects.ByID(f.in(orgA), p.ID)
	require.NoError(t, err)
	assert.Equal(t, &first.ID, got.LogoFileID)
	assert.Equal(t, first.ObjectKey, got.LogoKey)

	previous, err = f.projects.SetLogoFile(f.in(orgA), p.ID, &second.ID)
	require.NoError(t, err)
	assert.Equal(t, &first.ID, previous)
	got, err = f.projects.ByID(f.in(orgA), p.ID)
	require.NoError(t, err)
	assert.Equal(t, &second.ID, got.LogoFileID)
	assert.Empty(t, got.LogoKey, "a file that is not ready yet has no key")

	got.LogoFileID = nil
	require.NoError(t, f.projects.Save(f.in(orgA), got))
	kept, err := f.projects.ByID(f.in(orgA), p.ID)
	require.NoError(t, err)
	assert.Equal(t, &second.ID, kept.LogoFileID, "Save never changes the logo")

	other := newProject(t, orgB, "web", uuid.Nil)
	require.NoError(t, f.projects.Save(f.in(orgB), other))
	foreign := f.seedFile(t, orgB, other, storage.StatusReady)
	_, err = f.projects.SetLogoFile(f.in(orgA), p.ID, &foreign.ID)
	assert.True(t, project.IsNotFoundError(err), "another org's file is refused: %v", err)
	_, err = f.projects.SetLogoFile(f.in(orgB), p.ID, &foreign.ID)
	assert.True(t, project.IsNotFoundError(err), "another org's scope sees no project: %v", err)

	previous, err = f.projects.SetLogoFile(f.in(orgA), p.ID, nil)
	require.NoError(t, err)
	assert.Equal(t, &second.ID, previous)
	previous, err = f.projects.SetLogoFile(f.in(orgA), p.ID, nil)
	require.NoError(t, err)
	assert.Nil(t, previous, "clearing twice is fine")
}

func TestService_LogoUploadThroughStorage(t *testing.T) {
	f := newProjectFixture(t)
	orgID := f.seedOrg(t, "acme")
	p := newProject(t, orgID, "web", uuid.Nil)
	require.NoError(t, f.projects.Save(f.in(orgID), p))
	driver := memory.New()
	files := storage.NewService(driver, f.files, storage.Config{
		App: "othra", PublicBaseURL: "https://files.test", UploadURLTTL: time.Minute, PendingTTL: time.Hour,
	}, apperror.NewReporter(nil, false).Unexpected)
	svc := project.NewService(f.projects, db.NewTransactor(f.pool), fakes.NoMemberships{}, nil, "", apperror.NewReporter(nil, false).Unexpected).WithOrgSlugs(fakes.OrgSlugs("")).WithStorage(files)
	ctx := f.in(orgID)
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 24)...)

	upload := func() *storage.File {
		file, up, err := svc.CreateLogoUpload(ctx, p.ID, "image/png", int64(len(png)))
		require.NoError(t, err)
		assert.Equal(t, "PUT", up.Method)
		driver.Put(file.ObjectKey, "image/png", png)
		return file
	}
	first := upload()
	got, err := svc.CompleteLogoUpload(ctx, p.ID, first.PublicID)
	require.NoError(t, err)
	assert.Equal(t, first.ObjectKey, got.LogoKey)
	assert.Equal(t, "https://files.test/"+first.ObjectKey, svc.LogoURL(got.LogoKey))
	assert.Contains(t, first.ObjectKey, "public/othra/projects/"+p.PublicID+"/logo/")

	second := upload()
	got, err = svc.CompleteLogoUpload(ctx, p.ID, second.PublicID)
	require.NoError(t, err)
	assert.Equal(t, second.ObjectKey, got.LogoKey)
	swept, err := files.Sweep(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, swept)
	assert.False(t, driver.Has(first.ObjectKey), "the replaced logo is swept")
	assert.True(t, driver.Has(second.ObjectKey))

	_, _, err = svc.CreateLogoUpload(ctx, p.ID, "image/gif", 10)
	var invalid *project.InvalidLogoError
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, project.LogoType, invalid.Reason)
	_, _, err = svc.CreateLogoUpload(ctx, p.ID, "image/png", 256<<10+1)
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, project.LogoSize, invalid.Reason)
	_, err = svc.CompleteLogoUpload(ctx, p.ID, "unknwnunknwnunknwnabc")
	assert.True(t, storage.IsUploadNotFoundError(err))

	got, err = svc.DeleteLogo(ctx, p.ID)
	require.NoError(t, err)
	assert.Empty(t, got.LogoKey)
	_, err = files.Sweep(ctx)
	require.NoError(t, err)
	assert.False(t, driver.Has(second.ObjectKey), "a removed logo is swept")
}

// SECURITY: the issuer resolves before any tenant scope exists, and only by its exact handle.
func TestProjectStore_ResolveIssuer(t *testing.T) {
	f := newProjectFixture(t)
	orgID := f.seedOrg(t, "acme")
	p := newProject(t, orgID, "web", uuid.Nil)
	p.IssuerHandle = "acme-web"
	require.NoError(t, p.UpdateBranding("Acme", "", "help@acme.test"))
	require.NoError(t, f.projects.Save(f.in(orgID), p))

	i, err := f.projects.ResolveIssuer(context.Background(), "acme-web")
	require.NoError(t, err)
	assert.Equal(t, p.ID, i.ProjectID)
	assert.Equal(t, orgID, i.OrgID)
	assert.Equal(t, p.PublicID, i.PublicID)
	assert.Equal(t, "Acme", i.DisplayName)
	assert.Equal(t, "help@acme.test", i.SupportEmail)
	assert.True(t, i.SignInEmailLink)
	assert.Empty(t, i.LogoKey)
	logo := f.seedFile(t, orgID, p, storage.StatusReady)
	_, err = f.projects.SetLogoFile(f.in(orgID), p.ID, &logo.ID)
	require.NoError(t, err)
	i, err = f.projects.ResolveIssuer(context.Background(), "acme-web")
	require.NoError(t, err)
	assert.Equal(t, logo.ObjectKey, i.LogoKey)
	_, err = f.projects.ResolveIssuer(context.Background(), "ACME-WEB")
	assert.True(t, project.IsNotFoundError(err), "handles are exact")
}
