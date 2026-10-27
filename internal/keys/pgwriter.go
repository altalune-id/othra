package keys

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"

	"github.com/altalune-id/othra/internal/kernel/db"
)

// Insert records k with its sealed private key.
func (s *postgresStore) Insert(ctx context.Context, k *Key, sealed []byte) error {
	jwk, err := json.Marshal(k.JWK)
	if err != nil {
		return fmt.Errorf("keys.postgres: Insert: %w", err)
	}
	t := s.keys
	stmt := t.INSERT(t.AllColumns).VALUES(
		k.ID, k.PublicID, k.OrgID, k.ProjectID, k.KID, k.Alg,
		postgres.CAST(postgres.String(string(jwk))).AS("jsonb"), sealed, string(k.Status), k.CreatedAt.UTC(),
		timeExpr(k.ActivatesAt), timeExpr(k.ActivatedAt), timeExpr(k.RetireAfter), timeExpr(k.RetiredAt),
	)
	if err := db.Write(ctx, s.pool, func(q qrm.DB) error { _, err := stmt.ExecContext(ctx, q); return err }); err != nil {
		return fmt.Errorf("keys.postgres: Insert: %w", err)
	}
	return nil
}

// Update saves k's status and timestamps; a retired key loses its sealed private key.
func (s *postgresStore) Update(ctx context.Context, k *Key) error {
	t := s.keys
	stmt := t.UPDATE(t.Status, t.ActivatesAt, t.ActivatedAt, t.RetireAfter, t.RetiredAt).
		SET(postgres.String(string(k.Status)), timeExpr(k.ActivatesAt), timeExpr(k.ActivatedAt), timeExpr(k.RetireAfter), timeExpr(k.RetiredAt)).
		WHERE(t.ID.EQ(postgres.UUID(k.ID)).AND(t.ProjectID.EQ(postgres.UUID(k.ProjectID))))
	if k.Status == StatusRetired {
		// SECURITY: a key that never signs again keeps no private key.
		stmt = t.UPDATE(t.Status, t.ActivatesAt, t.ActivatedAt, t.RetireAfter, t.RetiredAt, t.PrivateKeyEnc).
			SET(postgres.String(string(k.Status)), timeExpr(k.ActivatesAt), timeExpr(k.ActivatedAt), timeExpr(k.RetireAfter),
				timeExpr(k.RetiredAt), postgres.NULL).
			WHERE(t.ID.EQ(postgres.UUID(k.ID)).AND(t.ProjectID.EQ(postgres.UUID(k.ProjectID))))
	}
	if err := db.Write(ctx, s.pool, func(q qrm.DB) error { _, err := stmt.ExecContext(ctx, q); return err }); err != nil {
		return fmt.Errorf("keys.postgres: Update: %w", err)
	}
	return nil
}

// Live locks and returns the project's next and active keys, either nil when absent.
func (s *postgresStore) Live(ctx context.Context, projectID uuid.UUID) (next, active *Key, err error) {
	t := s.keys
	// SECURITY/correctness: the row locks serialize concurrent rotations and promotions of one project.
	stmt := postgres.SELECT(s.publicColumns()).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).
			AND(t.Status.IN(postgres.String(string(StatusNext)), postgres.String(string(StatusActive))))).
		FOR(postgres.UPDATE())
	var rows []keyRow
	if err := db.Write(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &rows) }); err != nil && !db.IsNoRows(err) {
		return nil, nil, fmt.Errorf("keys.postgres: Live: %w", err)
	}
	keys, err := toKeys(rows)
	if err != nil {
		return nil, nil, err
	}
	for _, k := range keys {
		switch k.Status {
		case StatusNext:
			next = k
		case StatusActive:
			active = k
		}
	}
	return next, active, nil
}

// DeleteRetired deletes the retired keys retired before cutoff, with their sealed private keys.
func (s *postgresStore) DeleteRetired(ctx context.Context, projectID uuid.UUID, cutoff time.Time) (int64, error) {
	t := s.keys
	stmt := t.DELETE().WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).
		AND(t.Status.EQ(postgres.String(string(StatusRetired)))).
		AND(t.RetiredAt.LT(postgres.TimestampzT(cutoff.UTC()))))
	var n int64
	err := db.Write(ctx, s.pool, func(q qrm.DB) error {
		res, err := stmt.ExecContext(ctx, q)
		if err != nil {
			return err
		}
		n, err = res.RowsAffected()
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("keys.postgres: DeleteRetired: %w", err)
	}
	return n, nil
}

func timeExpr(t *time.Time) postgres.Expression {
	if t == nil {
		return postgres.NULL
	}
	return postgres.TimestampzT(t.UTC())
}
