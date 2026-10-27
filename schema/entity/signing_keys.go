package entity

import "github.com/go-jet/jet/v2/postgres"

// SigningKeys is the jet binding for the signing_keys table.
type SigningKeys struct {
	postgres.Table

	ID            postgres.ColumnString
	PublicID      postgres.ColumnString
	OrgID         postgres.ColumnString
	ProjectID     postgres.ColumnString
	Kid           postgres.ColumnString
	Alg           postgres.ColumnString
	PublicJWK     postgres.ColumnString
	PrivateKeyEnc postgres.ColumnBytea
	Status        postgres.ColumnString
	CreatedAt     postgres.ColumnTimestampz
	ActivatesAt   postgres.ColumnTimestampz
	ActivatedAt   postgres.ColumnTimestampz
	RetireAfter   postgres.ColumnTimestampz
	RetiredAt     postgres.ColumnTimestampz

	AllColumns postgres.ColumnList
}

// NewSigningKeys builds the signing_keys binding.
func NewSigningKeys(schema, tablePrefix string) *SigningKeys {
	if schema == "" {
		schema = "public"
	}
	var (
		id            = postgres.StringColumn("id")
		publicID      = postgres.StringColumn("public_id")
		orgID         = postgres.StringColumn("org_id")
		projectID     = postgres.StringColumn("project_id")
		kid           = postgres.StringColumn("kid")
		alg           = postgres.StringColumn("alg")
		publicJWK     = postgres.StringColumn("public_jwk")
		privateKeyEnc = postgres.ByteaColumn("private_key_enc")
		status        = postgres.StringColumn("status")
		createdAt     = postgres.TimestampzColumn("created_at")
		activatesAt   = postgres.TimestampzColumn("activates_at")
		activatedAt   = postgres.TimestampzColumn("activated_at")
		retireAfter   = postgres.TimestampzColumn("retire_after")
		retiredAt     = postgres.TimestampzColumn("retired_at")
		all           = postgres.ColumnList{
			id, publicID, orgID, projectID, kid, alg, publicJWK, privateKeyEnc, status,
			createdAt, activatesAt, activatedAt, retireAfter, retiredAt,
		}
	)
	return &SigningKeys{
		Table:         postgres.NewTable(schema, tablePrefix+"signing_keys", "signing_keys", all...),
		ID:            id,
		PublicID:      publicID,
		OrgID:         orgID,
		ProjectID:     projectID,
		Kid:           kid,
		Alg:           alg,
		PublicJWK:     publicJWK,
		PrivateKeyEnc: privateKeyEnc,
		Status:        status,
		CreatedAt:     createdAt,
		ActivatesAt:   activatesAt,
		ActivatedAt:   activatedAt,
		RetireAfter:   retireAfter,
		RetiredAt:     retiredAt,
		AllColumns:    all,
	}
}
