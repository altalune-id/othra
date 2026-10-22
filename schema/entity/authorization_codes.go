package entity

import "github.com/go-jet/jet/v2/postgres"

// AuthorizationCodes is the jet binding for the authorization_codes table.
type AuthorizationCodes struct {
	postgres.Table

	ID                  postgres.ColumnString
	OrgID               postgres.ColumnString
	ProjectID           postgres.ColumnString
	CodeHash            postgres.ColumnString
	ClientID            postgres.ColumnString
	EndUserID           postgres.ColumnString
	UserSessionID       postgres.ColumnString
	RedirectURI         postgres.ColumnString
	Scopes              postgres.ColumnString
	CodeChallenge       postgres.ColumnString
	CodeChallengeMethod postgres.ColumnString
	Nonce               postgres.ColumnString
	AuthTime            postgres.ColumnTimestampz
	CreatedAt           postgres.ColumnTimestampz
	ExpiresAt           postgres.ColumnTimestampz
	UsedAt              postgres.ColumnTimestampz
	FamilyID            postgres.ColumnString
	ResourceServerID    postgres.ColumnString

	AllColumns postgres.ColumnList
}

// NewAuthorizationCodes builds the authorization_codes binding.
func NewAuthorizationCodes(schema, tablePrefix string) *AuthorizationCodes {
	if schema == "" {
		schema = "public"
	}
	var (
		id                  = postgres.StringColumn("id")
		orgID               = postgres.StringColumn("org_id")
		projectID           = postgres.StringColumn("project_id")
		codeHash            = postgres.StringColumn("code_hash")
		clientID            = postgres.StringColumn("client_id")
		endUserID           = postgres.StringColumn("end_user_id")
		userSessionID       = postgres.StringColumn("user_session_id")
		redirectURI         = postgres.StringColumn("redirect_uri")
		scopes              = postgres.StringColumn("scopes")
		codeChallenge       = postgres.StringColumn("code_challenge")
		codeChallengeMethod = postgres.StringColumn("code_challenge_method")
		nonce               = postgres.StringColumn("nonce")
		authTime            = postgres.TimestampzColumn("auth_time")
		createdAt           = postgres.TimestampzColumn("created_at")
		expiresAt           = postgres.TimestampzColumn("expires_at")
		usedAt              = postgres.TimestampzColumn("used_at")
		familyID            = postgres.StringColumn("family_id")
		resourceServerID    = postgres.StringColumn("resource_server_id")
		all                 = postgres.ColumnList{
			id, orgID, projectID, codeHash, clientID, endUserID, userSessionID, redirectURI,
			scopes, codeChallenge, codeChallengeMethod, nonce, authTime, createdAt, expiresAt, usedAt,
			familyID, resourceServerID,
		}
	)
	return &AuthorizationCodes{
		Table:               postgres.NewTable(schema, tablePrefix+"authorization_codes", "authorization_codes", all...),
		ID:                  id,
		OrgID:               orgID,
		ProjectID:           projectID,
		CodeHash:            codeHash,
		ClientID:            clientID,
		EndUserID:           endUserID,
		UserSessionID:       userSessionID,
		RedirectURI:         redirectURI,
		Scopes:              scopes,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
		Nonce:               nonce,
		AuthTime:            authTime,
		CreatedAt:           createdAt,
		ExpiresAt:           expiresAt,
		UsedAt:              usedAt,
		FamilyID:            familyID,
		ResourceServerID:    resourceServerID,
		AllColumns:          all,
	}
}
