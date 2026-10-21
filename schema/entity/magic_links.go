package entity

import "github.com/go-jet/jet/v2/postgres"

// MagicLinks is the jet binding for the magic_links table.
type MagicLinks struct {
	postgres.Table

	ID               postgres.ColumnString
	PublicID         postgres.ColumnString
	OrgID            postgres.ColumnString
	ProjectID        postgres.ColumnString
	TokenHash        postgres.ColumnString
	Email            postgres.ColumnString
	ReturnTo         postgres.ColumnString
	RequestIP        postgres.ColumnString
	CreatedAt        postgres.ColumnTimestampz
	ExpiresAt        postgres.ColumnTimestampz
	UsedAt           postgres.ColumnTimestampz
	BrowserSessionID postgres.ColumnString

	AllColumns postgres.ColumnList
}

// NewMagicLinks builds the magic_links binding.
func NewMagicLinks(schema, tablePrefix string) *MagicLinks {
	if schema == "" {
		schema = "public"
	}
	var (
		id               = postgres.StringColumn("id")
		publicID         = postgres.StringColumn("public_id")
		orgID            = postgres.StringColumn("org_id")
		projectID        = postgres.StringColumn("project_id")
		tokenHash        = postgres.StringColumn("token_hash")
		email            = postgres.StringColumn("email")
		returnTo         = postgres.StringColumn("return_to")
		requestIP        = postgres.StringColumn("request_ip")
		createdAt        = postgres.TimestampzColumn("created_at")
		expiresAt        = postgres.TimestampzColumn("expires_at")
		usedAt           = postgres.TimestampzColumn("used_at")
		browserSessionID = postgres.StringColumn("browser_session_id")
		all              = postgres.ColumnList{
			id, publicID, orgID, projectID, tokenHash, email, returnTo, requestIP,
			createdAt, expiresAt, usedAt, browserSessionID,
		}
	)
	return &MagicLinks{
		Table:            postgres.NewTable(schema, tablePrefix+"magic_links", "magic_links", all...),
		ID:               id,
		PublicID:         publicID,
		OrgID:            orgID,
		ProjectID:        projectID,
		TokenHash:        tokenHash,
		Email:            email,
		ReturnTo:         returnTo,
		RequestIP:        requestIP,
		CreatedAt:        createdAt,
		ExpiresAt:        expiresAt,
		UsedAt:           usedAt,
		BrowserSessionID: browserSessionID,
		AllColumns:       all,
	}
}
