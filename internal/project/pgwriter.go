package project

import (
	"context"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/schema/entity"
)

// Save upserts a project by id, bumping updated_at; the issuer handle and its lock change only through their own methods.
func (s *postgresStore) Save(ctx context.Context, p *Project) error {
	now := time.Now().UTC()
	t := s.projects
	stmt := t.INSERT(t.AllColumns).
		VALUES(
			p.ID, p.PublicID, p.OrgID, p.Slug, p.Name, createdByExpr(p), p.CreatedAt.UTC(), now,
			p.IssuerHandle, timeExpr(p.IssuerHandleLockedAt), p.DisplayName, p.PrimaryColor, p.SupportEmail,
			p.TermsURL, p.PrivacyURL, p.TermsVersion, timeExpr(p.TermsPublishedAt), p.SignInEmailLink,
		).
		ON_CONFLICT(t.ID).
		DO_UPDATE(postgres.SET(
			t.Slug.SET(postgres.String(p.Slug)),
			t.Name.SET(postgres.String(p.Name)),
			t.DisplayName.SET(postgres.String(p.DisplayName)),
			t.PrimaryColor.SET(postgres.String(p.PrimaryColor)),
			t.SupportEmail.SET(postgres.String(p.SupportEmail)),
			t.TermsURL.SET(postgres.String(p.TermsURL)),
			t.PrivacyURL.SET(postgres.String(p.PrivacyURL)),
			t.TermsVersion.SET(postgres.Int32(p.TermsVersion)),
			t.TermsPublishedAt.SET(timeExpr(p.TermsPublishedAt)),
			t.SignInEmailLink.SET(postgres.Bool(p.SignInEmailLink)),
			t.UpdatedAt.SET(postgres.TimestampzT(now)),
		))
	if err := db.Write(ctx, s.pool, func(q qrm.DB) error { _, err := stmt.ExecContext(ctx, q); return err }); err != nil {
		return s.saveError("Save", p, err)
	}
	return nil
}

// SetIssuerHandle changes the handle of the project id while it is unlocked.
func (s *postgresStore) SetIssuerHandle(ctx context.Context, id uuid.UUID, handle string) error {
	t := s.projects
	// SECURITY: the unlocked check and the write are one conditional UPDATE, so a client created meanwhile cannot be raced.
	stmt := t.UPDATE(t.IssuerHandle, t.UpdatedAt).
		SET(postgres.String(handle), postgres.TimestampzT(time.Now().UTC())).
		WHERE(t.ID.EQ(postgres.UUID(id)).AND(t.IssuerHandleLockedAt.IS_NULL()))
	var n int64
	err := db.Write(ctx, s.pool, func(q qrm.DB) error {
		res, err := stmt.ExecContext(ctx, q)
		if err != nil {
			return err
		}
		n, err = res.RowsAffected()
		return err
	})
	if err != nil {
		return s.saveError("SetIssuerHandle", &Project{IssuerHandle: handle}, err)
	}
	if n == 0 {
		if _, err := s.ByID(ctx, id); err != nil {
			return err
		}
		return &IssuerHandleLockedError{}
	}
	return nil
}

// LockIssuerHandle freezes the handle of the project id at at; a locked handle keeps its first lock time.
func (s *postgresStore) LockIssuerHandle(ctx context.Context, id uuid.UUID, at time.Time) error {
	t := s.projects
	stmt := t.UPDATE(t.IssuerHandleLockedAt).
		SET(postgres.TimestampzT(at.UTC())).
		WHERE(t.ID.EQ(postgres.UUID(id)).AND(t.IssuerHandleLockedAt.IS_NULL()))
	if err := db.Write(ctx, s.pool, func(q qrm.DB) error { _, err := stmt.ExecContext(ctx, q); return err }); err != nil {
		return fmt.Errorf("project.postgres: LockIssuerHandle: %w", err)
	}
	return nil
}

// SaveLogo stores l as its project's only logo.
func (s *postgresStore) SaveLogo(ctx context.Context, l *Logo) error {
	t := s.logos
	stmt := t.INSERT(t.AllColumns).
		VALUES(l.ProjectID, l.OrgID, l.ContentType, l.Content, l.SHA256, l.UpdatedAt.UTC()).
		ON_CONFLICT(t.ProjectID).
		DO_UPDATE(postgres.SET(
			t.ContentType.SET(postgres.String(l.ContentType)),
			t.Content.SET(postgres.Bytea(l.Content)),
			t.SHA256.SET(postgres.String(l.SHA256)),
			t.UpdatedAt.SET(postgres.TimestampzT(l.UpdatedAt.UTC())),
		))
	if err := db.Write(ctx, s.pool, func(q qrm.DB) error { _, err := stmt.ExecContext(ctx, q); return err }); err != nil {
		return fmt.Errorf("project.postgres: SaveLogo: %w", err)
	}
	return nil
}

// DeleteLogo removes the project's logo; deleting a missing logo is not an error.
func (s *postgresStore) DeleteLogo(ctx context.Context, projectID uuid.UUID) error {
	stmt := s.logos.DELETE().WHERE(s.logos.ProjectID.EQ(postgres.UUID(projectID)))
	if err := db.Write(ctx, s.pool, func(q qrm.DB) error { _, err := stmt.ExecContext(ctx, q); return err }); err != nil {
		return fmt.Errorf("project.postgres: DeleteLogo: %w", err)
	}
	return nil
}

func (s *postgresStore) saveError(op string, p *Project, err error) error {
	if db.IsUniqueViolation(err) {
		if name, ok := db.ViolatedConstraint(err); ok && name == s.handleIndex {
			return &IssuerHandleTakenError{Handle: p.IssuerHandle}
		}
		return &AlreadyExistsError{Slug: p.Slug}
	}
	return fmt.Errorf("project.postgres: %s: %w", op, err)
}

func createdByExpr(p *Project) postgres.Expression {
	if p.CreatedBy.Valid {
		return postgres.UUID(p.CreatedBy.UUID)
	}
	return entity.NullUUID()
}

func timeExpr(t *time.Time) postgres.TimestampzExpression {
	if t == nil {
		return postgres.TimestampzExp(postgres.NULL)
	}
	return postgres.TimestampzT(t.UTC())
}
