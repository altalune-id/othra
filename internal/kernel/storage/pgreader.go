package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"

	"altalune.id/othra/internal/kernel/db"
)

// ByPublicID loads a file, or *UploadNotFoundError when the org has none with that public id.
func (s *postgresStore) ByPublicID(ctx context.Context, publicID string) (*File, error) {
	t := s.files
	stmt := postgres.SELECT(t.AllColumns).FROM(t).WHERE(t.PublicID.EQ(postgres.String(publicID))).LIMIT(1)
	var row fileRow
	if err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) }); err != nil {
		if db.IsNoRows(err) {
			return nil, &UploadNotFoundError{PublicID: publicID}
		}
		return nil, fmt.Errorf("storage.postgres: ByPublicID: %w", err)
	}
	return row.toFile(), nil
}

// Sweepable returns up to limit files that are deleting, or pending past their expiry at now.
func (s *postgresStore) Sweepable(ctx context.Context, now time.Time, limit int) ([]*File, error) {
	t := s.files
	stmt := postgres.SELECT(t.AllColumns).FROM(t).
		WHERE(t.Status.EQ(postgres.String(string(StatusDeleting))).
			OR(t.Status.EQ(postgres.String(string(StatusPending))).AND(t.ExpiresAt.LT(postgres.TimestampzT(now.UTC()))))).
		ORDER_BY(t.ExpiresAt.ASC()).
		LIMIT(int64(limit))
	var rows []fileRow
	if err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &rows) }); err != nil && !db.IsNoRows(err) {
		return nil, fmt.Errorf("storage.postgres: Sweepable: %w", err)
	}
	out := make([]*File, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].toFile())
	}
	return out, nil
}
