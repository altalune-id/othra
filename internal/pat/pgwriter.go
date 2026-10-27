package pat

import (
	"context"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"

	"github.com/altalune-id/othra/internal/kernel/db"
)

// Insert records t with the Versioned hash of its secret.
func (s *postgresStore) Insert(ctx context.Context, tok *Token, hash string) error {
	t := s.tokens
	stmt := t.INSERT(
		t.ID, t.PublicID, t.OrgID, t.ProjectID, t.EndUserID, t.ResourceServerID, t.Name, t.TokenHash, t.Hint, t.Scopes, t.CreatedAt,
		t.ExpiresAt,
	).VALUES(
		tok.ID, tok.PublicID, tok.OrgID, tok.ProjectID, tok.EndUserID, tok.ResourceServerID, tok.Name, hash, tok.Hint,
		db.TextArray(tok.Scopes), tok.CreatedAt.UTC(), tok.ExpiresAt.UTC(),
	)
	if _, err := s.exec(ctx, stmt); err != nil {
		return wrap("Insert", err)
	}
	return nil
}

// Revoke sets revoked_at on the end user's live token publicID; ok is false when there is none.
func (s *postgresStore) Revoke(ctx context.Context, projectID, endUserID uuid.UUID, publicID string, now time.Time) (bool, error) {
	t := s.tokens
	stmt := t.UPDATE(t.RevokedAt).SET(postgres.TimestampzT(now.UTC())).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.EndUserID.EQ(postgres.UUID(endUserID))).
			AND(t.PublicID.EQ(postgres.String(publicID))).AND(t.RevokedAt.IS_NULL()))
	n, err := s.exec(ctx, stmt)
	if err != nil {
		return false, wrap("Revoke", err)
	}
	return n == 1, nil
}

// TouchLastUsed moves each of the scoped project's tokens' last_used_at forward to its time; it never moves it back.
func (s *postgresStore) TouchLastUsed(ctx context.Context, used map[uuid.UUID]time.Time) error {
	t := s.tokens
	for id, at := range used {
		ts := postgres.TimestampzT(at.UTC())
		stmt := t.UPDATE(t.LastUsedAt).SET(ts).
			WHERE(t.ID.EQ(postgres.UUID(id)).AND(t.LastUsedAt.IS_NULL().OR(t.LastUsedAt.LT(ts))))
		if _, err := s.exec(ctx, stmt); err != nil {
			return wrap("TouchLastUsed", err)
		}
	}
	return nil
}

// Prune deletes the project's tokens that expired or were revoked before cutoff.
func (s *postgresStore) Prune(ctx context.Context, projectID uuid.UUID, cutoff time.Time) (int, error) {
	t := s.tokens
	at := postgres.TimestampzT(cutoff.UTC())
	stmt := t.DELETE().WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.ExpiresAt.LT(at).OR(t.RevokedAt.LT(at))))
	n, err := s.exec(ctx, stmt)
	if err != nil {
		return 0, wrap("Prune", err)
	}
	return int(n), nil
}

func (s *postgresStore) exec(ctx context.Context, stmt postgres.Statement) (int64, error) {
	var n int64
	err := db.Write(ctx, s.pool, func(q qrm.DB) error {
		res, err := stmt.ExecContext(ctx, q)
		if err != nil {
			return err
		}
		n, err = res.RowsAffected()
		return err
	})
	return n, err
}
