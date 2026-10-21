package entity

import "github.com/go-jet/jet/v2/postgres"

// UserSessions is the jet binding for the user_sessions table.
type UserSessions struct {
	postgres.Table

	ID               postgres.ColumnString
	PublicID         postgres.ColumnString
	OrgID            postgres.ColumnString
	ProjectID        postgres.ColumnString
	BrowserSessionID postgres.ColumnString
	EndUserID        postgres.ColumnString
	Method           postgres.ColumnString
	AuthTime         postgres.ColumnTimestampz
	SelectedAt       postgres.ColumnTimestampz
	CreatedAt        postgres.ColumnTimestampz
	LastSeenAt       postgres.ColumnTimestampz
	EndedAt          postgres.ColumnTimestampz

	AllColumns postgres.ColumnList
}

// NewUserSessions builds the user_sessions binding.
func NewUserSessions(schema, tablePrefix string) *UserSessions {
	if schema == "" {
		schema = "public"
	}
	var (
		id               = postgres.StringColumn("id")
		publicID         = postgres.StringColumn("public_id")
		orgID            = postgres.StringColumn("org_id")
		projectID        = postgres.StringColumn("project_id")
		browserSessionID = postgres.StringColumn("browser_session_id")
		endUserID        = postgres.StringColumn("end_user_id")
		method           = postgres.StringColumn("method")
		authTime         = postgres.TimestampzColumn("auth_time")
		selectedAt       = postgres.TimestampzColumn("selected_at")
		createdAt        = postgres.TimestampzColumn("created_at")
		lastSeenAt       = postgres.TimestampzColumn("last_seen_at")
		endedAt          = postgres.TimestampzColumn("ended_at")
		all              = postgres.ColumnList{
			id, publicID, orgID, projectID, browserSessionID, endUserID, method, authTime,
			selectedAt, createdAt, lastSeenAt, endedAt,
		}
	)
	return &UserSessions{
		Table:            postgres.NewTable(schema, tablePrefix+"user_sessions", "user_sessions", all...),
		ID:               id,
		PublicID:         publicID,
		OrgID:            orgID,
		ProjectID:        projectID,
		BrowserSessionID: browserSessionID,
		EndUserID:        endUserID,
		Method:           method,
		AuthTime:         authTime,
		SelectedAt:       selectedAt,
		CreatedAt:        createdAt,
		LastSeenAt:       lastSeenAt,
		EndedAt:          endedAt,
		AllColumns:       all,
	}
}
