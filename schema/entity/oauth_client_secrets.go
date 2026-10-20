package entity

import "github.com/go-jet/jet/v2/postgres"

// OAuthClientSecrets is the jet binding for the oauth_client_secrets table.
type OAuthClientSecrets struct {
	postgres.Table

	ID         postgres.ColumnString
	PublicID   postgres.ColumnString
	OrgID      postgres.ColumnString
	ProjectID  postgres.ColumnString
	ClientID   postgres.ColumnString
	SecretHash postgres.ColumnString
	Hint       postgres.ColumnString
	CreatedAt  postgres.ColumnTimestampz
	ExpiresAt  postgres.ColumnTimestampz

	AllColumns postgres.ColumnList
}

// NewOAuthClientSecrets builds the oauth_client_secrets binding.
func NewOAuthClientSecrets(schema, tablePrefix string) *OAuthClientSecrets {
	if schema == "" {
		schema = "public"
	}
	var (
		id         = postgres.StringColumn("id")
		publicID   = postgres.StringColumn("public_id")
		orgID      = postgres.StringColumn("org_id")
		projectID  = postgres.StringColumn("project_id")
		clientID   = postgres.StringColumn("client_id")
		secretHash = postgres.StringColumn("secret_hash")
		hint       = postgres.StringColumn("hint")
		createdAt  = postgres.TimestampzColumn("created_at")
		expiresAt  = postgres.TimestampzColumn("expires_at")
		all        = postgres.ColumnList{
			id, publicID, orgID, projectID, clientID, secretHash, hint, createdAt, expiresAt,
		}
	)
	return &OAuthClientSecrets{
		Table:      postgres.NewTable(schema, tablePrefix+"oauth_client_secrets", "oauth_client_secrets", all...),
		ID:         id,
		PublicID:   publicID,
		OrgID:      orgID,
		ProjectID:  projectID,
		ClientID:   clientID,
		SecretHash: secretHash,
		Hint:       hint,
		CreatedAt:  createdAt,
		ExpiresAt:  expiresAt,
		AllColumns: all,
	}
}
