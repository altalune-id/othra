package consent

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"

	"github.com/altalune-id/othra/internal/kernel/db"
)

// NOTE: one statement adds the new scopes to the stored ones, so two approvals at once never narrow or fail.
// NOTE: stored scopes outside keep go first, so a server's catalog churn can't grow a consent past its 72 scopes.
func (s *postgresStore) unionScopes(keep []string) postgres.Expression {
	stored := s.table + ".scopes"
	added := " || ARRAY(SELECT s FROM unnest(excluded.scopes) AS s WHERE s <> ALL (" + stored + "))"
	if keep == nil {
		return postgres.Raw(stored + added)
	}
	return postgres.Raw("ARRAY(SELECT s FROM unnest("+stored+") AS s WHERE s = ANY (CAST(#keep AS text[])))"+added,
		postgres.RawArgs{"#keep": db.TextArrayLiteral(keep)})
}

// Upsert inserts c, or adds its scopes to the stored consent of c.Key after dropping those outside keep, and stamps UpdatedAt.
func (s *postgresStore) Upsert(ctx context.Context, c *Consent, keep []string) (*Consent, error) {
	t := s.consents
	at := postgres.TimestampzT(c.UpdatedAt.UTC())
	stmt := t.INSERT(t.ID, t.PublicID, t.OrgID, t.ProjectID, t.EndUserID, t.ClientID, t.ResourceServerID, t.Scopes,
		t.GrantedAt, t.UpdatedAt, t.GrantedIP, t.GrantedUserAgent).
		VALUES(c.ID, c.PublicID, c.OrgID, c.ProjectID, c.Key.EndUserID, c.Key.ClientID, nullUUID(c.Key.ResourceServerID),
			db.TextArray(c.Scopes), postgres.TimestampzT(c.GrantedAt.UTC()), at, inet(c.IP), nullString(c.UserAgent)).
		ON_CONFLICT().ON_CONSTRAINT(s.grantKey).DO_UPDATE(postgres.SET(
		t.Scopes.SET(postgres.StringExp(s.unionScopes(keep))),
		t.UpdatedAt.SET(at),
	)).
		RETURNING(s.columns())
	var row consentRow
	if err := db.Write(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) }); err != nil {
		return nil, wrap("Upsert", err)
	}
	return row.toConsent()
}

// DeleteClient removes every consent of clientID.
func (s *postgresStore) DeleteClient(ctx context.Context, projectID, clientID uuid.UUID) error {
	t := s.consents
	stmt := t.DELETE().WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.ClientID.EQ(postgres.UUID(clientID))))
	if err := db.Write(ctx, s.pool, func(q qrm.DB) error { _, err := stmt.ExecContext(ctx, q); return err }); err != nil {
		return wrap("DeleteClient", err)
	}
	return nil
}

// Delete removes the consent key names, reporting whether there was one.
func (s *postgresStore) DeleteEndUserClient(ctx context.Context, projectID, endUserID, clientID uuid.UUID) (int, error) {
	t := s.consents
	stmt := t.DELETE().WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.EndUserID.EQ(postgres.UUID(endUserID))).
		AND(t.ClientID.EQ(postgres.UUID(clientID))))
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
		return 0, wrap("DeleteEndUserClient", err)
	}
	return int(n), nil
}
