//go:build integration

package schema_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/internal/kernel/tenant"
	"github.com/altalune-id/othra/internal/testutil/pgtest"
)

type rlsTenant struct {
	org, project, user, key, endpoint, client, rs uuid.UUID
	endUser, browser, userSession, family         uuid.UUID
	upstream                                      uuid.UUID
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
		client: uuid.New(), rs: uuid.New(), endUser: uuid.New(), browser: uuid.New(), userSession: uuid.New(), family: uuid.New(),
		upstream: uuid.New(),
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
		{"end_users", func(s string) string {
			return `INSERT INTO ` + s + `.end_users (id, public_id, org_id, project_id, email, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $6)`
		}, func(tn rlsTenant, _ string) []any {
			return []any{tn.endUser, "e" + id()[:12], tn.org, tn.project, "eu" + id()[:8] + "@example.com", now}
		}},
		{"browser_sessions", func(s string) string {
			return `INSERT INTO ` + s + `.browser_sessions (id, org_id, project_id, secret_hash, created_at, last_seen_at, absolute_expires_at) VALUES ($1, $2, $3, encode(sha256(convert_to($4, 'UTF8')), 'hex'), $5::timestamptz, $5::timestamptz, $5::timestamptz + interval '1 day')`
		}, func(tn rlsTenant, _ string) []any { return []any{tn.browser, tn.org, tn.project, id(), now} }},
		{"user_sessions", func(s string) string {
			return `INSERT INTO ` + s + `.user_sessions (id, public_id, org_id, project_id, browser_session_id, end_user_id, method, auth_time, selected_at, created_at, last_seen_at) VALUES ($1, $2, $3, $4, $5, $6, 'magic_link', $7, $7, $7, $7)`
		}, func(tn rlsTenant, _ string) []any {
			return []any{tn.userSession, "s" + id()[:12], tn.org, tn.project, tn.browser, tn.endUser, now}
		}},
		{"end_user_terms", func(s string) string {
			return `INSERT INTO ` + s + `.end_user_terms (org_id, project_id, end_user_id, terms_version, terms_url, accepted_at, user_session_id, user_session_public_id) VALUES ($1, $2, $3, 1, 'https://acme.test/terms', $4, $5, 'us-public')`
		}, func(tn rlsTenant, _ string) []any { return []any{tn.org, tn.project, tn.endUser, now, tn.userSession} }},
		{"magic_links", func(s string) string {
			return `INSERT INTO ` + s + `.magic_links (id, public_id, org_id, project_id, token_hash, email, return_to, created_at, expires_at) VALUES ($1, $2, $3, $4, encode(sha256(convert_to($5, 'UTF8')), 'hex'), $6, '/p/x/account/', $7::timestamptz, $7::timestamptz + interval '15 minutes')`
		}, func(tn rlsTenant, _ string) []any {
			return []any{uuid.New(), "m" + id()[:12], tn.org, tn.project, id(), "ml" + id()[:8] + "@example.com", now}
		}},
		{"grant_families", func(s string) string {
			return `INSERT INTO ` + s + `.grant_families (id, public_id, org_id, project_id, client_id, end_user_id, user_session_id, user_session_public_id, scopes, auth_time, created_at, last_used_at, absolute_expires_at) VALUES ($1, $2, $3, $4, $5, $6, $7, 'us-public', '{openid}', $8::timestamptz, $8::timestamptz, $8::timestamptz, $8::timestamptz + interval '90 days')`
		}, func(tn rlsTenant, _ string) []any {
			return []any{tn.family, "g" + id()[:12], tn.org, tn.project, tn.client, tn.endUser, tn.userSession, now}
		}},
		{"authorization_codes", func(s string) string {
			return `INSERT INTO ` + s + `.authorization_codes (id, org_id, project_id, code_hash, client_id, end_user_id, user_session_id, redirect_uri, scopes, code_challenge, code_challenge_method, auth_time, created_at, expires_at) VALUES ($1, $2, $3, encode(sha256(convert_to($4, 'UTF8')), 'hex'), $5, $6, $7, 'https://a.example/cb', '{openid}', repeat('a', 43), 'S256', $8::timestamptz, $8::timestamptz, $8::timestamptz + interval '1 minute')`
		}, func(tn rlsTenant, _ string) []any {
			return []any{uuid.New(), tn.org, tn.project, id(), tn.client, tn.endUser, tn.userSession, now}
		}},
		{"access_tokens", func(s string) string {
			return `INSERT INTO ` + s + `.access_tokens (id, org_id, project_id, family_id, token_hash, scopes, created_at, expires_at) VALUES ($1, $2, $3, $4, encode(sha256(convert_to($5, 'UTF8')), 'hex'), '{openid}', $6::timestamptz, $6::timestamptz + interval '15 minutes')`
		}, func(tn rlsTenant, _ string) []any { return []any{uuid.New(), tn.org, tn.project, tn.family, id(), now} }},
		{"refresh_tokens", func(s string) string {
			return `INSERT INTO ` + s + `.refresh_tokens (id, org_id, project_id, family_id, token_hash, created_at, expires_at) VALUES ($1, $2, $3, $4, encode(sha256(convert_to($5, 'UTF8')), 'hex'), $6::timestamptz, $6::timestamptz + interval '30 days')`
		}, func(tn rlsTenant, _ string) []any { return []any{uuid.New(), tn.org, tn.project, tn.family, id(), now} }},
		{"scope_consents", func(s string) string {
			return `INSERT INTO ` + s + `.scope_consents (id, public_id, org_id, project_id, end_user_id, client_id, resource_server_id, scopes, granted_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, '{openid,read}', $8, $8)`
		}, func(tn rlsTenant, _ string) []any {
			return []any{uuid.New(), "k" + id()[:12], tn.org, tn.project, tn.endUser, tn.client, tn.rs, now}
		}},
		{"upstream_providers", func(s string) string {
			return `INSERT INTO ` + s + `.upstream_providers (id, public_id, org_id, project_id, kind, client_id, client_secret_sealed, created_at, updated_at) VALUES ($1, $2, $3, $4, 'google', 'cid.apps.googleusercontent.com', decode(repeat('ab', 40), 'hex'), $5, $5)`
		}, func(tn rlsTenant, _ string) []any {
			return []any{tn.upstream, "u" + id()[:12], tn.org, tn.project, now}
		}},
		{"end_user_identities", func(s string) string {
			return `INSERT INTO ` + s + `.end_user_identities (id, public_id, org_id, project_id, end_user_id, upstream_provider_id, issuer, subject, email, email_verified, linked_at, last_used_at) VALUES ($1, $2, $3, $4, $5, $6, 'https://accounts.google.com', $7, 'g@example.com', true, $8, $8)`
		}, func(tn rlsTenant, _ string) []any {
			return []any{uuid.New(), "i" + id()[:12], tn.org, tn.project, tn.endUser, tn.upstream, "sub-" + id(), now}
		}},
		{"resource_server_secrets", func(s string) string {
			return `INSERT INTO ` + s + `.resource_server_secrets (id, public_id, org_id, project_id, resource_server_id, secret_hash, hint, created_at) VALUES ($1, $2, $3, $4, $5, 'sha256$' || encode(sha256(convert_to($6, 'UTF8')), 'hex'), 'abcd', $7)`
		}, func(tn rlsTenant, _ string) []any {
			return []any{uuid.New(), "r" + id()[:12], tn.org, tn.project, tn.rs, id(), now}
		}},
		{"personal_tokens", func(s string) string {
			return `INSERT INTO ` + s + `.personal_tokens (id, public_id, org_id, project_id, end_user_id, resource_server_id, name, token_hash, hint, scopes, created_at, expires_at) VALUES ($1, $2, $3, $4, $5, $6, 'cli', 'sha256$' || encode(sha256(convert_to($7, 'UTF8')), 'hex'), 'abcd', '{read}', $8::timestamptz, $8::timestamptz + interval '30 days')`
		}, func(tn rlsTenant, _ string) []any {
			return []any{uuid.New(), "t" + id()[:12], tn.org, tn.project, tn.endUser, tn.rs, id(), now}
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

// SECURITY: as the NOBYPASSRLS service role every org-owned table shows one tenant's rows, none without a scope, and refuses cross-tenant writes.
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

//nolint:gochecknoglobals // the project-owned client tables.
var clientTables = []string{"oauth_clients", "oauth_client_secrets", "resource_servers", "oauth_client_resources"}

//nolint:gochecknoglobals // the project-owned end-user tables.
var endUserTables = []string{"end_users", "browser_sessions", "user_sessions", "end_user_terms", "magic_links"}

//nolint:gochecknoglobals // the project-owned grant tables.
var grantTables = []string{"grant_families", "authorization_codes", "access_tokens", "refresh_tokens", "scope_consents"}

//nolint:gochecknoglobals // the project-owned federation tables.
var federationTables = []string{"upstream_providers", "end_user_identities"}

//nolint:gochecknoglobals // the project-owned portal tables.
var portalTables = []string{"resource_server_secrets", "personal_tokens"}

func (f *rlsFixture) siblingProject(t *testing.T) uuid.UUID {
	t.Helper()
	sibling := uuid.New()
	_, err := f.admin.ExecContext(t.Context(),
		`INSERT INTO `+f.schema+`.projects (id, public_id, org_id, slug, name, issuer_handle, created_at, updated_at) VALUES ($1, $2, $3, $4, 'P', $4, $5, $5)`,
		sibling, "p"+sibling.String()[:12], f.a.org, "p"+sibling.String()[:12], time.Now().UTC())
	require.NoError(t, err)
	return sibling
}

// SECURITY: clients, resource servers, end users, their sessions, terms, magic links and grants are scoped to their project as well as their org, for reads and writes.
func TestRLS_ProjectOwnedTablesAreProjectScoped(t *testing.T) {
	f := newRLSFixture(t)
	sibling := f.siblingProject(t)
	own := tenant.Into(t.Context(), tenant.Context{OrgID: f.a.org, ProjectID: f.a.project})
	other := tenant.Into(t.Context(), tenant.Context{OrgID: f.a.org, ProjectID: sibling})
	orgOnly := tenant.Into(t.Context(), tenant.Context{OrgID: f.a.org})
	byTable := map[string]rlsInsert{}
	for _, ins := range rlsInserts() {
		byTable[ins.table] = ins
	}
	for _, table := range slices.Concat(clientTables, endUserTables, grantTables, federationTables, portalTables) {
		t.Run(table, func(t *testing.T) {
			assert.Equal(t, []uuid.UUID{f.a.org}, f.visible(t, own, table))
			assert.Empty(t, f.visible(t, other, table), "another project of the same org sees none")
			assert.Empty(t, f.visible(t, orgOnly, table), "an org scope without a project sees none")

			mine := f.a
			mine.client, mine.rs = uuid.New(), uuid.New()
			mine.endUser, mine.browser, mine.userSession, mine.family = uuid.New(), uuid.New(), uuid.New(), uuid.New()
			mine.upstream = uuid.New()
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

// SECURITY: consent rows are evidence: the service role reads and appends them but can't change or delete them.
func TestRLS_EndUserTermsAreAppendOnly(t *testing.T) {
	f := newRLSFixture(t)
	own := tenant.Into(t.Context(), tenant.Context{OrgID: f.a.org, ProjectID: f.a.project})
	for _, stmt := range []string{
		`UPDATE ` + f.schema + `.end_user_terms SET terms_url = 'https://evil.test/' WHERE end_user_id = $1`,
		`DELETE FROM ` + f.schema + `.end_user_terms WHERE end_user_id = $1`,
	} {
		var n int64
		require.NoError(t, db.RunInTx(own, f.svc, func(ctx context.Context) error {
			tx, _ := db.CurrentTx(ctx)
			res, err := tx.ExecContext(ctx, stmt, f.a.endUser)
			if err != nil {
				return err
			}
			n, err = res.RowsAffected()
			return err
		}))
		assert.Zero(t, n, "%s touches nothing, even in its own project", stmt)
	}
	var url string
	require.NoError(t, f.admin.QueryRowContext(t.Context(), `SELECT terms_url FROM `+f.schema+`.end_user_terms WHERE end_user_id = $1`, f.a.endUser).Scan(&url))
	assert.Equal(t, "https://acme.test/terms", url)
	assert.Equal(t, []uuid.UUID{f.a.org}, f.visible(t, own, "end_user_terms"), "the project still reads its rows")
}

// One live magic link per email and one live user session per browser and account; the same email may exist in another project; a project delete removes every end-user row.
func TestSchema_EndUserConstraints(t *testing.T) {
	f := newRLSFixture(t)
	ctx, now := t.Context(), time.Now().UTC()
	link := func(email string) error {
		_, err := f.admin.ExecContext(ctx,
			`INSERT INTO `+f.schema+`.magic_links (id, public_id, org_id, project_id, token_hash, email, return_to, created_at, expires_at)
			 VALUES ($1, $2, $3, $4, encode(sha256(convert_to($7, 'UTF8')), 'hex'), $5, '/p/x/account/', $6::timestamptz, $6::timestamptz + interval '15 minutes')`,
			uuid.New(), "m"+uuid.NewString()[:12], f.a.org, f.a.project, email, now, uuid.NewString())
		return err
	}
	require.NoError(t, link("ada@example.com"))
	assert.Equal(t, "23505", pgCode(link("ada@example.com")), "one unused link per email")
	_, err := f.admin.ExecContext(ctx, `UPDATE `+f.schema+`.magic_links SET used_at = $1 WHERE email = 'ada@example.com'`, now)
	require.NoError(t, err)
	assert.NoError(t, link("ada@example.com"), "a used link frees the email")
	assert.Equal(t, "23514", pgCode(link("Ada@Example.com")), "emails are stored normalized")

	sibling := f.siblingProject(t)
	var email string
	require.NoError(t, f.admin.QueryRowContext(ctx, `SELECT email FROM `+f.schema+`.end_users WHERE id = $1`, f.a.endUser).Scan(&email))
	_, err = f.admin.ExecContext(ctx,
		`INSERT INTO `+f.schema+`.end_users (id, public_id, org_id, project_id, email, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $6)`,
		uuid.New(), "e"+uuid.NewString()[:12], f.a.org, sibling, email, now)
	assert.NoError(t, err, "the same email is another end user in another project")
	_, err = f.admin.ExecContext(ctx,
		`INSERT INTO `+f.schema+`.end_users (id, public_id, org_id, project_id, email, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $6)`,
		uuid.New(), "e"+uuid.NewString()[:12], f.a.org, f.a.project, email, now)
	assert.Equal(t, "23505", pgCode(err), "one end user per email in a project")

	_, err = f.admin.ExecContext(ctx,
		`INSERT INTO `+f.schema+`.user_sessions (id, public_id, org_id, project_id, browser_session_id, end_user_id, method, auth_time, selected_at, created_at, last_seen_at)
		 VALUES ($1, $2, $3, $4, $5, $6, 'magic_link', $7, $7, $7, $7)`,
		uuid.New(), "s"+uuid.NewString()[:12], f.a.org, f.a.project, f.a.browser, f.a.endUser, now)
	assert.Equal(t, "23505", pgCode(err), "one live user session per browser and account")

	_, err = f.admin.ExecContext(ctx, `DELETE FROM `+f.schema+`.projects WHERE id = $1`, f.a.project)
	require.NoError(t, err)
	for _, table := range endUserTables {
		var n int
		require.NoError(t, f.admin.QueryRowContext(ctx, `SELECT count(*) FROM `+f.schema+`.`+table+` WHERE project_id = $1`, f.a.project).Scan(&n))
		assert.Zero(t, n, table)
	}
}

// SECURITY: one live refresh token per family, a stored PKCE challenge is S256, a code links a family only once used, grants stay inside their project and die with it.
func TestSchema_GrantConstraints(t *testing.T) {
	f := newRLSFixture(t)
	ctx, now := t.Context(), time.Now().UTC()
	refresh := func(family uuid.UUID) error {
		_, err := f.admin.ExecContext(ctx,
			`INSERT INTO `+f.schema+`.refresh_tokens (id, org_id, project_id, family_id, token_hash, created_at, expires_at)
			 VALUES ($1, $2, $3, $4, encode(sha256(convert_to($5, 'UTF8')), 'hex'), $6::timestamptz, $6::timestamptz + interval '30 days')`,
			uuid.New(), f.a.org, f.a.project, family, uuid.NewString(), now)
		return err
	}
	assert.Equal(t, "23505", pgCode(refresh(f.a.family)), "one live refresh token per family")
	_, err := f.admin.ExecContext(ctx, `UPDATE `+f.schema+`.refresh_tokens SET used_at = $1 WHERE family_id = $2`, now, f.a.family)
	require.NoError(t, err)
	assert.NoError(t, refresh(f.a.family), "a used token frees the family for its successor")

	code := func(challenge, method any, used any, family any) error {
		_, err := f.admin.ExecContext(ctx,
			`INSERT INTO `+f.schema+`.authorization_codes (id, org_id, project_id, code_hash, client_id, end_user_id, redirect_uri, scopes, code_challenge, code_challenge_method, auth_time, created_at, expires_at, used_at, family_id)
			 VALUES ($1, $2, $3, encode(sha256(convert_to($4, 'UTF8')), 'hex'), $5, $6, 'https://a.example/cb', '{openid}', $7, $8, $9::timestamptz, $9::timestamptz, $9::timestamptz + interval '1 minute', $10, $11)`,
			uuid.New(), f.a.org, f.a.project, uuid.NewString(), f.a.client, f.a.endUser, challenge, method, now, used, family)
		return err
	}
	challenge := strings.Repeat("a", 43)
	assert.NoError(t, code(nil, nil, nil, nil), "a confidential client may skip PKCE")
	assert.Equal(t, "23514", pgCode(code(challenge, "plain", nil, nil)), "plain is refused")
	assert.Equal(t, "23514", pgCode(code("short", "S256", nil, nil)), "a challenge is 43 characters")
	assert.Equal(t, "23514", pgCode(code(challenge, nil, nil, nil)), "a challenge needs its method")
	assert.Equal(t, "23514", pgCode(code(nil, "S256", nil, nil)), "a method needs its challenge")
	assert.Equal(t, "23514", pgCode(code(strings.Repeat("+", 43), "S256", nil, nil)), "a challenge is base64url")
	assert.Equal(t, "23514", pgCode(code(challenge, "S256", nil, f.a.family)), "an unused code has no family")
	assert.NoError(t, code(challenge, "S256", now, f.a.family))

	_, err = f.admin.ExecContext(ctx, `UPDATE `+f.schema+`.grant_families SET revoked_at = $1 WHERE id = $2`, now, f.a.family)
	assert.Equal(t, "23514", pgCode(err), "a revocation names its reason")
	_, err = f.admin.ExecContext(ctx, `UPDATE `+f.schema+`.grant_families SET revoked_at = $1, revoked_reason = 'bored' WHERE id = $2`, now, f.a.family)
	assert.Equal(t, "23514", pgCode(err), "an unknown reason is refused")

	_, err = f.admin.ExecContext(ctx,
		`INSERT INTO `+f.schema+`.access_tokens (id, org_id, project_id, family_id, token_hash, scopes, created_at, expires_at)
		 VALUES ($1, $2, $3, $4, encode(sha256(convert_to($5, 'UTF8')), 'hex'), '{openid}', $6::timestamptz, $6::timestamptz + interval '15 minutes')`,
		uuid.New(), f.a.org, f.siblingProject(t), f.a.family, uuid.NewString(), now)
	assert.Equal(t, "23503", pgCode(err), "a token can't join another project's family")

	_, err = f.admin.ExecContext(ctx, `DELETE FROM `+f.schema+`.user_sessions WHERE id = $1`, f.a.userSession)
	require.NoError(t, err)
	var sid sql.NullString
	var publicSID string
	require.NoError(t, f.admin.QueryRowContext(ctx,
		`SELECT user_session_id, user_session_public_id FROM `+f.schema+`.grant_families WHERE id = $1`, f.a.family).Scan(&sid, &publicSID))
	assert.False(t, sid.Valid, "an ended user session leaves the family")
	assert.Equal(t, "us-public", publicSID, "the family keeps the sid its ID tokens carry")

	_, err = f.admin.ExecContext(ctx, `DELETE FROM `+f.schema+`.grant_families WHERE id = $1`, f.a.family)
	require.NoError(t, err)
	var tokens, linked int
	require.NoError(t, f.admin.QueryRowContext(ctx,
		`SELECT (SELECT count(*) FROM `+f.schema+`.access_tokens WHERE project_id = $1) + (SELECT count(*) FROM `+f.schema+`.refresh_tokens WHERE project_id = $1)`,
		f.a.project).Scan(&tokens))
	assert.Zero(t, tokens, "a family takes its tokens with it")
	require.NoError(t, f.admin.QueryRowContext(ctx,
		`SELECT count(*) FROM `+f.schema+`.authorization_codes WHERE project_id = $1 AND family_id IS NOT NULL`, f.a.project).Scan(&linked))
	assert.Zero(t, linked, "its codes stay, unlinked, for replay detection until pruned")

	_, err = f.admin.ExecContext(ctx, `DELETE FROM `+f.schema+`.projects WHERE id = $1`, f.a.project)
	require.NoError(t, err)
	for _, table := range grantTables {
		var n int
		require.NoError(t, f.admin.QueryRowContext(ctx, `SELECT count(*) FROM `+f.schema+`.`+table+` WHERE project_id = $1`, f.a.project).Scan(&n))
		assert.Zero(t, n, table)
	}
}

// One remembered consent per end user, client and resource server (an OIDC-only one included); deleting a resource server ends its grants and consents.
func TestSchema_ConsentsAndResourceTokens(t *testing.T) {
	f := newRLSFixture(t)
	ctx, now := t.Context(), time.Now().UTC()
	consent := func(rs any) error {
		_, err := f.admin.ExecContext(ctx,
			`INSERT INTO `+f.schema+`.scope_consents (id, public_id, org_id, project_id, end_user_id, client_id, resource_server_id, scopes, granted_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, '{openid}', $8, $8)`,
			uuid.New(), "k"+uuid.NewString()[:12], f.a.org, f.a.project, f.a.endUser, f.a.client, rs, now)
		return err
	}
	assert.Equal(t, "23505", pgCode(consent(f.a.rs)), "one consent per end user, client and resource server")
	require.NoError(t, consent(nil), "an OIDC-only consent has no resource server")
	assert.Equal(t, "23505", pgCode(consent(nil)), "NULLS NOT DISTINCT: one OIDC-only consent too")

	_, err := f.admin.ExecContext(ctx, `UPDATE `+f.schema+`.access_tokens SET resource_server_id = $1, revoked_at = $2 WHERE family_id = $3`, f.a.rs, now, f.a.family)
	require.NoError(t, err)
	_, err = f.admin.ExecContext(ctx, `UPDATE `+f.schema+`.grant_families SET resource_server_id = $1, revoked_at = $2, revoked_reason = 'revoked' WHERE id = $3`, f.a.rs, now, f.a.family)
	require.NoError(t, err, "RFC 7009 revocation is a reason")

	_, err = f.admin.ExecContext(ctx, `DELETE FROM `+f.schema+`.oauth_client_resources WHERE resource_server_id = $1`, f.a.rs)
	require.NoError(t, err)
	_, err = f.admin.ExecContext(ctx, `DELETE FROM `+f.schema+`.resource_servers WHERE id = $1`, f.a.rs)
	require.NoError(t, err)
	for _, table := range []string{"grant_families", "access_tokens", "refresh_tokens"} {
		var n int
		require.NoError(t, f.admin.QueryRowContext(ctx, `SELECT count(*) FROM `+f.schema+`.`+table+` WHERE project_id = $1`, f.a.project).Scan(&n))
		assert.Zero(t, n, "%s: a deleted resource server takes its grants with it", table)
	}
	var consents int
	require.NoError(t, f.admin.QueryRowContext(ctx, `SELECT count(*) FROM `+f.schema+`.scope_consents WHERE project_id = $1`, f.a.project).Scan(&consents))
	assert.Equal(t, 1, consents, "only the OIDC-only consent survives")
}

// A CIMD shadow client is public, native, auth none and always asks for consent, with its metadata URL; resource servers hold their admission.
func TestSchema_CIMDColumns(t *testing.T) {
	f := newRLSFixture(t)
	ctx, now := t.Context(), time.Now().UTC()
	shadow := func(clientType, app, method, consent string, url, fetched any) error {
		_, err := f.admin.ExecContext(ctx,
			`INSERT INTO `+f.schema+`.oauth_clients (id, public_id, org_id, project_id, name, client_type, application_type, token_endpoint_auth_method,
			   grant_types, allowed_scopes, redirect_uris, consent_policy, registration_kind, metadata_url, metadata_fetched_at, metadata_expires_at, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, 'Claude', $5, $6, $7, '{authorization_code,refresh_token}', '{openid}', '{http://localhost/cb}', $8, 'cimd', $9, $10, $11, $11, $11)`,
			uuid.New(), "c"+uuid.NewString()[:12], f.a.org, f.a.project, clientType, app, method, consent, url, fetched, now)
		return err
	}
	url := "https://claude.ai/oauth/claude-code-client-metadata"
	require.NoError(t, shadow("public", "native", "none", "require", url, now))
	assert.Equal(t, "23505", pgCode(shadow("public", "native", "none", "require", url, now)), "one shadow per metadata URL in a project")
	other := "https://other.example/client"
	for name, err := range map[string]error{
		"confidential":     shadow("confidential", "web", "client_secret_basic", "require", other, now),
		"implicit consent": shadow("public", "native", "none", "implicit", other, now),
		"no metadata url":  shadow("public", "native", "none", "require", nil, now),
		"not fetched":      shadow("public", "native", "none", "require", other, nil),
		"http":             shadow("public", "native", "none", "require", "http://other.example/client", now),
	} {
		assert.Equal(t, "23514", pgCode(err), name)
	}
	_, err := f.admin.ExecContext(ctx, `UPDATE `+f.schema+`.oauth_clients SET metadata_url = $1 WHERE id = $2`, other, f.a.client)
	assert.Equal(t, "23514", pgCode(err), "a registered client has no metadata URL")

	for name, stmt := range map[string]string{
		"no scopes pinned":   `UPDATE ` + f.schema + `.resource_servers SET cimd_scopes = '{}' WHERE id = $1`,
		"descriptions array": `UPDATE ` + f.schema + `.resource_servers SET scope_descriptions = '[]' WHERE id = $1`,
	} {
		_, err := f.admin.ExecContext(ctx, stmt, f.a.rs)
		assert.Equal(t, "23514", pgCode(err), name)
	}
	_, err = f.admin.ExecContext(ctx,
		`UPDATE `+f.schema+`.resource_servers SET accept_cimd_clients = true, cimd_client_hosts = '{claude.ai}', cimd_scopes = '{read}', scope_descriptions = '{"read":"Read"}' WHERE id = $1`, f.a.rs)
	assert.NoError(t, err)
}

// OIDC §5.7: one Othra end user per upstream (iss, sub), one identity per upstream per end user; an upstream with identities can't be deleted alone.
func TestSchema_FederationConstraints(t *testing.T) {
	f := newRLSFixture(t)
	ctx, now := t.Context(), time.Now().UTC()
	identity := func(endUser uuid.UUID, subject string) error {
		_, err := f.admin.ExecContext(ctx,
			`INSERT INTO `+f.schema+`.end_user_identities (id, public_id, org_id, project_id, end_user_id, upstream_provider_id, issuer, subject, linked_at, last_used_at)
			 VALUES ($1, $2, $3, $4, $5, $6, 'https://accounts.google.com', $7, $8, $8)`,
			uuid.New(), "i"+uuid.NewString()[:12], f.a.org, f.a.project, endUser, f.a.upstream, subject, now)
		return err
	}
	var sub string
	require.NoError(t, f.admin.QueryRowContext(ctx, `SELECT subject FROM `+f.schema+`.end_user_identities WHERE end_user_id = $1`, f.a.endUser).Scan(&sub))
	other := uuid.New()
	_, err := f.admin.ExecContext(ctx, `INSERT INTO `+f.schema+`.end_users (id, public_id, org_id, project_id, email, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $6)`,
		other, "e"+other.String()[:12], f.a.org, f.a.project, "other@example.com", now)
	require.NoError(t, err)
	assert.Equal(t, "23505", pgCode(identity(other, sub)), "one end user per (iss, sub)")
	assert.Equal(t, "23505", pgCode(identity(f.a.endUser, "another-sub")), "one Google identity per end user")
	require.NoError(t, identity(other, "another-sub"))

	_, err = f.admin.ExecContext(ctx, `INSERT INTO `+f.schema+`.upstream_providers (id, public_id, org_id, project_id, kind, client_id, client_secret_sealed, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'google', 'x', decode(repeat('ab', 40), 'hex'), $5, $5)`, uuid.New(), "u"+uuid.NewString()[:12], f.a.org, f.a.project, now)
	assert.Equal(t, "23505", pgCode(err), "one Google per project")
	_, err = f.admin.ExecContext(ctx, `DELETE FROM `+f.schema+`.upstream_providers WHERE id = $1`, f.a.upstream)
	assert.Equal(t, "23503", pgCode(err), "an upstream with linked identities can't be deleted")

	_, err = f.admin.ExecContext(ctx, `UPDATE `+f.schema+`.user_sessions SET method = 'google' WHERE id = $1`, f.a.userSession)
	assert.NoError(t, err, "a federated session")
	_, err = f.admin.ExecContext(ctx, `UPDATE `+f.schema+`.user_sessions SET method = 'github' WHERE id = $1`, f.a.userSession)
	assert.Equal(t, "23514", pgCode(err))
	_, err = f.admin.ExecContext(ctx, `UPDATE `+f.schema+`.magic_links SET pending_identity = '[]' WHERE project_id = $1`, f.a.project)
	assert.Equal(t, "23514", pgCode(err), "a pending identity is an object")

	_, err = f.admin.ExecContext(ctx, `DELETE FROM `+f.schema+`.projects WHERE id = $1`, f.a.project)
	require.NoError(t, err, "a project delete still removes the upstream and its identities")
	for _, table := range federationTables {
		var n int
		require.NoError(t, f.admin.QueryRowContext(ctx, `SELECT count(*) FROM `+f.schema+`.`+table+` WHERE project_id = $1`, f.a.project).Scan(&n))
		assert.Zero(t, n, table)
	}
}

// a personal token lives at most 90 days, has a name, scopes and a versioned hash; one current secret per resource server.
func TestSchema_PortalConstraints(t *testing.T) {
	f := newRLSFixture(t)
	ctx, now := t.Context(), time.Now().UTC()
	token := func(name, hash, scopes string, lifetime time.Duration) error {
		_, err := f.admin.ExecContext(ctx,
			`INSERT INTO `+f.schema+`.personal_tokens (id, public_id, org_id, project_id, end_user_id, resource_server_id, name, token_hash, hint, scopes, created_at, expires_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'abcd', $9::text[], $10::timestamptz, $10::timestamptz + $11::interval)`,
			uuid.New(), "t"+uuid.NewString()[:12], f.a.org, f.a.project, f.a.endUser, f.a.rs, name, hash, scopes, now,
			fmt.Sprintf("%d seconds", int(lifetime.Seconds())))
		return err
	}
	require.NoError(t, token("cli", "sha256$"+strings.Repeat("a", 64), "{read}", 90*24*time.Hour), "exactly 90 days")
	assert.Equal(t, "23514", pgCode(token("cli", "sha256$"+strings.Repeat("b", 64), "{read}", 90*24*time.Hour+time.Second)), "over 90 days")
	assert.Equal(t, "23514", pgCode(token("cli", "sha256$"+strings.Repeat("c", 64), "{read}", 0)), "already expired")
	assert.Equal(t, "23514", pgCode(token("", "sha256$"+strings.Repeat("d", 64), "{read}", time.Hour)), "a name")
	assert.Equal(t, "23514", pgCode(token("cli", "sha256$"+strings.Repeat("e", 64), "{}", time.Hour)), "at least one scope")
	assert.Equal(t, "23514", pgCode(token("cli", strings.Repeat("f", 64), "{read}", time.Hour)), "the versioned hash form")
	assert.Equal(t, "23505", pgCode(token("cli", "sha256$"+strings.Repeat("a", 64), "{read}", time.Hour)), "one row per hash")

	var accept bool
	require.NoError(t, f.admin.QueryRowContext(ctx, `SELECT accept_personal_tokens FROM `+f.schema+`.resource_servers WHERE id = $1`, f.a.rs).Scan(&accept))
	assert.False(t, accept, "a resource server accepts personal tokens only when switched on")

	secret := func(expires *time.Time) error {
		_, err := f.admin.ExecContext(ctx,
			`INSERT INTO `+f.schema+`.resource_server_secrets (id, public_id, org_id, project_id, resource_server_id, secret_hash, hint, created_at, expires_at)
			 VALUES ($1, $2, $3, $4, $5, 'sha256$' || encode(sha256(convert_to($6, 'UTF8')), 'hex'), 'abcd', $7, $8)`,
			uuid.New(), "r"+uuid.NewString()[:12], f.a.org, f.a.project, f.a.rs, uuid.NewString(), now, expires)
		return err
	}
	grace := now.Add(time.Hour)
	require.NoError(t, secret(&grace), "a rotated secret in its grace period")
	assert.Equal(t, "23505", pgCode(secret(nil)), "the fixture already has one current secret")

	_, err := f.admin.ExecContext(ctx, `DELETE FROM `+f.schema+`.oauth_client_resources WHERE resource_server_id = $1`, f.a.rs)
	require.NoError(t, err, "a linked server is unlinked before it can be deleted")
	_, err = f.admin.ExecContext(ctx, `DELETE FROM `+f.schema+`.resource_servers WHERE id = $1`, f.a.rs)
	require.NoError(t, err, "deleting the resource server removes its secrets and tokens")
	for _, table := range portalTables {
		var n int
		require.NoError(t, f.admin.QueryRowContext(ctx, `SELECT count(*) FROM `+f.schema+`.`+table+` WHERE resource_server_id = $1`, f.a.rs).Scan(&n))
		assert.Zero(t, n, table)
	}
}
