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
