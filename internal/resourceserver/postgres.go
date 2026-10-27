package resourceserver

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/kernel/secret"
	"altalune.id/othra/schema/entity"
)

type postgresStore struct {
	pool          db.Pool
	servers       *entity.ResourceServers
	links         *entity.OAuthClientResources
	secrets       *entity.ResourceServerSecrets
	identifierKey string
	clientFKey    string
}

var _ Store = (*postgresStore)(nil)

// NewStore builds the Postgres store over pool.
func NewStore(pool db.Pool, schema, tablePrefix string) *postgresStore {
	return &postgresStore{
		pool: pool, servers: entity.NewResourceServers(schema, tablePrefix), links: entity.NewOAuthClientResources(schema, tablePrefix),
		secrets:       entity.NewResourceServerSecrets(schema, tablePrefix),
		identifierKey: tablePrefix + "resource_servers_identifier_key",
		clientFKey:    tablePrefix + "oauth_client_resources_client_fkey",
	}
}

type serverRow struct {
	ID                  uuid.UUID  `alias:"resource_servers.id"`
	PublicID            string     `alias:"resource_servers.public_id"`
	OrgID               uuid.UUID  `alias:"resource_servers.org_id"`
	ProjectID           uuid.UUID  `alias:"resource_servers.project_id"`
	Name                string     `alias:"resource_servers.name"`
	Identifier          string     `alias:"resource_servers.identifier"`
	Scopes              string     `alias:"resource_servers.scopes"`
	VerificationState   string     `alias:"resource_servers.verification_state"`
	VerifiedAt          *time.Time `alias:"resource_servers.verified_at"`
	LastCheckedAt       *time.Time `alias:"resource_servers.last_checked_at"`
	LastError           *string    `alias:"resource_servers.last_error"`
	ConsecutiveFailures int32      `alias:"resource_servers.consecutive_failures"`
	Trusted             bool       `alias:"resource_servers.trusted"`
	TrustedAt           *time.Time `alias:"resource_servers.trusted_at"`
	TrustedBy           *uuid.UUID `alias:"resource_servers.trusted_by"`
	CreatedAt           time.Time  `alias:"resource_servers.created_at"`
	UpdatedAt           time.Time  `alias:"resource_servers.updated_at"`
	AcceptCIMD          bool       `alias:"resource_servers.accept_cimd_clients"`
	CIMDHosts           string     `alias:"resource_servers.cimd_client_hosts"`
	CIMDScopes          *string    `alias:"resource_servers.cimd_scopes"`
	Descriptions        string     `alias:"resource_servers.scope_descriptions"`
	AcceptPersonal      bool       `alias:"resource_servers.accept_personal_tokens"`
}

type secretRow struct {
	ID        uuid.UUID  `alias:"resource_server_secrets.id"`
	PublicID  string     `alias:"resource_server_secrets.public_id"`
	ServerID  uuid.UUID  `alias:"resource_server_secrets.resource_server_id"`
	Hint      string     `alias:"resource_server_secrets.hint"`
	CreatedAt time.Time  `alias:"resource_server_secrets.created_at"`
	ExpiresAt *time.Time `alias:"resource_server_secrets.expires_at"`
	Hash      string     `alias:"resource_server_secrets.secret_hash"`
}

func (r *secretRow) toStored(withHash bool) secret.Stored {
	s := secret.Stored{ID: r.ID, PublicID: r.PublicID, Hint: r.Hint, CreatedAt: r.CreatedAt.UTC(), ExpiresAt: utc(r.ExpiresAt)}
	if withHash {
		s = s.WithHash(r.Hash)
	}
	return s
}

type linkRow struct {
	Server    serverRow
	ClientID  uuid.UUID `alias:"oauth_client_resources.client_id"`
	Pins      string    `alias:"oauth_client_resources.scopes"`
	CreatedAt time.Time `alias:"oauth_client_resources.created_at"`
	UpdatedAt time.Time `alias:"oauth_client_resources.updated_at"`
}

func (r *serverRow) toServer() (*ResourceServer, error) {
	scopes, err := db.ParseJSONArray(r.Scopes)
	if err != nil {
		return nil, fmt.Errorf("resourceserver.postgres: scopes of %s: %w", r.PublicID, err)
	}
	out := &ResourceServer{
		ID: r.ID, PublicID: r.PublicID, OrgID: r.OrgID, ProjectID: r.ProjectID, Name: r.Name, Identifier: r.Identifier,
		Scopes: scopes, State: State(r.VerificationState), VerifiedAt: utc(r.VerifiedAt), LastCheckedAt: utc(r.LastCheckedAt),
		ConsecutiveFailures: r.ConsecutiveFailures, Trusted: r.Trusted, TrustedAt: utc(r.TrustedAt),
		CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(), AcceptPersonalTokens: r.AcceptPersonal,
	}
	if r.TrustedBy != nil {
		out.TrustedBy = *r.TrustedBy
	}
	if out.Admission, err = r.admission(); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(r.Descriptions), &out.Descriptions); err != nil {
		return nil, fmt.Errorf("resourceserver.postgres: scope_descriptions of %s: %w", r.PublicID, err)
	}
	if r.LastError != nil {
		var f Failure
		if err := json.Unmarshal([]byte(*r.LastError), &f); err != nil {
			return nil, fmt.Errorf("resourceserver.postgres: last_error of %s: %w", r.PublicID, err)
		}
		f.CheckedAt = f.CheckedAt.UTC()
		out.LastError = &f
	}
	return out, nil
}

func (r *linkRow) toLink() (*Link, error) {
	server, err := r.Server.toServer()
	if err != nil {
		return nil, err
	}
	pins, err := db.ParseJSONArray(r.Pins)
	if err != nil {
		return nil, fmt.Errorf("resourceserver.postgres: link scopes of %s: %w", server.PublicID, err)
	}
	return &Link{
		OrgID: server.OrgID, ProjectID: server.ProjectID, ClientID: r.ClientID, ResourceServer: server, Scopes: pins,
		CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
	}, nil
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func timeExpr(t *time.Time) postgres.Expression {
	if t == nil {
		return postgres.NULL
	}
	return postgres.TimestampzT(t.UTC())
}

func (r *serverRow) admission() (Admission, error) {
	a := Admission{Accept: r.AcceptCIMD}
	hosts, err := db.ParseJSONArray(r.CIMDHosts)
	if err != nil {
		return a, fmt.Errorf("resourceserver.postgres: cimd_client_hosts of %s: %w", r.PublicID, err)
	}
	if len(hosts) > 0 {
		a.Hosts = hosts
	}
	if r.CIMDScopes != nil {
		if a.Scopes, err = db.ParseJSONArray(*r.CIMDScopes); err != nil {
			return a, fmt.Errorf("resourceserver.postgres: cimd_scopes of %s: %w", r.PublicID, err)
		}
	}
	return a, nil
}

func nullArray(v []string) postgres.Expression {
	if v == nil {
		return postgres.NULL
	}
	return db.TextArray(v)
}

func descriptionsExpr(d map[string]string) (postgres.StringExpression, error) {
	if d == nil {
		d = map[string]string{}
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	return postgres.StringExp(postgres.CAST(postgres.String(string(raw))).AS("jsonb")), nil
}
