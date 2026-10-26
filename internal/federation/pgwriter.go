package federation

import (
	"context"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/db"
)

// Insert links id to endUserID, or returns errSubjectTaken or errUpstreamTaken.
func (s *postgresStore) Insert(
	ctx context.Context, orgID, projectID, endUserID uuid.UUID, id Identity, publicID string, now time.Time,
) error {
	t := s.identities
	at := now.UTC()
	stmt := t.INSERT(t.ID, t.PublicID, t.OrgID, t.ProjectID, t.EndUserID, t.UpstreamProviderID, t.Issuer, t.Subject, t.Email,
		t.EmailVerified, t.LinkedAt, t.LastUsedAt).
		VALUES(uuid.New(), publicID, orgID, projectID, endUserID, id.UpstreamID, id.Issuer, id.Subject, nullable(id.Email),
			id.EmailVerified, at, at)
	_, err := s.exec(ctx, "Insert", stmt)
	if name, ok := db.ViolatedConstraint(err); ok {
		switch name {
		case s.subjectKey:
			return errSubjectTaken
		case s.upstreamKey:
			return errUpstreamTaken
		}
	}
	return err
}

// Touch records a sign-in and the upstream's email now. NOTE: last_used_at never moves back, nor before linked_at, whatever the clocks.
func (s *postgresStore) Touch(ctx context.Context, projectID, id uuid.UUID, email string, emailVerified bool, now time.Time) error {
	t := s.identities
	stmt := t.UPDATE(t.Email, t.EmailVerified, t.LastUsedAt).
		SET(nullable(email), postgres.Bool(emailVerified), postgres.GREATEST(t.LinkedAt, t.LastUsedAt, postgres.TimestampzT(now.UTC()))).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.ID.EQ(postgres.UUID(id))))
	_, err := s.exec(ctx, "Touch", stmt)
	return err
}

// Delete removes the end user's identity publicID; ok is false when there is none.
func (s *postgresStore) Delete(ctx context.Context, projectID, endUserID uuid.UUID, publicID string) (bool, error) {
	t := s.identities
	stmt := t.DELETE().WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.EndUserID.EQ(postgres.UUID(endUserID))).
		AND(t.PublicID.EQ(postgres.String(publicID))))
	n, err := s.exec(ctx, "Delete", stmt)
	return n > 0, err
}
