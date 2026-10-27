package enduser

import (
	"context"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/nanoid"
)

// DeleteUnusedLinks removes the unused links of email created at or before before.
func (s *postgresStore) DeleteUnusedLinks(ctx context.Context, projectID uuid.UUID, email string, before time.Time) error {
	t := s.links
	stmt := t.DELETE().WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.Email.EQ(postgres.String(email))).AND(t.UsedAt.IS_NULL()).
		AND(t.CreatedAt.LT_EQ(postgres.TimestampzT(before.UTC()))))
	_, err := s.exec(ctx, "DeleteUnusedLinks", stmt)
	return err
}

// InsertLink records l by the hash of its token.
func (s *postgresStore) InsertLink(ctx context.Context, orgID, projectID uuid.UUID, l *Link, tokenHash, ip string) error {
	t := s.links
	stmt := t.INSERT(
		t.ID, t.PublicID, t.OrgID, t.ProjectID, t.TokenHash, t.Email, t.ReturnTo, t.RequestIP, t.CreatedAt, t.ExpiresAt, t.PendingIdentity,
	).VALUES(
		l.ID, l.PublicID, orgID, projectID, tokenHash, l.Email, l.ReturnTo, inet(ip), l.CreatedAt.UTC(), l.ExpiresAt.UTC(),
		pendingValue(l.Pending),
	)
	_, err := s.exec(ctx, "InsertLink", stmt)
	if name, ok := db.ViolatedConstraint(err); ok && name == s.liveLink {
		return ErrLiveLink
	}
	return err
}

// UseLink marks the link with tokenHash used by browserID at now when it is unused and unexpired, or returns *LinkNotUsableError.
func (s *postgresStore) UseLink(
	ctx context.Context, projectID uuid.UUID, tokenHash string, browserID uuid.UUID, now time.Time,
) (*Link, error) {
	t := s.links
	stmt := t.UPDATE(t.UsedAt, t.BrowserSessionID).SET(postgres.TimestampzT(now.UTC()), postgres.UUID(browserID)).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.TokenHash.EQ(postgres.String(tokenHash))).
			AND(t.UsedAt.IS_NULL()).AND(t.ExpiresAt.GT(postgres.TimestampzT(now.UTC())))).
		RETURNING(s.linkColumns())
	var row linkRow
	err := db.Write(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) })
	switch {
	case db.IsNoRows(err):
		return nil, &LinkNotUsableError{}
	case err != nil:
		return nil, wrap("UseLink", err)
	}
	return row.toLink(), nil
}

// InsertBrowser records b by the hash of its secret.
func (s *postgresStore) InsertBrowser(ctx context.Context, orgID, projectID uuid.UUID, b *Browser, secretHash, userAgent string) error {
	t := s.browsers
	stmt := t.INSERT(t.ID, t.OrgID, t.ProjectID, t.SecretHash, t.UserAgent, t.CreatedAt, t.LastSeenAt, t.AbsoluteExpiresAt).
		VALUES(b.ID, orgID, projectID, secretHash, userAgent, b.CreatedAt.UTC(), b.LastSeenAt.UTC(), b.AbsoluteEnd.UTC())
	_, err := s.exec(ctx, "InsertBrowser", stmt)
	return err
}

// RotateBrowser replaces b's secret hash and marks it seen at now.
func (s *postgresStore) RotateBrowser(ctx context.Context, projectID, browserID uuid.UUID, secretHash string, now time.Time) error {
	t := s.browsers
	stmt := t.UPDATE(t.SecretHash, t.LastSeenAt).SET(postgres.String(secretHash), postgres.TimestampzT(now.UTC())).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.ID.EQ(postgres.UUID(browserID))))
	_, err := s.exec(ctx, "RotateBrowser", stmt)
	return err
}

// TouchBrowser marks the browser seen at now.
func (s *postgresStore) TouchBrowser(ctx context.Context, projectID, browserID uuid.UUID, now time.Time) error {
	t := s.browsers
	stmt := t.UPDATE(t.LastSeenAt).SET(postgres.TimestampzT(now.UTC())).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.ID.EQ(postgres.UUID(browserID))))
	_, err := s.exec(ctx, "TouchBrowser", stmt)
	return err
}

// SaveEndUser finds or creates the end user with email, verifying it and recording a sign-in at now.
func (s *postgresStore) SaveEndUser(ctx context.Context, orgID, projectID uuid.UUID, email string, now time.Time) (uuid.UUID, error) {
	publicID, err := nanoid.New(publicIDLen)
	if err != nil {
		return uuid.Nil, err
	}
	t := s.users
	at := postgres.TimestampzT(now.UTC())
	stmt := t.INSERT(t.ID, t.PublicID, t.OrgID, t.ProjectID, t.Email, t.EmailVerifiedAt, t.LastSignInAt, t.CreatedAt, t.UpdatedAt).
		VALUES(uuid.New(), publicID, orgID, projectID, email, at, at, at, at).
		ON_CONFLICT(t.ProjectID, t.Email).DO_UPDATE(postgres.SET(
		t.EmailVerifiedAt.SET(postgres.TimestampzExp(postgres.COALESCE(t.EmailVerifiedAt, at))),
		t.LastSignInAt.SET(at),
		t.UpdatedAt.SET(at),
	)).
		RETURNING(t.ID)
	var row struct {
		ID uuid.UUID `alias:"end_users.id"`
	}
	if err := db.Write(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) }); err != nil {
		return uuid.Nil, wrap("SaveEndUser", err)
	}
	return row.ID, nil
}

// SignIn starts, or refreshes, the user session of endUserID on the browser and selects it at now.
func (s *postgresStore) SignIn(ctx context.Context, orgID, projectID, browserID, endUserID uuid.UUID, method string, now time.Time) error {
	publicID, err := nanoid.New(publicIDLen)
	if err != nil {
		return err
	}
	t := s.sessions
	at := postgres.TimestampzT(now.UTC())
	stmt := t.INSERT(t.ID, t.PublicID, t.OrgID, t.ProjectID, t.BrowserSessionID, t.EndUserID, t.Method, t.AuthTime, t.SelectedAt,
		t.CreatedAt, t.LastSeenAt).
		VALUES(uuid.New(), publicID, orgID, projectID, browserID, endUserID, method, at, at, at, at).
		ON_CONFLICT(t.BrowserSessionID, t.EndUserID).WHERE(t.EndedAt.IS_NULL()).DO_UPDATE(postgres.SET(
		t.Method.SET(postgres.String(method)),
		t.AuthTime.SET(at),
		t.SelectedAt.SET(at),
		t.LastSeenAt.SET(at),
	))
	_, err = s.exec(ctx, "SignIn", stmt)
	return err
}

// Select marks the live user session publicID of the browser selected at now, reporting whether it was live.
func (s *postgresStore) Select(ctx context.Context, projectID, browserID uuid.UUID, publicID string, now time.Time) (bool, error) {
	t := s.sessions
	at := postgres.TimestampzT(now.UTC())
	stmt := t.UPDATE(t.SelectedAt, t.LastSeenAt).SET(at, at).WHERE(s.liveSession(projectID, browserID, publicID))
	n, err := s.exec(ctx, "Select", stmt)
	return n > 0, err
}

// EndUserSession ends the live user session publicID of the browser at now.
func (s *postgresStore) EndUserSession(ctx context.Context, projectID, browserID uuid.UUID, publicID string, now time.Time) error {
	t := s.sessions
	stmt := t.UPDATE(t.EndedAt).SET(postgres.TimestampzT(now.UTC())).WHERE(s.liveSession(projectID, browserID, publicID))
	_, err := s.exec(ctx, "EndUserSession", stmt)
	return err
}

// EndUserSessionAnywhere ends the live user session publicID at now, on whichever browser holds it.
func (s *postgresStore) EndUserSessionAnywhere(ctx context.Context, projectID uuid.UUID, publicID string, now time.Time) error {
	t := s.sessions
	stmt := t.UPDATE(t.EndedAt).SET(postgres.TimestampzT(now.UTC())).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.PublicID.EQ(postgres.String(publicID))).AND(t.EndedAt.IS_NULL()))
	_, err := s.exec(ctx, "EndUserSessionAnywhere", stmt)
	return err
}

// EndOwnedSession ends the live user session publicID of endUserID at now, reporting whether it was live and theirs.
func (s *postgresStore) EndOwnedSession(ctx context.Context, projectID, endUserID uuid.UUID, publicID string, now time.Time) (bool, error) {
	t := s.sessions
	stmt := t.UPDATE(t.EndedAt).SET(postgres.TimestampzT(now.UTC())).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.EndUserID.EQ(postgres.UUID(endUserID))).
			AND(t.PublicID.EQ(postgres.String(publicID))).AND(t.EndedAt.IS_NULL()))
	n, err := s.exec(ctx, "EndOwnedSession", stmt)
	return n > 0, err
}

func (s *postgresStore) liveSession(projectID, browserID uuid.UUID, publicID string) postgres.BoolExpression {
	t := s.sessions
	return t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.BrowserSessionID.EQ(postgres.UUID(browserID))).
		AND(t.PublicID.EQ(postgres.String(publicID))).AND(t.EndedAt.IS_NULL())
}

// KeepNewest ends every live user session of the browser beyond the keep most recently selected.
func (s *postgresStore) KeepNewest(ctx context.Context, projectID, browserID uuid.UUID, keep int, now time.Time) error {
	t := s.sessions
	live := t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.BrowserSessionID.EQ(postgres.UUID(browserID))).AND(t.EndedAt.IS_NULL())
	newest := postgres.SELECT(t.ID).FROM(t).WHERE(live).ORDER_BY(t.SelectedAt.DESC(), t.ID.ASC()).LIMIT(int64(keep))
	stmt := t.UPDATE(t.EndedAt).SET(postgres.TimestampzT(now.UTC())).WHERE(live.AND(t.ID.NOT_IN(newest)))
	_, err := s.exec(ctx, "KeepNewest", stmt)
	return err
}

// InsertTerms appends a terms acceptance; accepting the same version again changes nothing.
func (s *postgresStore) InsertTerms(ctx context.Context, orgID, projectID uuid.UUID, a TermsAcceptance) error {
	t := s.terms
	stmt := t.INSERT(t.OrgID, t.ProjectID, t.EndUserID, t.TermsVersion, t.TermsURL, t.PrivacyURL, t.AcceptedAt, t.UserSessionID,
		t.UserSessionPublicID, t.IP, t.UserAgent).
		VALUES(orgID, projectID, a.EndUserID, a.Version, a.TermsURL, a.PrivacyURL, a.AcceptedAt.UTC(), a.UserSessionID,
			a.UserSessionPublicID, inet(a.IP), a.UserAgent).
		ON_CONFLICT(t.EndUserID, t.TermsVersion).DO_NOTHING()
	_, err := s.exec(ctx, "InsertTerms", stmt)
	return err
}

// Prune deletes the project's expired links and sessions, and ended user sessions; it returns how many rows went.
func (s *postgresStore) Prune(ctx context.Context, projectID uuid.UUID, now time.Time, idle time.Duration) (int, error) {
	at := postgres.TimestampzT(now.UTC())
	project := postgres.UUID(projectID)
	l, us, b := s.links, s.sessions, s.browsers
	total := int64(0)
	for _, step := range []struct {
		op   string
		stmt postgres.Statement
	}{
		{"Prune links", l.DELETE().WHERE(l.ProjectID.EQ(project).AND(l.ExpiresAt.LT_EQ(at)))},
		{"Prune user sessions", us.DELETE().WHERE(us.ProjectID.EQ(project).AND(us.EndedAt.IS_NOT_NULL()))},
		{"Prune browser sessions", b.DELETE().WHERE(b.ProjectID.EQ(project).AND(
			b.AbsoluteExpiresAt.LT_EQ(at).OR(b.LastSeenAt.LT_EQ(postgres.TimestampzT(now.Add(-idle).UTC()))).OR(b.EndedAt.IS_NOT_NULL()),
		))},
	} {
		n, err := s.exec(ctx, step.op, step.stmt)
		if err != nil {
			return int(total), err
		}
		total += n
	}
	return int(total), nil
}

func (s *postgresStore) exec(ctx context.Context, op string, stmt postgres.Statement) (int64, error) {
	var n int64
	err := db.Write(ctx, s.pool, func(q qrm.DB) error {
		res, err := stmt.ExecContext(ctx, q)
		if err != nil {
			return err
		}
		n, err = res.RowsAffected()
		return err
	})
	if err != nil {
		return 0, wrap(op, err)
	}
	return n, nil
}

// CreateEndUser inserts a verified end user with email and name, or returns ErrEmailTaken; it never updates an existing one.
func (s *postgresStore) CreateEndUser(
	ctx context.Context, orgID, projectID uuid.UUID, email, name string, now time.Time,
) (uuid.UUID, error) {
	publicID, err := nanoid.New(publicIDLen)
	if err != nil {
		return uuid.Nil, err
	}
	id := uuid.New()
	t := s.users
	at := now.UTC()
	stmt := t.INSERT(t.ID, t.PublicID, t.OrgID, t.ProjectID, t.Email, t.EmailVerifiedAt, t.Name, t.LastSignInAt, t.CreatedAt, t.UpdatedAt).
		VALUES(id, publicID, orgID, projectID, email, at, name, at, at, at)
	_, err = s.exec(ctx, "CreateEndUser", stmt)
	if db.IsUniqueViolation(err) {
		return uuid.Nil, ErrEmailTaken
	}
	return id, err
}

// FillName sets the end user's name at now only while it is empty.
func (s *postgresStore) FillName(ctx context.Context, projectID, endUserID uuid.UUID, name string, now time.Time) error {
	t := s.users
	stmt := t.UPDATE(t.Name, t.UpdatedAt).SET(postgres.String(name), postgres.TimestampzT(now.UTC())).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.ID.EQ(postgres.UUID(endUserID))).AND(t.Name.EQ(postgres.String(""))))
	_, err := s.exec(ctx, "FillName", stmt)
	return err
}

// UpdateName sets the end user's name at now, reporting whether the end user exists.
func (s *postgresStore) UpdateName(ctx context.Context, projectID, endUserID uuid.UUID, name string, now time.Time) (bool, error) {
	t := s.users
	stmt := t.UPDATE(t.Name, t.UpdatedAt).SET(postgres.String(name), postgres.TimestampzT(now.UTC())).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.ID.EQ(postgres.UUID(endUserID))))
	n, err := s.exec(ctx, "UpdateName", stmt)
	return n > 0, err
}
