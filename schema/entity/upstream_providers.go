package entity

import "github.com/go-jet/jet/v2/postgres"

// UpstreamProviders is the jet binding for the upstream_providers table.
type UpstreamProviders struct {
	postgres.Table

	ID                 postgres.ColumnString
	PublicID           postgres.ColumnString
	OrgID              postgres.ColumnString
	ProjectID          postgres.ColumnString
	Kind               postgres.ColumnString
	ClientID           postgres.ColumnString
	ClientSecretSealed postgres.ColumnString
	Enabled            postgres.ColumnBool
	CreatedAt          postgres.ColumnTimestampz
	UpdatedAt          postgres.ColumnTimestampz

	AllColumns postgres.ColumnList
}

// NewUpstreamProviders builds the upstream_providers binding.
func NewUpstreamProviders(schema, tablePrefix string) *UpstreamProviders {
	if schema == "" {
		schema = "public"
	}
	var (
		id                 = postgres.StringColumn("id")
		publicID           = postgres.StringColumn("public_id")
		orgID              = postgres.StringColumn("org_id")
		projectID          = postgres.StringColumn("project_id")
		kind               = postgres.StringColumn("kind")
		clientID           = postgres.StringColumn("client_id")
		clientSecretSealed = postgres.StringColumn("client_secret_sealed")
		enabled            = postgres.BoolColumn("enabled")
		createdAt          = postgres.TimestampzColumn("created_at")
		updatedAt          = postgres.TimestampzColumn("updated_at")
		all                = postgres.ColumnList{
			id, publicID, orgID, projectID, kind, clientID, clientSecretSealed, enabled,
			createdAt, updatedAt,
		}
	)
	return &UpstreamProviders{
		Table:              postgres.NewTable(schema, tablePrefix+"upstream_providers", "upstream_providers", all...),
		ID:                 id,
		PublicID:           publicID,
		OrgID:              orgID,
		ProjectID:          projectID,
		Kind:               kind,
		ClientID:           clientID,
		ClientSecretSealed: clientSecretSealed,
		Enabled:            enabled,
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
		AllColumns:         all,
	}
}
