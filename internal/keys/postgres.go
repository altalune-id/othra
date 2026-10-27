package keys

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/schema/entity"
)

type postgresStore struct {
	pool db.Pool
	keys *entity.SigningKeys
}

var _ Store = (*postgresStore)(nil)

// NewStore builds the Postgres store over pool.
func NewStore(pool db.Pool, schema, tablePrefix string) *postgresStore {
	return &postgresStore{pool: pool, keys: entity.NewSigningKeys(schema, tablePrefix)}
}

type keyRow struct {
	ID          uuid.UUID  `alias:"signing_keys.id"`
	PublicID    string     `alias:"signing_keys.public_id"`
	OrgID       uuid.UUID  `alias:"signing_keys.org_id"`
	ProjectID   uuid.UUID  `alias:"signing_keys.project_id"`
	Kid         string     `alias:"signing_keys.kid"`
	Alg         string     `alias:"signing_keys.alg"`
	PublicJWK   string     `alias:"signing_keys.public_jwk"`
	Status      string     `alias:"signing_keys.status"`
	CreatedAt   time.Time  `alias:"signing_keys.created_at"`
	ActivatesAt *time.Time `alias:"signing_keys.activates_at"`
	ActivatedAt *time.Time `alias:"signing_keys.activated_at"`
	RetireAfter *time.Time `alias:"signing_keys.retire_after"`
	RetiredAt   *time.Time `alias:"signing_keys.retired_at"`
	Sealed      []byte     `alias:"signing_keys.private_key_enc"`
}

func (r *keyRow) toKey() (*Key, error) {
	var jwk JWK
	if err := json.Unmarshal([]byte(r.PublicJWK), &jwk); err != nil {
		return nil, fmt.Errorf("keys.postgres: public_jwk of %s: %w", r.PublicID, err)
	}
	return &Key{
		ID: r.ID, PublicID: r.PublicID, OrgID: r.OrgID, ProjectID: r.ProjectID, KID: r.Kid, Alg: r.Alg, JWK: jwk,
		Status: Status(r.Status), CreatedAt: r.CreatedAt.UTC(), ActivatesAt: utc(r.ActivatesAt),
		ActivatedAt: utc(r.ActivatedAt), RetireAfter: utc(r.RetireAfter), RetiredAt: utc(r.RetiredAt),
	}, nil
}

func toKeys(rows []keyRow) ([]*Key, error) {
	out := make([]*Key, 0, len(rows))
	for i := range rows {
		k, err := rows[i].toKey()
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, nil
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}
