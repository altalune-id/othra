package resourceserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/db"
)

// Insert records r, or returns *IdentifierTakenError.
func (s *postgresStore) Insert(ctx context.Context, r *ResourceServer) error {
	lastError, err := failureExpr(r.LastError)
	if err != nil {
		return fmt.Errorf("resourceserver.postgres: Insert: %w", err)
	}
	descriptions, err := descriptionsExpr(r.Descriptions)
	if err != nil {
		return fmt.Errorf("resourceserver.postgres: Insert: %w", err)
	}
	t := s.servers
	stmt := t.INSERT(
		t.ID, t.PublicID, t.OrgID, t.ProjectID, t.Name, t.Identifier, t.Scopes, t.VerificationState, t.VerifiedAt, t.LastCheckedAt,
		t.LastError, t.ConsecutiveFailures, t.Trusted, t.TrustedAt, t.TrustedBy, t.CreatedAt, t.UpdatedAt,
		t.AcceptCIMDClients, t.CIMDClientHosts, t.CIMDScopes, t.ScopeDescriptions,
	).VALUES(
		r.ID, r.PublicID, r.OrgID, r.ProjectID, r.Name, r.Identifier, db.TextArray(r.Scopes), string(r.State),
		timeExpr(r.VerifiedAt), timeExpr(r.LastCheckedAt), lastError, r.ConsecutiveFailures, r.Trusted,
		timeExpr(r.TrustedAt), nullableUUID(r.TrustedBy), r.CreatedAt.UTC(), r.UpdatedAt.UTC(),
		r.Admission.Accept, db.TextArray(hostsOf(r.Admission)), nullArray(r.Admission.Scopes), descriptions,
	)
	err = db.Write(ctx, s.pool, func(q qrm.DB) error { _, err := stmt.ExecContext(ctx, q); return err })
	if db.IsUniqueViolation(err) {
		if name, ok := db.ViolatedConstraint(err); ok && name == s.identifierKey {
			return &IdentifierTakenError{Identifier: r.Identifier}
		}
	}
	if err != nil {
		return fmt.Errorf("resourceserver.postgres: Insert: %w", err)
	}
	return nil
}

// Update saves r's name, scopes and their descriptions, verification, trust and admission.
func (s *postgresStore) Update(ctx context.Context, r *ResourceServer) error {
	lastError, err := failureExpr(r.LastError)
	if err != nil {
		return fmt.Errorf("resourceserver.postgres: Update: %w", err)
	}
	descriptions, err := descriptionsExpr(r.Descriptions)
	if err != nil {
		return fmt.Errorf("resourceserver.postgres: Update: %w", err)
	}
	t := s.servers
	stmt := t.UPDATE().SET(
		t.Name.SET(postgres.String(r.Name)),
		t.Scopes.SET(db.TextArray(r.Scopes)),
		t.VerificationState.SET(postgres.String(string(r.State))),
		t.VerifiedAt.SET(postgres.TimestampzExp(timeExpr(r.VerifiedAt))),
		t.LastCheckedAt.SET(postgres.TimestampzExp(timeExpr(r.LastCheckedAt))),
		t.LastError.SET(postgres.StringExp(lastError)),
		t.ConsecutiveFailures.SET(postgres.Int(int64(r.ConsecutiveFailures))),
		t.Trusted.SET(postgres.Bool(r.Trusted)),
		t.TrustedAt.SET(postgres.TimestampzExp(timeExpr(r.TrustedAt))),
		t.TrustedBy.SET(postgres.StringExp(uuidExpr(r.TrustedBy))),
		t.AcceptCIMDClients.SET(postgres.Bool(r.Admission.Accept)),
		t.CIMDClientHosts.SET(db.TextArray(hostsOf(r.Admission))),
		t.CIMDScopes.SET(postgres.StringExp(nullArray(r.Admission.Scopes))),
		t.ScopeDescriptions.SET(descriptions),
		t.UpdatedAt.SET(postgres.TimestampzT(r.UpdatedAt.UTC())),
	).WHERE(t.ID.EQ(postgres.UUID(r.ID)).AND(t.ProjectID.EQ(postgres.UUID(r.ProjectID))))
	if err := db.Write(ctx, s.pool, func(q qrm.DB) error { _, err := stmt.ExecContext(ctx, q); return err }); err != nil {
		return fmt.Errorf("resourceserver.postgres: Update: %w", err)
	}
	return nil
}

// Delete removes the server id.
func (s *postgresStore) Delete(ctx context.Context, projectID, id uuid.UUID) error {
	t := s.servers
	stmt := t.DELETE().WHERE(t.ID.EQ(postgres.UUID(id)).AND(t.ProjectID.EQ(postgres.UUID(projectID))))
	if err := db.Write(ctx, s.pool, func(q qrm.DB) error { _, err := stmt.ExecContext(ctx, q); return err }); err != nil {
		return fmt.Errorf("resourceserver.postgres: Delete: %w", err)
	}
	return nil
}

// Lock is ByPublicID that also locks the row until the unit of work ends.
func (s *postgresStore) Lock(ctx context.Context, projectID uuid.UUID, publicID string) (*ResourceServer, error) {
	stmt := s.byPublicID(projectID, publicID).FOR(postgres.UPDATE())
	var row serverRow
	if err := db.Write(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) }); err != nil {
		if db.IsNoRows(err) {
			return nil, &NotFoundError{}
		}
		return nil, fmt.Errorf("resourceserver.postgres: Lock: %w", err)
	}
	return row.toServer()
}

// UpsertLink records l, replacing the scopes of an existing link; an unknown client returns *ClientNotFoundError.
func (s *postgresStore) UpsertLink(ctx context.Context, l *Link) error {
	t := s.links
	stmt := t.INSERT(t.AllColumns).VALUES(
		l.OrgID, l.ProjectID, l.ClientID, l.ResourceServer.ID, db.TextArray(l.Scopes), l.CreatedAt.UTC(), l.UpdatedAt.UTC(),
	).ON_CONFLICT(t.ClientID, t.ResourceServerID).
		// SECURITY: the conflict clause is guarded by project, as every write is.
		DO_UPDATE(postgres.SET(
			t.Scopes.SET(db.TextArray(l.Scopes)),
			t.UpdatedAt.SET(postgres.TimestampzT(l.UpdatedAt.UTC())),
		).WHERE(t.ProjectID.EQ(postgres.UUID(l.ProjectID))))
	err := db.Write(ctx, s.pool, func(q qrm.DB) error { _, err := stmt.ExecContext(ctx, q); return err })
	if db.IsForeignKeyViolation(err) {
		if name, ok := db.ViolatedConstraint(err); ok && name == s.clientFKey {
			return &ClientNotFoundError{}
		}
	}
	if err != nil {
		return fmt.Errorf("resourceserver.postgres: UpsertLink: %w", err)
	}
	return nil
}

// UpdateLink replaces the scopes of l and reports whether the link exists.
func (s *postgresStore) UpdateLink(ctx context.Context, l *Link) (bool, error) {
	t := s.links
	stmt := t.UPDATE().SET(
		t.Scopes.SET(db.TextArray(l.Scopes)),
		t.UpdatedAt.SET(postgres.TimestampzT(l.UpdatedAt.UTC())),
	).WHERE(t.ProjectID.EQ(postgres.UUID(l.ProjectID)).
		AND(t.ClientID.EQ(postgres.UUID(l.ClientID))).
		AND(t.ResourceServerID.EQ(postgres.UUID(l.ResourceServer.ID))))
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
		return false, fmt.Errorf("resourceserver.postgres: UpdateLink: %w", err)
	}
	return n > 0, nil
}

// DeleteLink removes the link between clientID and serverID, if any.
func (s *postgresStore) DeleteLink(ctx context.Context, projectID, clientID, serverID uuid.UUID) error {
	t := s.links
	stmt := t.DELETE().WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).
		AND(t.ClientID.EQ(postgres.UUID(clientID))).
		AND(t.ResourceServerID.EQ(postgres.UUID(serverID))))
	if err := db.Write(ctx, s.pool, func(q qrm.DB) error { _, err := stmt.ExecContext(ctx, q); return err }); err != nil {
		return fmt.Errorf("resourceserver.postgres: DeleteLink: %w", err)
	}
	return nil
}

func failureExpr(f *Failure) (postgres.Expression, error) {
	if f == nil {
		return postgres.NULL, nil
	}
	body, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	return postgres.CAST(postgres.String(string(body))).AS("jsonb"), nil
}

func nullableUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}

func uuidExpr(id uuid.UUID) postgres.Expression {
	if id == uuid.Nil {
		return postgres.NULL
	}
	return postgres.UUID(id)
}

func hostsOf(a Admission) []string {
	if a.Hosts == nil {
		return []string{}
	}
	return a.Hosts
}
