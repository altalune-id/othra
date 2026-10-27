package enduser

import (
	"context"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/db"
)

func (s *postgresStore) linkColumns() postgres.ProjectionList {
	t := s.links
	return postgres.ProjectionList{
		t.ID, t.PublicID, t.Email, t.ReturnTo, t.CreatedAt, t.ExpiresAt, t.UsedAt, t.BrowserSessionID, t.PendingIdentity,
	}
}

// HasRecentLink reports whether email has an unused link created after since.
func (s *postgresStore) HasRecentLink(ctx context.Context, projectID uuid.UUID, email string, since time.Time) (bool, error) {
	t := s.links
	stmt := postgres.SELECT(t.ID).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.Email.EQ(postgres.String(email))).
			AND(t.UsedAt.IS_NULL()).AND(t.CreatedAt.GT(postgres.TimestampzT(since.UTC())))).
		LIMIT(1)
	var rows []struct {
		ID uuid.UUID `alias:"magic_links.id"`
	}
	if err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &rows) }); err != nil && !db.IsNoRows(err) {
		return false, wrap("HasRecentLink", err)
	}
	return len(rows) > 0, nil
}

// LinkByTokenHash returns the link with tokenHash, or *LinkNotUsableError.
func (s *postgresStore) LinkByTokenHash(ctx context.Context, projectID uuid.UUID, tokenHash string) (*Link, error) {
	return s.link(ctx, "LinkByTokenHash", projectID, s.links.TokenHash.EQ(postgres.String(tokenHash)))
}

// LinkByPublicID returns the link publicID, or *LinkNotUsableError.
func (s *postgresStore) LinkByPublicID(ctx context.Context, projectID uuid.UUID, publicID string) (*Link, error) {
	return s.link(ctx, "LinkByPublicID", projectID, s.links.PublicID.EQ(postgres.String(publicID)))
}

func (s *postgresStore) link(ctx context.Context, op string, projectID uuid.UUID, match postgres.BoolExpression) (*Link, error) {
	t := s.links
	stmt := postgres.SELECT(s.linkColumns()).FROM(t).WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(match))
	var row linkRow
	err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) })
	switch {
	case db.IsNoRows(err):
		return nil, &LinkNotUsableError{}
	case err != nil:
		return nil, wrap(op, err)
	}
	return row.toLink(), nil
}

// BrowserBySecretHash returns the browser session with secretHash that has not ended, or nil.
func (s *postgresStore) BrowserBySecretHash(ctx context.Context, projectID uuid.UUID, secretHash string) (*Browser, error) {
	t := s.browsers
	stmt := postgres.SELECT(t.ID, t.CreatedAt, t.LastSeenAt, t.AbsoluteExpiresAt).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.SecretHash.EQ(postgres.String(secretHash))).AND(t.EndedAt.IS_NULL()))
	var row browserRow
	err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) })
	switch {
	case db.IsNoRows(err):
		return nil, nil
	case err != nil:
		return nil, wrap("BrowserBySecretHash", err)
	}
	return &Browser{ID: row.ID, CreatedAt: row.CreatedAt.UTC(), LastSeenAt: row.LastSeenAt.UTC(), AbsoluteEnd: row.AbsoluteEnd.UTC()}, nil
}

// Accounts returns the browser's live accounts, the most recently selected first.
func (s *postgresStore) Accounts(ctx context.Context, projectID, browserID uuid.UUID) ([]Account, error) {
	us, eu := s.sessions, s.users
	stmt := postgres.SELECT(us.ID, us.PublicID, us.AuthTime, us.SelectedAt, us.Method, eu.ID, eu.PublicID, eu.Email, eu.Name).
		FROM(us.INNER_JOIN(eu, eu.ID.EQ(us.EndUserID).AND(eu.ProjectID.EQ(us.ProjectID)))).
		WHERE(us.ProjectID.EQ(postgres.UUID(projectID)).AND(us.BrowserSessionID.EQ(postgres.UUID(browserID))).AND(us.EndedAt.IS_NULL())).
		ORDER_BY(us.SelectedAt.DESC(), us.ID.ASC())
	var rows []accountRow
	if err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &rows) }); err != nil && !db.IsNoRows(err) {
		return nil, wrap("Accounts", err)
	}
	out := make([]Account, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].toAccount())
	}
	return out, nil
}

// LatestTerms returns the highest terms version endUserID accepted, or 0.
func (s *postgresStore) LatestTerms(ctx context.Context, projectID, endUserID uuid.UUID) (int, error) {
	t := s.terms
	stmt := postgres.SELECT(t.TermsVersion).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.EndUserID.EQ(postgres.UUID(endUserID)))).
		ORDER_BY(t.TermsVersion.DESC()).LIMIT(1)
	var rows []struct {
		Version int `alias:"end_user_terms.terms_version"`
	}
	if err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &rows) }); err != nil && !db.IsNoRows(err) {
		return 0, wrap("LatestTerms", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return rows[0].Version, nil
}

// EndUserExists reports whether the project has an end user with email.
func (s *postgresStore) EndUserExists(ctx context.Context, projectID uuid.UUID, email string) (bool, error) {
	t := s.users
	stmt := postgres.SELECT(t.ID).FROM(t).WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.Email.EQ(postgres.String(email))))
	var rows []struct {
		ID uuid.UUID `alias:"end_users.id"`
	}
	if err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &rows) }); err != nil && !db.IsNoRows(err) {
		return false, wrap("EndUserExists", err)
	}
	return len(rows) > 0, nil
}

// EndUserByID returns the end user id, or nil.
func (s *postgresStore) EndUserByID(ctx context.Context, projectID, id uuid.UUID) (*EndUser, error) {
	t := s.users
	stmt := postgres.SELECT(s.endUserColumns()).FROM(t).WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.ID.EQ(postgres.UUID(id))))
	var row endUserRow
	err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) })
	switch {
	case db.IsNoRows(err):
		return nil, nil
	case err != nil:
		return nil, wrap("EndUserByID", err)
	}
	return row.toEndUser(), nil
}

// ListEndUsers returns up to limit end users, newest first, after the one with public id after, whose email starts with prefix.
func (s *postgresStore) ListEndUsers(ctx context.Context, projectID uuid.UUID, prefix, after string, limit int) ([]*EndUser, error) {
	t := s.users
	where := t.ProjectID.EQ(postgres.UUID(projectID))
	if prefix != "" {
		where = where.AND(t.Email.LIKE(postgres.String(escapeLike(prefix) + "%")))
	}
	if after != "" {
		cur, err := s.EndUserByPublicID(ctx, projectID, after)
		if err != nil || cur == nil {
			return nil, err
		}
		at := postgres.TimestampzT(cur.CreatedAt)
		where = where.AND(t.CreatedAt.LT(at).OR(t.CreatedAt.EQ(at).AND(t.ID.GT(postgres.UUID(cur.ID)))))
	}
	stmt := postgres.SELECT(s.endUserColumns()).FROM(t).WHERE(where).ORDER_BY(t.CreatedAt.DESC(), t.ID.ASC()).LIMIT(int64(limit))
	var rows []endUserRow
	if err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &rows) }); err != nil && !db.IsNoRows(err) {
		return nil, wrap("ListEndUsers", err)
	}
	out := make([]*EndUser, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].toEndUser())
	}
	return out, nil
}

// EndUserByPublicID returns the end user publicID, or nil.
func (s *postgresStore) EndUserByPublicID(ctx context.Context, projectID uuid.UUID, publicID string) (*EndUser, error) {
	t := s.users
	stmt := postgres.SELECT(s.endUserColumns()).FROM(t).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.PublicID.EQ(postgres.String(publicID))))
	var row endUserRow
	err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) })
	switch {
	case db.IsNoRows(err):
		return nil, nil
	case err != nil:
		return nil, wrap("EndUserByPublicID", err)
	}
	return row.toEndUser(), nil
}

// LiveSessions returns the end user's live user sessions on every browser, the most recently seen first.
func (s *postgresStore) LiveSessions(ctx context.Context, projectID, endUserID uuid.UUID) ([]LiveSession, error) {
	us, b := s.sessions, s.browsers
	stmt := postgres.SELECT(us.ID, us.PublicID, us.Method, us.AuthTime, us.LastSeenAt, b.UserAgent).
		FROM(us.INNER_JOIN(b, b.ID.EQ(us.BrowserSessionID).AND(b.ProjectID.EQ(us.ProjectID)))).
		WHERE(us.ProjectID.EQ(postgres.UUID(projectID)).AND(us.EndUserID.EQ(postgres.UUID(endUserID))).AND(us.EndedAt.IS_NULL()).
			AND(b.EndedAt.IS_NULL())).
		ORDER_BY(us.LastSeenAt.DESC(), us.ID.ASC())
	var rows []struct {
		ID         uuid.UUID `alias:"user_sessions.id"`
		PublicID   string    `alias:"user_sessions.public_id"`
		Method     string    `alias:"user_sessions.method"`
		AuthTime   time.Time `alias:"user_sessions.auth_time"`
		LastSeenAt time.Time `alias:"user_sessions.last_seen_at"`
		UserAgent  string    `alias:"browser_sessions.user_agent"`
	}
	if err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &rows) }); err != nil && !db.IsNoRows(err) {
		return nil, wrap("LiveSessions", err)
	}
	out := make([]LiveSession, 0, len(rows))
	for _, r := range rows {
		out = append(out, LiveSession{
			PublicID: r.PublicID, Method: r.Method, UserAgent: r.UserAgent, AuthTime: r.AuthTime.UTC(), LastSeenAt: r.LastSeenAt.UTC(),
		})
	}
	return out, nil
}

// EndUserIDByEmail returns the id of the project's end user with email; ok is false when there is none.
func (s *postgresStore) EndUserIDByEmail(ctx context.Context, projectID uuid.UUID, email string) (uuid.UUID, bool, error) {
	t := s.users
	stmt := postgres.SELECT(t.ID).FROM(t).WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.Email.EQ(postgres.String(email))))
	var row struct {
		ID uuid.UUID `alias:"end_users.id"`
	}
	err := db.ReadPrimary(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) })
	switch {
	case db.IsNoRows(err):
		return uuid.Nil, false, nil
	case err != nil:
		return uuid.Nil, false, wrap("EndUserIDByEmail", err)
	}
	return row.ID, true, nil
}
