package federation

import (
	"context"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/upstream"
	"altalune.id/othra/schema/entity"
)

type postgresStore struct {
	pool        db.Pool
	subjectKey  string
	upstreamKey string
	identities  *entity.EndUserIdentities
	upstreams   *entity.UpstreamProviders
	users       *entity.EndUsers
}

var _ Store = (*postgresStore)(nil)

// NewStore builds the Postgres store over pool.
func NewStore(pool db.Pool, schema, tablePrefix string) *postgresStore {
	if schema == "" {
		schema = "public"
	}
	return &postgresStore{
		pool:        pool,
		subjectKey:  tablePrefix + "end_user_identities_subject_key",
		upstreamKey: tablePrefix + "end_user_identities_one_per_upstream",
		identities:  entity.NewEndUserIdentities(schema, tablePrefix),
		upstreams:   entity.NewUpstreamProviders(schema, tablePrefix),
		users:       entity.NewEndUsers(schema, tablePrefix),
	}
}

type identityRow struct {
	ID            uuid.UUID `alias:"end_user_identities.id"`
	PublicID      string    `alias:"end_user_identities.public_id"`
	EndUserID     uuid.UUID `alias:"end_user_identities.end_user_id"`
	UpstreamID    uuid.UUID `alias:"end_user_identities.upstream_provider_id"`
	Issuer        string    `alias:"end_user_identities.issuer"`
	Subject       string    `alias:"end_user_identities.subject"`
	Email         *string   `alias:"end_user_identities.email"`
	EmailVerified bool      `alias:"end_user_identities.email_verified"`
	LinkedAt      time.Time `alias:"end_user_identities.linked_at"`
	LastUsedAt    time.Time `alias:"end_user_identities.last_used_at"`
	Kind          string    `alias:"upstream_providers.kind"`
}

func (r *identityRow) toIdentity() Identity {
	email := ""
	if r.Email != nil {
		email = *r.Email
	}
	return Identity{
		UpstreamID: r.UpstreamID, Issuer: r.Issuer, Subject: r.Subject, Email: email, EmailVerified: r.EmailVerified,
		PublicID: r.PublicID, Kind: upstream.Kind(r.Kind), LinkedAt: r.LinkedAt.UTC(), LastUsedAt: r.LastUsedAt.UTC(),
	}
}

func (s *postgresStore) columns() postgres.ProjectionList {
	t, u := s.identities, s.upstreams
	return postgres.ProjectionList{
		t.ID, t.PublicID, t.EndUserID, t.UpstreamProviderID, t.Issuer, t.Subject, t.Email, t.EmailVerified, t.LinkedAt, t.LastUsedAt, u.Kind,
	}
}

func (s *postgresStore) from() postgres.ReadableTable {
	t, u := s.identities, s.upstreams
	return t.INNER_JOIN(u, u.ID.EQ(t.UpstreamProviderID))
}

func (s *postgresStore) exec(ctx context.Context, op string, stmt postgres.Statement) (int64, error) {
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
		return 0, wrap(op, err)
	}
	return n, nil
}

func nullable(v string) postgres.Expression {
	if v == "" {
		return postgres.NULL
	}
	return postgres.String(v)
}

func wrap(op string, err error) error {
	return fmt.Errorf("federation.postgres: %s: %w", op, err)
}
