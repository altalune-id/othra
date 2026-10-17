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

// Logo returns the project's logo, or nil when it has none.
func (s *postgresStore) Logo(ctx context.Context, projectID uuid.UUID) (*Logo, error) {
	stmt := postgres.SELECT(s.logos.AllColumns).FROM(s.logos).WHERE(s.logos.ProjectID.EQ(postgres.UUID(projectID))).LIMIT(1)
	var row struct {
		ProjectID   uuid.UUID `alias:"project_logos.project_id"`
		OrgID       uuid.UUID `alias:"project_logos.org_id"`
		ContentType string    `alias:"project_logos.content_type"`
		Content     []byte    `alias:"project_logos.content"`
		SHA256      string    `alias:"project_logos.sha256"`
		UpdatedAt   time.Time `alias:"project_logos.updated_at"`
	}
	if err := db.Read(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) }); err != nil {
		if db.IsNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("project.postgres: Logo: %w", err)
	}
	return &Logo{
		ProjectID: row.ProjectID, OrgID: row.OrgID, ContentType: row.ContentType, Content: row.Content,
		SHA256: row.SHA256, UpdatedAt: row.UpdatedAt.UTC(),
	}, nil
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
		LogoUpdatedAt    *time.Time `alias:"issuers.logo_updated_at"`
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
		LogoUpdatedAt: utcPtr(row.LogoUpdatedAt), TermsURL: row.TermsURL, PrivacyURL: row.PrivacyURL,
		TermsVersion: row.TermsVersion, TermsPublishedAt: utcPtr(row.TermsPublishedAt), SignInEmailLink: row.SignInEmailLink,
	}, nil
}
