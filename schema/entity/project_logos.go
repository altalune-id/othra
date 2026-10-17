package entity

import "github.com/go-jet/jet/v2/postgres"

// ProjectLogos is the jet binding for the project_logos table.
type ProjectLogos struct {
	postgres.Table

	ProjectID   postgres.ColumnString
	OrgID       postgres.ColumnString
	ContentType postgres.ColumnString
	Content     postgres.ColumnBytea
	SHA256      postgres.ColumnString
	UpdatedAt   postgres.ColumnTimestampz

	AllColumns postgres.ColumnList
}

// NewProjectLogos builds the project_logos binding.
func NewProjectLogos(schema, tablePrefix string) *ProjectLogos {
	if schema == "" {
		schema = "public"
	}
	var (
		projectID   = postgres.StringColumn("project_id")
		orgID       = postgres.StringColumn("org_id")
		contentType = postgres.StringColumn("content_type")
		content     = postgres.ByteaColumn("content")
		sha         = postgres.StringColumn("sha256")
		updatedAt   = postgres.TimestampzColumn("updated_at")
		all         = postgres.ColumnList{projectID, orgID, contentType, content, sha, updatedAt}
	)
	return &ProjectLogos{
		Table:       postgres.NewTable(schema, tablePrefix+"project_logos", "project_logos", all...),
		ProjectID:   projectID,
		OrgID:       orgID,
		ContentType: contentType,
		Content:     content,
		SHA256:      sha,
		UpdatedAt:   updatedAt,
		AllColumns:  all,
	}
}
