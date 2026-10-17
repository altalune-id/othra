//go:build integration

package storage_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/kernel/storage"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/testutil/pgtest"
	"altalune.id/othra/nanoid"
)

type fixture struct {
	db    *sql.DB
	pool  db.Pool
	store storage.Store
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	pool, sqlDB, schemaName := pgtest.MigratedRLS(t)
	return &fixture{db: sqlDB, pool: pool, store: storage.NewStore(pool, schemaName, "")}
}

type seeded struct {
	ctx     context.Context
	org     uuid.UUID
	project uuid.UUID
}

func (f *fixture) seed(t *testing.T) seeded {
	t.Helper()
	org, project := uuid.New(), uuid.New()
	now := time.Now().UTC()
	f.exec(t, `INSERT INTO orgs (id, public_id, slug, name, created_at, updated_at) VALUES ($1, $2, $2, 'Org', $3, $3)`, org, org.String(), now)
	f.exec(t, `INSERT INTO projects (id, public_id, org_id, slug, name, issuer_handle, created_at, updated_at) VALUES ($1, $2, $3, $2, 'P', $5, $4, $4)`,
		project, project.String(), org, now, "h"+project.String()[:8])
	return seeded{ctx: tenant.Into(t.Context(), tenant.Context{OrgID: org}), org: org, project: project}
}

func (f *fixture) exec(t *testing.T, q string, args ...any) {
	t.Helper()
	_, err := f.db.ExecContext(t.Context(), q, args...)
	require.NoError(t, err)
}

func newFile(s seeded, status storage.Status, expires time.Time) *storage.File {
	pub := nanoid.MustNew(21)
	return &storage.File{
		ID: uuid.New(), PublicID: pub, OrgID: s.org, OwnerKind: storage.OwnerProjects, OwnerID: s.project,
		Purpose: "logo", Visibility: storage.Public, ObjectKey: "public/othra/projects/" + s.project.String() + "/logo/" + pub + ".png",
		ContentType: "image/png", SizeBytes: 100, OriginalName: "logo.png", Status: status,
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond), ExpiresAt: expires.UTC().Truncate(time.Microsecond),
	}
}

func TestStore_InsertAndReadBack(t *testing.T) {
	f := newFixture(t)
	s := f.seed(t)
	in := newFile(s, storage.StatusPending, time.Now().Add(time.Hour))
	require.NoError(t, f.store.Insert(s.ctx, in))

	got, err := f.store.ByPublicID(s.ctx, in.PublicID)
	require.NoError(t, err)
	assert.Equal(t, in, got)

	_, err = f.store.ByPublicID(s.ctx, "nope")
	assert.True(t, storage.IsUploadNotFoundError(err))
}

// SECURITY: a file of one org is invisible and untouchable from another org's scope.
func TestStore_IsTenantScoped(t *testing.T) {
	f := newFixture(t)
	a, b := f.seed(t), f.seed(t)
	file := newFile(a, storage.StatusPending, time.Now().Add(time.Hour))
	require.NoError(t, f.store.Insert(a.ctx, file))

	_, err := f.store.ByPublicID(b.ctx, file.PublicID)
	assert.True(t, storage.IsUploadNotFoundError(err))
	assert.True(t, storage.IsUploadNotFoundError(f.store.MarkReady(b.ctx, file.ID, time.Now())))
	assert.True(t, storage.IsUploadNotFoundError(f.store.MarkDeleting(b.ctx, file.ID)))
	require.NoError(t, f.store.Delete(b.ctx, file.ID))
	got, err := f.store.ByPublicID(a.ctx, file.PublicID)
	require.NoError(t, err)
	assert.Equal(t, storage.StatusPending, got.Status)

	foreign := newFile(a, storage.StatusPending, time.Now().Add(time.Hour))
	assert.Error(t, f.store.Insert(b.ctx, foreign), "a row naming another org is refused by the policy")
}

func TestStore_StatusTransitions(t *testing.T) {
	f := newFixture(t)
	s := f.seed(t)
	file := newFile(s, storage.StatusPending, time.Now().Add(time.Hour))
	require.NoError(t, f.store.Insert(s.ctx, file))

	at := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, f.store.MarkReady(s.ctx, file.ID, at))
	require.NoError(t, f.store.MarkReady(s.ctx, file.ID, at.Add(time.Hour)), "ready twice is fine")
	got, err := f.store.ByPublicID(s.ctx, file.PublicID)
	require.NoError(t, err)
	assert.Equal(t, storage.StatusReady, got.Status)
	require.NotNil(t, got.CompletedAt)
	assert.Equal(t, at, *got.CompletedAt, "the first completion time stays")

	require.NoError(t, f.store.MarkDeleting(s.ctx, file.ID))
	assert.True(t, storage.IsUploadNotFoundError(f.store.MarkReady(s.ctx, file.ID, at)), "a deleting file is never attached")
	assert.True(t, storage.IsUploadNotFoundError(f.store.MarkReady(s.ctx, uuid.New(), at)))
	assert.True(t, storage.IsUploadNotFoundError(f.store.MarkDeleting(s.ctx, uuid.New())))
}

func TestStore_MarkReadyJoinsTheCallersUnitOfWork(t *testing.T) {
	f := newFixture(t)
	s := f.seed(t)
	file := newFile(s, storage.StatusPending, time.Now().Add(time.Hour))
	require.NoError(t, f.store.Insert(s.ctx, file))

	rollback := errors.New("rollback")
	err := db.RunInTx(s.ctx, f.pool, func(ctx context.Context) error {
		require.NoError(t, f.store.MarkReady(ctx, file.ID, time.Now()))
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	got, err := f.store.ByPublicID(s.ctx, file.PublicID)
	require.NoError(t, err)
	assert.Equal(t, storage.StatusPending, got.Status, "the domain's rollback undoes the attach")
}

func TestStore_SweepableAndDelete(t *testing.T) {
	f := newFixture(t)
	s := f.seed(t)
	now := time.Now().UTC()
	expired := newFile(s, storage.StatusPending, now.Add(-time.Minute))
	fresh := newFile(s, storage.StatusPending, now.Add(time.Hour))
	deleting := newFile(s, storage.StatusDeleting, now.Add(time.Hour))
	ready := newFile(s, storage.StatusReady, now.Add(-time.Hour))
	for _, file := range []*storage.File{expired, fresh, deleting, ready} {
		require.NoError(t, f.store.Insert(s.ctx, file))
	}

	got, err := f.store.Sweepable(s.ctx, now, 10)
	require.NoError(t, err)
	ids := []uuid.UUID{}
	for _, g := range got {
		ids = append(ids, g.ID)
	}
	assert.ElementsMatch(t, []uuid.UUID{expired.ID, deleting.ID}, ids)

	got, err = f.store.Sweepable(s.ctx, now, 1)
	require.NoError(t, err)
	assert.Len(t, got, 1)

	require.NoError(t, f.store.Delete(s.ctx, expired.ID))
	_, err = f.store.ByPublicID(s.ctx, expired.PublicID)
	assert.True(t, storage.IsUploadNotFoundError(err))
}

func TestSchema_FileRules(t *testing.T) {
	f := newFixture(t)
	a, b := f.seed(t), f.seed(t)
	file := newFile(a, storage.StatusReady, time.Now().Add(time.Hour))
	require.NoError(t, f.store.Insert(a.ctx, file))

	dup := newFile(a, storage.StatusPending, time.Now().Add(time.Hour))
	dup.ObjectKey = file.ObjectKey
	err := f.store.Insert(a.ctx, dup)
	require.Error(t, err)
	assert.True(t, db.IsUniqueViolation(err), "one row per object key")

	_, err = f.db.ExecContext(t.Context(), `UPDATE projects SET logo_file_id = $1 WHERE id = $2`, file.ID, b.project)
	assert.True(t, db.IsForeignKeyViolation(err), "a logo is a file of the project's own org: %v", err)

	f.exec(t, `UPDATE projects SET logo_file_id = $1 WHERE id = $2`, file.ID, a.project)
	f.exec(t, `DELETE FROM files WHERE id = $1`, file.ID)
	var logo *uuid.UUID
	var org uuid.UUID
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT logo_file_id, org_id FROM projects WHERE id = $1`, a.project).Scan(&logo, &org))
	assert.Nil(t, logo, "deleting the file clears the logo")
	assert.Equal(t, a.org, org, "and keeps the project's org")

	_, err = f.db.ExecContext(t.Context(), `UPDATE files SET status = 'done' WHERE org_id = $1`, a.org)
	require.NoError(t, err, "no rows left to violate")
	bad := newFile(a, "done", time.Now())
	assert.Error(t, f.store.Insert(a.ctx, bad), "status is checked")

	other := newFile(a, storage.StatusPending, time.Now().Add(time.Hour))
	require.NoError(t, f.store.Insert(a.ctx, other))
	f.exec(t, `DELETE FROM orgs WHERE id = $1`, a.org)
	var n int
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM files WHERE org_id = $1`, a.org).Scan(&n))
	assert.Zero(t, n, "files go with their org")
}
