package entity

import "github.com/go-jet/jet/v2/postgres"

// GrantFamilies is the jet binding for the grant_families table.
type GrantFamilies struct {
	postgres.Table

	ID                  postgres.ColumnString
	PublicID            postgres.ColumnString
	OrgID               postgres.ColumnString
	ProjectID           postgres.ColumnString
	ClientID            postgres.ColumnString
	EndUserID           postgres.ColumnString
	UserSessionID       postgres.ColumnString
	UserSessionPublicID postgres.ColumnString
	Scopes              postgres.ColumnString
	AuthTime            postgres.ColumnTimestampz
	CreatedAt           postgres.ColumnTimestampz
	LastUsedAt          postgres.ColumnTimestampz
	AbsoluteExpiresAt   postgres.ColumnTimestampz
	RevokedAt           postgres.ColumnTimestampz
	RevokedReason       postgres.ColumnString
	ResourceServerID    postgres.ColumnString

	AllColumns postgres.ColumnList
}

// NewGrantFamilies builds the grant_families binding.
func NewGrantFamilies(schema, tablePrefix string) *GrantFamilies {
	if schema == "" {
		schema = "public"
	}
	var (
		id                  = postgres.StringColumn("id")
		publicID            = postgres.StringColumn("public_id")
		orgID               = postgres.StringColumn("org_id")
		projectID           = postgres.StringColumn("project_id")
		clientID            = postgres.StringColumn("client_id")
		endUserID           = postgres.StringColumn("end_user_id")
		userSessionID       = postgres.StringColumn("user_session_id")
		userSessionPublicID = postgres.StringColumn("user_session_public_id")
		scopes              = postgres.StringColumn("scopes")
		authTime            = postgres.TimestampzColumn("auth_time")
		createdAt           = postgres.TimestampzColumn("created_at")
		lastUsedAt          = postgres.TimestampzColumn("last_used_at")
		absoluteExpiresAt   = postgres.TimestampzColumn("absolute_expires_at")
		revokedAt           = postgres.TimestampzColumn("revoked_at")
		revokedReason       = postgres.StringColumn("revoked_reason")
		resourceServerID    = postgres.StringColumn("resource_server_id")
		all                 = postgres.ColumnList{
			id, publicID, orgID, projectID, clientID, endUserID, userSessionID, userSessionPublicID,
			scopes, authTime, createdAt, lastUsedAt, absoluteExpiresAt, revokedAt, revokedReason, resourceServerID,
		}
	)
	return &GrantFamilies{
		Table:               postgres.NewTable(schema, tablePrefix+"grant_families", "grant_families", all...),
		ID:                  id,
		PublicID:            publicID,
		OrgID:               orgID,
		ProjectID:           projectID,
		ClientID:            clientID,
		EndUserID:           endUserID,
		UserSessionID:       userSessionID,
		UserSessionPublicID: userSessionPublicID,
		Scopes:              scopes,
		AuthTime:            authTime,
		CreatedAt:           createdAt,
		LastUsedAt:          lastUsedAt,
		AbsoluteExpiresAt:   absoluteExpiresAt,
		RevokedAt:           revokedAt,
		RevokedReason:       revokedReason,
		ResourceServerID:    resourceServerID,
		AllColumns:          all,
	}
}
