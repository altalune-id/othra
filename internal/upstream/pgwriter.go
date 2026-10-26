package upstream

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/db"
)

// Insert records u with its sealed secret, or returns *ExistsError when the project has the kind.
func (s *postgresStore) Insert(ctx context.Context, u *Upstream, sealed []byte) error {
	t := s.upstreams
	stmt := t.INSERT(t.ID, t.PublicID, t.OrgID, t.ProjectID, t.Kind, t.ClientID, t.ClientSecretSealed, t.Enabled, t.CreatedAt, t.UpdatedAt).
		VALUES(u.ID, u.PublicID, u.OrgID, u.ProjectID, string(u.Kind), u.ClientID, sealed, u.Enabled, u.CreatedAt.UTC(), u.UpdatedAt.UTC())
	err := db.Write(ctx, s.pool, func(q qrm.DB) error { _, err := stmt.ExecContext(ctx, q); return err })
	if db.IsUniqueViolation(err) {
		if name, ok := db.ViolatedConstraint(err); ok && name == s.kindKey {
			return &ExistsError{}
		}
	}
	if err != nil {
		return wrap("Insert", err)
	}
	return nil
}

// Update saves u's client id, enabled flag and updated_at, and its sealed secret when sealed is not nil.
func (s *postgresStore) Update(ctx context.Context, u *Upstream, sealed []byte) error {
	t := s.upstreams
	stmt := t.UPDATE(t.ClientID, t.Enabled, t.UpdatedAt).SET(u.ClientID, u.Enabled, u.UpdatedAt.UTC())
	if sealed != nil {
		stmt = t.UPDATE(t.ClientID, t.Enabled, t.UpdatedAt, t.ClientSecretSealed).SET(u.ClientID, u.Enabled, u.UpdatedAt.UTC(), sealed)
	}
	stmt = stmt.WHERE(t.ID.EQ(postgres.UUID(u.ID)).AND(t.ProjectID.EQ(postgres.UUID(u.ProjectID))))
	if err := db.Write(ctx, s.pool, func(q qrm.DB) error { _, err := stmt.ExecContext(ctx, q); return err }); err != nil {
		return wrap("Update", err)
	}
	return nil
}

// Delete removes the upstream id, or returns *HasIdentitiesError while end users have linked it.
func (s *postgresStore) Delete(ctx context.Context, projectID, id uuid.UUID) error {
	t := s.upstreams
	stmt := t.DELETE().WHERE(t.ID.EQ(postgres.UUID(id)).AND(t.ProjectID.EQ(postgres.UUID(projectID))))
	err := db.Write(ctx, s.pool, func(q qrm.DB) error { _, err := stmt.ExecContext(ctx, q); return err })
	switch {
	case db.IsForeignKeyViolation(err):
		return &HasIdentitiesError{}
	case err != nil:
		return wrap("Delete", err)
	}
	return nil
}

// Lock is ByPublicID that also locks the row until the unit of work ends.
func (s *postgresStore) Lock(ctx context.Context, projectID uuid.UUID, publicID string) (*Upstream, error) {
	t := s.upstreams
	return s.one(ctx, "Lock", postgres.SELECT(s.columns()).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.PublicID.EQ(postgres.String(publicID)))).FOR(postgres.UPDATE().OF(t)), db.Write)
}
