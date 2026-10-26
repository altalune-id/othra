package enduser

import (
	"fmt"
	"net/netip"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/schema/entity"
)

type postgresStore struct {
	pool     db.Pool
	liveLink string
	users    *entity.EndUsers
	browsers *entity.BrowserSessions
	sessions *entity.UserSessions
	terms    *entity.EndUserTerms
	links    *entity.MagicLinks
}

var _ Store = (*postgresStore)(nil)

// NewStore builds the Postgres store over pool.
func NewStore(pool db.Pool, schema, tablePrefix string) *postgresStore {
	if schema == "" {
		schema = "public"
	}
	return &postgresStore{
		pool:     pool,
		liveLink: tablePrefix + "magic_links_one_live",
		users:    entity.NewEndUsers(schema, tablePrefix),
		browsers: entity.NewBrowserSessions(schema, tablePrefix),
		sessions: entity.NewUserSessions(schema, tablePrefix),
		terms:    entity.NewEndUserTerms(schema, tablePrefix),
		links:    entity.NewMagicLinks(schema, tablePrefix),
	}
}

type linkRow struct {
	ID               uuid.UUID  `alias:"magic_links.id"`
	PublicID         string     `alias:"magic_links.public_id"`
	Email            string     `alias:"magic_links.email"`
	ReturnTo         string     `alias:"magic_links.return_to"`
	CreatedAt        time.Time  `alias:"magic_links.created_at"`
	ExpiresAt        time.Time  `alias:"magic_links.expires_at"`
	UsedAt           *time.Time `alias:"magic_links.used_at"`
	BrowserSessionID *uuid.UUID `alias:"magic_links.browser_session_id"`
	PendingIdentity  *string    `alias:"magic_links.pending_identity"`
}

func (r *linkRow) toLink() *Link {
	return &Link{
		ID: r.ID, PublicID: r.PublicID, Email: r.Email, ReturnTo: r.ReturnTo, CreatedAt: r.CreatedAt.UTC(),
		ExpiresAt: r.ExpiresAt.UTC(), UsedAt: utc(r.UsedAt), BrowserSessionID: r.BrowserSessionID,
		Pending: pendingBytes(r.PendingIdentity),
	}
}

type browserRow struct {
	ID          uuid.UUID `alias:"browser_sessions.id"`
	CreatedAt   time.Time `alias:"browser_sessions.created_at"`
	LastSeenAt  time.Time `alias:"browser_sessions.last_seen_at"`
	AbsoluteEnd time.Time `alias:"browser_sessions.absolute_expires_at"`
}

type accountRow struct {
	UserSessionID uuid.UUID `alias:"user_sessions.id"`
	PublicID      string    `alias:"user_sessions.public_id"`
	AuthTime      time.Time `alias:"user_sessions.auth_time"`
	SelectedAt    time.Time `alias:"user_sessions.selected_at"`
	Method        string    `alias:"user_sessions.method"`
	EndUserID     uuid.UUID `alias:"end_users.id"`
	Subject       string    `alias:"end_users.public_id"`
	Email         string    `alias:"end_users.email"`
	Name          string    `alias:"end_users.name"`
}

func (r *accountRow) toAccount() Account {
	return Account{
		UserSessionID: r.UserSessionID, PublicID: r.PublicID, EndUserID: r.EndUserID, Subject: r.Subject,
		Email: r.Email, Name: r.Name, AuthTime: r.AuthTime.UTC(), SelectedAt: r.SelectedAt.UTC(),
		Method: r.Method,
	}
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// SECURITY: a value that is not an IP address or prefix is stored as NULL, never cast from client input.
func inet(ip string) postgres.Expression {
	if _, err := netip.ParseAddr(ip); err == nil {
		return postgres.CAST(postgres.String(ip)).AS("inet")
	}
	if _, err := netip.ParsePrefix(ip); err == nil {
		return postgres.CAST(postgres.String(ip)).AS("inet")
	}
	return postgres.NULL
}

func pendingValue(pending []byte) postgres.Expression {
	if pending == nil {
		return postgres.NULL
	}
	return postgres.CAST(postgres.String(string(pending))).AS("jsonb")
}

func pendingBytes(raw *string) []byte {
	if raw == nil {
		return nil
	}
	return []byte(*raw)
}

func wrap(op string, err error) error {
	return fmt.Errorf("enduser.postgres: %s: %w", op, err)
}
