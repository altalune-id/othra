package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"

	"github.com/altalune-id/othra/internal/kernel/db"
)

func (s *resetPostgres) HasRecentUnused(ctx context.Context, userID uuid.UUID, since time.Time) (bool, error) {
	t := s.resets
	stmt := postgres.SELECT(t.ID).FROM(t).WHERE(
		t.UserID.EQ(postgres.UUID(userID)).AND(t.UsedAt.IS_NULL()).AND(t.CreatedAt.GT(postgres.TimestampzT(since.UTC()))),
	).LIMIT(1)
	var row struct {
		ID uuid.UUID `alias:"password_resets.id"`
	}
	if err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) }); err != nil {
		if db.IsNoRows(err) {
			return false, nil
		}
		return false, fmt.Errorf("auth.resets: HasRecentUnused: %w", err)
	}
	return true, nil
}

// NOTE: reads use the writer; a lagging replica could show a used reset as usable.
func (s *resetPostgres) ByTokenHash(ctx context.Context, hash string) (*Reset, error) {
	t := s.resets
	return s.one(ctx, "ByTokenHash", postgres.SELECT(t.AllColumns).FROM(t).WHERE(t.TokenHash.EQ(postgres.String(hash))))
}

func (s *resetPostgres) ByPublicID(ctx context.Context, publicID string) (*Reset, error) {
	t := s.resets
	return s.one(ctx, "ByPublicID", postgres.SELECT(t.AllColumns).FROM(t).WHERE(t.PublicID.EQ(postgres.String(publicID))))
}

func (s *resetPostgres) one(ctx context.Context, op string, stmt postgres.SelectStatement) (*Reset, error) {
	var row resetRow
	if err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) }); err != nil {
		if db.IsNoRows(err) {
			return nil, &ResetNotUsableError{Reason: ResetNotFound}
		}
		return nil, fmt.Errorf("auth.resets: %s: %w", op, err)
	}
	return row.toReset(), nil
}
