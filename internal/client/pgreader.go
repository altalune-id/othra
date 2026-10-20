package client

import (
	"context"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/db"
)

func (s *postgresStore) clientColumns() postgres.ProjectionList {
	t := s.clients
	return postgres.ProjectionList{
		t.ID, t.PublicID, t.OrgID, t.ProjectID, t.Name, t.ClientType, t.ApplicationType, t.TokenEndpointAuthMethod,
		jsonArray(t.GrantTypes, "oauth_clients.grant_types"),
		jsonArray(t.AllowedScopes, "oauth_clients.allowed_scopes"),
		jsonArray(t.RedirectURIs, "oauth_clients.redirect_uris"),
		jsonArray(t.PostLogoutRedirectURIs, "oauth_clients.post_logout_redirect_uris"),
		t.ConsentPolicy, t.CreatedAt, t.UpdatedAt,
	}
}

// List returns the project's clients, oldest first, each with its live secrets at now, the current one first.
func (s *postgresStore) List(ctx context.Context, projectID uuid.UUID, now time.Time) ([]*Client, error) {
	t := s.clients
	stmt := postgres.SELECT(s.clientColumns()).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID))).
		ORDER_BY(t.CreatedAt.ASC(), t.PublicID.ASC())
	return s.read(ctx, "List", stmt, projectID, now)
}

// ByPublicID returns one client with its live secrets at now, the current one first, or *NotFoundError.
func (s *postgresStore) ByPublicID(ctx context.Context, projectID uuid.UUID, publicID string, now time.Time) (*Client, error) {
	return s.one(ctx, "ByPublicID", projectID, publicID, now, false)
}

// Lock locks a client's row and returns it with its live secrets at now, or *NotFoundError.
func (s *postgresStore) Lock(ctx context.Context, projectID uuid.UUID, publicID string, now time.Time) (*Client, error) {
	return s.one(ctx, "Lock", projectID, publicID, now, true)
}

func (s *postgresStore) one(
	ctx context.Context, op string, projectID uuid.UUID, publicID string, now time.Time, lock bool,
) (*Client, error) {
	t := s.clients
	stmt := postgres.SELECT(s.clientColumns()).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.PublicID.EQ(postgres.String(publicID))))
	if lock {
		stmt = stmt.FOR(postgres.UPDATE())
	}
	out, err := s.read(ctx, op, stmt, projectID, now)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, &NotFoundError{}
	}
	return out[0], nil
}

func (s *postgresStore) read(
	ctx context.Context, op string, stmt postgres.SelectStatement, projectID uuid.UUID, now time.Time,
) ([]*Client, error) {
	var rows []clientRow
	if err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &rows) }); err != nil && !db.IsNoRows(err) {
		return nil, fmt.Errorf("client.postgres: %s: %w", op, err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	out := make([]*Client, 0, len(rows))
	byID := make(map[uuid.UUID]*Client, len(rows))
	ids := make([]postgres.Expression, 0, len(rows))
	for i := range rows {
		c, err := rows[i].toClient()
		if err != nil {
			return nil, err
		}
		out = append(out, c)
		byID[c.ID] = c
		ids = append(ids, postgres.UUID(c.ID))
	}
	sec := s.secrets
	secStmt := postgres.SELECT(sec.ID, sec.PublicID, sec.ClientID, sec.Hint, sec.CreatedAt, sec.ExpiresAt).FROM(sec).
		WHERE(sec.ProjectID.EQ(postgres.UUID(projectID)).
			AND(sec.ClientID.IN(ids...)).
			AND(sec.ExpiresAt.IS_NULL().OR(sec.ExpiresAt.GT(postgres.TimestampzT(now.UTC()))))).
		ORDER_BY(sec.ExpiresAt.IS_NULL().DESC(), sec.CreatedAt.DESC(), sec.PublicID.ASC())
	var secRows []secretRow
	err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return secStmt.QueryContext(ctx, q, &secRows) })
	if err != nil && !db.IsNoRows(err) {
		return nil, fmt.Errorf("client.postgres: %s secrets: %w", op, err)
	}
	for i := range secRows {
		if c := byID[secRows[i].ClientID]; c != nil {
			c.Secrets = append(c.Secrets, secRows[i].toSecret())
		}
	}
	return out, nil
}
