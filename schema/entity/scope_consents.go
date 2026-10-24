package entity

import "github.com/go-jet/jet/v2/postgres"

// ScopeConsents is the jet binding for the scope_consents table.
type ScopeConsents struct {
	postgres.Table

	ID               postgres.ColumnString
	PublicID         postgres.ColumnString
	OrgID            postgres.ColumnString
	ProjectID        postgres.ColumnString
	EndUserID        postgres.ColumnString
	ClientID         postgres.ColumnString
	ResourceServerID postgres.ColumnString
	Scopes           postgres.ColumnString
	GrantedAt        postgres.ColumnTimestampz
	UpdatedAt        postgres.ColumnTimestampz
	GrantedIP        postgres.ColumnString
	GrantedUserAgent postgres.ColumnString

	AllColumns postgres.ColumnList
}

// NewScopeConsents builds the scope_consents binding.
func NewScopeConsents(schema, tablePrefix string) *ScopeConsents {
	if schema == "" {
		schema = "public"
	}
	var (
		id               = postgres.StringColumn("id")
		publicID         = postgres.StringColumn("public_id")
		orgID            = postgres.StringColumn("org_id")
		projectID        = postgres.StringColumn("project_id")
		endUserID        = postgres.StringColumn("end_user_id")
		clientID         = postgres.StringColumn("client_id")
		resourceServerID = postgres.StringColumn("resource_server_id")
		scopes           = postgres.StringColumn("scopes")
		grantedAt        = postgres.TimestampzColumn("granted_at")
		updatedAt        = postgres.TimestampzColumn("updated_at")
		grantedIP        = postgres.StringColumn("granted_ip")
		grantedUserAgent = postgres.StringColumn("granted_user_agent")
		all              = postgres.ColumnList{
			id, publicID, orgID, projectID, endUserID, clientID, resourceServerID, scopes,
			grantedAt, updatedAt, grantedIP, grantedUserAgent,
		}
	)
	return &ScopeConsents{
		Table:            postgres.NewTable(schema, tablePrefix+"scope_consents", "scope_consents", all...),
		ID:               id,
		PublicID:         publicID,
		OrgID:            orgID,
		ProjectID:        projectID,
		EndUserID:        endUserID,
		ClientID:         clientID,
		ResourceServerID: resourceServerID,
		Scopes:           scopes,
		GrantedAt:        grantedAt,
		UpdatedAt:        updatedAt,
		GrantedIP:        grantedIP,
		GrantedUserAgent: grantedUserAgent,
		AllColumns:       all,
	}
}
