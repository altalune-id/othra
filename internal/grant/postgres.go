package grant

import (
	"context"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/schema/entity"
)

type postgresStore struct {
	pool     db.Pool
	codes    *entity.AuthorizationCodes
	families *entity.GrantFamilies
	access   *entity.AccessTokens
	refresh  *entity.RefreshTokens
	clients  *entity.OAuthClients
	sessions *entity.UserSessions
	users    *entity.EndUsers
}

var _ Store = (*postgresStore)(nil)

// NewStore builds the Postgres store over pool.
func NewStore(pool db.Pool, schema, tablePrefix string) *postgresStore {
	if schema == "" {
		schema = "public"
	}
	return &postgresStore{
		pool:     pool,
		codes:    entity.NewAuthorizationCodes(schema, tablePrefix),
		families: entity.NewGrantFamilies(schema, tablePrefix),
		access:   entity.NewAccessTokens(schema, tablePrefix),
		refresh:  entity.NewRefreshTokens(schema, tablePrefix),
		clients:  entity.NewOAuthClients(schema, tablePrefix),
		sessions: entity.NewUserSessions(schema, tablePrefix),
		users:    entity.NewEndUsers(schema, tablePrefix),
	}
}

type familyRow struct {
	ID                uuid.UUID  `alias:"grant_families.id"`
	PublicID          string     `alias:"grant_families.public_id"`
	ClientID          uuid.UUID  `alias:"grant_families.client_id"`
	EndUserID         uuid.UUID  `alias:"grant_families.end_user_id"`
	UserSessionID     *uuid.UUID `alias:"grant_families.user_session_id"`
	UserSession       string     `alias:"grant_families.user_session_public_id"`
	Scopes            string     `alias:"grant_families.scopes"`
	AuthTime          time.Time  `alias:"grant_families.auth_time"`
	CreatedAt         time.Time  `alias:"grant_families.created_at"`
	LastUsedAt        time.Time  `alias:"grant_families.last_used_at"`
	AbsoluteExpiresAt time.Time  `alias:"grant_families.absolute_expires_at"`
	RevokedAt         *time.Time `alias:"grant_families.revoked_at"`
}

func (r *familyRow) toFamily() (*Family, error) {
	scopes, err := db.ParseJSONArray(r.Scopes)
	if err != nil {
		return nil, fmt.Errorf("grant.postgres: scopes of %s: %w", r.PublicID, err)
	}
	return &Family{
		ID: r.ID, PublicID: r.PublicID, ClientID: r.ClientID, EndUserID: r.EndUserID, UserSessionID: r.UserSessionID,
		UserSession: r.UserSession, Scopes: scopes, AuthTime: r.AuthTime.UTC(), CreatedAt: r.CreatedAt.UTC(),
		LastUsedAt: r.LastUsedAt.UTC(), AbsoluteExpiresAt: r.AbsoluteExpiresAt.UTC(), RevokedAt: utc(r.RevokedAt),
	}, nil
}

type codeRow struct {
	ID            uuid.UUID  `alias:"authorization_codes.id"`
	ClientID      uuid.UUID  `alias:"authorization_codes.client_id"`
	EndUserID     uuid.UUID  `alias:"authorization_codes.end_user_id"`
	UserSessionID *uuid.UUID `alias:"authorization_codes.user_session_id"`
	RedirectURI   string     `alias:"authorization_codes.redirect_uri"`
	Scopes        string     `alias:"authorization_codes.scopes"`
	CodeChallenge *string    `alias:"authorization_codes.code_challenge"`
	Nonce         *string    `alias:"authorization_codes.nonce"`
	AuthTime      time.Time  `alias:"authorization_codes.auth_time"`
	ExpiresAt     time.Time  `alias:"authorization_codes.expires_at"`
	UsedAt        *time.Time `alias:"authorization_codes.used_at"`
	FamilyID      *uuid.UUID `alias:"authorization_codes.family_id"`
	UserSession   *string    `alias:"user_sessions.public_id"`
	FamilySID     *string    `alias:"grant_families.public_id"`
}

func (r *codeRow) toCode() (*Code, error) {
	scopes, err := db.ParseJSONArray(r.Scopes)
	if err != nil {
		return nil, fmt.Errorf("grant.postgres: code scopes: %w", err)
	}
	return &Code{
		ID: r.ID, ClientID: r.ClientID, EndUserID: r.EndUserID, UserSessionID: r.UserSessionID, UserSession: deref(r.UserSession),
		RedirectURI: r.RedirectURI, Scopes: scopes, CodeChallenge: deref(r.CodeChallenge), Nonce: deref(r.Nonce),
		AuthTime: r.AuthTime.UTC(), ExpiresAt: r.ExpiresAt.UTC(), UsedAt: utc(r.UsedAt), FamilyID: r.FamilyID,
		FamilySID: deref(r.FamilySID),
	}, nil
}

func (s *postgresStore) familyColumns() postgres.ProjectionList {
	t := s.families
	return postgres.ProjectionList{
		t.ID, t.PublicID, t.ClientID, t.EndUserID, t.UserSessionID, t.UserSessionPublicID,
		db.ArrayAsJSON(t.Scopes, "grant_families.scopes"), t.AuthTime, t.CreatedAt, t.LastUsedAt, t.AbsoluteExpiresAt, t.RevokedAt,
	}
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

func nullString(v string) postgres.Expression {
	if v == "" {
		return postgres.NULL
	}
	return postgres.String(v)
}

func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func wrap(op string, err error) error {
	return fmt.Errorf("grant.postgres: %s: %w", op, err)
}
