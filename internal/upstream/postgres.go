package upstream

import (
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/schema/entity"
)

type postgresStore struct {
	pool       db.Pool
	upstreams  *entity.UpstreamProviders
	identities *entity.EndUserIdentities
	kindKey    string
}

var _ Store = (*postgresStore)(nil)

// NewStore builds the Postgres store over pool.
func NewStore(pool db.Pool, schema, tablePrefix string) *postgresStore {
	if schema == "" {
		schema = "public"
	}
	return &postgresStore{
		pool: pool, upstreams: entity.NewUpstreamProviders(schema, tablePrefix), identities: entity.NewEndUserIdentities(schema, tablePrefix),
		kindKey: tablePrefix + "upstream_providers_kind_key",
	}
}

type upstreamRow struct {
	ID        uuid.UUID `alias:"upstream_providers.id"`
	PublicID  string    `alias:"upstream_providers.public_id"`
	OrgID     uuid.UUID `alias:"upstream_providers.org_id"`
	ProjectID uuid.UUID `alias:"upstream_providers.project_id"`
	Kind      string    `alias:"upstream_providers.kind"`
	ClientID  string    `alias:"upstream_providers.client_id"`
	Enabled   bool      `alias:"upstream_providers.enabled"`
	Linked    bool      `alias:"upstream_providers.identities_linked"`
	CreatedAt time.Time `alias:"upstream_providers.created_at"`
	UpdatedAt time.Time `alias:"upstream_providers.updated_at"`
}

func (r *upstreamRow) toUpstream() *Upstream {
	return &Upstream{
		ID: r.ID, PublicID: r.PublicID, OrgID: r.OrgID, ProjectID: r.ProjectID, Kind: Kind(r.Kind), ClientID: r.ClientID,
		Enabled: r.Enabled, IdentitiesLinked: r.Linked, CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
	}
}

func (s *postgresStore) columns() postgres.ProjectionList {
	t, i := s.upstreams, s.identities
	linked := postgres.EXISTS(postgres.SELECT(i.ID).FROM(i).WHERE(i.ProjectID.EQ(t.ProjectID).AND(i.UpstreamProviderID.EQ(t.ID))))
	return postgres.ProjectionList{
		t.ID, t.PublicID, t.OrgID, t.ProjectID, t.Kind, t.ClientID, t.Enabled, linked.AS("upstream_providers.identities_linked"),
		t.CreatedAt, t.UpdatedAt,
	}
}

func wrap(op string, err error) error {
	return fmt.Errorf("upstream.postgres: %s: %w", op, err)
}
