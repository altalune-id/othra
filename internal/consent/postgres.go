package consent

import (
	"fmt"
	"net/netip"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/schema/entity"
)

type postgresStore struct {
	pool     db.Pool
	consents *entity.ScopeConsents
	grantKey string
	table    string
}

var _ Store = (*postgresStore)(nil)

// NewStore builds the Postgres store over pool.
func NewStore(pool db.Pool, schema, tablePrefix string) *postgresStore {
	if schema == "" {
		schema = "public"
	}
	return &postgresStore{
		pool: pool, consents: entity.NewScopeConsents(schema, tablePrefix),
		grantKey: tablePrefix + "scope_consents_grant_key", table: tablePrefix + "scope_consents",
	}
}

type consentRow struct {
	ID               uuid.UUID  `alias:"scope_consents.id"`
	PublicID         string     `alias:"scope_consents.public_id"`
	OrgID            uuid.UUID  `alias:"scope_consents.org_id"`
	ProjectID        uuid.UUID  `alias:"scope_consents.project_id"`
	EndUserID        uuid.UUID  `alias:"scope_consents.end_user_id"`
	ClientID         uuid.UUID  `alias:"scope_consents.client_id"`
	ResourceServerID *uuid.UUID `alias:"scope_consents.resource_server_id"`
	Scopes           string     `alias:"scope_consents.scopes"`
	GrantedAt        time.Time  `alias:"scope_consents.granted_at"`
	UpdatedAt        time.Time  `alias:"scope_consents.updated_at"`
}

func (r *consentRow) toConsent() (*Consent, error) {
	scopes, err := db.ParseJSONArray(r.Scopes)
	if err != nil {
		return nil, wrap("scopes", err)
	}
	return &Consent{
		ID: r.ID, PublicID: r.PublicID, OrgID: r.OrgID, ProjectID: r.ProjectID,
		Key: Key{EndUserID: r.EndUserID, ClientID: r.ClientID, ResourceServerID: r.ResourceServerID}, Scopes: scopes,
		GrantedAt: r.GrantedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
	}, nil
}

func (s *postgresStore) columns() postgres.ProjectionList {
	t := s.consents
	return postgres.ProjectionList{
		t.ID, t.PublicID, t.OrgID, t.ProjectID, t.EndUserID, t.ClientID, t.ResourceServerID,
		db.ArrayAsJSON(t.Scopes, "scope_consents.scopes"), t.GrantedAt, t.UpdatedAt,
	}
}

func (s *postgresStore) keyed(projectID uuid.UUID, k Key) postgres.BoolExpression {
	t := s.consents
	where := t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.EndUserID.EQ(postgres.UUID(k.EndUserID))).
		AND(t.ClientID.EQ(postgres.UUID(k.ClientID)))
	if k.ResourceServerID == nil {
		return where.AND(t.ResourceServerID.IS_NULL())
	}
	return where.AND(t.ResourceServerID.EQ(postgres.UUID(*k.ResourceServerID)))
}

func nullUUID(v *uuid.UUID) postgres.Expression {
	if v == nil {
		return postgres.NULL
	}
	return postgres.UUID(*v)
}

// SECURITY: a value that is not an IP address is stored as NULL, never cast from client input.
func inet(ip string) postgres.Expression {
	if _, err := netip.ParseAddr(ip); err == nil {
		return postgres.CAST(postgres.String(ip)).AS("inet")
	}
	return postgres.NULL
}

func nullString(v string) postgres.Expression {
	if v == "" {
		return postgres.NULL
	}
	return postgres.String(v)
}

func wrap(op string, err error) error {
	return fmt.Errorf("consent.postgres: %s: %w", op, err)
}
