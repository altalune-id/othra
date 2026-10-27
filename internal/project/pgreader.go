package project

import (
	"context"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/db"
)

// ByID loads a project by id.
func (s *postgresStore) ByID(ctx context.Context, id uuid.UUID) (*Project, error) {
	stmt := postgres.SELECT(s.columns()).
		FROM(s.projects).
		WHERE(s.projects.ID.EQ(postgres.UUID(id))).
		LIMIT(1)
	var row projectRow
	if err := db.Read(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) }); err != nil {
		if db.IsNoRows(err) {
			return nil, &NotFoundError{ID: id.String()}
		}
		return nil, fmt.Errorf("project.postgres: ByID: %w", err)
	}
	return row.toProject(), nil
}

// ByPublicID loads a project by its public id.
func (s *postgresStore) ByPublicID(ctx context.Context, publicID string) (*Project, error) {
	stmt := postgres.SELECT(s.columns()).
		FROM(s.projects).
		WHERE(s.projects.PublicID.EQ(postgres.String(publicID))).
		LIMIT(1)
	var row projectRow
	if err := db.Read(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) }); err != nil {
		if db.IsNoRows(err) {
			return nil, &NotFoundError{ID: publicID}
		}
		return nil, fmt.Errorf("project.postgres: ByPublicID: %w", err)
	}
	return row.toProject(), nil
}

// RefByPublicID loads a project's ids and slugs, with its org's slug, by its public id.
func (s *postgresStore) RefByPublicID(ctx context.Context, publicID string) (*Ref, error) {
	p, o := s.projects, s.orgs
	stmt := postgres.SELECT(p.ID, p.OrgID, p.Slug, o.Slug).
		FROM(p.INNER_JOIN(o, o.ID.EQ(p.OrgID))).
		WHERE(p.PublicID.EQ(postgres.String(publicID))).
		LIMIT(1)
	var row struct {
		ID      uuid.UUID `alias:"projects.id"`
		OrgID   uuid.UUID `alias:"projects.org_id"`
		Slug    string    `alias:"projects.slug"`
		OrgSlug string    `alias:"orgs.slug"`
	}
	if err := db.Read(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) }); err != nil {
		if db.IsNoRows(err) {
			return nil, &NotFoundError{ID: publicID}
		}
		return nil, fmt.Errorf("project.postgres: RefByPublicID: %w", err)
	}
	return &Ref{ID: row.ID, OrgID: row.OrgID, Slug: row.Slug, OrgSlug: row.OrgSlug}, nil
}

// BySlug loads a project by slug within an organization.
func (s *postgresStore) BySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Project, error) {
	stmt := postgres.SELECT(s.columns()).
		FROM(s.projects).
		WHERE(s.projects.OrgID.EQ(postgres.UUID(orgID)).AND(s.projects.Slug.EQ(postgres.String(slug)))).
		LIMIT(1)
	var row projectRow
	if err := db.Read(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) }); err != nil {
		if db.IsNoRows(err) {
			return nil, &NotFoundError{Slug: slug}
		}
		return nil, fmt.Errorf("project.postgres: BySlug: %w", err)
	}
	return row.toProject(), nil
}

// List returns an organization's projects ordered by creation time.
func (s *postgresStore) List(ctx context.Context, orgID uuid.UUID) ([]*Project, error) {
	stmt := postgres.SELECT(s.columns()).
		FROM(s.projects).
		WHERE(s.projects.OrgID.EQ(postgres.UUID(orgID))).
		ORDER_BY(s.projects.CreatedAt.ASC())
	var rows []projectRow
	if err := db.Read(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &rows) }); err != nil {
		return nil, fmt.Errorf("project.postgres: List: %w", err)
	}
	out := make([]*Project, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].toProject())
	}
	return out, nil
}

// IssuerHandleExists reports whether any org's project uses handle, through a definer.
func (s *postgresStore) IssuerHandleExists(ctx context.Context, handle string) (bool, error) {
	var row struct {
		Exists bool `alias:"handles.exists"`
	}
	stmt := postgres.RawStatement(s.handleExists, postgres.RawArgs{"#handle": handle})
	if err := db.Read(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) }); err != nil {
		return false, fmt.Errorf("project.postgres: IssuerHandleExists: %w", err)
	}
	return row.Exists, nil
}

// ResolveIssuer finds the project behind handle through a definer, before any tenant scope exists.
func (s *postgresStore) ResolveIssuer(ctx context.Context, handle string) (*Issuer, error) {
	var row struct {
		ProjectID        uuid.UUID  `alias:"issuers.project_id"`
		PublicID         string     `alias:"issuers.public_id"`
		OrgID            uuid.UUID  `alias:"issuers.org_id"`
		Handle           string     `alias:"issuers.handle"`
		Name             string     `alias:"issuers.name"`
		DisplayName      string     `alias:"issuers.display_name"`
		PrimaryColor     string     `alias:"issuers.primary_color"`
		SupportEmail     string     `alias:"issuers.support_email"`
		LogoKey          *string    `alias:"issuers.logo_object_key"`
		TermsURL         string     `alias:"issuers.terms_url"`
		PrivacyURL       string     `alias:"issuers.privacy_url"`
		TermsVersion     int32      `alias:"issuers.terms_version"`
		TermsPublishedAt *time.Time `alias:"issuers.terms_published_at"`
		SignInEmailLink  bool       `alias:"issuers.signin_email_link"`
	}
	stmt := postgres.RawStatement(s.resolveIssuer, postgres.RawArgs{"#handle": handle})
	if err := db.Read(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) }); err != nil {
		if db.IsNoRows(err) {
			return nil, &NotFoundError{}
		}
		return nil, fmt.Errorf("project.postgres: ResolveIssuer: %w", err)
	}
	return &Issuer{
		ProjectID: row.ProjectID, OrgID: row.OrgID, PublicID: row.PublicID, Handle: row.Handle, Name: row.Name,
		DisplayName: row.DisplayName, PrimaryColor: row.PrimaryColor, SupportEmail: row.SupportEmail,
		LogoKey: derefString(row.LogoKey), TermsURL: row.TermsURL, PrivacyURL: row.PrivacyURL,
		TermsVersion: row.TermsVersion, TermsPublishedAt: utcPtr(row.TermsPublishedAt), SignInEmailLink: row.SignInEmailLink,
	}, nil
}
