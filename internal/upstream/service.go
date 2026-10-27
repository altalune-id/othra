package upstream

import (
	"context"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/nanoid"
	"altalune.id/othra/sealer"
)

const (
	publicIDLen    = 21
	maxClientIDLen = 255
	maxSecretLen   = 512
)

// Service manages a project's upstream IdPs and opens their secrets for a sign-in.
type Service struct {
	store      Store
	uow        db.Transactor
	seal       sealer.Sealer
	guard      SignInGuard
	issuers    Issuers
	unexpected apperror.UnexpectedFunc
	now        func() time.Time
	log        *slog.Logger
}

// NewService builds the Service; seal keeps client secrets and guard reads the project's email-link switch.
func NewService(store Store, uow db.Transactor, seal sealer.Sealer, guard SignInGuard, unexpected apperror.UnexpectedFunc) *Service {
	return &Service{
		store: store, uow: uow, seal: seal, guard: guard, unexpected: unexpected,
		now: func() time.Time { return time.Now().UTC() }, log: slog.New(slog.DiscardHandler),
	}
}

// WithIssuers gives the Service the project issuer URLs RedirectURI builds on.
func (s *Service) WithIssuers(i Issuers) *Service {
	s.issuers = i
	return s
}

// RedirectURI returns the exact URI the admin registers at the upstream for the project (P11).
func (s *Service) RedirectURI(ctx context.Context, orgID, projectID uuid.UUID, kind Kind) (string, error) {
	if s.issuers == nil {
		return "", nil
	}
	issuer, err := s.issuers.IssuerURL(scope(ctx, orgID, projectID), projectID)
	if err != nil {
		return "", s.errOrReport(ctx, "upstream: redirect uri", err)
	}
	return RedirectURI(issuer, kind), nil
}

// WithLogger sets where upstream changes are logged.
func (s *Service) WithLogger(log *slog.Logger) *Service {
	s.log = log
	return s
}

// WithClock sets the clock that stamps changes.
func (s *Service) WithClock(now func() time.Time) *Service {
	s.now = now
	return s
}

func scope(ctx context.Context, orgID, projectID uuid.UUID) context.Context {
	return tenant.WithProject(tenant.WithOrg(ctx, orgID), projectID)
}

// CreateRequest is a new upstream of Kind with its client at the upstream IdP.
type CreateRequest struct {
	Kind         Kind
	ClientID     string
	ClientSecret string
	Enabled      bool
}

// List returns the project's upstreams.
func (s *Service) List(ctx context.Context, orgID, projectID uuid.UUID) ([]*Upstream, error) {
	ctx = scope(ctx, orgID, projectID)
	out, err := s.store.List(ctx, projectID)
	if err != nil {
		return nil, s.errOrReport(ctx, "upstream: list", err)
	}
	return out, nil
}

// Get returns the project's upstream publicID, or *NotFoundError.
func (s *Service) Get(ctx context.Context, orgID, projectID uuid.UUID, publicID string) (*Upstream, error) {
	ctx = scope(ctx, orgID, projectID)
	u, err := s.store.ByPublicID(ctx, projectID, publicID)
	if err != nil {
		return nil, s.errOrReport(ctx, "upstream: get", err)
	}
	return u, nil
}

// Create adds an upstream of a kind the project doesn't have yet, or returns *ExistsError.
func (s *Service) Create(ctx context.Context, orgID, projectID uuid.UUID, req CreateRequest) (*Upstream, error) {
	ctx = scope(ctx, orgID, projectID)
	clientID := strings.TrimSpace(req.ClientID)
	if _, ok := SpecOf(req.Kind); !ok {
		return nil, &InvalidConfigError{Reason: ConfigClientID}
	}
	if err := validate(clientID, req.ClientSecret, true); err != nil {
		return nil, err
	}
	publicID, err := nanoid.New(publicIDLen)
	if err != nil {
		return nil, s.unexpected(ctx, "upstream: create", err)
	}
	now := s.now()
	u := &Upstream{
		ID: uuid.New(), PublicID: publicID, OrgID: orgID, ProjectID: projectID, Kind: req.Kind, ClientID: clientID,
		Enabled: req.Enabled, CreatedAt: now, UpdatedAt: now,
	}
	sealed, err := s.seal.Seal([]byte(req.ClientSecret), aad(u))
	if err != nil {
		return nil, s.unexpected(ctx, "upstream: create", err)
	}
	if err := s.store.Insert(ctx, u, sealed); err != nil {
		return nil, s.errOrReport(ctx, "upstream: create", err)
	}
	s.logChange(ctx, "upstream: created", u)
	return u, nil
}

// Update replaces an upstream's client id, and its secret when secret is not empty.
func (s *Service) Update(ctx context.Context, orgID, projectID uuid.UUID, publicID, clientID, secret string) (*Upstream, error) {
	ctx = scope(ctx, orgID, projectID)
	clientID = strings.TrimSpace(clientID)
	if err := validate(clientID, secret, false); err != nil {
		return nil, err
	}
	var out *Upstream
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		u, err := s.store.Lock(ctx, projectID, publicID)
		if err != nil {
			return err
		}
		var sealed []byte
		if secret != "" {
			if sealed, err = s.seal.Seal([]byte(secret), aad(u)); err != nil {
				return err
			}
		}
		u.ClientID, u.UpdatedAt = clientID, s.now()
		out = u
		return s.store.Update(ctx, u, sealed)
	})
	if err != nil {
		return nil, s.errOrReport(ctx, "upstream: update", err)
	}
	s.logChange(ctx, "upstream: updated", out)
	return out, nil
}

// SetEnabled turns an upstream on or off; turning the last sign-in method off is refused with *LastMethodError.
func (s *Service) SetEnabled(ctx context.Context, orgID, projectID uuid.UUID, publicID string, enabled bool) (*Upstream, error) {
	ctx = scope(ctx, orgID, projectID)
	var out *Upstream
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		u, err := s.store.Lock(ctx, projectID, publicID)
		if err != nil {
			return err
		}
		if u.Enabled && !enabled {
			if err := s.keepsAMethod(ctx, projectID); err != nil {
				return err
			}
		}
		u.Enabled, u.UpdatedAt = enabled, s.now()
		out = u
		return s.store.Update(ctx, u, nil)
	})
	if err != nil {
		return nil, s.errOrReport(ctx, "upstream: set enabled", err)
	}
	s.logChange(ctx, "upstream: enabled changed", out)
	return out, nil
}

// Delete removes an upstream; it is refused while end users have linked it (*HasIdentitiesError) or when it is the last method.
func (s *Service) Delete(ctx context.Context, orgID, projectID uuid.UUID, publicID string) error {
	ctx = scope(ctx, orgID, projectID)
	var gone *Upstream
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		u, err := s.store.Lock(ctx, projectID, publicID)
		if err != nil {
			return err
		}
		if u.Enabled {
			if err := s.keepsAMethod(ctx, projectID); err != nil {
				return err
			}
		}
		gone = u
		return s.store.Delete(ctx, projectID, u.ID)
	})
	if err != nil {
		return s.errOrReport(ctx, "upstream: delete", err)
	}
	s.logChange(ctx, "upstream: deleted", gone)
	return nil
}

// SECURITY: F8, R1: with the project row locked, an upstream goes off only while the email link stays on.
func (s *Service) keepsAMethod(ctx context.Context, projectID uuid.UUID) error {
	on, err := s.guard.EmailLinkOn(ctx, projectID)
	switch {
	case err != nil:
		return err
	case !on:
		return &LastMethodError{}
	}
	return nil
}

// Offered returns the kinds of the project's enabled upstreams: the one read of which upstream IdPs end users may sign in with.
func (s *Service) Offered(ctx context.Context, orgID, projectID uuid.UUID) ([]Kind, error) {
	list, err := s.List(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	var out []Kind
	for _, u := range list {
		if u.Enabled {
			out = append(out, u.Kind)
		}
	}
	return out, nil
}

// AnyEnabled reports whether the project offers an upstream IdP, so its end users can sign in without the email link.
func (s *Service) AnyEnabled(ctx context.Context, orgID, projectID uuid.UUID) (bool, error) {
	kinds, err := s.Offered(ctx, orgID, projectID)
	return len(kinds) > 0, err
}

// Credentials returns the project's enabled upstream of kind with its secret opened, or *NotFoundError.
func (s *Service) Credentials(ctx context.Context, orgID, projectID uuid.UUID, kind Kind) (*Credentials, error) {
	ctx = scope(ctx, orgID, projectID)
	u, err := s.store.ByKind(ctx, projectID, kind)
	switch {
	case err != nil:
		return nil, s.errOrReport(ctx, "upstream: credentials", err)
	case !u.Enabled:
		return nil, &NotFoundError{}
	}
	sealed, err := s.store.Secret(ctx, projectID, u.ID)
	if err != nil {
		return nil, s.errOrReport(ctx, "upstream: credentials", err)
	}
	secret, err := s.seal.Open(sealed, aad(u))
	if err != nil {
		return nil, s.unexpected(ctx, "upstream: credentials", err)
	}
	return &Credentials{Upstream: *u, ClientSecret: string(secret)}, nil
}

// Seed adds an enabled upstream of kind from config when the project has none, and never changes one (F13, authl seeder).
func (s *Service) Seed(ctx context.Context, orgID, projectID uuid.UUID, kind Kind, clientID, secret string) error {
	if clientID == "" || secret == "" {
		return nil
	}
	existing, err := s.store.ByKind(scope(ctx, orgID, projectID), projectID, kind)
	switch {
	case err == nil:
		if existing.ClientID != strings.TrimSpace(clientID) {
			s.log.LogAttrs(ctx, slog.LevelWarn, "upstream: seed skipped, the stored client id differs from config",
				slog.String("upstream", existing.PublicID), slog.String("kind", string(kind)))
		}
		return nil
	case !IsNotFoundError(err):
		return s.errOrReport(ctx, "upstream: seed", err)
	}
	_, err = s.Create(ctx, orgID, projectID, CreateRequest{Kind: kind, ClientID: clientID, ClientSecret: secret, Enabled: true})
	return err
}

func validate(clientID, secret string, create bool) error {
	switch n := utf8.RuneCountInString(clientID); {
	case n == 0 || n > maxClientIDLen:
		return &InvalidConfigError{Reason: ConfigClientID}
	case create && secret == "", len(secret) > maxSecretLen:
		return &InvalidConfigError{Reason: ConfigClientSecret}
	}
	return nil
}

// SECURITY: ADR-0012, authl gap 12: the AAD binds a sealed secret to its project and row, so it can't be moved to another.
func aad(u *Upstream) []byte {
	return []byte("upstream_provider:" + u.ProjectID.String() + ":" + u.ID.String() + ":client_secret")
}

func (s *Service) logChange(ctx context.Context, msg string, u *Upstream) {
	s.log.LogAttrs(ctx, slog.LevelInfo, msg, slog.String("upstream", u.PublicID), slog.String("kind", string(u.Kind)),
		slog.Bool("enabled", u.Enabled))
}

func (s *Service) errOrReport(ctx context.Context, message string, err error) error {
	if _, ok := apperror.AsAppError(err); ok || IsExpectedError(err) {
		return err
	}
	return s.unexpected(ctx, message, err)
}
