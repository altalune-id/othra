package entity

import "github.com/go-jet/jet/v2/postgres"

// OAuthClients is the jet binding for the oauth_clients table.
type OAuthClients struct {
	postgres.Table

	ID                      postgres.ColumnString
	PublicID                postgres.ColumnString
	OrgID                   postgres.ColumnString
	ProjectID               postgres.ColumnString
	Name                    postgres.ColumnString
	ClientType              postgres.ColumnString
	ApplicationType         postgres.ColumnString
	TokenEndpointAuthMethod postgres.ColumnString
	GrantTypes              postgres.ColumnString
	AllowedScopes           postgres.ColumnString
	RedirectURIs            postgres.ColumnString
	PostLogoutRedirectURIs  postgres.ColumnString
	ConsentPolicy           postgres.ColumnString
	CreatedAt               postgres.ColumnTimestampz
	UpdatedAt               postgres.ColumnTimestampz

	AllColumns postgres.ColumnList
}

// NewOAuthClients builds the oauth_clients binding.
func NewOAuthClients(schema, tablePrefix string) *OAuthClients {
	if schema == "" {
		schema = "public"
	}
	var (
		id                      = postgres.StringColumn("id")
		publicID                = postgres.StringColumn("public_id")
		orgID                   = postgres.StringColumn("org_id")
		projectID               = postgres.StringColumn("project_id")
		name                    = postgres.StringColumn("name")
		clientType              = postgres.StringColumn("client_type")
		applicationType         = postgres.StringColumn("application_type")
		tokenEndpointAuthMethod = postgres.StringColumn("token_endpoint_auth_method")
		grantTypes              = postgres.StringColumn("grant_types")
		allowedScopes           = postgres.StringColumn("allowed_scopes")
		redirectURIs            = postgres.StringColumn("redirect_uris")
		postLogoutRedirectURIs  = postgres.StringColumn("post_logout_redirect_uris")
		consentPolicy           = postgres.StringColumn("consent_policy")
		createdAt               = postgres.TimestampzColumn("created_at")
		updatedAt               = postgres.TimestampzColumn("updated_at")
		all                     = postgres.ColumnList{
			id, publicID, orgID, projectID, name, clientType, applicationType, tokenEndpointAuthMethod,
			grantTypes, allowedScopes, redirectURIs, postLogoutRedirectURIs, consentPolicy, createdAt, updatedAt,
		}
	)
	return &OAuthClients{
		Table:                   postgres.NewTable(schema, tablePrefix+"oauth_clients", "oauth_clients", all...),
		ID:                      id,
		PublicID:                publicID,
		OrgID:                   orgID,
		ProjectID:               projectID,
		Name:                    name,
		ClientType:              clientType,
		ApplicationType:         applicationType,
		TokenEndpointAuthMethod: tokenEndpointAuthMethod,
		GrantTypes:              grantTypes,
		AllowedScopes:           allowedScopes,
		RedirectURIs:            redirectURIs,
		PostLogoutRedirectURIs:  postLogoutRedirectURIs,
		ConsentPolicy:           consentPolicy,
		CreatedAt:               createdAt,
		UpdatedAt:               updatedAt,
		AllColumns:              all,
	}
}
