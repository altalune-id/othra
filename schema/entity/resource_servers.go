package entity

import "github.com/go-jet/jet/v2/postgres"

// ResourceServers is the jet binding for the resource_servers table.
type ResourceServers struct {
	postgres.Table

	ID                  postgres.ColumnString
	PublicID            postgres.ColumnString
	OrgID               postgres.ColumnString
	ProjectID           postgres.ColumnString
	Name                postgres.ColumnString
	Identifier          postgres.ColumnString
	Scopes              postgres.ColumnString
	VerificationState   postgres.ColumnString
	VerifiedAt          postgres.ColumnTimestampz
	LastCheckedAt       postgres.ColumnTimestampz
	LastError           postgres.ColumnString
	ConsecutiveFailures postgres.ColumnInteger
	Trusted             postgres.ColumnBool
	TrustedAt           postgres.ColumnTimestampz
	TrustedBy           postgres.ColumnString
	CreatedAt           postgres.ColumnTimestampz
	UpdatedAt           postgres.ColumnTimestampz

	AllColumns postgres.ColumnList
}

// NewResourceServers builds the resource_servers binding.
func NewResourceServers(schema, tablePrefix string) *ResourceServers {
	if schema == "" {
		schema = "public"
	}
	var (
		id                  = postgres.StringColumn("id")
		publicID            = postgres.StringColumn("public_id")
		orgID               = postgres.StringColumn("org_id")
		projectID           = postgres.StringColumn("project_id")
		name                = postgres.StringColumn("name")
		identifier          = postgres.StringColumn("identifier")
		scopes              = postgres.StringColumn("scopes")
		verificationState   = postgres.StringColumn("verification_state")
		verifiedAt          = postgres.TimestampzColumn("verified_at")
		lastCheckedAt       = postgres.TimestampzColumn("last_checked_at")
		lastError           = postgres.StringColumn("last_error")
		consecutiveFailures = postgres.IntegerColumn("consecutive_failures")
		trusted             = postgres.BoolColumn("trusted")
		trustedAt           = postgres.TimestampzColumn("trusted_at")
		trustedBy           = postgres.StringColumn("trusted_by")
		createdAt           = postgres.TimestampzColumn("created_at")
		updatedAt           = postgres.TimestampzColumn("updated_at")
		all                 = postgres.ColumnList{
			id, publicID, orgID, projectID, name, identifier, scopes, verificationState, verifiedAt,
			lastCheckedAt, lastError, consecutiveFailures, trusted, trustedAt, trustedBy, createdAt, updatedAt,
		}
	)
	return &ResourceServers{
		Table:               postgres.NewTable(schema, tablePrefix+"resource_servers", "resource_servers", all...),
		ID:                  id,
		PublicID:            publicID,
		OrgID:               orgID,
		ProjectID:           projectID,
		Name:                name,
		Identifier:          identifier,
		Scopes:              scopes,
		VerificationState:   verificationState,
		VerifiedAt:          verifiedAt,
		LastCheckedAt:       lastCheckedAt,
		LastError:           lastError,
		ConsecutiveFailures: consecutiveFailures,
		Trusted:             trusted,
		TrustedAt:           trustedAt,
		TrustedBy:           trustedBy,
		CreatedAt:           createdAt,
		UpdatedAt:           updatedAt,
		AllColumns:          all,
	}
}
