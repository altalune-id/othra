package entity

import "github.com/go-jet/jet/v2/postgres"

// RefreshTokens is the jet binding for the refresh_tokens table.
type RefreshTokens struct {
	postgres.Table

	ID        postgres.ColumnString
	OrgID     postgres.ColumnString
	ProjectID postgres.ColumnString
	FamilyID  postgres.ColumnString
	TokenHash postgres.ColumnString
	CreatedAt postgres.ColumnTimestampz
	ExpiresAt postgres.ColumnTimestampz
	UsedAt    postgres.ColumnTimestampz

	AllColumns postgres.ColumnList
}

// NewRefreshTokens builds the refresh_tokens binding.
func NewRefreshTokens(schema, tablePrefix string) *RefreshTokens {
	if schema == "" {
		schema = "public"
	}
	var (
		id        = postgres.StringColumn("id")
		orgID     = postgres.StringColumn("org_id")
		projectID = postgres.StringColumn("project_id")
		familyID  = postgres.StringColumn("family_id")
		tokenHash = postgres.StringColumn("token_hash")
		createdAt = postgres.TimestampzColumn("created_at")
		expiresAt = postgres.TimestampzColumn("expires_at")
		usedAt    = postgres.TimestampzColumn("used_at")
		all       = postgres.ColumnList{
			id, orgID, projectID, familyID, tokenHash, createdAt, expiresAt, usedAt,
		}
	)
	return &RefreshTokens{
		Table:      postgres.NewTable(schema, tablePrefix+"refresh_tokens", "refresh_tokens", all...),
		ID:         id,
		OrgID:      orgID,
		ProjectID:  projectID,
		FamilyID:   familyID,
		TokenHash:  tokenHash,
		CreatedAt:  createdAt,
		ExpiresAt:  expiresAt,
		UsedAt:     usedAt,
		AllColumns: all,
	}
}
