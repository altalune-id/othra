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
	org, project, user, key, endpoint, client, rs uuid.UUID
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
	tn := rlsTenant{
		org: uuid.New(), project: uuid.New(), user: uuid.New(), key: uuid.New(), endpoint: uuid.New(),
		client: uuid.New(), rs: uuid.New(),
	}
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
		{"signing_keys", func(s string) string {
			return `INSERT INTO ` + s + `.signing_keys (id, public_id, org_id, project_id, kid, alg, public_jwk, private_key_enc, status, created_at) VALUES ($1, $2, $3, $4, $5, 'RS256', '{}', '\x01', 'active', $6)`
		}, func(tn rlsTenant, _ string) []any {
			return []any{uuid.New(), "k" + id()[:12], tn.org, tn.project, "kid" + id(), now}
		}},
		{"oauth_clients", func(s string) string {
			return `INSERT INTO ` + s + `.oauth_clients (id, public_id, org_id, project_id, name, client_type, application_type, token_endpoint_auth_method, grant_types, allowed_scopes, redirect_uris, created_at, updated_at) VALUES ($1, $2, $3, $4, 'Web', 'confidential', 'web', 'client_secret_basic', '{authorization_code}', '{openid}', '{https://a.example/cb}', $5, $5)`
		}, func(tn rlsTenant, _ string) []any {
			return []any{tn.client, "c" + id()[:12], tn.org, tn.project, now}
		}},
		{"oauth_client_secrets", func(s string) string {
			return `INSERT INTO ` + s + `.oauth_client_secrets (id, public_id, org_id, project_id, client_id, secret_hash, hint, created_at) VALUES ($1, $2, $3, $4, $5, 'sha256$' || encode(sha256(convert_to($6, 'UTF8')), 'hex'), 'abcd', $7)`
		}, func(tn rlsTenant, _ string) []any {
			return []any{uuid.New(), "s" + id()[:12], tn.org, tn.project, tn.client, id(), now}
		}},
		{"resource_servers", func(s string) string {
			return `INSERT INTO ` + s + `.resource_servers (id, public_id, org_id, project_id, name, identifier, scopes, created_at, updated_at) VALUES ($1, $2, $3, $4, 'API', 'https://api.example/' || $2, '{read}', $5, $5)`
		}, func(tn rlsTenant, _ string) []any {
			return []any{tn.rs, "r" + id()[:12], tn.org, tn.project, now}
		}},
		{"oauth_client_resources", func(s string) string {
			return `INSERT INTO ` + s + `.oauth_client_resources (org_id, project_id, client_id, resource_server_id, scopes, created_at, updated_at) VALUES ($1, $2, $3, $4, '{read}', $5, $5)`
		}, func(tn rlsTenant, _ string) []any {
			return []any{tn.org, tn.project, tn.client, tn.rs, now}
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
	scopeA := tenant.Into(t.Context(), tenant.Context{OrgID: f.a.org, ProjectID: f.a.project})
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

// SECURITY: signing keys are scoped to their project as well as their org, and no definer hands out a key column.
func TestRLS_SigningKeysAreProjectScoped(t *testing.T) {
	f := newRLSFixture(t)
	other := uuid.New()
	now := time.Now().UTC()
	_, err := f.admin.ExecContext(t.Context(),
		`INSERT INTO `+f.schema+`.projects (id, public_id, org_id, slug, name, issuer_handle, created_at, updated_at) VALUES ($1, $2, $3, $4, 'P', $4, $5, $5)`,
		other, "p"+other.String()[:12], f.a.org, "p"+other.String()[:12], now)
	require.NoError(t, err)

	own := tenant.Into(t.Context(), tenant.Context{OrgID: f.a.org, ProjectID: f.a.project})
	sibling := tenant.Into(t.Context(), tenant.Context{OrgID: f.a.org, ProjectID: other})
	orgOnly := tenant.Into(t.Context(), tenant.Context{OrgID: f.a.org})
	assert.Equal(t, []uuid.UUID{f.a.org}, f.visible(t, own, "signing_keys"))
	assert.Empty(t, f.visible(t, sibling, "signing_keys"), "another project of the same org sees none")
	assert.Empty(t, f.visible(t, orgOnly, "signing_keys"), "an org scope without a project sees none")

	var cols []string
	require.NoError(t, db.Read(t.Context(), f.svc, func(q qrm.DB) error {
		rows, err := q.QueryContext(t.Context(), `SELECT * FROM `+f.schema+`.resolve_project_by_issuer_handle('nope') LIMIT 0`)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		cols, err = rows.Columns()
		return err
	}))
	for _, col := range cols {
		assert.NotContains(t, []string{"kid", "public_jwk", "private_key_enc"}, col, "the issuer definer returns no key column")
	}
}

//nolint:gochecknoglobals // the project-owned M3 tables.
var clientTables = []string{"oauth_clients", "oauth_client_secrets", "resource_servers", "oauth_client_resources"}

func (f *rlsFixture) siblingProject(t *testing.T) uuid.UUID {
	t.Helper()
	sibling := uuid.New()
	_, err := f.admin.ExecContext(t.Context(),
		`INSERT INTO `+f.schema+`.projects (id, public_id, org_id, slug, name, issuer_handle, created_at, updated_at) VALUES ($1, $2, $3, $4, 'P', $4, $5, $5)`,
		sibling, "p"+sibling.String()[:12], f.a.org, "p"+sibling.String()[:12], time.Now().UTC())
	require.NoError(t, err)
	return sibling
}

// SECURITY: clients, their secrets, resource servers and links are scoped to their project as well as their org, for reads and writes.
func TestRLS_ClientsAndResourceServersAreProjectScoped(t *testing.T) {
	f := newRLSFixture(t)
	sibling := f.siblingProject(t)
	own := tenant.Into(t.Context(), tenant.Context{OrgID: f.a.org, ProjectID: f.a.project})
	other := tenant.Into(t.Context(), tenant.Context{OrgID: f.a.org, ProjectID: sibling})
	orgOnly := tenant.Into(t.Context(), tenant.Context{OrgID: f.a.org})
	byTable := map[string]rlsInsert{}
	for _, ins := range rlsInserts() {
		byTable[ins.table] = ins
	}
	for _, table := range clientTables {
		t.Run(table, func(t *testing.T) {
			assert.Equal(t, []uuid.UUID{f.a.org}, f.visible(t, own, table))
			assert.Empty(t, f.visible(t, other, table), "another project of the same org sees none")
			assert.Empty(t, f.visible(t, orgOnly, table), "an org scope without a project sees none")

			mine := f.a
			mine.client, mine.rs = uuid.New(), uuid.New()
			err := db.RunInTx(other, f.svc, func(ctx context.Context) error {
				tx, _ := db.CurrentTx(ctx)
				_, err := tx.ExecContext(ctx, byTable[table].sql(f.schema), byTable[table].args(mine, "x")...)
				return err
			})
			assert.True(t, isRLSViolation(err), "writing project A's row under the sibling's scope: %v", err)
		})
	}
}

// SECURITY: the composite keys make a link between two projects impossible, a linked resource server can't be deleted, and a project delete still removes everything.
func TestSchema_LinksStayInsideOneProject(t *testing.T) {
	f := newRLSFixture(t)
	sibling := f.siblingProject(t)
	now := time.Now().UTC()
	rs := uuid.New()
	_, err := f.admin.ExecContext(t.Context(),
		`INSERT INTO `+f.schema+`.resource_servers (id, public_id, org_id, project_id, name, identifier, scopes, created_at, updated_at) VALUES ($1, $2, $3, $4, 'API', 'https://api.example', '{read}', $5, $5)`,
		rs, "r"+rs.String()[:12], f.a.org, sibling, now)
	require.NoError(t, err)

	for name, project := range map[string]uuid.UUID{"the client's project": f.a.project, "the server's project": sibling} {
		_, err = f.admin.ExecContext(t.Context(),
			`INSERT INTO `+f.schema+`.oauth_client_resources (org_id, project_id, client_id, resource_server_id, scopes, created_at, updated_at) VALUES ($1, $2, $3, $4, '{read}', $5, $5)`,
			f.a.org, project, f.a.client, rs, now)
		assert.Equal(t, "23503", pgCode(err), "a link named under %s: %v", name, err)
	}

	_, err = f.admin.ExecContext(t.Context(), `DELETE FROM `+f.schema+`.resource_servers WHERE id = $1`, f.a.rs)
	assert.Equal(t, "23503", pgCode(err), "a linked resource server stays: %v", err)

	_, err = f.admin.ExecContext(t.Context(), `DELETE FROM `+f.schema+`.projects WHERE id = $1`, f.a.project)
	require.NoError(t, err, "a project delete cascades to its clients, servers and links")
	for _, table := range clientTables {
		var n int
		require.NoError(t, f.admin.QueryRowContext(t.Context(),
			`SELECT count(*) FROM `+f.schema+`.`+table+` WHERE project_id = $1`, f.a.project).Scan(&n))
		assert.Zero(t, n, table)
	}
}

// SECURITY: inside one org-scoped unit of work the project scope follows each joining context: A sees A's keys, B only B's, no project none; org-only tables keep working and another org is refused.
func TestRLS_TheProjectScopeFollowsTheJoiningContext(t *testing.T) {
	f := newRLSFixture(t)
	sibling := uuid.New()
	now := time.Now().UTC()
	_, err := f.admin.ExecContext(t.Context(),
		`INSERT INTO `+f.schema+`.projects (id, public_id, org_id, slug, name, issuer_handle, created_at, updated_at) VALUES ($1, $2, $3, $4, 'P', $4, $5, $5)`,
		sibling, "p"+sibling.String()[:12], f.a.org, "p"+sibling.String()[:12], now)
	require.NoError(t, err)

	insertKey := func(ctx context.Context, project uuid.UUID) error {
		return db.InTx(ctx, f.svc, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx,
				`INSERT INTO `+f.schema+`.signing_keys (id, public_id, org_id, project_id, kid, alg, public_jwk, private_key_enc, status, created_at) VALUES ($1, $2, $3, $4, $5, 'RS256', '{}', '\x01', 'retired', $6)`,
				uuid.New(), "k"+uuid.NewString()[:12], f.a.org, project, "kid"+uuid.NewString(), now)
			return err
		})
	}
	keyProjects := func(ctx context.Context) []uuid.UUID {
		var out []uuid.UUID
		require.NoError(t, db.Read(ctx, f.svc, func(q qrm.DB) error {
			rows, err := q.QueryContext(ctx, `SELECT DISTINCT project_id FROM `+f.schema+`.signing_keys`)
			if err != nil {
				return err
			}
			defer func() { _ = rows.Close() }()
			for rows.Next() {
				var id uuid.UUID
				if err := rows.Scan(&id); err != nil {
					return err
				}
				out = append(out, id)
			}
			return rows.Err()
		}))
		return out
	}
	orgScope := tenant.Into(t.Context(), tenant.Context{OrgID: f.a.org})
	err = db.RunInTx(orgScope, f.svc, func(ctx context.Context) error {
		inA, inB := tenant.WithProject(ctx, f.a.project), tenant.WithProject(ctx, sibling)
		require.NoError(t, insertKey(inA, f.a.project))
		assert.Equal(t, []uuid.UUID{f.a.project}, keyProjects(inA), "A sees only A's keys")

		require.NoError(t, insertKey(inB, sibling), "the scope moves to B")
		assert.Equal(t, []uuid.UUID{sibling}, keyProjects(inB), "B sees only B's keys")

		assert.Empty(t, keyProjects(ctx), "a context with no project sees no keys")
		tx, ok := db.CurrentTx(ctx)
		require.True(t, ok)
		var invites int
		require.NoError(t, tx.QueryRowContext(ctx, `SELECT count(*) FROM `+f.schema+`.invites`).Scan(&invites))
		assert.Positive(t, invites, "org-only tables still answer")

		assert.ErrorIs(t, insertKey(tenant.Into(ctx, tenant.Context{OrgID: f.b.org, ProjectID: f.b.project}), f.b.project), db.ErrTenantMismatch,
			"another org is refused")
		return insertKey(inA, sibling)
	})
	assert.True(t, isRLSViolation(err), "B's key is not writable under A's scope: %v", err)
}
