package client

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/schema/entity"
)

type postgresStore struct {
	pool    db.Pool
	clients *entity.OAuthClients
	secrets *entity.OAuthClientSecrets
}

var _ Store = (*postgresStore)(nil)

// NewStore builds the Postgres store over pool.
func NewStore(pool db.Pool, schema, tablePrefix string) *postgresStore {
	return &postgresStore{
		pool:    pool,
		clients: entity.NewOAuthClients(schema, tablePrefix),
		secrets: entity.NewOAuthClientSecrets(schema, tablePrefix),
	}
}

type clientRow struct {
	ID                     uuid.UUID `alias:"oauth_clients.id"`
	PublicID               string    `alias:"oauth_clients.public_id"`
	OrgID                  uuid.UUID `alias:"oauth_clients.org_id"`
	ProjectID              uuid.UUID `alias:"oauth_clients.project_id"`
	Name                   string    `alias:"oauth_clients.name"`
	ClientType             string    `alias:"oauth_clients.client_type"`
	ApplicationType        string    `alias:"oauth_clients.application_type"`
	AuthMethod             string    `alias:"oauth_clients.token_endpoint_auth_method"`
	GrantTypes             string    `alias:"oauth_clients.grant_types"`
	AllowedScopes          string    `alias:"oauth_clients.allowed_scopes"`
	RedirectURIs           string    `alias:"oauth_clients.redirect_uris"`
	PostLogoutRedirectURIs string    `alias:"oauth_clients.post_logout_redirect_uris"`
	ConsentPolicy          string    `alias:"oauth_clients.consent_policy"`
	CreatedAt              time.Time `alias:"oauth_clients.created_at"`
	UpdatedAt              time.Time `alias:"oauth_clients.updated_at"`
}

func (r *clientRow) toClient() (*Client, error) {
	var grants, scopes, redirects, postLogout []string
	for _, a := range []struct {
		col string
		dst *[]string
	}{{r.GrantTypes, &grants}, {r.AllowedScopes, &scopes}, {r.RedirectURIs, &redirects}, {r.PostLogoutRedirectURIs, &postLogout}} {
		if err := json.Unmarshal([]byte(a.col), a.dst); err != nil {
			return nil, fmt.Errorf("client.postgres: arrays of %s: %w", r.PublicID, err)
		}
	}
	if postLogout == nil {
		postLogout = []string{}
	}
	return &Client{
		ID: r.ID, PublicID: r.PublicID, OrgID: r.OrgID, ProjectID: r.ProjectID, Name: r.Name,
		ClientType: r.ClientType, ApplicationType: r.ApplicationType, TokenEndpointAuthMethod: r.AuthMethod,
		GrantTypes: grants, AllowedScopes: scopes, RedirectURIs: redirects, PostLogoutRedirectURIs: postLogout,
		ConsentPolicy: ConsentPolicy(r.ConsentPolicy),
		CreatedAt:     r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(), Secrets: []Secret{},
	}, nil
}

type secretRow struct {
	ID        uuid.UUID  `alias:"oauth_client_secrets.id"`
	PublicID  string     `alias:"oauth_client_secrets.public_id"`
	ClientID  uuid.UUID  `alias:"oauth_client_secrets.client_id"`
	Hint      string     `alias:"oauth_client_secrets.hint"`
	CreatedAt time.Time  `alias:"oauth_client_secrets.created_at"`
	ExpiresAt *time.Time `alias:"oauth_client_secrets.expires_at"`
}

func (r *secretRow) toSecret() Secret {
	s := Secret{ID: r.ID, PublicID: r.PublicID, Hint: r.Hint, CreatedAt: r.CreatedAt.UTC()}
	if r.ExpiresAt != nil {
		at := r.ExpiresAt.UTC()
		s.ExpiresAt = &at
	}
	return s
}

func jsonArray(col postgres.Column, alias string) postgres.Projection {
	return postgres.CAST(postgres.Func("to_json", col)).AS_TEXT().AS(alias)
}
