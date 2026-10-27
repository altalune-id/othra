package entity

import "github.com/go-jet/jet/v2/postgres"

// PersonalTokens is the jet binding for the personal_tokens table.
type PersonalTokens struct {
	postgres.Table

	ID               postgres.ColumnString
	PublicID         postgres.ColumnString
	OrgID            postgres.ColumnString
	ProjectID        postgres.ColumnString
	EndUserID        postgres.ColumnString
	ResourceServerID postgres.ColumnString
	Name             postgres.ColumnString
	TokenHash        postgres.ColumnString
	Hint             postgres.ColumnString
	Scopes           postgres.ColumnString
	CreatedAt        postgres.ColumnTimestampz
	ExpiresAt        postgres.ColumnTimestampz
	LastUsedAt       postgres.ColumnTimestampz
	RevokedAt        postgres.ColumnTimestampz

	AllColumns postgres.ColumnList
}

// NewPersonalTokens builds the personal_tokens binding.
func NewPersonalTokens(schema, tablePrefix string) *PersonalTokens {
	if schema == "" {
		schema = "public"
	}
	var (
		id               = postgres.StringColumn("id")
		publicID         = postgres.StringColumn("public_id")
		orgID            = postgres.StringColumn("org_id")
		projectID        = postgres.StringColumn("project_id")
		endUserID        = postgres.StringColumn("end_user_id")
		resourceServerID = postgres.StringColumn("resource_server_id")
		name             = postgres.StringColumn("name")
		tokenHash        = postgres.StringColumn("token_hash")
		hint             = postgres.StringColumn("hint")
		scopes           = postgres.StringColumn("scopes")
		createdAt        = postgres.TimestampzColumn("created_at")
		expiresAt        = postgres.TimestampzColumn("expires_at")
		lastUsedAt       = postgres.TimestampzColumn("last_used_at")
		revokedAt        = postgres.TimestampzColumn("revoked_at")
		all              = postgres.ColumnList{
			id, publicID, orgID, projectID, endUserID, resourceServerID, name, tokenHash,
			hint, scopes, createdAt, expiresAt, lastUsedAt, revokedAt,
		}
	)
	return &PersonalTokens{
		Table:            postgres.NewTable(schema, tablePrefix+"personal_tokens", "personal_tokens", all...),
		ID:               id,
		PublicID:         publicID,
		OrgID:            orgID,
		ProjectID:        projectID,
		EndUserID:        endUserID,
		ResourceServerID: resourceServerID,
		Name:             name,
		TokenHash:        tokenHash,
		Hint:             hint,
		Scopes:           scopes,
		CreatedAt:        createdAt,
		ExpiresAt:        expiresAt,
		LastUsedAt:       lastUsedAt,
		RevokedAt:        revokedAt,
		AllColumns:       all,
	}
}
