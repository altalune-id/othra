package grant

import (
	"context"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"

	"github.com/altalune-id/othra/internal/kernel/db"
)

// InsertCode stores c when its client exists and its user session is live and belongs to its end user, else *SessionGoneError.
func (s *postgresStore) InsertCode(ctx context.Context, projectID uuid.UUID, c NewCode) error {
	t, cl, us, eu := s.codes, s.clients, s.sessions, s.users
	project := postgres.UUID(projectID)
	method := nullString("")
	if c.CodeChallenge != "" {
		method = nullString(MethodS256)
	}
	stmt := t.INSERT(t.ID, t.OrgID, t.ProjectID, t.CodeHash, t.ClientID, t.EndUserID, t.UserSessionID, t.RedirectURI, t.Scopes,
		t.CodeChallenge, t.CodeChallengeMethod, t.Nonce, t.AuthTime, t.CreatedAt, t.ExpiresAt, t.ResourceServerID).
		QUERY(postgres.SELECT(
			postgres.UUID(c.ID), cl.OrgID, cl.ProjectID, postgres.String(c.Hash), cl.ID, us.EndUserID, us.ID,
			postgres.String(c.RedirectURI), db.TextArray(c.Scopes), nullString(c.CodeChallenge), method, nullString(c.Nonce),
			postgres.TimestampzT(c.AuthTime.UTC()), postgres.TimestampzT(c.CreatedAt.UTC()), postgres.TimestampzT(c.ExpiresAt.UTC()),
			nullUUID(c.ResourceServerID),
		).
			FROM(cl.INNER_JOIN(us, us.ProjectID.EQ(cl.ProjectID)).INNER_JOIN(eu, eu.ID.EQ(us.EndUserID))).
			WHERE(cl.ProjectID.EQ(project).AND(cl.PublicID.EQ(postgres.String(c.Client))).
				AND(us.PublicID.EQ(postgres.String(c.UserSession))).AND(us.EndedAt.IS_NULL()).
				AND(eu.PublicID.EQ(postgres.String(c.EndUser)))))
	n, err := s.exec(ctx, "InsertCode", stmt)
	if err == nil && n == 0 {
		return &SessionGoneError{}
	}
	return err
}

// UseCode marks the code used at now by the family it was redeemed into.
func (s *postgresStore) UseCode(ctx context.Context, projectID, codeID, familyID uuid.UUID, now time.Time) error {
	t := s.codes
	stmt := t.UPDATE(t.UsedAt, t.FamilyID).SET(postgres.TimestampzT(now.UTC()), postgres.UUID(familyID)).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.ID.EQ(postgres.UUID(codeID))))
	_, err := s.exec(ctx, "UseCode", stmt)
	return err
}

// InsertFamily stores f.
func (s *postgresStore) InsertFamily(ctx context.Context, orgID, projectID uuid.UUID, f *Family) error {
	t := s.families
	stmt := t.INSERT(t.ID, t.PublicID, t.OrgID, t.ProjectID, t.ClientID, t.EndUserID, t.UserSessionID, t.UserSessionPublicID, t.Scopes,
		t.AuthTime, t.CreatedAt, t.LastUsedAt, t.AbsoluteExpiresAt, t.ResourceServerID).
		VALUES(f.ID, f.PublicID, orgID, projectID, f.ClientID, f.EndUserID, f.UserSessionID, f.UserSession, db.TextArray(f.Scopes),
			f.AuthTime.UTC(), f.CreatedAt.UTC(), f.LastUsedAt.UTC(), f.AbsoluteExpiresAt.UTC(), nullUUID(f.ResourceServerID))
	_, err := s.exec(ctx, "InsertFamily", stmt)
	return err
}

// TouchFamily records a use of the family at now.
func (s *postgresStore) TouchFamily(ctx context.Context, projectID, familyID uuid.UUID, now time.Time) error {
	t := s.families
	stmt := t.UPDATE(t.LastUsedAt).SET(postgres.TimestampzT(now.UTC())).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.ID.EQ(postgres.UUID(familyID))))
	_, err := s.exec(ctx, "TouchFamily", stmt)
	return err
}

// RevokeFamily revokes the family for reason at now, unless it is already revoked.
func (s *postgresStore) RevokeFamily(ctx context.Context, projectID, familyID uuid.UUID, reason string, now time.Time) error {
	t := s.families
	stmt := t.UPDATE(t.RevokedAt, t.RevokedReason).SET(postgres.TimestampzT(now.UTC()), postgres.String(reason)).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.ID.EQ(postgres.UUID(familyID))).AND(t.RevokedAt.IS_NULL()))
	_, err := s.exec(ctx, "RevokeFamily", stmt)
	return err
}

// RevokeUserSession revokes the live families of the user session userSession without offline_access, and drops its unused codes.
func (s *postgresStore) RevokeUserSession(ctx context.Context, projectID uuid.UUID, userSession, reason string, now time.Time) error {
	f, c, us := s.families, s.codes, s.sessions
	project := postgres.UUID(projectID)
	online := postgres.RawBool("NOT (#offline = ANY(scopes))", postgres.RawArgs{"#offline": ScopeOfflineAccess})
	revoke := f.UPDATE(f.RevokedAt, f.RevokedReason).SET(postgres.TimestampzT(now.UTC()), postgres.String(reason)).
		WHERE(f.ProjectID.EQ(project).AND(f.UserSessionPublicID.EQ(postgres.String(userSession))).AND(f.RevokedAt.IS_NULL()).AND(online))
	if _, err := s.exec(ctx, "RevokeUserSession", revoke); err != nil {
		return err
	}
	drop := c.DELETE().WHERE(c.ProjectID.EQ(project).AND(c.UsedAt.IS_NULL()).AND(c.UserSessionID.IN(
		postgres.SELECT(us.ID).FROM(us).WHERE(us.ProjectID.EQ(project).AND(us.PublicID.EQ(postgres.String(userSession)))),
	)))
	_, err := s.exec(ctx, "RevokeUserSession", drop)
	return err
}

// RevokeClient revokes every live family of the end user and client for reason at now, drops their unused codes, and counts them.
func (s *postgresStore) RevokeClient(
	ctx context.Context, projectID, endUserID, clientID uuid.UUID, reason string, now time.Time,
) ([]string, error) {
	f, a, c := s.families, s.access, s.codes
	project, user, client, at := postgres.UUID(projectID), postgres.UUID(endUserID), postgres.UUID(clientID), postgres.TimestampzT(now.UTC())
	revoke := f.UPDATE(f.RevokedAt, f.RevokedReason).SET(at, postgres.String(reason)).
		WHERE(f.ProjectID.EQ(project).AND(f.EndUserID.EQ(user)).AND(f.ClientID.EQ(client)).AND(f.RevokedAt.IS_NULL())).
		RETURNING(f.ID, f.PublicID)
	var rows []struct {
		ID       uuid.UUID `alias:"grant_families.id"`
		PublicID string    `alias:"grant_families.public_id"`
	}
	if err := db.Write(ctx, s.pool, func(q qrm.DB) error { return revoke.QueryContext(ctx, q, &rows) }); err != nil && !db.IsNoRows(err) {
		return nil, wrap("RevokeClient", err)
	}
	ids, sids := make([]postgres.Expression, 0, len(rows)), make([]string, 0, len(rows))
	for _, r := range rows {
		ids, sids = append(ids, postgres.UUID(r.ID)), append(sids, r.PublicID)
	}
	if len(ids) > 0 {
		stamp := a.UPDATE(a.RevokedAt).SET(at).WHERE(a.ProjectID.EQ(project).AND(a.FamilyID.IN(ids...)).AND(a.RevokedAt.IS_NULL()))
		if _, err := s.exec(ctx, "RevokeClient", stamp); err != nil {
			return nil, err
		}
	}
	drop := c.DELETE().WHERE(c.ProjectID.EQ(project).AND(c.EndUserID.EQ(user)).AND(c.ClientID.EQ(client)).AND(c.UsedAt.IS_NULL()))
	if _, err := s.exec(ctx, "RevokeClient", drop); err != nil {
		return nil, err
	}
	return sids, nil
}

// InsertAccessToken stores an access token of the family, bound to resourceServerID or nil, by its hash.
func (s *postgresStore) InsertAccessToken(
	ctx context.Context, orgID, projectID, familyID uuid.UUID, resourceServerID *uuid.UUID, hash string, scopes []string,
	now, expires time.Time,
) error {
	t := s.access
	stmt := t.INSERT(t.ID, t.OrgID, t.ProjectID, t.FamilyID, t.TokenHash, t.Scopes, t.CreatedAt, t.ExpiresAt, t.ResourceServerID).
		VALUES(uuid.New(), orgID, projectID, familyID, hash, db.TextArray(scopes), now.UTC(), expires.UTC(), nullUUID(resourceServerID))
	_, err := s.exec(ctx, "InsertAccessToken", stmt)
	return err
}

// RevokeAccessToken revokes the access token tokenID alone at now.
func (s *postgresStore) RevokeAccessToken(ctx context.Context, projectID, tokenID uuid.UUID, now time.Time) error {
	t := s.access
	stmt := t.UPDATE(t.RevokedAt).SET(postgres.TimestampzT(now.UTC())).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.ID.EQ(postgres.UUID(tokenID))).AND(t.RevokedAt.IS_NULL()))
	_, err := s.exec(ctx, "RevokeAccessToken", stmt)
	return err
}

// InsertRefreshToken stores a refresh token of the family by its hash.
func (s *postgresStore) InsertRefreshToken(
	ctx context.Context, orgID, projectID, familyID uuid.UUID, hash string, now, expires time.Time,
) error {
	t := s.refresh
	stmt := t.INSERT(t.ID, t.OrgID, t.ProjectID, t.FamilyID, t.TokenHash, t.CreatedAt, t.ExpiresAt).
		VALUES(uuid.New(), orgID, projectID, familyID, hash, now.UTC(), expires.UTC())
	_, err := s.exec(ctx, "InsertRefreshToken", stmt)
	return err
}

// UseRefreshToken marks the refresh token used at now.
func (s *postgresStore) UseRefreshToken(ctx context.Context, projectID, tokenID uuid.UUID, now time.Time) error {
	t := s.refresh
	stmt := t.UPDATE(t.UsedAt).SET(postgres.TimestampzT(now.UTC())).
		WHERE(t.ProjectID.EQ(postgres.UUID(projectID)).AND(t.ID.EQ(postgres.UUID(tokenID))))
	_, err := s.exec(ctx, "UseRefreshToken", stmt)
	return err
}

// Prune deletes what is past the cutoffs and returns how many rows went.
func (s *postgresStore) Prune(ctx context.Context, projectID uuid.UUID, cut Cutoffs) (int, error) {
	c, a, r, f := s.codes, s.access, s.refresh, s.families
	project := postgres.UUID(projectID)
	now := postgres.TimestampzT(cut.Now.UTC())
	total := 0
	for _, stmt := range []postgres.Statement{
		f.DELETE().WHERE(f.ProjectID.EQ(project).AND(f.AbsoluteExpiresAt.LT_EQ(now).
			OR(f.RevokedAt.LT(postgres.TimestampzT(cut.Revoked.UTC()))).
			OR(f.LastUsedAt.LT(postgres.TimestampzT(cut.Idle.UTC()))))),
		c.DELETE().WHERE(c.ProjectID.EQ(project).AND(c.UsedAt.IS_NULL().AND(c.ExpiresAt.LT_EQ(now)).
			OR(c.UsedAt.LT(postgres.TimestampzT(cut.UsedCodes.UTC()))))),
		a.DELETE().WHERE(a.ProjectID.EQ(project).AND(a.ExpiresAt.LT_EQ(now))),
		r.DELETE().WHERE(r.ProjectID.EQ(project).AND(r.ExpiresAt.LT_EQ(now))),
	} {
		n, err := s.exec(ctx, "Prune", stmt)
		total += int(n)
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
