package resourceserver

import (
	"context"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/db"
)

func (s *postgresStore) serverColumns() postgres.ProjectionList {
	t := s.servers
	return postgres.ProjectionList{
		t.ID, t.PublicID, t.OrgID, t.ProjectID, t.Name, t.Identifier, db.ArrayAsJSON(t.Scopes, "resource_servers.scopes"),
		t.VerificationState, t.VerifiedAt, t.LastCheckedAt, postgres.CAST(t.LastError).AS_TEXT().AS("resource_servers.last_error"),
		t.ConsecutiveFailures, t.Trusted, t.TrustedAt, t.TrustedBy, t.CreatedAt, t.UpdatedAt,
	}
}

// List returns the project's servers, by name.
func (s *postgresStore) List(ctx context.Context, projectID uuid.UUID) ([]*ResourceServer, error) {
	t := s.servers
	stmt := postgres.SELECT(s.serverColumns()).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID))).
		ORDER_BY(t.Name.ASC(), t.CreatedAt.ASC())
	return s.query(ctx, "List", stmt)
}

// ByPublicID returns the project's server publicID, or *NotFoundError.
func (s *postgresStore) ByPublicID(ctx context.Context, projectID uuid.UUID, publicID string) (*ResourceServer, error) {
	return s.one(ctx, "ByPublicID", s.byPublicID(projectID, publicID))
}

// Due returns up to limit untrusted servers never checked or last checked before cutoff, oldest first.
func (s *postgresStore) Due(ctx context.Context, projectID uuid.UUID, cutoff time.Time, limit int) ([]*ResourceServer, error) {
	t := s.servers
	stmt := postgres.SELECT(s.serverColumns()).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.Trusted.IS_FALSE()).
			AND(t.LastCheckedAt.IS_NULL().OR(t.LastCheckedAt.LT(postgres.TimestampzT(cutoff.UTC()))))).
		ORDER_BY(t.LastCheckedAt.ASC().NULLS_FIRST(), t.CreatedAt.ASC()).
		LIMIT(int64(limit))
	return s.query(ctx, "Due", stmt)
}

// Links returns the project's links of clientID or of serverID (whichever is not uuid.Nil), with their servers.
func (s *postgresStore) Links(ctx context.Context, projectID, clientID, serverID uuid.UUID) ([]*Link, error) {
	t, l := s.servers, s.links
	where := l.ProjectID.EQ(postgres.UUID(projectID))
	if clientID != uuid.Nil {
		where = where.AND(l.ClientID.EQ(postgres.UUID(clientID)))
	}
	if serverID != uuid.Nil {
		where = where.AND(l.ResourceServerID.EQ(postgres.UUID(serverID)))
	}
	stmt := postgres.SELECT(
		s.serverColumns(), l.ClientID, db.ArrayAsJSON(l.Scopes, "oauth_client_resources.scopes"), l.CreatedAt, l.UpdatedAt,
	).FROM(l.INNER_JOIN(t, t.ID.EQ(l.ResourceServerID).AND(t.ProjectID.EQ(l.ProjectID)))).
		WHERE(where).
		ORDER_BY(t.Name.ASC(), l.CreatedAt.ASC())
	var rows []linkRow
	if err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &rows) }); err != nil && !db.IsNoRows(err) {
		return nil, fmt.Errorf("resourceserver.postgres: Links: %w", err)
	}
	out := make([]*Link, 0, len(rows))
	for i := range rows {
		link, err := rows[i].toLink()
		if err != nil {
			return nil, err
		}
		out = append(out, link)
	}
	return out, nil
}

func (s *postgresStore) byPublicID(projectID uuid.UUID, publicID string) postgres.SelectStatement {
	t := s.servers
	return postgres.SELECT(s.serverColumns()).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.PublicID.EQ(postgres.String(publicID))))
}

func (s *postgresStore) one(ctx context.Context, op string, stmt postgres.SelectStatement) (*ResourceServer, error) {
	var row serverRow
	if err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) }); err != nil {
		if db.IsNoRows(err) {
			return nil, &NotFoundError{}
		}
		return nil, fmt.Errorf("resourceserver.postgres: %s: %w", op, err)
	}
	return row.toServer()
}

func (s *postgresStore) query(ctx context.Context, op string, stmt postgres.SelectStatement) ([]*ResourceServer, error) {
	var rows []serverRow
	if err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &rows) }); err != nil && !db.IsNoRows(err) {
		return nil, fmt.Errorf("resourceserver.postgres: %s: %w", op, err)
	}
	out := make([]*ResourceServer, 0, len(rows))
	for i := range rows {
		r, err := rows[i].toServer()
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}
