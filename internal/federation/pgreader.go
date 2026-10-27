package federation

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"

	"github.com/altalune-id/othra/internal/kernel/db"
)

// BySubject returns the identity (issuer, subject), or nil.
func (s *postgresStore) BySubject(ctx context.Context, projectID uuid.UUID, issuer, subject string) (*storedIdentity, error) {
	t := s.identities
	stmt := postgres.SELECT(s.columns()).FROM(s.from()).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.Issuer.EQ(postgres.String(issuer))).AND(t.Subject.EQ(postgres.String(subject))))
	var row identityRow
	err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) })
	switch {
	case db.IsNoRows(err):
		return nil, nil
	case err != nil:
		return nil, wrap("BySubject", err)
	}
	return &storedIdentity{ID: row.ID, PublicID: row.PublicID, EndUserID: row.EndUserID, Identity: row.toIdentity()}, nil
}

// ListByEndUser returns the end user's identities with their upstream kind, oldest first.
func (s *postgresStore) ListByEndUser(ctx context.Context, projectID, endUserID uuid.UUID) ([]Identity, error) {
	t := s.identities
	stmt := postgres.SELECT(s.columns()).FROM(s.from()).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.EndUserID.EQ(postgres.UUID(endUserID)))).
		ORDER_BY(t.LinkedAt.ASC(), t.PublicID.ASC())
	var rows []identityRow
	if err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &rows) }); err != nil && !db.IsNoRows(err) {
		return nil, wrap("ListByEndUser", err)
	}
	out := make([]Identity, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].toIdentity())
	}
	return out, nil
}

// LockEndUser locks the end user's row for the unit of work; ok is false when it is gone.
func (s *postgresStore) LockEndUser(ctx context.Context, projectID, endUserID uuid.UUID) (bool, error) {
	u := s.users
	stmt := postgres.SELECT(u.ID).FROM(u).WHERE(u.ProjectID.EQ(postgres.UUID(projectID)).AND(u.ID.EQ(postgres.UUID(endUserID)))).
		FOR(postgres.UPDATE())
	var row struct {
		ID uuid.UUID `alias:"end_users.id"`
	}
	err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) })
	switch {
	case db.IsNoRows(err):
		return false, nil
	case err != nil:
		return false, wrap("LockEndUser", err)
	}
	return true, nil
}

// EveryEndUserLinked reports whether every end user of the project has an identity.
func (s *postgresStore) EveryEndUserLinked(ctx context.Context, projectID uuid.UUID) (bool, error) {
	u, t := s.users, s.identities
	project := postgres.UUID(projectID)
	stmt := postgres.SELECT(u.ID).FROM(u).
		WHERE(u.ProjectID.EQ(project).AND(postgres.NOT(postgres.EXISTS(
			postgres.SELECT(t.ID).FROM(t).WHERE(t.ProjectID.EQ(project).AND(t.EndUserID.EQ(u.ID))),
		)))).
		LIMIT(1)
	var rows []struct {
		ID uuid.UUID `alias:"end_users.id"`
	}
	if err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &rows) }); err != nil && !db.IsNoRows(err) {
		return false, wrap("EveryEndUserLinked", err)
	}
	return len(rows) == 0, nil
}
