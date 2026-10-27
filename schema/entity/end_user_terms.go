package entity

import "github.com/go-jet/jet/v2/postgres"

// EndUserTerms is the jet binding for the end_user_terms table.
type EndUserTerms struct {
	postgres.Table

	OrgID               postgres.ColumnString
	ProjectID           postgres.ColumnString
	EndUserID           postgres.ColumnString
	TermsVersion        postgres.ColumnInteger
	TermsURL            postgres.ColumnString
	PrivacyURL          postgres.ColumnString
	AcceptedAt          postgres.ColumnTimestampz
	UserSessionID       postgres.ColumnString
	UserSessionPublicID postgres.ColumnString
	IP                  postgres.ColumnString
	UserAgent           postgres.ColumnString

	AllColumns postgres.ColumnList
}

// NewEndUserTerms builds the end_user_terms binding.
func NewEndUserTerms(schema, tablePrefix string) *EndUserTerms {
	if schema == "" {
		schema = "public"
	}
	var (
		orgID               = postgres.StringColumn("org_id")
		projectID           = postgres.StringColumn("project_id")
		endUserID           = postgres.StringColumn("end_user_id")
		termsVersion        = postgres.IntegerColumn("terms_version")
		termsURL            = postgres.StringColumn("terms_url")
		privacyURL          = postgres.StringColumn("privacy_url")
		acceptedAt          = postgres.TimestampzColumn("accepted_at")
		userSessionID       = postgres.StringColumn("user_session_id")
		userSessionPublicID = postgres.StringColumn("user_session_public_id")
		ip                  = postgres.StringColumn("ip")
		userAgent           = postgres.StringColumn("user_agent")
		all                 = postgres.ColumnList{
			orgID, projectID, endUserID, termsVersion, termsURL, privacyURL, acceptedAt, userSessionID,
			userSessionPublicID, ip, userAgent,
		}
	)
	return &EndUserTerms{
		Table:               postgres.NewTable(schema, tablePrefix+"end_user_terms", "end_user_terms", all...),
		OrgID:               orgID,
		ProjectID:           projectID,
		EndUserID:           endUserID,
		TermsVersion:        termsVersion,
		TermsURL:            termsURL,
		PrivacyURL:          privacyURL,
		AcceptedAt:          acceptedAt,
		UserSessionID:       userSessionID,
		UserSessionPublicID: userSessionPublicID,
		IP:                  ip,
		UserAgent:           userAgent,
		AllColumns:          all,
	}
}
