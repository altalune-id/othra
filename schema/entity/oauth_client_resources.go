package entity

import "github.com/go-jet/jet/v2/postgres"

// OAuthClientResources is the jet binding for the oauth_client_resources table.
type OAuthClientResources struct {
	postgres.Table

	OrgID            postgres.ColumnString
	ProjectID        postgres.ColumnString
	ClientID         postgres.ColumnString
	ResourceServerID postgres.ColumnString
	Scopes           postgres.ColumnString
	CreatedAt        postgres.ColumnTimestampz
	UpdatedAt        postgres.ColumnTimestampz

	AllColumns postgres.ColumnList
}

// NewOAuthClientResources builds the oauth_client_resources binding.
func NewOAuthClientResources(schema, tablePrefix string) *OAuthClientResources {
	if schema == "" {
		schema = "public"
	}
	var (
		orgID            = postgres.StringColumn("org_id")
		projectID        = postgres.StringColumn("project_id")
		clientID         = postgres.StringColumn("client_id")
		resourceServerID = postgres.StringColumn("resource_server_id")
		scopes           = postgres.StringColumn("scopes")
		createdAt        = postgres.TimestampzColumn("created_at")
		updatedAt        = postgres.TimestampzColumn("updated_at")
		all              = postgres.ColumnList{
			orgID, projectID, clientID, resourceServerID, scopes, createdAt, updatedAt,
		}
	)
	return &OAuthClientResources{
		Table:            postgres.NewTable(schema, tablePrefix+"oauth_client_resources", "oauth_client_resources", all...),
		OrgID:            orgID,
		ProjectID:        projectID,
		ClientID:         clientID,
		ResourceServerID: resourceServerID,
		Scopes:           scopes,
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
		AllColumns:       all,
	}
}
