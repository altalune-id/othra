package upstream

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"

	"github.com/altalune-id/othra/internal/kernel/db"
)

// List returns the project's upstreams by kind.
func (s *postgresStore) List(ctx context.Context, projectID uuid.UUID) ([]*Upstream, error) {
	t := s.upstreams
	stmt := postgres.SELECT(s.columns()).FROM(t).WHERE(t.ProjectID.EQ(postgres.UUID(projectID))).ORDER_BY(t.Kind.ASC())
	var rows []upstreamRow
	if err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &rows) }); err != nil && !db.IsNoRows(err) {
		return nil, wrap("List", err)
	}
	out := make([]*Upstream, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].toUpstream())
	}
	return out, nil
}

// ByPublicID returns the project's upstream publicID, or *NotFoundError.
func (s *postgresStore) ByPublicID(ctx context.Context, projectID uuid.UUID, publicID string) (*Upstream, error) {
	t := s.upstreams
	return s.one(ctx, "ByPublicID", postgres.SELECT(s.columns()).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.PublicID.EQ(postgres.String(publicID)))), db.ReadPrimary)
}

// ByKind returns the project's upstream of kind, or *NotFoundError.
func (s *postgresStore) ByKind(ctx context.Context, projectID uuid.UUID, kind Kind) (*Upstream, error) {
	t := s.upstreams
	return s.one(ctx, "ByKind", postgres.SELECT(s.columns()).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.Kind.EQ(postgres.String(string(kind))))), db.ReadPrimary)
}

// Secret returns the sealed client secret of the upstream id.
func (s *postgresStore) Secret(ctx context.Context, projectID, id uuid.UUID) ([]byte, error) {
	t := s.upstreams
	stmt := postgres.SELECT(t.ClientSecretSealed).FROM(t).WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.ID.EQ(postgres.UUID(id))))
	var row struct {
		Sealed []byte `alias:"upstream_providers.client_secret_sealed"`
	}
	err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) })
	switch {
	case db.IsNoRows(err):
		return nil, &NotFoundError{}
	case err != nil:
		return nil, wrap("Secret", err)
	}
	return row.Sealed, nil
}

func (s *postgresStore) one(
	ctx context.Context, op string, stmt postgres.SelectStatement, run func(context.Context, db.Pool, func(qrm.DB) error) error,
) (*Upstream, error) {
	var row upstreamRow
	if err := run(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) }); err != nil {
		if db.IsNoRows(err) {
			return nil, &NotFoundError{}
		}
		return nil, wrap(op, err)
	}
	return row.toUpstream(), nil
}
