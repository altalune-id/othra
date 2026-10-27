package pat

import (
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/schema/entity"
)

type postgresStore struct {
	pool   db.Pool
	tokens *entity.PersonalTokens
	users  *entity.EndUsers
}

var _ Store = (*postgresStore)(nil)

// NewStore builds the Postgres store over pool.
func NewStore(pool db.Pool, schema, tablePrefix string) *postgresStore {
	return &postgresStore{pool: pool, tokens: entity.NewPersonalTokens(schema, tablePrefix), users: entity.NewEndUsers(schema, tablePrefix)}
}

type tokenRow struct {
	ID               uuid.UUID  `alias:"personal_tokens.id"`
	PublicID         string     `alias:"personal_tokens.public_id"`
	OrgID            uuid.UUID  `alias:"personal_tokens.org_id"`
	ProjectID        uuid.UUID  `alias:"personal_tokens.project_id"`
	EndUserID        uuid.UUID  `alias:"personal_tokens.end_user_id"`
	ResourceServerID uuid.UUID  `alias:"personal_tokens.resource_server_id"`
	Name             string     `alias:"personal_tokens.name"`
	Hint             string     `alias:"personal_tokens.hint"`
	Scopes           string     `alias:"personal_tokens.scopes"`
	CreatedAt        time.Time  `alias:"personal_tokens.created_at"`
	ExpiresAt        time.Time  `alias:"personal_tokens.expires_at"`
	LastUsedAt       *time.Time `alias:"personal_tokens.last_used_at"`
	RevokedAt        *time.Time `alias:"personal_tokens.revoked_at"`
}

func (s *postgresStore) columns() postgres.ProjectionList {
	t := s.tokens
	return postgres.ProjectionList{
		t.ID, t.PublicID, t.OrgID, t.ProjectID, t.EndUserID, t.ResourceServerID, t.Name, t.Hint,
		db.ArrayAsJSON(t.Scopes, "personal_tokens.scopes"), t.CreatedAt, t.ExpiresAt, t.LastUsedAt, t.RevokedAt,
	}
}

func (r *tokenRow) toToken() (*Token, error) {
	scopes, err := db.ParseJSONArray(r.Scopes)
	if err != nil {
		return nil, fmt.Errorf("pat.postgres: scopes of %s: %w", r.PublicID, err)
	}
	return &Token{
		ID: r.ID, PublicID: r.PublicID, OrgID: r.OrgID, ProjectID: r.ProjectID, EndUserID: r.EndUserID,
		ResourceServerID: r.ResourceServerID, Name: r.Name, Hint: r.Hint, Scopes: scopes, CreatedAt: r.CreatedAt.UTC(),
		ExpiresAt: r.ExpiresAt.UTC(), LastUsedAt: utc(r.LastUsedAt), RevokedAt: utc(r.RevokedAt),
	}, nil
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func wrap(op string, err error) error { return fmt.Errorf("pat.postgres: %s: %w", op, err) }
