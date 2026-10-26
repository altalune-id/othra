package federation

import (
	"context"
	"crypto/sha256"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/apperror"
	"altalune.id/othra/emailaddr"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/upstream"
	"altalune.id/othra/nanoid"
	"altalune.id/othra/oidcrp"
)

const publicIDLen = 21

// Service runs the round trip with a project's upstream IdPs and keeps its end users' identities.
type Service struct {
	store      Store
	uow        db.Transactor
	upstreams  Upstreams
	users      EndUsers
	projects   SignInSwitch
	cfg        Config
	unexpected apperror.UnexpectedFunc
	now        func() time.Time
	log        *slog.Logger

	mu      sync.Mutex
	clients map[uuid.UUID]cachedClient
}

// NewService builds the Service.
func NewService(
	store Store, uow db.Transactor, upstreams Upstreams, users EndUsers, projects SignInSwitch, cfg Config,
	unexpected apperror.UnexpectedFunc,
) *Service {
	return &Service{
		store: store, uow: uow, upstreams: upstreams, users: users, projects: projects, cfg: cfg, unexpected: unexpected,
		now: func() time.Time { return time.Now().UTC() }, log: slog.New(slog.DiscardHandler), clients: map[uuid.UUID]cachedClient{},
	}
}

// WithLogger sets where links, unlinks and refused sign-ins are logged.
func (s *Service) WithLogger(log *slog.Logger) *Service {
	s.log = log
	return s
}

// WithConfig replaces the Config and drops the cached upstream clients, for tests against a fake IdP.
func (s *Service) WithConfig(cfg Config) *Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg, s.clients = cfg, map[uuid.UUID]cachedClient{}
	return s
}

// WithClock sets the clock, for tests.
func (s *Service) WithClock(now func() time.Time) *Service {
	s.now = now
	return s
}

func scope(ctx context.Context, p Project) context.Context {
	return tenant.WithProject(tenant.WithOrg(ctx, p.OrgID), p.ID)
}

// Enabled reports whether the project's upstream of kind is on, so its sign-in button shows.
func (s *Service) Enabled(ctx context.Context, p Project, kind upstream.Kind) (bool, error) {
	all, err := s.upstreams.List(ctx, p.OrgID, p.ID)
	if err != nil {
		return false, err
	}
	for _, u := range all {
		if u.Kind == kind && u.Enabled {
			return true, nil
		}
	}
	return false, nil
}

// Start makes one sign-in attempt's secrets after checking the upstream is on; loginHint is P7's linked upstream email, or empty.
func (s *Service) Start(ctx context.Context, p Project, kind upstream.Kind, loginHint string) (oidcrp.AuthRequest, error) {
	if _, err := s.credentials(ctx, p, kind); err != nil {
		return oidcrp.AuthRequest{}, err
	}
	req, err := oidcrp.NewAuthRequest(s.spec(kind).Prompt, loginHint)
	if err != nil {
		return oidcrp.AuthRequest{}, s.unexpected(ctx, "federation: start", err)
	}
	return req, nil
}

// AuthURL is the upstream's authorization URL for req, which sends the browser back to redirectURI.
func (s *Service) AuthURL(ctx context.Context, p Project, kind upstream.Kind, redirectURI string, req oidcrp.AuthRequest) (string, error) {
	c, _, err := s.client(ctx, p, kind, redirectURI)
	if err != nil {
		return "", err
	}
	u, err := c.AuthCodeURL(ctx, req)
	if err != nil {
		return "", s.upstreamError(ctx, p, err)
	}
	return u, nil
}

// Complete checks the callback against req and returns the upstream's identity, keyed by the kind's canonical issuer (P3).
func (s *Service) Complete(
	ctx context.Context, p Project, kind upstream.Kind, redirectURI string, cb oidcrp.Callback, req oidcrp.AuthRequest,
) (Identity, error) {
	c, creds, err := s.client(ctx, p, kind, redirectURI)
	if err != nil {
		return Identity{}, err
	}
	tok, err := c.Exchange(ctx, cb, req)
	if err != nil {
		return Identity{}, s.upstreamError(ctx, p, err)
	}
	email, err := emailaddr.Normalize(tok.Email)
	if err != nil {
		email = ""
	}
	return Identity{
		UpstreamID: creds.Upstream.ID, Issuer: s.spec(kind).Issuer, Subject: tok.Subject, Email: email,
		EmailVerified: tok.EmailVerified && email != "", HostedDomain: tok.HostedDomain, Name: strings.TrimSpace(tok.Name), Kind: kind,
	}, nil
}

// SignIn signs id's person in (ADR-0014): a new verified email creates an end user; another end user's email is a *CollisionError.
func (s *Service) SignIn(ctx context.Context, p Project, id Identity, req SessionRequest) (*Session, error) {
	if !trustedEmail(id) {
		s.refused(ctx, p, id, "email_unverified")
		return nil, &UnverifiedEmailError{}
	}
	ctx = scope(ctx, p)
	var out *Session
	var err error
	// NOTE: A21: a racing request that took the (iss, sub) or its email first is re-read by (iss, sub) and signed in as that end user.
	for range 2 {
		out, err = s.signIn(ctx, p, id, req)
		if !errors.Is(err, errSubjectTaken) && !errors.Is(err, ErrEmailTaken) {
			break
		}
	}
	switch {
	case errors.Is(err, ErrEmailTaken), errors.Is(err, errSubjectTaken):
		return nil, s.collision(ctx, p, id)
	case IsCollisionError(err):
		s.refused(ctx, p, id, "collision")
		return nil, err
	case IsUnprovenEmailError(err):
		s.refused(ctx, p, id, "unproven_email")
		return nil, err
	case err != nil:
		return nil, s.errOrReport(ctx, "federation: sign in", err)
	}
	return out, nil
}

func trustedEmail(id Identity) bool { return id.EmailVerified && id.Email != "" }

// SECURITY: P8 = A, R-1: Google vouches only for its own mailboxes and Workspace (hd) accounts; any other email needs its inbox.
func authoritative(id Identity) bool {
	if id.HostedDomain != "" {
		return true
	}
	email := strings.ToLower(id.Email)
	return strings.HasSuffix(email, "@gmail.com") || strings.HasSuffix(email, "@googlemail.com")
}

func (s *Service) signIn(ctx context.Context, p Project, id Identity, req SessionRequest) (*Session, error) {
	var out *Session
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		now := s.now()
		stored, err := s.lookup(ctx, p, id)
		if err != nil {
			return err
		}
		if stored == nil && !authoritative(id) {
			return &UnprovenEmailError{Email: id.Email, Identity: id}
		}
		var endUserID uuid.UUID
		if stored != nil {
			endUserID = stored.EndUserID
			if err := s.store.Touch(ctx, p.ID, stored.ID, id.Email, id.EmailVerified, now); err != nil {
				return err
			}
		} else {
			if endUserID, err = s.users.Create(ctx, p, id.Email, id.Name); err != nil {
				return err
			}
			if err := s.insert(ctx, p, endUserID, id, now); err != nil {
				return err
			}
		}
		req.EndUserID, req.Method = endUserID, string(id.Kind)
		out, err = s.users.SignIn(ctx, p, req)
		return err
	})
	return out, err
}

// NOTE: A21: each read sees what committed before it, so a race winner can commit between the two; the email hit re-reads (iss, sub).
func (s *Service) lookup(ctx context.Context, p Project, id Identity) (*storedIdentity, error) {
	stored, err := s.store.BySubject(ctx, p.ID, id.Issuer, id.Subject)
	if err != nil || stored != nil {
		return stored, err
	}
	_, taken, err := s.users.EndUserIDByEmail(ctx, p, id.Email)
	if err != nil || !taken {
		return nil, err
	}
	if stored, err = s.store.BySubject(ctx, p.ID, id.Issuer, id.Subject); err != nil || stored != nil {
		return stored, err
	}
	return nil, &CollisionError{Email: id.Email, Identity: id}
}

func (s *Service) collision(ctx context.Context, p Project, id Identity) error {
	s.refused(ctx, p, id, "collision")
	return &CollisionError{Email: id.Email, Identity: id}
}

// Link links id to endUserID in the unit of work on ctx; linking it again to the same end user is a no-op (ADR-0014 LinkToUser).
func (s *Service) Link(ctx context.Context, p Project, endUserID uuid.UUID, id Identity) error {
	ctx = scope(ctx, p)
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		stored, err := s.store.BySubject(ctx, p.ID, id.Issuer, id.Subject)
		switch {
		case err != nil:
			return err
		case stored != nil && stored.EndUserID == endUserID:
			return nil
		case stored != nil:
			return &LinkedElsewhereError{}
		}
		err = s.insert(ctx, p, endUserID, id, s.now())
		if errors.Is(err, errSubjectTaken) || errors.Is(err, errUpstreamTaken) {
			return &LinkedElsewhereError{}
		}
		return err
	})
	if err != nil {
		return s.errOrReport(ctx, "federation: link", err)
	}
	s.log.LogAttrs(ctx, slog.LevelInfo, "federation: linked", slog.String("project", p.PublicID), slog.String("kind", string(id.Kind)))
	return nil
}

func (s *Service) insert(ctx context.Context, p Project, endUserID uuid.UUID, id Identity, now time.Time) error {
	publicID, err := nanoid.New(publicIDLen)
	if err != nil {
		return err
	}
	return s.store.Insert(ctx, p.OrgID, p.ID, endUserID, id, publicID, now)
}

// Unlink removes an identity unless no sign-in method remains. SECURITY: SIG019: it locks the project row first, like the switch.
func (s *Service) Unlink(ctx context.Context, p Project, endUserID uuid.UUID, identityPublicID string) error {
	ctx = scope(ctx, p)
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		emailLink, err := s.projects.LockEmailLink(ctx, p.ID)
		if err != nil {
			return err
		}
		ok, err := s.store.LockEndUser(ctx, p.ID, endUserID)
		switch {
		case err != nil:
			return err
		case !ok:
			return &IdentityNotFoundError{}
		}
		ids, err := s.store.ListByEndUser(ctx, p.ID, endUserID)
		if err != nil {
			return err
		}
		remaining := len(ids) - 1
		if emailLink {
			remaining++
		}
		if remaining < 1 {
			return &LastMethodError{}
		}
		deleted, err := s.store.Delete(ctx, p.ID, endUserID, identityPublicID)
		if err == nil && !deleted {
			return &IdentityNotFoundError{}
		}
		return err
	})
	if err != nil {
		return s.errOrReport(ctx, "federation: unlink", err)
	}
	s.log.LogAttrs(ctx, slog.LevelInfo, "federation: unlinked", slog.String("project", p.PublicID), slog.String("identity", identityPublicID))
	return nil
}

// Identities returns the end user's linked identities, oldest first.
func (s *Service) Identities(ctx context.Context, p Project, endUserID uuid.UUID) ([]Identity, error) {
	ctx = scope(ctx, p)
	ids, err := s.store.ListByEndUser(ctx, p.ID, endUserID)
	if err != nil {
		return nil, s.errOrReport(ctx, "federation: identities", err)
	}
	return ids, nil
}

// EveryEndUserLinked reports whether every end user of the project has an identity, for the email-link guard (PRJ010).
func (s *Service) EveryEndUserLinked(ctx context.Context, orgID, projectID uuid.UUID) (bool, error) {
	ctx = scope(ctx, Project{OrgID: orgID, ID: projectID})
	ok, err := s.store.EveryEndUserLinked(ctx, projectID)
	if err != nil {
		return false, s.errOrReport(ctx, "federation: every end user linked", err)
	}
	return ok, nil
}

func (s *Service) credentials(ctx context.Context, p Project, kind upstream.Kind) (*upstream.Credentials, error) {
	creds, err := s.upstreams.Credentials(ctx, p.OrgID, p.ID, kind)
	if upstream.IsNotFoundError(err) {
		return nil, &UpstreamError{Reason: reasonDisabled, Err: err}
	}
	return creds, err
}

// NOTE: F11: upstreams.google.issuerOverride swaps Google's issuer for a qa fake IdP, scheme-less in place of accounts.google.com.
func (s *Service) spec(kind upstream.Kind) upstream.Spec {
	spec, _ := upstream.SpecOf(kind)
	if kind == upstream.KindGoogle && s.cfg.GoogleIssuerOverride != "" {
		spec.Issuer = s.cfg.GoogleIssuerOverride
		spec.AcceptIssuers = []string{strings.TrimPrefix(strings.TrimPrefix(s.cfg.GoogleIssuerOverride, "https://"), "http://")}
	}
	return spec
}

// NOTE: R-6: one client per upstream, replaced when its client id, secret, redirect URI or issuer changes, so no old secret stays.
func (s *Service) client(
	ctx context.Context, p Project, kind upstream.Kind, redirectURI string,
) (*oidcrp.Client, *upstream.Credentials, error) {
	creds, err := s.credentials(ctx, p, kind)
	if err != nil {
		return nil, nil, err
	}
	spec := s.spec(kind)
	if !strings.HasPrefix(spec.Issuer, "https://") {
		return nil, nil, s.upstreamError(ctx, p, &oidcrp.Error{Reason: oidcrp.ReasonMisconfigured, Err: errors.New("the issuer is not https")})
	}
	fp := sha256.Sum256([]byte(creds.Upstream.ClientID + "\x00" + creds.ClientSecret + "\x00" + redirectURI + "\x00" + spec.Issuer))
	s.mu.Lock()
	defer s.mu.Unlock()
	if cached, ok := s.clients[creds.Upstream.ID]; ok && cached.fingerprint == fp {
		return cached.client, creds, nil
	}
	c := oidcrp.New(oidcrp.Config{
		Issuer: spec.Issuer, ClientID: creds.Upstream.ClientID, ClientSecret: creds.ClientSecret, RedirectURI: redirectURI,
		Scopes: spec.Scopes, AcceptIssuers: spec.AcceptIssuers, HTTPClient: s.cfg.HTTPClient,
	}).WithClock(s.now)
	s.clients[creds.Upstream.ID] = cachedClient{fingerprint: fp, client: c}
	return c, creds, nil
}

type cachedClient struct {
	fingerprint [sha256.Size]byte
	client      *oidcrp.Client
}

// NOTE: A16, BL-7: a misconfigured upstream logs at ERROR, a person's own cancel at INFO, any other upstream failure at WARN.
func (s *Service) upstreamError(ctx context.Context, p Project, err error) error {
	reason, level := oidcrp.ReasonOf(err), slog.LevelWarn
	switch reason {
	case oidcrp.ReasonMisconfigured:
		level = slog.LevelError
	case oidcrp.ReasonCancelled:
		level = slog.LevelInfo
	}
	s.log.LogAttrs(ctx, level, "federation: upstream refused", slog.String("project", p.PublicID),
		slog.String("reason", string(reason)), slog.String("error", err.Error()))
	return &UpstreamError{Reason: reason, Err: err}
}

func (s *Service) refused(ctx context.Context, p Project, id Identity, reason string) {
	s.log.LogAttrs(ctx, slog.LevelInfo, "federation: sign-in refused", slog.String("project", p.PublicID),
		slog.String("kind", string(id.Kind)), slog.String("reason", reason))
}

func (s *Service) errOrReport(ctx context.Context, message string, err error) error {
	if IsExpectedError(err) {
		return err
	}
	if _, ok := apperror.AsAppError(err); ok {
		return err
	}
	return s.unexpected(ctx, message, err)
}
