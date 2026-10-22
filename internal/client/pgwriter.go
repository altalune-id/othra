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

// Insert records a new client.
func (s *postgresStore) Insert(ctx context.Context, c *Client) error {
	t := s.clients
	stmt := t.INSERT(t.AllColumns).VALUES(
		c.ID, c.PublicID, c.OrgID, c.ProjectID, c.Name, c.ClientType, c.ApplicationType, c.TokenEndpointAuthMethod,
		db.TextArray(c.GrantTypes), db.TextArray(c.AllowedScopes), db.TextArray(c.RedirectURIs), db.TextArray(c.PostLogoutRedirectURIs),
		string(c.ConsentPolicy), c.PKCERequired, c.CreatedAt.UTC(), c.UpdatedAt.UTC(),
	)
	return s.write(ctx, "Insert", stmt)
}

// Update saves a client's settings and updated_at.
func (s *postgresStore) Update(ctx context.Context, c *Client) error {
	t := s.clients
	stmt := t.UPDATE(t.Name, t.GrantTypes, t.AllowedScopes, t.RedirectURIs, t.PostLogoutRedirectURIs, t.ConsentPolicy,
		t.TokenEndpointAuthMethod, t.PKCERequired, t.UpdatedAt).
		SET(postgres.String(c.Name), db.TextArray(c.GrantTypes), db.TextArray(c.AllowedScopes), db.TextArray(c.RedirectURIs),
			db.TextArray(c.PostLogoutRedirectURIs), postgres.String(string(c.ConsentPolicy)), postgres.String(c.TokenEndpointAuthMethod),
			postgres.Bool(c.PKCERequired), postgres.TimestampzT(c.UpdatedAt.UTC())).
		WHERE(t.ID.EQ(postgres.UUID(c.ID)).AND(t.ProjectID.EQ(postgres.UUID(c.ProjectID))))
	return s.write(ctx, "Update", stmt)
}

// Delete removes a client with its secrets and links, or returns *NotFoundError.
func (s *postgresStore) Delete(ctx context.Context, projectID, id uuid.UUID) error {
	t := s.clients
	stmt := t.DELETE().WHERE(t.ID.EQ(postgres.UUID(id)).AND(t.ProjectID.EQ(postgres.UUID(projectID))))
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
		return fmt.Errorf("client.postgres: Delete: %w", err)
	}
	if n == 0 {
		return &NotFoundError{}
	}
	return nil
}

// InsertSecret records secret sec of client c by its hash.
func (s *postgresStore) InsertSecret(ctx context.Context, c *Client, sec Secret, hash string) error {
	t := s.secrets
	stmt := t.INSERT(t.AllColumns).VALUES(sec.ID, sec.PublicID, c.OrgID, c.ProjectID, c.ID, hash, sec.Hint, sec.CreatedAt.UTC(), postgres.NULL)
	return s.write(ctx, "InsertSecret", stmt)
}

// ExpireSecret makes the secret id of client clientID stop working at at.
func (s *postgresStore) ExpireSecret(ctx context.Context, projectID, clientID, id uuid.UUID, at time.Time) error {
	t := s.secrets
	stmt := t.UPDATE(t.ExpiresAt).SET(postgres.TimestampzT(at.UTC())).
		WHERE(t.ID.EQ(postgres.UUID(id)).AND(t.ClientID.EQ(postgres.UUID(clientID))).AND(t.ProjectID.EQ(postgres.UUID(projectID))))
	return s.write(ctx, "ExpireSecret", stmt)
}

// DeleteSecrets removes the client's secrets with the given ids and every secret that expired at or before now.
func (s *postgresStore) DeleteSecrets(ctx context.Context, projectID, clientID uuid.UUID, ids []uuid.UUID, now time.Time) error {
	t := s.secrets
	dead := t.ExpiresAt.LT_EQ(postgres.TimestampzT(now.UTC()))
	if len(ids) > 0 {
		in := make([]postgres.Expression, 0, len(ids))
		for _, id := range ids {
			in = append(in, postgres.UUID(id))
		}
		dead = dead.OR(t.ID.IN(in...))
	}
	owner := t.ClientID.EQ(postgres.UUID(clientID)).AND(t.ProjectID.EQ(postgres.UUID(projectID)))
	return s.write(ctx, "DeleteSecrets", t.DELETE().WHERE(owner.AND(dead)))
}

func (s *postgresStore) write(ctx context.Context, op string, stmt postgres.Statement) error {
	if err := db.Write(ctx, s.pool, func(q qrm.DB) error { _, err := stmt.ExecContext(ctx, q); return err }); err != nil {
		return fmt.Errorf("client.postgres: %s: %w", op, err)
	}
	return nil
}
