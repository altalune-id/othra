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
		db.ArrayAsJSON(t.GrantTypes, "oauth_clients.grant_types"),
		db.ArrayAsJSON(t.AllowedScopes, "oauth_clients.allowed_scopes"),
		db.ArrayAsJSON(t.RedirectURIs, "oauth_clients.redirect_uris"),
		db.ArrayAsJSON(t.PostLogoutRedirectURIs, "oauth_clients.post_logout_redirect_uris"),
		t.ConsentPolicy, t.PKCERequired, t.CreatedAt, t.UpdatedAt, t.MetadataURL, t.ClientURI, t.MetadataFetchedAt, t.MetadataExpiresAt,
		t.RegistrationKind,
	}
}

// List returns the project's clients, oldest first, each with its live secrets at now, the current one first.
func (s *postgresStore) List(ctx context.Context, projectID uuid.UUID, now time.Time) ([]*Client, error) {
	t := s.clients
	stmt := postgres.SELECT(s.clientColumns()).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.MetadataURL.IS_NULL())).
		ORDER_BY(t.CreatedAt.ASC(), t.PublicID.ASC())
	return s.read(ctx, "List", stmt, projectID, now, false)
}

// ByIDs returns the project's clients with the given ids, shadow clients included, without secrets.
func (s *postgresStore) ByIDs(ctx context.Context, projectID uuid.UUID, ids []uuid.UUID) ([]*Client, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	t := s.clients
	in := make([]postgres.Expression, 0, len(ids))
	for _, id := range ids {
		in = append(in, postgres.UUID(id))
	}
	stmt := postgres.SELECT(s.clientColumns()).FROM(t).WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.ID.IN(in...)))
	var rows []clientRow
	if err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &rows) }); err != nil && !db.IsNoRows(err) {
		return nil, fmt.Errorf("client.postgres: ByIDs: %w", err)
	}
	out := make([]*Client, 0, len(rows))
	for i := range rows {
		c, err := rows[i].toClient()
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// ByPublicID returns one client with its live secrets at now, the current one first, or *NotFoundError.
func (s *postgresStore) ByPublicID(ctx context.Context, projectID uuid.UUID, publicID string, now time.Time) (*Client, error) {
	return s.one(ctx, "ByPublicID", projectID, publicID, now, readPlain)
}

// Lock locks a client's row and returns it with its live secrets at now, or *NotFoundError.
func (s *postgresStore) Lock(ctx context.Context, projectID uuid.UUID, publicID string, now time.Time) (*Client, error) {
	return s.one(ctx, "Lock", projectID, publicID, now, readLocked)
}

// ForAuthentication returns one client with its live secrets at now and their hashes, for VerifySecret only, or *NotFoundError.
func (s *postgresStore) ForAuthentication(ctx context.Context, projectID uuid.UUID, publicID string, now time.Time) (*Client, error) {
	return s.one(ctx, "ForAuthentication", projectID, publicID, now, readHashes)
}

// LockSystem locks the project's console client row and returns it, or *NotFoundError when the project has none.
func (s *postgresStore) LockSystem(ctx context.Context, projectID uuid.UUID, now time.Time) (*Client, error) {
	t := s.clients
	stmt := postgres.SELECT(s.clientColumns()).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.RegistrationKind.EQ(postgres.String(kindSystem)))).
		FOR(postgres.UPDATE())
	out, err := s.read(ctx, "LockSystem", stmt, projectID, now, false)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, &NotFoundError{}
	}
	return out[0], nil
}

type readMode int

const (
	readPlain readMode = iota
	readLocked
	readHashes
)

func (s *postgresStore) one(
	ctx context.Context, op string, projectID uuid.UUID, publicID string, now time.Time, mode readMode,
) (*Client, error) {
	t := s.clients
	by := t.PublicID
	if IsMetadataURL(publicID) {
		by = t.MetadataURL
	}
	stmt := postgres.SELECT(s.clientColumns()).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(by.EQ(postgres.String(publicID))))
	if mode == readLocked {
		stmt = stmt.FOR(postgres.UPDATE())
	}
	out, err := s.read(ctx, op, stmt, projectID, now, mode == readHashes)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, &NotFoundError{}
	}
	return out[0], nil
}

func (s *postgresStore) read(
	ctx context.Context, op string, stmt postgres.SelectStatement, projectID uuid.UUID, now time.Time, hashes bool,
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
	cols := postgres.ProjectionList{sec.ID, sec.PublicID, sec.ClientID, sec.Hint, sec.CreatedAt, sec.ExpiresAt}
	if hashes {
		cols = append(cols, sec.SecretHash)
	}
	secStmt := postgres.SELECT(cols).FROM(sec).
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
