package entity

import "github.com/go-jet/jet/v2/postgres"

// ResourceServerSecrets is the jet binding for the resource_server_secrets table.
type ResourceServerSecrets struct {
	postgres.Table

	ID               postgres.ColumnString
	PublicID         postgres.ColumnString
	OrgID            postgres.ColumnString
	ProjectID        postgres.ColumnString
	ResourceServerID postgres.ColumnString
	SecretHash       postgres.ColumnString
	Hint             postgres.ColumnString
	CreatedAt        postgres.ColumnTimestampz
	ExpiresAt        postgres.ColumnTimestampz

	AllColumns postgres.ColumnList
}

// NewResourceServerSecrets builds the resource_server_secrets binding.
func NewResourceServerSecrets(schema, tablePrefix string) *ResourceServerSecrets {
	if schema == "" {
		schema = "public"
	}
	var (
		id               = postgres.StringColumn("id")
		publicID         = postgres.StringColumn("public_id")
		orgID            = postgres.StringColumn("org_id")
		projectID        = postgres.StringColumn("project_id")
		resourceServerID = postgres.StringColumn("resource_server_id")
		secretHash       = postgres.StringColumn("secret_hash")
		hint             = postgres.StringColumn("hint")
		createdAt        = postgres.TimestampzColumn("created_at")
		expiresAt        = postgres.TimestampzColumn("expires_at")
		all              = postgres.ColumnList{
			id, publicID, orgID, projectID, resourceServerID, secretHash, hint, createdAt,
			expiresAt,
		}
	)
	return &ResourceServerSecrets{
		Table:            postgres.NewTable(schema, tablePrefix+"resource_server_secrets", "resource_server_secrets", all...),
		ID:               id,
		PublicID:         publicID,
		OrgID:            orgID,
		ProjectID:        projectID,
		ResourceServerID: resourceServerID,
		SecretHash:       secretHash,
		Hint:             hint,
		CreatedAt:        createdAt,
		ExpiresAt:        expiresAt,
		AllColumns:       all,
	}
}
