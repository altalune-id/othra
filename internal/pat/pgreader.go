package pat

import (
	"context"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"

	"github.com/altalune-id/othra/internal/kernel/db"
)

// ByHash returns the project's token with that hash, in any state, or nil. SECURITY: it reads the primary, so a revoke counts at once.
func (s *postgresStore) ByHash(ctx context.Context, projectID uuid.UUID, hash string) (*Token, error) {
	t := s.tokens
	stmt := postgres.SELECT(s.columns()).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.TokenHash.EQ(postgres.String(hash))))
	var row tokenRow
	err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) })
	switch {
	case db.IsNoRows(err):
		return nil, nil
	case err != nil:
		return nil, wrap("ByHash", err)
	}
	return row.toToken()
}

// ListByEndUser returns the end user's tokens, newest first.
func (s *postgresStore) ListByEndUser(ctx context.Context, projectID, endUserID uuid.UUID) ([]*Token, error) {
	t := s.tokens
	stmt := postgres.SELECT(s.columns()).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.EndUserID.EQ(postgres.UUID(endUserID)))).
		ORDER_BY(t.CreatedAt.DESC(), t.PublicID.ASC())
	var rows []tokenRow
	if err := db.Read(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &rows) }); err != nil && !db.IsNoRows(err) {
		return nil, wrap("ListByEndUser", err)
	}
	out := make([]*Token, 0, len(rows))
	for i := range rows {
		tok, err := rows[i].toToken()
		if err != nil {
			return nil, err
		}
		out = append(out, tok)
	}
	return out, nil
}

// CountLive counts the end user's tokens neither revoked nor expired at now; it reads the primary, so a cap check sees its own writes.
func (s *postgresStore) CountLive(ctx context.Context, projectID, endUserID uuid.UUID, now time.Time) (int, error) {
	t := s.tokens
	stmt := postgres.SELECT(postgres.COUNT(postgres.STAR).AS("count")).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.EndUserID.EQ(postgres.UUID(endUserID))).
			AND(t.RevokedAt.IS_NULL()).AND(t.ExpiresAt.GT(postgres.TimestampzT(now.UTC()))))
	var row struct {
		Count int `alias:"count"`
	}
	if err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) }); err != nil {
		return 0, wrap("CountLive", err)
	}
	return row.Count, nil
}

// LockEndUser locks the end user's row until the unit of work ends; ok is false when it is gone.
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
