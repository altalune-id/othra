package entity

import "github.com/go-jet/jet/v2/postgres"

// AccessTokens is the jet binding for the access_tokens table.
type AccessTokens struct {
	postgres.Table

	ID               postgres.ColumnString
	OrgID            postgres.ColumnString
	ProjectID        postgres.ColumnString
	FamilyID         postgres.ColumnString
	TokenHash        postgres.ColumnString
	Scopes           postgres.ColumnString
	CreatedAt        postgres.ColumnTimestampz
	ExpiresAt        postgres.ColumnTimestampz
	ResourceServerID postgres.ColumnString
	RevokedAt        postgres.ColumnTimestampz

	AllColumns postgres.ColumnList
}

// NewAccessTokens builds the access_tokens binding.
func NewAccessTokens(schema, tablePrefix string) *AccessTokens {
	if schema == "" {
		schema = "public"
	}
	var (
		id               = postgres.StringColumn("id")
		orgID            = postgres.StringColumn("org_id")
		projectID        = postgres.StringColumn("project_id")
		familyID         = postgres.StringColumn("family_id")
		tokenHash        = postgres.StringColumn("token_hash")
		scopes           = postgres.StringColumn("scopes")
		createdAt        = postgres.TimestampzColumn("created_at")
		expiresAt        = postgres.TimestampzColumn("expires_at")
		resourceServerID = postgres.StringColumn("resource_server_id")
		revokedAt        = postgres.TimestampzColumn("revoked_at")
		all              = postgres.ColumnList{
			id, orgID, projectID, familyID, tokenHash, scopes, createdAt, expiresAt,
			resourceServerID, revokedAt,
		}
	)
	return &AccessTokens{
		Table:            postgres.NewTable(schema, tablePrefix+"access_tokens", "access_tokens", all...),
		ID:               id,
		OrgID:            orgID,
		ProjectID:        projectID,
		FamilyID:         familyID,
		TokenHash:        tokenHash,
		Scopes:           scopes,
		CreatedAt:        createdAt,
		ExpiresAt:        expiresAt,
		ResourceServerID: resourceServerID,
		RevokedAt:        revokedAt,
		AllColumns:       all,
	}
}
