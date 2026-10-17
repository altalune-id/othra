package entity

import "github.com/go-jet/jet/v2/postgres"

// Files is the jet binding for the files table.
type Files struct {
	postgres.Table

	ID           postgres.ColumnString
	PublicID     postgres.ColumnString
	OrgID        postgres.ColumnString
	OwnerKind    postgres.ColumnString
	OwnerID      postgres.ColumnString
	Purpose      postgres.ColumnString
	Visibility   postgres.ColumnString
	ObjectKey    postgres.ColumnString
	ContentType  postgres.ColumnString
	SizeBytes    postgres.ColumnInteger
	OriginalName postgres.ColumnString
	Status       postgres.ColumnString
	CreatedAt    postgres.ColumnTimestampz
	CompletedAt  postgres.ColumnTimestampz
	ExpiresAt    postgres.ColumnTimestampz

	AllColumns postgres.ColumnList
}

// NewFiles builds the files binding.
func NewFiles(schema, tablePrefix string) *Files {
	if schema == "" {
		schema = "public"
	}
	var (
		id           = postgres.StringColumn("id")
		publicID     = postgres.StringColumn("public_id")
		orgID        = postgres.StringColumn("org_id")
		ownerKind    = postgres.StringColumn("owner_kind")
		ownerID      = postgres.StringColumn("owner_id")
		purpose      = postgres.StringColumn("purpose")
		visibility   = postgres.StringColumn("visibility")
		objectKey    = postgres.StringColumn("object_key")
		contentType  = postgres.StringColumn("content_type")
		sizeBytes    = postgres.IntegerColumn("size_bytes")
		originalName = postgres.StringColumn("original_name")
		status       = postgres.StringColumn("status")
		createdAt    = postgres.TimestampzColumn("created_at")
		completedAt  = postgres.TimestampzColumn("completed_at")
		expiresAt    = postgres.TimestampzColumn("expires_at")
		all          = postgres.ColumnList{
			id, publicID, orgID, ownerKind, ownerID, purpose, visibility, objectKey, contentType,
			sizeBytes, originalName, status, createdAt, completedAt, expiresAt,
		}
	)
	return &Files{
		Table:        postgres.NewTable(schema, tablePrefix+"files", "files", all...),
		ID:           id,
		PublicID:     publicID,
		OrgID:        orgID,
		OwnerKind:    ownerKind,
		OwnerID:      ownerID,
		Purpose:      purpose,
		Visibility:   visibility,
		ObjectKey:    objectKey,
		ContentType:  contentType,
		SizeBytes:    sizeBytes,
		OriginalName: originalName,
		Status:       status,
		CreatedAt:    createdAt,
		CompletedAt:  completedAt,
		ExpiresAt:    expiresAt,
		AllColumns:   all,
	}
}
