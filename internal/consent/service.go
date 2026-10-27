package consent

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/nanoid"
)

const publicIDLen = 21

const userAgentMax = 512

// Project is the project a consent belongs to.
type Project struct {
	OrgID    uuid.UUID
	ID       uuid.UUID
	PublicID string
}

// Connection is the end user and client a Disconnect is for, by id and by the public ids its audit line carries.
type Connection struct {
	EndUserID       uuid.UUID
	EndUserPublicID string
	ClientID        uuid.UUID
	ClientOAuthID   string
}

// GrantRequest is an end user approving Scopes for Key, from IP with UserAgent.
type GrantRequest struct {
	Key    Key
	Scopes []string
	// Allowed are the scopes a stored consent may keep: the OIDC scopes and the server's catalog; nil keeps every stored scope.
	Allowed   []string
	IP        string
	UserAgent string
}

// Service remembers the scopes end users approve and withdraws them.
type Service struct {
	store      Store
	grants     Grants
	uow        db.Transactor
	unexpected apperror.UnexpectedFunc
	now        func() time.Time
	log        *slog.Logger
}

// NewService builds the Service over store; grants are revoked with an app's consents in one unit of work on uow.
func NewService(store Store, uow db.Transactor, grants Grants, unexpected apperror.UnexpectedFunc) *Service {
	return &Service{
		store: store, uow: uow, grants: grants, unexpected: unexpected,
		now: func() time.Time { return time.Now().UTC() }, log: slog.New(slog.DiscardHandler),
	}
}

// WithLogger sets where approvals and revokes are logged.
func (s *Service) WithLogger(log *slog.Logger) *Service {
	s.log = log
	return s
}

// WithClock sets the clock that stamps approvals.
func (s *Service) WithClock(now func() time.Time) *Service {
	s.now = now
	return s
}

func scope(ctx context.Context, p Project) context.Context {
	return tenant.WithProject(tenant.WithOrg(ctx, p.OrgID), p.ID)
}

// Find returns the consent key names, or nil.
func (s *Service) Find(ctx context.Context, p Project, key Key) (*Consent, error) {
	ctx = scope(ctx, p)
	c, err := s.store.Find(ctx, p.ID, key)
	if err != nil {
		return nil, s.unexpected(ctx, "consent: find", err)
	}
	return c, nil
}

// Grant adds req.Scopes to the consent of req.Key, creating it on the first approval, and returns what is now approved.
func (s *Service) Grant(ctx context.Context, p Project, req GrantRequest) (*Consent, error) {
	ctx = scope(ctx, p)
	scopes := Union(nil, req.Scopes)
	if len(scopes) == 0 || len(scopes) > MaxScopes {
		return nil, s.unexpected(ctx, "consent: grant", fmt.Errorf("consent: %d scopes", len(scopes)))
	}
	publicID, err := nanoid.New(publicIDLen)
	if err != nil {
		return nil, s.unexpected(ctx, "consent: grant", err)
	}
	now := s.now()
	c, err := s.store.Upsert(ctx, &Consent{
		ID: uuid.New(), PublicID: publicID, OrgID: p.OrgID, ProjectID: p.ID, Key: req.Key, Scopes: scopes,
		GrantedAt: now, UpdatedAt: now, IP: req.IP, UserAgent: clip(req.UserAgent, userAgentMax),
	}, req.Allowed)
	if err != nil {
		return nil, s.unexpected(ctx, "consent: grant", err)
	}
	s.log.LogAttrs(ctx, slog.LevelInfo, "consent.granted", slog.String("project", p.PublicID), slog.String("consent", c.PublicID),
		slog.Any("scopes", scopes))
	return c, nil
}

// ForgetClient deletes every consent of clientID, so its end users see the full consent page again (CIMD §8.4).
func (s *Service) ForgetClient(ctx context.Context, p Project, clientID uuid.UUID) error {
	ctx = scope(ctx, p)
	if err := s.store.DeleteClient(ctx, p.ID, clientID); err != nil {
		return s.unexpected(ctx, "consent: forget client", err)
	}
	return nil
}

// ConnectedApps returns every client the end user has a consent or a live grant with, grouped by client (profile §2.5).
func (s *Service) ConnectedApps(ctx context.Context, p Project, endUserID uuid.UUID) ([]App, error) {
	ctx = scope(ctx, p)
	consents, err := s.store.ListByEndUser(ctx, p.ID, endUserID)
	if err != nil {
		return nil, s.unexpected(ctx, "consent: connected apps", err)
	}
	families, err := s.grants.LiveFamilies(ctx, p, endUserID)
	if err != nil {
		return nil, err
	}
	return Apps(consents, families, s.grants.ResourceScopes), nil
}

// Disconnect drops the end user's consents for clientID and revokes its grants in one unit of work, or *AppNotConnectedError.
func (s *Service) Disconnect(ctx context.Context, p Project, c Connection) error {
	ctx = scope(ctx, p)
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		deleted, err := s.store.DeleteEndUserClient(ctx, p.ID, c.EndUserID, c.ClientID)
		if err != nil {
			return err
		}
		revoked, err := s.grants.RevokeClient(ctx, p, c.EndUserID, c.ClientID)
		if err != nil {
			return err
		}
		if deleted == 0 && !revoked {
			return &AppNotConnectedError{}
		}
		return nil
	})
	if IsAppNotConnectedError(err) {
		return err
	}
	if err != nil {
		return s.errOrReport(ctx, "consent: disconnect", err)
	}
	s.log.LogAttrs(ctx, slog.LevelInfo, "consent.disconnected", slog.String("project", p.PublicID),
		slog.String("end_user", c.EndUserPublicID), slog.String("client_id", c.ClientOAuthID))
	return nil
}

func (s *Service) errOrReport(ctx context.Context, message string, err error) error {
	if _, ok := apperror.AsAppError(err); ok {
		return err
	}
	return s.unexpected(ctx, message, err)
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
