package project

import (
	"github.com/go-jet/jet/v2/postgres"

	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/schema/entity"
)

type postgresStore struct {
	pool          db.Pool
	projects      *entity.Projects
	orgs          *entity.Orgs
	logoKeyExpr   postgres.Expression
	handleIndex   string
	resolveIssuer string
	handleExists  string
	setLogoFile   string
}

// NewStore builds the Postgres store over pool.
func NewStore(pool db.Pool, schema, tablePrefix string) *postgresStore {
	if schema == "" {
		schema = "public"
	}
	fn := schema + "." + tablePrefix
	return &postgresStore{
		pool:        pool,
		projects:    entity.NewProjects(schema, tablePrefix),
		orgs:        entity.NewOrgs(schema, tablePrefix),
		handleIndex: tablePrefix + "projects_issuer_handle_key",
		// NOTE: Raw because go-jet has no builder for a correlated subquery in a projection.
		logoKeyExpr: postgres.Raw("(SELECT f.object_key FROM " + fn + "files f WHERE f.id = projects.logo_file_id AND f.status = 'ready')"),
		// NOTE: RawStatement because go-jet has no builder for a function in FROM position; the aliases mirror jet's so the rows map.
		resolveIssuer: `SELECT i.id AS "issuers.project_id", i.public_id AS "issuers.public_id", i.org_id AS "issuers.org_id", ` +
			`i.issuer_handle AS "issuers.handle", i.name AS "issuers.name", i.display_name AS "issuers.display_name", ` +
			`i.primary_color AS "issuers.primary_color", i.support_email AS "issuers.support_email", ` +
			`i.logo_object_key AS "issuers.logo_object_key", i.terms_url AS "issuers.terms_url", i.privacy_url AS "issuers.privacy_url", ` +
			`i.terms_version AS "issuers.terms_version", i.terms_published_at AS "issuers.terms_published_at", ` +
			`i.signin_email_link AS "issuers.signin_email_link" FROM ` + fn + "resolve_project_by_issuer_handle(#handle) i",
		handleExists: `SELECT ` + fn + `issuer_handle_exists(#handle) AS "handles.exists"`,
		// NOTE: RawStatement because go-jet has no UPDATE … FROM; the locked subselect returns the logo being replaced.
		setLogoFile: `UPDATE ` + fn + `projects p SET logo_file_id = #file, updated_at = #now ` +
			`FROM (SELECT id, logo_file_id FROM ` + fn + `projects WHERE id = #id FOR UPDATE) old ` +
			`WHERE p.id = old.id RETURNING old.logo_file_id AS "logos.previous"`,
	}
}

func (s *postgresStore) columns() postgres.ProjectionList {
	return postgres.ProjectionList{s.projects.AllColumns, s.logoKeyExpr.AS("projects.logo_object_key")}
}
