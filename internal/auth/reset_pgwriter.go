package auth

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

func (s *resetPostgres) Insert(ctx context.Context, r *Reset) error {
	t := s.resets
	stmt := t.INSERT(t.AllColumns).VALUES(
		r.ID, r.PublicID, r.UserID, r.TokenHash, r.ExpiresAt.UTC(), entity.NullTimestampz(), r.CreatedAt.UTC(),
	)
	if err := db.Write(ctx, s.pool, func(q qrm.DB) error { _, err := stmt.ExecContext(ctx, q); return err }); err != nil {
		return fmt.Errorf("auth.resets: Insert: %w", err)
	}
	return nil
}

func (s *resetPostgres) DeleteUnused(ctx context.Context, userID uuid.UUID) error {
	t := s.resets
	stmt := t.DELETE().WHERE(t.UserID.EQ(postgres.UUID(userID)).AND(t.UsedAt.IS_NULL()))
	if err := db.Write(ctx, s.pool, func(q qrm.DB) error { _, err := stmt.ExecContext(ctx, q); return err }); err != nil {
		return fmt.Errorf("auth.resets: DeleteUnused: %w", err)
	}
	return nil
}

// SECURITY: one conditional UPDATE, so two submits of one link cannot both use it.
func (s *resetPostgres) MarkUsed(ctx context.Context, publicID string, now time.Time) (*Reset, error) {
	t := s.resets
	at := postgres.TimestampzT(now.UTC())
	stmt := t.UPDATE().SET(t.UsedAt.SET(at)).
		WHERE(t.PublicID.EQ(postgres.String(publicID)).AND(t.UsedAt.IS_NULL()).AND(t.ExpiresAt.GT(at))).
		RETURNING(t.AllColumns)
	var row resetRow
	if err := db.Write(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) }); err != nil {
		if db.IsNoRows(err) {
			return nil, &ResetNotUsableError{Reason: ResetNotFound}
		}
		return nil, fmt.Errorf("auth.resets: MarkUsed: %w", err)
	}
	return row.toReset(), nil
}
