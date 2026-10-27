package entity

import "github.com/go-jet/jet/v2/postgres"

// Projects is the jet binding for the projects table.
type Projects struct {
	postgres.Table

	ID                   postgres.ColumnString
	PublicID             postgres.ColumnString
	OrgID                postgres.ColumnString
	Slug                 postgres.ColumnString
	Name                 postgres.ColumnString
	CreatedBy            postgres.ColumnString
	CreatedAt            postgres.ColumnTimestampz
	UpdatedAt            postgres.ColumnTimestampz
	IssuerHandle         postgres.ColumnString
	IssuerHandleLockedAt postgres.ColumnTimestampz
	DisplayName          postgres.ColumnString
	PrimaryColor         postgres.ColumnString
	SupportEmail         postgres.ColumnString
	TermsURL             postgres.ColumnString
	PrivacyURL           postgres.ColumnString
	TermsVersion         postgres.ColumnInteger
	TermsPublishedAt     postgres.ColumnTimestampz
	SignInEmailLink      postgres.ColumnBool
	LogoFileID           postgres.ColumnString

	AllColumns postgres.ColumnList
}

// NewProjects builds the projects binding.
func NewProjects(schema, tablePrefix string) *Projects {
	if schema == "" {
		schema = "public"
	}
	var (
		id                   = postgres.StringColumn("id")
		publicID             = postgres.StringColumn("public_id")
		orgID                = postgres.StringColumn("org_id")
		slug                 = postgres.StringColumn("slug")
		name                 = postgres.StringColumn("name")
		createdBy            = postgres.StringColumn("created_by")
		createdAt            = postgres.TimestampzColumn("created_at")
		updatedAt            = postgres.TimestampzColumn("updated_at")
		issuerHandle         = postgres.StringColumn("issuer_handle")
		issuerHandleLockedAt = postgres.TimestampzColumn("issuer_handle_locked_at")
		displayName          = postgres.StringColumn("display_name")
		primaryColor         = postgres.StringColumn("primary_color")
		supportEmail         = postgres.StringColumn("support_email")
		termsURL             = postgres.StringColumn("terms_url")
		privacyURL           = postgres.StringColumn("privacy_url")
		termsVersion         = postgres.IntegerColumn("terms_version")
		termsPublishedAt     = postgres.TimestampzColumn("terms_published_at")
		signInEmailLink      = postgres.BoolColumn("signin_email_link")
		logoFileID           = postgres.StringColumn("logo_file_id")
		all                  = postgres.ColumnList{
			id, publicID, orgID, slug, name, createdBy, createdAt, updatedAt, issuerHandle, issuerHandleLockedAt,
			displayName, primaryColor, supportEmail, termsURL, privacyURL, termsVersion, termsPublishedAt, signInEmailLink,
			logoFileID,
		}
	)
	return &Projects{
		Table:                postgres.NewTable(schema, tablePrefix+"projects", "projects", all...),
		ID:                   id,
		PublicID:             publicID,
		OrgID:                orgID,
		Slug:                 slug,
		Name:                 name,
		CreatedBy:            createdBy,
		CreatedAt:            createdAt,
		UpdatedAt:            updatedAt,
		IssuerHandle:         issuerHandle,
		IssuerHandleLockedAt: issuerHandleLockedAt,
		DisplayName:          displayName,
		PrimaryColor:         primaryColor,
		SupportEmail:         supportEmail,
		TermsURL:             termsURL,
		PrivacyURL:           privacyURL,
		TermsVersion:         termsVersion,
		TermsPublishedAt:     termsPublishedAt,
		SignInEmailLink:      signInEmailLink,
		LogoFileID:           logoFileID,
		AllColumns:           all,
	}
}
