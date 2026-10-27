package client

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/internal/kernel/secret"
	"github.com/altalune-id/othra/internal/kernel/tenant"
	"github.com/altalune-id/othra/internal/web/layout"
	"github.com/altalune-id/othra/nanoid"
)

const publicIDLen = 21

// MaxGrace is the longest an old secret keeps working after a rotation.
const MaxGrace = secret.MaxGrace

// Service registers OAuth clients and manages their secrets.
type Service struct {
	store      Store
	uow        db.Transactor
	issuer     IssuerLock
	unexpected apperror.UnexpectedFunc
	now        func() time.Time
	newSecret  func() (plaintext, hash, hint string, err error)
	log        *slog.Logger

	fetcher          MetadataFetcher
	links            Links
	meta             MetadataConfig
	fetchLimits      layout.Limits
	onMetadataChange func(ctx context.Context, orgID, projectID, clientID uuid.UUID) error
}

// NewService builds the Service; issuer locks the issuer handle at the first client, fetcher and meta admit metadata document clients.
func NewService(
	store Store, uow db.Transactor, issuer IssuerLock, fetcher MetadataFetcher, meta MetadataConfig, unexpected apperror.UnexpectedFunc,
) *Service {
	return &Service{
		store: store, uow: uow, issuer: issuer, fetcher: fetcher, meta: meta, unexpected: unexpected, now: time.Now, newSecret: NewSecret,
		log: slog.New(slog.DiscardHandler),
	}
}

// WithLogger sets where secret changes are logged.
func (s *Service) WithLogger(log *slog.Logger) *Service {
	s.log = log
	return s
}

func scope(ctx context.Context, orgID, projectID uuid.UUID) context.Context {
	return tenant.WithProject(tenant.WithOrg(ctx, orgID), projectID)
}

// Create registers a client of kind with settings and locks the project's issuer handle in the same unit of work.
func (s *Service) Create(ctx context.Context, orgID, projectID uuid.UUID, kind Kind, settings Settings) (*Client, error) {
	ctx = scope(ctx, orgID, projectID)
	system, err := s.issuer.IsSystem(ctx, projectID)
	switch {
	case err != nil:
		return nil, s.errOrReport(ctx, "client: create", err)
	case system:
		return nil, &SystemProjectError{}
	}
	publicID, err := nanoid.New(publicIDLen)
	if err != nil {
		return nil, s.unexpected(ctx, "client: create", err)
	}
	now := s.now().UTC()
	c := &Client{
		ID: uuid.New(), PublicID: publicID, OrgID: orgID, ProjectID: projectID,
		CreatedAt: now, UpdatedAt: now, Secrets: []Secret{},
	}
	c.Preset(kind)
	if err := c.Apply(settings); err != nil {
		return nil, err
	}
	err = s.uow.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.store.Insert(ctx, c); err != nil {
			return err
		}
		return s.issuer.LockIssuerHandle(ctx, projectID)
	})
	if err != nil {
		return nil, s.errOrReport(ctx, "client: create", err)
	}
	return c, nil
}

// CreateSystem seeds a project's console client and locks the project's issuer handle, in the caller's unit of work.
func (s *Service) CreateSystem(ctx context.Context, orgID, projectID uuid.UUID, redirectURI, postLogoutURI string) (*Client, error) {
	ctx = scope(ctx, orgID, projectID)
	publicID, err := nanoid.New(publicIDLen)
	if err != nil {
		return nil, s.unexpected(ctx, "client: create system", err)
	}
	c, err := NewSystem(orgID, projectID, publicID, redirectURI, postLogoutURI, s.now().UTC())
	if err != nil {
		return nil, err
	}
	err = s.uow.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.store.Insert(ctx, c); err != nil {
			return err
		}
		return s.issuer.LockIssuerHandle(ctx, projectID)
	})
	if err != nil {
		return nil, s.errOrReport(ctx, "client: create system", err)
	}
	return c, nil
}

// ReconcileSystem rewrites the project's console client URIs under its row lock and reports whether they changed.
func (s *Service) ReconcileSystem(
	ctx context.Context, orgID, projectID uuid.UUID, redirectURI, postLogoutURI string,
) (changed bool, err error) {
	ctx = scope(ctx, orgID, projectID)
	err = s.uow.RunInTx(ctx, func(ctx context.Context) error {
		c, err := s.store.LockSystem(ctx, projectID, s.now())
		if IsNotFoundError(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if changed, err = c.SetSystemURIs(redirectURI, postLogoutURI); err != nil || !changed {
			return err
		}
		c.UpdatedAt = s.now().UTC()
		return s.store.Update(ctx, c)
	})
	if err != nil {
		return false, s.errOrReport(ctx, "client: reconcile system", err)
	}
	return changed, nil
}

// System returns the project's console client, or *NotFoundError when it has none.
func (s *Service) System(ctx context.Context, orgID, projectID uuid.UUID) (*Client, error) {
	list, err := s.List(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	for _, c := range list {
		if c.System {
			return c, nil
		}
	}
	return nil, &NotFoundError{}
}

// Update replaces a client's settings; its kind never changes.
func (s *Service) Update(ctx context.Context, orgID, projectID uuid.UUID, publicID string, settings Settings) (*Client, error) {
	ctx = scope(ctx, orgID, projectID)
	var out *Client
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		c, err := s.store.Lock(ctx, projectID, publicID, s.now())
		switch {
		case err != nil:
			return err
		case c.System:
			return &SystemManagedError{}
		case c.Managed():
			return &MetadataManagedError{}
		}
		if err := c.Apply(settings); err != nil {
			return err
		}
		c.UpdatedAt = s.now().UTC()
		out = c
		return s.store.Update(ctx, c)
	})
	if err != nil {
		return nil, s.errOrReport(ctx, "client: update", err)
	}
	return out, nil
}

// Delete removes a client with its secrets and links.
func (s *Service) Delete(ctx context.Context, orgID, projectID uuid.UUID, publicID string) error {
	ctx = scope(ctx, orgID, projectID)
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		c, err := s.store.Lock(ctx, projectID, publicID, s.now())
		switch {
		case err != nil:
			return err
		case c.System:
			return &SystemManagedError{}
		}
		return s.store.Delete(ctx, projectID, c.ID)
	})
	if err != nil {
		return s.errOrReport(ctx, "client: delete", err)
	}
	return nil
}

// List returns a project's clients, oldest first.
func (s *Service) List(ctx context.Context, orgID, projectID uuid.UUID) ([]*Client, error) {
	ctx = scope(ctx, orgID, projectID)
	out, err := s.store.List(ctx, projectID, s.now())
	if err != nil {
		return nil, s.errOrReport(ctx, "client: list", err)
	}
	return out, nil
}

// Get returns one client of a project by its client_id, a public id or a stored metadata document URL, or *NotFoundError.
func (s *Service) Get(ctx context.Context, orgID, projectID uuid.UUID, publicID string) (*Client, error) {
	ctx = scope(ctx, orgID, projectID)
	c, err := s.store.ByPublicID(ctx, projectID, publicID, s.now())
	if err != nil {
		return nil, s.errOrReport(ctx, "client: get", err)
	}
	return c, nil
}

// ByIDs returns the project's clients with the given ids, metadata document clients included.
func (s *Service) ByIDs(ctx context.Context, orgID, projectID uuid.UUID, ids []uuid.UUID) ([]*Client, error) {
	ctx = scope(ctx, orgID, projectID)
	out, err := s.store.ByIDs(ctx, projectID, ids)
	if err != nil {
		return nil, s.errOrReport(ctx, "client: by ids", err)
	}
	return out, nil
}

// Authenticate returns one client of a project by its client_id, able to VerifySecret, or *NotFoundError.
func (s *Service) Authenticate(ctx context.Context, orgID, projectID uuid.UUID, publicID string) (*Client, error) {
	ctx = scope(ctx, orgID, projectID)
	c, err := s.store.ForAuthentication(ctx, projectID, publicID, s.now())
	if err != nil {
		return nil, s.errOrReport(ctx, "client: authenticate", err)
	}
	return c, nil
}

// GenerateSecret gives a web client without a current secret a new one and returns its plaintext once.
func (s *Service) GenerateSecret(ctx context.Context, orgID, projectID uuid.UUID, publicID string) (*Client, string, error) {
	return s.issueSecret(ctx, orgID, projectID, publicID, "client: generate secret", func(_ context.Context, c *Client, _ time.Time) error {
		if _, ok := c.CurrentSecret(); ok {
			return &SecretExistsError{}
		}
		return nil
	})
}

// RotateSecret makes a new current secret; the old one keeps working for grace, and an older one still in its grace ends at once.
func (s *Service) RotateSecret(
	ctx context.Context, orgID, projectID uuid.UUID, publicID string, grace time.Duration,
) (*Client, string, error) {
	return s.issueSecret(ctx, orgID, projectID, publicID, "client: rotate secret", func(ctx context.Context, c *Client, now time.Time) error {
		r := secret.Rotate(c.Secrets, grace, now)
		for _, id := range r.Expire {
			if err := s.store.ExpireSecret(ctx, c.ProjectID, c.ID, id, r.At); err != nil {
				return err
			}
		}
		return s.store.DeleteSecrets(ctx, c.ProjectID, c.ID, r.Drop, now)
	})
}

func (s *Service) issueSecret(
	ctx context.Context, orgID, projectID uuid.UUID, publicID, message string,
	before func(ctx context.Context, c *Client, now time.Time) error,
) (*Client, string, error) {
	ctx = scope(ctx, orgID, projectID)
	var (
		out       *Client
		plaintext string
		issued    Secret
	)
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		now := s.now().UTC()
		c, err := s.store.Lock(ctx, projectID, publicID, now)
		if err := secretsAllowed(c, err); err != nil {
			return err
		}
		if err := before(ctx, c, now); err != nil {
			return err
		}
		pt, hash, hint, err := s.newSecret()
		if err != nil {
			return err
		}
		id, err := nanoid.New(publicIDLen)
		if err != nil {
			return err
		}
		issued = Secret{ID: uuid.New(), PublicID: id, Hint: hint, CreatedAt: now}
		if err := s.store.InsertSecret(ctx, c, issued, hash); err != nil {
			return err
		}
		out, err = s.store.ByPublicID(ctx, projectID, publicID, now)
		plaintext = pt
		return err
	})
	if err != nil {
		return nil, "", s.errOrReport(ctx, message, err)
	}
	s.log.LogAttrs(ctx, slog.LevelInfo, message,
		slog.String("client", publicID), slog.String("secret_id", issued.PublicID), slog.Int("live_secrets", len(out.Secrets)))
	return out, plaintext, nil
}

// RevokeSecret ends one of a web client's secrets at once.
func (s *Service) RevokeSecret(ctx context.Context, orgID, projectID uuid.UUID, publicID, secretPublicID string) (*Client, error) {
	ctx = scope(ctx, orgID, projectID)
	var out *Client
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		now := s.now().UTC()
		c, err := s.store.Lock(ctx, projectID, publicID, now)
		if err := secretsAllowed(c, err); err != nil {
			return err
		}
		var target *Secret
		for i := range c.Secrets {
			if c.Secrets[i].PublicID == secretPublicID {
				target = &c.Secrets[i]
			}
		}
		if target == nil {
			return &SecretNotFoundError{}
		}
		if err := s.store.DeleteSecrets(ctx, c.ProjectID, c.ID, []uuid.UUID{target.ID}, now); err != nil {
			return err
		}
		out, err = s.store.ByPublicID(ctx, projectID, publicID, now)
		return err
	})
	if err != nil {
		return nil, s.errOrReport(ctx, "client: revoke secret", err)
	}
	s.log.LogAttrs(ctx, slog.LevelInfo, "client: revoke secret",
		slog.String("client", publicID), slog.String("secret_id", secretPublicID), slog.Int("live_secrets", len(out.Secrets)))
	return out, nil
}

func secretsAllowed(c *Client, err error) error {
	switch {
	case err != nil:
		return err
	case c.System:
		return &SystemManagedError{}
	case c.Managed():
		return &MetadataManagedError{}
	case !c.UsesSecret():
		return &NoSecretError{}
	}
	return nil
}

func (s *Service) errOrReport(ctx context.Context, message string, err error) error {
	if _, reported := apperror.AsAppError(err); reported || IsExpectedError(err) {
		return err
	}
	return s.unexpected(ctx, message, err)
}
