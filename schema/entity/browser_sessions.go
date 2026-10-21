package entity

import "github.com/go-jet/jet/v2/postgres"

// BrowserSessions is the jet binding for the browser_sessions table.
type BrowserSessions struct {
	postgres.Table

	ID                postgres.ColumnString
	OrgID             postgres.ColumnString
	ProjectID         postgres.ColumnString
	SecretHash        postgres.ColumnString
	UserAgent         postgres.ColumnString
	CreatedAt         postgres.ColumnTimestampz
	LastSeenAt        postgres.ColumnTimestampz
	AbsoluteExpiresAt postgres.ColumnTimestampz
	EndedAt           postgres.ColumnTimestampz

	AllColumns postgres.ColumnList
}

// NewBrowserSessions builds the browser_sessions binding.
func NewBrowserSessions(schema, tablePrefix string) *BrowserSessions {
	if schema == "" {
		schema = "public"
	}
	var (
		id                = postgres.StringColumn("id")
		orgID             = postgres.StringColumn("org_id")
		projectID         = postgres.StringColumn("project_id")
		secretHash        = postgres.StringColumn("secret_hash")
		userAgent         = postgres.StringColumn("user_agent")
		createdAt         = postgres.TimestampzColumn("created_at")
		lastSeenAt        = postgres.TimestampzColumn("last_seen_at")
		absoluteExpiresAt = postgres.TimestampzColumn("absolute_expires_at")
		endedAt           = postgres.TimestampzColumn("ended_at")
		all               = postgres.ColumnList{
			id, orgID, projectID, secretHash, userAgent, createdAt, lastSeenAt, absoluteExpiresAt,
			endedAt,
		}
	)
	return &BrowserSessions{
		Table:             postgres.NewTable(schema, tablePrefix+"browser_sessions", "browser_sessions", all...),
		ID:                id,
		OrgID:             orgID,
		ProjectID:         projectID,
		SecretHash:        secretHash,
		UserAgent:         userAgent,
		CreatedAt:         createdAt,
		LastSeenAt:        lastSeenAt,
		AbsoluteExpiresAt: absoluteExpiresAt,
		EndedAt:           endedAt,
		AllColumns:        all,
	}
}
