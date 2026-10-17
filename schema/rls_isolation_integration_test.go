//go:build integration

package schema_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/testutil/pgtest"
)

type rlsTenant struct {
	org, project, user, key, endpoint uuid.UUID
}

type rlsFixture struct {
	svc    db.Pool
	admin  *sql.DB
	schema string
	a, b   rlsTenant
}

func newRLSFixture(t *testing.T) *rlsFixture {
	t.Helper()
	svc, admin, schemaName := pgtest.MigratedRLS(t)
	f := &rlsFixture{svc: svc, admin: admin, schema: schemaName}
	f.a, f.b = f.seed(t, "a"), f.seed(t, "b")
	return f
}

func (f *rlsFixture) seed(t *testing.T, tag string) rlsTenant {
	t.Helper()
	tn := rlsTenant{org: uuid.New(), project: uuid.New(), user: uuid.New(), key: uuid.New(), endpoint: uuid.New()}
	for _, ins := range rlsInserts() {
		_, err := f.admin.ExecContext(t.Context(), ins.sql(f.schema), ins.args(tn, tag)...)
		require.NoError(t, err, ins.table)
	}
	return tn
}

type rlsInsert struct {
	table string
	sql   func(schema string) string
	args  func(tn rlsTenant, tag string) []any
}

func rlsInserts() []rlsInsert {
	now := time.Now().UTC()
	id := func() string { return uuid.NewString() }
	return []rlsInsert{
		{"users", func(s string) string {
			return `INSERT INTO ` + s + `.users (id, public_id, email, created_at, updated_at) VALUES ($1, $2, $3, $4, $4) ON CONFLICT (id) DO NOTHING`
		}, func(tn rlsTenant, tag string) []any {
			return []any{tn.user, "u" + id()[:12], tag + id()[:8] + "@example.com", now}
		}},
		{"orgs", func(s string) string {
			return `INSERT INTO ` + s + `.orgs (id, public_id, slug, name, created_at, updated_at) VALUES ($1, $2, $3, 'Org', $4, $4)`
		}, func(tn rlsTenant, _ string) []any { return []any{tn.org, "o" + id()[:12], "s" + id()[:12], now} }},
		{"memberships", func(s string) string {
			return `INSERT INTO ` + s + `.memberships (id, public_id, org_id, user_id, role, created_at) VALUES ($1, $2, $3, $4, 'owner', $5)`
		}, func(tn rlsTenant, _ string) []any { return []any{uuid.New(), "m" + id()[:12], tn.org, tn.user, now} }},
		{"projects", func(s string) string {
			return `INSERT INTO ` + s + `.projects (id, public_id, org_id, slug, name, issuer_handle, created_at, updated_at) VALUES ($1, $2, $3, $4, 'P', $4, $5, $5)`
		}, func(tn rlsTenant, _ string) []any {
			return []any{tn.project, "p" + id()[:12], tn.org, "p" + id()[:12], now}
		}},
		{"files", func(s string) string {
			return `INSERT INTO ` + s + `.files (id, public_id, org_id, owner_kind, owner_id, purpose, visibility, object_key, content_type, size_bytes, status, created_at, expires_at) VALUES ($1, $2, $3, 'projects', $4, 'logo', 'public', $5, 'image/png', 1, 'pending', $6, $6)`
		}, func(tn rlsTenant, _ string) []any {
			return []any{uuid.New(), "f" + id()[:12], tn.org, tn.project, "public/othra/projects/" + id() + "/logo/f.png", now}
		}},
		{"invites", func(s string) string {
			return `INSERT INTO ` + s + `.invites (id, public_id, org_id, email, role, token_hash, expires_at, created_at, updated_at) VALUES ($1, $2, $3, $4, 'member', $5, $6, $6, $6)`
		}, func(tn rlsTenant, _ string) []any {
			return []any{uuid.New(), "i" + id()[:12], tn.org, "i" + id()[:8] + "@example.com", id(), now}
		}},
		{"api_keys", func(s string) string {
			return `INSERT INTO ` + s + `.api_keys (id, org_id, kind, all_projects, name, secret_hash, secret_hint, created_at) VALUES ($1, $2, 'org', true, 'k', decode(md5($3), 'hex'), substr(md5($3), 1, 4), $4)`
		}, func(tn rlsTenant, _ string) []any { return []any{tn.key, tn.org, id(), now} }},
		{"api_key_projects", func(s string) string {
			return `INSERT INTO ` + s + `.api_key_projects (org_id, key_id, project_id, created_at) VALUES ($1, $2, $3, $4)`
		}, func(tn rlsTenant, _ string) []any { return []any{tn.org, tn.key, tn.project, now} }},
		{"outbox_entries", func(s string) string {
			return `INSERT INTO ` + s + `.outbox_entries (id, event_id, org_id, project_id, target, payload, next_attempt_at, created_at, public_id, event_public_id) VALUES ($1, $1, $2, $3, 'webhook', '\x01', $4, $4, $5, 'e')`
		}, func(tn rlsTenant, _ string) []any { return []any{uuid.New(), tn.org, tn.project, now, "d" + id()[:12]} }},
		{"webhook_endpoints", func(s string) string {
			return `INSERT INTO ` + s + `.webhook_endpoints (id, public_id, org_id, project_id, url, event_types, secret_primary, active, created_at, updated_at) VALUES ($1, $2, $3, $4, 'https://e.com', 'invite.created', '\x01', true, $5, $5)`
		}, func(tn rlsTenant, _ string) []any {
			return []any{tn.endpoint, "w" + id()[:12], tn.org, tn.project, now}
		}},
		{"webhook_deliveries", func(s string) string {
			return `INSERT INTO ` + s + `.webhook_deliveries (id, org_id, project_id, endpoint_id, delivery_id, event_id, event_type, attempt, duration_ms, created_at) VALUES ($1, $2, $3, $4, $1, $1, 'invite.created', 1, 1, $5)`
		}, func(tn rlsTenant, _ string) []any { return []any{uuid.New(), tn.org, tn.project, tn.endpoint, now} }},
	}
}

func orgColumn(table string) string {
	if table == "orgs" {
		return "id"
	}
	return "org_id"
}

func (f *rlsFixture) visible(t *testing.T, ctx context.Context, table string) []uuid.UUID {
	t.Helper()
	var orgs []uuid.UUID
	err := db.Read(ctx, f.svc, func(q qrm.DB) error {
		rows, err := q.QueryContext(ctx, `SELECT `+orgColumn(table)+` FROM `+f.schema+`.`+table)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				return err
			}
			orgs = append(orgs, id)
		}
		return rows.Err()
	})
	require.NoError(t, err, table)
	return orgs
}

func isRLSViolation(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	return ok && pgErr.Code == "42501"
}

// SECURITY: R1/O18, as the NOBYPASSRLS service role every org-owned table shows one tenant's rows, none without a scope, and refuses cross-tenant writes.
func TestRLS_EveryOrgOwnedTableIsolatesTenants(t *testing.T) {
	f := newRLSFixture(t)
	scopeA := tenant.Into(t.Context(), tenant.Context{OrgID: f.a.org})
	for _, ins := range rlsInserts() {
		if ins.table == "users" {
			continue
		}
		t.Run(ins.table, func(t *testing.T) {
			got := f.visible(t, scopeA, ins.table)
			require.NotEmpty(t, got, "scope A sees its own rows")
			for _, org := range got {
				assert.Equal(t, f.a.org, org, "scope A sees only org A")
			}
			assert.Empty(t, f.visible(t, t.Context(), ins.table), "no scope sees nothing")

			foreign := f.b
			if ins.table == "orgs" {
				foreign.org = uuid.New()
			}
			err := db.RunInTx(scopeA, f.svc, func(ctx context.Context) error {
				tx, _ := db.CurrentTx(ctx)
				_, err := tx.ExecContext(ctx, ins.sql(f.schema), ins.args(foreign, "x")...)
				return err
			})
			assert.True(t, isRLSViolation(err), "inserting org B's row under scope A: %v", err)

			col := orgColumn(ins.table)
			for _, stmt := range []string{
				`UPDATE ` + f.schema + `.` + ins.table + ` SET ` + col + ` = ` + col + ` WHERE ` + col + ` = $1`,
				`DELETE FROM ` + f.schema + `.` + ins.table + ` WHERE ` + col + ` = $1`,
			} {
				var n int64
				require.NoError(t, db.RunInTx(scopeA, f.svc, func(ctx context.Context) error {
					tx, _ := db.CurrentTx(ctx)
					res, err := tx.ExecContext(ctx, stmt, f.b.org)
					if err != nil {
						return err
					}
					n, err = res.RowsAffected()
					return err
				}))
				assert.Zero(t, n, "%s touches no row of org B", stmt)
			}
			var left int
			require.NoError(t, f.admin.QueryRowContext(t.Context(),
				`SELECT count(*) FROM `+f.schema+`.`+ins.table+` WHERE `+col+` = $1`, f.b.org).Scan(&left))
			assert.Positive(t, left, "org B's rows survive")
		})
	}
}

// SECURITY: a member reads the orgs and memberships of their own user without an org scope, and nobody else's.
func TestRLS_AUserSeesTheirOwnMemberships(t *testing.T) {
	f := newRLSFixture(t)
	ctx := tenant.Into(t.Context(), tenant.Context{UserID: f.a.user})
	assert.Equal(t, []uuid.UUID{f.a.org}, f.visible(t, ctx, "memberships"))
	assert.Equal(t, []uuid.UUID{f.a.org}, f.visible(t, ctx, "orgs"))
}

// SECURITY: a transaction-local set_config leaves ” behind on the pooled connection; the NULLIF helper turns that into "no tenant" instead of a 22P02 cast error.
func TestRLS_AConnectionThatCarriedAScopeReadsNothingAfterward(t *testing.T) {
	f := newRLSFixture(t)
	f.svc.W.SetMaxOpenConns(1)
	require.NotEmpty(t, f.visible(t, tenant.Into(t.Context(), tenant.Context{OrgID: f.a.org, UserID: f.a.user}), "projects"))
	assert.Empty(t, f.visible(t, t.Context(), "projects"))
	assert.Empty(t, f.visible(t, t.Context(), "orgs"))
}

// SECURITY: a definer resolves only the row it matches, even for the service role with no scope.
func TestRLS_DefinersResolveOneTenantForTheServiceRole(t *testing.T) {
	f := newRLSFixture(t)
	var ids []uuid.UUID
	require.NoError(t, db.Read(t.Context(), f.svc, func(q qrm.DB) error {
		rows, err := q.QueryContext(t.Context(), `SELECT id FROM `+f.schema+`.list_orgs_for_user($1)`, f.a.user)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return rows.Err()
	}))
	assert.Equal(t, []uuid.UUID{f.a.org}, ids)
}
