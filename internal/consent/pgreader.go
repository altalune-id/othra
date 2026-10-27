package consent

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/db"
)

// Find returns the consent key names, or nil.
func (s *postgresStore) Find(ctx context.Context, projectID uuid.UUID, key Key) (*Consent, error) {
	stmt := postgres.SELECT(s.columns()).FROM(s.consents).WHERE(s.keyed(projectID, key))
	var row consentRow
	err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) })
	switch {
	case db.IsNoRows(err):
		return nil, nil
	case err != nil:
		return nil, wrap("Find", err)
	}
	return row.toConsent()
}

// ListByEndUser returns every consent of the end user.
func (s *postgresStore) ListByEndUser(ctx context.Context, projectID, endUserID uuid.UUID) ([]*Consent, error) {
	t := s.consents
	stmt := postgres.SELECT(s.columns()).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.EndUserID.EQ(postgres.UUID(endUserID)))).
		ORDER_BY(t.GrantedAt.ASC(), t.ID.ASC())
	var rows []consentRow
	if err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &rows) }); err != nil && !db.IsNoRows(err) {
		return nil, wrap("ListByEndUser", err)
	}
	out := make([]*Consent, 0, len(rows))
	for i := range rows {
		c, err := rows[i].toConsent()
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}
