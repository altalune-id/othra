package grant

import (
	"context"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/db"
)

// CodeByHash locks and returns the code with hash, or nil.
func (s *postgresStore) CodeByHash(ctx context.Context, projectID uuid.UUID, hash string) (*Code, error) {
	c, us, f := s.codes, s.sessions, s.families
	stmt := postgres.SELECT(
		c.ID, c.ClientID, c.EndUserID, c.UserSessionID, c.RedirectURI, db.ArrayAsJSON(c.Scopes, "authorization_codes.scopes"),
		c.CodeChallenge, c.Nonce, c.AuthTime, c.ExpiresAt, c.UsedAt, c.FamilyID, us.PublicID, f.PublicID,
	).
		FROM(c.LEFT_JOIN(us, us.ID.EQ(c.UserSessionID)).LEFT_JOIN(f, f.ID.EQ(c.FamilyID))).
		WHERE(c.ProjectID.EQ(postgres.UUID(projectID)).AND(c.CodeHash.EQ(postgres.String(hash)))).
		FOR(postgres.UPDATE().OF(c))
	var row codeRow
	err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) })
	switch {
	case db.IsNoRows(err):
		return nil, nil
	case err != nil:
		return nil, wrap("CodeByHash", err)
	}
	return row.toCode()
}

// FamilyBySID returns the family with the public id sid, or nil.
func (s *postgresStore) FamilyBySID(ctx context.Context, projectID uuid.UUID, sid string) (*Family, error) {
	t := s.families
	stmt := postgres.SELECT(s.familyColumns()).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.PublicID.EQ(postgres.String(sid))))
	var row familyRow
	err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) })
	switch {
	case db.IsNoRows(err):
		return nil, nil
	case err != nil:
		return nil, wrap("FamilyBySID", err)
	}
	return row.toFamily()
}

// AccessByHash returns the access token with hash and its family, or nil.
func (s *postgresStore) AccessByHash(ctx context.Context, projectID uuid.UUID, hash string) (*Access, error) {
	a, f := s.access, s.families
	stmt := postgres.SELECT(s.familyColumns(), db.ArrayAsJSON(a.Scopes, "access_tokens.scopes"), a.ExpiresAt).
		FROM(a.INNER_JOIN(f, f.ID.EQ(a.FamilyID))).
		WHERE(a.ProjectID.EQ(postgres.UUID(projectID)).AND(a.TokenHash.EQ(postgres.String(hash))))
	var row struct {
		Family    familyRow
		Scopes    string    `alias:"access_tokens.scopes"`
		ExpiresAt time.Time `alias:"access_tokens.expires_at"`
	}
	err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) })
	switch {
	case db.IsNoRows(err):
		return nil, nil
	case err != nil:
		return nil, wrap("AccessByHash", err)
	}
	fam, err := row.Family.toFamily()
	if err != nil {
		return nil, err
	}
	scopes, err := db.ParseJSONArray(row.Scopes)
	if err != nil {
		return nil, wrap("AccessByHash", err)
	}
	return &Access{Family: fam, Scopes: scopes, ExpiresAt: row.ExpiresAt.UTC()}, nil
}

// RefreshByHash locks and returns the refresh token with hash and its family, or nil.
func (s *postgresStore) RefreshByHash(ctx context.Context, projectID uuid.UUID, hash string) (*RefreshToken, *Family, error) {
	r, f := s.refresh, s.families
	stmt := postgres.SELECT(s.familyColumns(), r.ID, r.FamilyID, r.ExpiresAt, r.UsedAt).
		FROM(r.INNER_JOIN(f, f.ID.EQ(r.FamilyID))).
		WHERE(r.ProjectID.EQ(postgres.UUID(projectID)).AND(r.TokenHash.EQ(postgres.String(hash)))).
		FOR(postgres.UPDATE())
	var row struct {
		Family    familyRow
		TokenID   uuid.UUID  `alias:"refresh_tokens.id"`
		FamilyID  uuid.UUID  `alias:"refresh_tokens.family_id"`
		ExpiresAt time.Time  `alias:"refresh_tokens.expires_at"`
		UsedAt    *time.Time `alias:"refresh_tokens.used_at"`
	}
	err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) })
	switch {
	case db.IsNoRows(err):
		return nil, nil, nil
	case err != nil:
		return nil, nil, wrap("RefreshByHash", err)
	}
	fam, err := row.Family.toFamily()
	if err != nil {
		return nil, nil, err
	}
	return &RefreshToken{ID: row.TokenID, FamilyID: row.FamilyID, ExpiresAt: row.ExpiresAt.UTC(), UsedAt: utc(row.UsedAt)}, fam, nil
}
