package entity

import "github.com/go-jet/jet/v2/postgres"

// EndUsers is the jet binding for the end_users table.
type EndUsers struct {
	postgres.Table

	ID              postgres.ColumnString
	PublicID        postgres.ColumnString
	OrgID           postgres.ColumnString
	ProjectID       postgres.ColumnString
	Email           postgres.ColumnString
	EmailVerifiedAt postgres.ColumnTimestampz
	Name            postgres.ColumnString
	LastSignInAt    postgres.ColumnTimestampz
	CreatedAt       postgres.ColumnTimestampz
	UpdatedAt       postgres.ColumnTimestampz

	AllColumns postgres.ColumnList
}

// NewEndUsers builds the end_users binding.
func NewEndUsers(schema, tablePrefix string) *EndUsers {
	if schema == "" {
		schema = "public"
	}
	var (
		id              = postgres.StringColumn("id")
		publicID        = postgres.StringColumn("public_id")
		orgID           = postgres.StringColumn("org_id")
		projectID       = postgres.StringColumn("project_id")
		email           = postgres.StringColumn("email")
		emailVerifiedAt = postgres.TimestampzColumn("email_verified_at")
		name            = postgres.StringColumn("name")
		lastSignInAt    = postgres.TimestampzColumn("last_sign_in_at")
		createdAt       = postgres.TimestampzColumn("created_at")
		updatedAt       = postgres.TimestampzColumn("updated_at")
		all             = postgres.ColumnList{
			id, publicID, orgID, projectID, email, emailVerifiedAt, name, lastSignInAt,
			createdAt, updatedAt,
		}
	)
	return &EndUsers{
		Table:           postgres.NewTable(schema, tablePrefix+"end_users", "end_users", all...),
		ID:              id,
		PublicID:        publicID,
		OrgID:           orgID,
		ProjectID:       projectID,
		Email:           email,
		EmailVerifiedAt: emailVerifiedAt,
		Name:            name,
		LastSignInAt:    lastSignInAt,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
		AllColumns:      all,
	}
}
