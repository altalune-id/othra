package pat

import (
	"cmp"
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/internal/kernel/secret"
	"github.com/altalune-id/othra/internal/kernel/tenant"
	"github.com/altalune-id/othra/internal/kernel/usage"
	"github.com/altalune-id/othra/nanoid"
)

// Service mints, lists, revokes and introspects personal tokens.
type Service struct {
	store      Store
	uow        db.Transactor
	servers    Servers
	subjects   Subjects
	usage      *usage.Worker
	unexpected apperror.UnexpectedFunc
	now        func() time.Time
	log        *slog.Logger
}

// NewService builds the Service; recorder may be nil, which records no last use.
func NewService(
	store Store, uow db.Transactor, servers Servers, subjects Subjects, recorder *usage.Worker, unexpected apperror.UnexpectedFunc,
) *Service {
	return &Service{
		store: store, uow: uow, servers: servers, subjects: subjects, usage: recorder, unexpected: unexpected,
		now: func() time.Time { return time.Now().UTC() }, log: slog.New(slog.DiscardHandler),
	}
}

// WithLogger sets where token creation and revocation are logged.
func (s *Service) WithLogger(log *slog.Logger) *Service {
	s.log = log
	return s
}

// WithClock sets the clock, for tests.
func (s *Service) WithClock(now func() time.Time) *Service {
	s.now = now
	return s
}

// CreateRequest asks for a personal token for EndUserID.
type CreateRequest struct {
	EndUserID      uuid.UUID
	Name           string
	ServerPublicID string
	Scopes         []string
	Expiry         Expiry
}

// Created is a token just minted; Secret is shown once and never stored.
type Created struct {
	Token  *Token
	Server Server
	Secret string
}

// Listed is a token with its server, for the portal and the admin view.
type Listed struct {
	Token  *Token
	Server Server
}

// Introspected is a personal token that is active for the calling resource server (RFC 7662 §2.2).
type Introspected struct {
	Token    *Token
	Subject  string
	Audience string
}

const (
	publicIDLen = 21
	pruneAfter  = 30 * 24 * time.Hour
)

func scope(ctx context.Context, p Project) context.Context {
	return tenant.WithProject(tenant.WithOrg(ctx, p.OrgID), p.ID)
}

// TokenServers returns the servers an end user can mint a token for.
func (s *Service) TokenServers(ctx context.Context, p Project) ([]Server, error) {
	ctx = scope(ctx, p)
	servers, err := s.servers.Accepting(ctx, p)
	if err != nil {
		return nil, s.errOrReport(ctx, "pat: token servers", err)
	}
	return servers, nil
}

// List returns the end user's tokens with their servers, newest first.
func (s *Service) List(ctx context.Context, p Project, endUserID uuid.UUID) ([]Listed, error) {
	ctx = scope(ctx, p)
	tokens, err := s.store.ListByEndUser(ctx, p.ID, endUserID)
	if err != nil {
		return nil, s.errOrReport(ctx, "pat: list", err)
	}
	servers := map[uuid.UUID]Server{}
	out := make([]Listed, 0, len(tokens))
	for _, t := range tokens {
		srv, ok := servers[t.ResourceServerID]
		if !ok {
			if srv, _, err = s.servers.ByID(ctx, p, t.ResourceServerID); err != nil {
				return nil, s.errOrReport(ctx, "pat: list", err)
			}
			servers[t.ResourceServerID] = srv
		}
		out = append(out, Listed{Token: t, Server: srv})
	}
	return out, nil
}

// Create mints a token: ACC005 for bad input, ACC006 for a closed server, ACC007 past MaxLive.
func (s *Service) Create(ctx context.Context, p Project, req CreateRequest) (*Created, error) {
	ctx = scope(ctx, p)
	now := s.now()
	name, ok := NormalizeName(req.Name)
	switch {
	case !ok:
		return nil, &InvalidError{Reason: InvalidName}
	case !req.Expiry.Valid():
		return nil, &InvalidError{Reason: InvalidExpiry}
	}
	srv, ok, err := s.servers.ByPublicID(ctx, p, req.ServerPublicID)
	switch {
	case err != nil:
		return nil, s.errOrReport(ctx, "pat: create", err)
	case !ok:
		return nil, &InvalidError{Reason: InvalidServer}
	case !srv.Accepting:
		return nil, &ServerClosedError{Project: p.Name}
	case !srv.ScopesWithin(req.Scopes):
		return nil, &InvalidError{Reason: InvalidScopes}
	}
	var out *Created
	err = s.uow.RunInTx(ctx, func(ctx context.Context) error {
		if found, err := s.store.LockEndUser(ctx, p.ID, req.EndUserID); err != nil || !found {
			return cmp.Or(err, error(&NotFoundError{}))
		}
		live, err := s.store.CountLive(ctx, p.ID, req.EndUserID, now)
		if err != nil {
			return err
		}
		if live >= MaxLive {
			return &LimitError{Max: MaxLive}
		}
		m, err := Scheme.Mint()
		if err != nil {
			return err
		}
		id, err := nanoid.New(publicIDLen)
		if err != nil {
			return err
		}
		t := &Token{
			ID: uuid.New(), PublicID: id, OrgID: p.OrgID, ProjectID: p.ID, EndUserID: req.EndUserID, ResourceServerID: srv.ID,
			Name: name, Hint: m.Hint, Scopes: slices.Compact(slices.Sorted(slices.Values(req.Scopes))), CreatedAt: now,
			ExpiresAt: now.Add(req.Expiry.Duration()),
		}
		if err := s.store.Insert(ctx, t, secret.Versioned(m.Plaintext)); err != nil {
			return err
		}
		out = &Created{Token: t, Server: srv, Secret: m.Plaintext}
		return nil
	})
	if err != nil {
		return nil, s.errOrReport(ctx, "pat: create", err)
	}
	s.log.LogAttrs(ctx, slog.LevelInfo, "pat: created", slog.String("project", p.PublicID), slog.String("token_id", out.Token.PublicID),
		slog.String("resource_server", srv.PublicID), slog.Int("days", int(req.Expiry)))
	return out, nil
}

// Revoke revokes the end user's token publicID at once, or returns *NotFoundError.
func (s *Service) Revoke(ctx context.Context, p Project, endUserID uuid.UUID, publicID string) error {
	ctx = scope(ctx, p)
	ok, err := s.store.Revoke(ctx, p.ID, endUserID, publicID, s.now())
	switch {
	case err != nil:
		return s.errOrReport(ctx, "pat: revoke", err)
	case !ok:
		return &NotFoundError{}
	}
	s.log.LogAttrs(ctx, slog.LevelInfo, "pat: revoked", slog.String("project", p.PublicID), slog.String("token_id", publicID))
	return nil
}

// Introspect returns the token when it is active for the resource server callerID. SECURITY: RFC 7662 §2.2: any other case is false.
func (s *Service) Introspect(ctx context.Context, p Project, callerID uuid.UUID, token string) (in Introspected, ok bool, err error) {
	if !Scheme.Valid(token) {
		return Introspected{}, false, nil
	}
	ctx = scope(ctx, p)
	now := s.now()
	t, err := s.store.ByHash(ctx, p.ID, secret.Versioned(token))
	if err != nil {
		return Introspected{}, false, s.errOrReport(ctx, "pat: introspect", err)
	}
	if t == nil || t.State(now) != StateActive || t.ResourceServerID != callerID {
		return Introspected{}, false, nil
	}
	srv, found, err := s.servers.ByID(ctx, p, t.ResourceServerID)
	if err != nil {
		return Introspected{}, false, s.errOrReport(ctx, "pat: introspect", err)
	}
	if !found || !srv.Accepting {
		return Introspected{}, false, nil
	}
	sub, err := s.subjects.Subject(ctx, p, t.EndUserID)
	if err != nil {
		return Introspected{}, false, s.errOrReport(ctx, "pat: introspect", err)
	}
	if sub == "" {
		return Introspected{}, false, nil
	}
	if s.usage != nil {
		s.usage.Record(tenant.Context{OrgID: p.OrgID, ProjectID: p.ID}, t.ID, now)
	}
	return Introspected{Token: t, Subject: sub, Audience: srv.Identifier}, true, nil
}

// Prune deletes the project's tokens expired or revoked over 30 days ago.
func (s *Service) Prune(ctx context.Context, orgID, projectID uuid.UUID) (int, error) {
	ctx = scope(ctx, Project{OrgID: orgID, ID: projectID})
	n, err := s.store.Prune(ctx, projectID, s.now().Add(-pruneAfter))
	if err != nil {
		return 0, s.errOrReport(ctx, "pat: prune", err)
	}
	return n, nil
}

func (s *Service) errOrReport(ctx context.Context, message string, err error) error {
	if _, reported := apperror.AsAppError(err); reported || IsExpectedError(err) {
		return err
	}
	return s.unexpected(ctx, message, err)
}
