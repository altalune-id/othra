package keys

import (
	"context"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"

	"github.com/altalune-id/othra/internal/kernel/db"
)

func (s *postgresStore) publicColumns() postgres.ProjectionList {
	t := s.keys
	return postgres.ProjectionList{
		t.ID, t.PublicID, t.OrgID, t.ProjectID, t.Kid, t.Alg, t.PublicJWK, t.Status,
		t.CreatedAt, t.ActivatesAt, t.ActivatedAt, t.RetireAfter, t.RetiredAt,
	}
}

// List returns every key of the project, newest first.
func (s *postgresStore) List(ctx context.Context, projectID uuid.UUID) ([]*Key, error) {
	t := s.keys
	stmt := postgres.SELECT(s.publicColumns()).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID))).
		ORDER_BY(t.CreatedAt.DESC(), t.Kid.ASC())
	return s.query(ctx, "List", stmt)
}

// Published returns the next, active and retiring keys, oldest first, then by kid.
func (s *postgresStore) Published(ctx context.Context, projectID uuid.UUID) ([]*Key, error) {
	t := s.keys
	stmt := postgres.SELECT(s.publicColumns()).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).
			AND(t.Status.IN(postgres.String(string(StatusNext)), postgres.String(string(StatusActive)), postgres.String(string(StatusRetiring))))).
		ORDER_BY(t.CreatedAt.ASC(), t.Kid.ASC())
	return s.query(ctx, "Published", stmt)
}

// Active returns the active key and its sealed private key, or *NoActiveKeyError.
func (s *postgresStore) Active(ctx context.Context, projectID uuid.UUID) (*Key, []byte, error) {
	t := s.keys
	stmt := postgres.SELECT(s.publicColumns(), t.PrivateKeyEnc).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.Status.EQ(postgres.String(string(StatusActive))))).
		LIMIT(1)
	var row keyRow
	if err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) }); err != nil {
		if db.IsNoRows(err) {
			return nil, nil, &NoActiveKeyError{ProjectID: projectID.String()}
		}
		return nil, nil, fmt.Errorf("keys.postgres: Active: %w", err)
	}
	k, err := row.toKey()
	if err != nil {
		return nil, nil, err
	}
	return k, row.Sealed, nil
}

// Retiring returns the retiring keys whose retire_after is at or before now.
func (s *postgresStore) Retiring(ctx context.Context, projectID uuid.UUID, now time.Time) ([]*Key, error) {
	t := s.keys
	stmt := postgres.SELECT(s.publicColumns()).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).
			AND(t.Status.EQ(postgres.String(string(StatusRetiring)))).
			AND(t.RetireAfter.LT_EQ(postgres.TimestampzT(now.UTC())))).
		ORDER_BY(t.CreatedAt.ASC())
	return s.query(ctx, "Retiring", stmt)
}

func (s *postgresStore) query(ctx context.Context, op string, stmt postgres.SelectStatement) ([]*Key, error) {
	var rows []keyRow
	if err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &rows) }); err != nil && !db.IsNoRows(err) {
		return nil, fmt.Errorf("keys.postgres: %s: %w", op, err)
	}
	return toKeys(rows)
}
