package entity

import "github.com/go-jet/jet/v2/postgres"

// EndUserIdentities is the jet binding for the end_user_identities table.
type EndUserIdentities struct {
	postgres.Table

	ID                 postgres.ColumnString
	PublicID           postgres.ColumnString
	OrgID              postgres.ColumnString
	ProjectID          postgres.ColumnString
	EndUserID          postgres.ColumnString
	UpstreamProviderID postgres.ColumnString
	Issuer             postgres.ColumnString
	Subject            postgres.ColumnString
	Email              postgres.ColumnString
	EmailVerified      postgres.ColumnBool
	LinkedAt           postgres.ColumnTimestampz
	LastUsedAt         postgres.ColumnTimestampz

	AllColumns postgres.ColumnList
}

// NewEndUserIdentities builds the end_user_identities binding.
func NewEndUserIdentities(schema, tablePrefix string) *EndUserIdentities {
	if schema == "" {
		schema = "public"
	}
	var (
		id                 = postgres.StringColumn("id")
		publicID           = postgres.StringColumn("public_id")
		orgID              = postgres.StringColumn("org_id")
		projectID          = postgres.StringColumn("project_id")
		endUserID          = postgres.StringColumn("end_user_id")
		upstreamProviderID = postgres.StringColumn("upstream_provider_id")
		issuer             = postgres.StringColumn("issuer")
		subject            = postgres.StringColumn("subject")
		email              = postgres.StringColumn("email")
		emailVerified      = postgres.BoolColumn("email_verified")
		linkedAt           = postgres.TimestampzColumn("linked_at")
		lastUsedAt         = postgres.TimestampzColumn("last_used_at")
		all                = postgres.ColumnList{
			id, publicID, orgID, projectID, endUserID, upstreamProviderID, issuer, subject,
			email, emailVerified, linkedAt, lastUsedAt,
		}
	)
	return &EndUserIdentities{
		Table:              postgres.NewTable(schema, tablePrefix+"end_user_identities", "end_user_identities", all...),
		ID:                 id,
		PublicID:           publicID,
		OrgID:              orgID,
		ProjectID:          projectID,
		EndUserID:          endUserID,
		UpstreamProviderID: upstreamProviderID,
		Issuer:             issuer,
		Subject:            subject,
		Email:              email,
		EmailVerified:      emailVerified,
		LinkedAt:           linkedAt,
		LastUsedAt:         lastUsedAt,
		AllColumns:         all,
	}
}
