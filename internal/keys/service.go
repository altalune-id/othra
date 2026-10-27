package keys

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/internal/kernel/tenant"
	"github.com/altalune-id/othra/nanoid"
	"github.com/altalune-id/othra/sealer"
)

// CacheTTL bounds how long a process serves a project's JWKS and signing key before it reads them again.
const CacheTTL = 30 * time.Second

const publicIDLen = 21

// SigningKey is the active key with its private half, opened for signing; it is never logged or returned to a surface.
type SigningKey struct {
	KID     string
	Alg     string
	private *rsa.PrivateKey
}

type cached struct {
	jwks      []byte
	jwksUntil time.Time
	signer    *SigningKey
	signUntil time.Time
}

// Service generates, rotates and publishes each project's signing keys.
type Service struct {
	store      Store
	uow        db.Transactor
	sealer     sealer.Sealer
	cfg        Config
	unexpected apperror.UnexpectedFunc
	now        func() time.Time
	generate   func(bits int) (*rsa.PrivateKey, error)
	onChange   func(projectID uuid.UUID)
	log        *slog.Logger

	mu    sync.Mutex
	cache map[uuid.UUID]*cached
}

// NewService builds the Service; sl must be enabled, since every key is sealed.
func NewService(store Store, uow db.Transactor, sl sealer.Sealer, cfg Config, unexpected apperror.UnexpectedFunc) *Service {
	return &Service{
		store: store, uow: uow, sealer: sl, cfg: cfg, unexpected: unexpected, now: time.Now,
		generate: func(bits int) (*rsa.PrivateKey, error) { return rsa.GenerateKey(rand.Reader, bits) },
		cache:    map[uuid.UUID]*cached{},
		log:      slog.New(slog.DiscardHandler),
	}
}

// WithLogger sets where rotations and promotions are logged.
func (s *Service) WithLogger(log *slog.Logger) *Service {
	s.log = log
	return s
}

// WithOnChange calls fn with a project's id after its published keys change.
func (s *Service) WithOnChange(fn func(projectID uuid.UUID)) *Service {
	s.onChange = fn
	return s
}

func scope(ctx context.Context, orgID, projectID uuid.UUID) context.Context {
	return tenant.WithProject(tenant.WithOrg(ctx, orgID), projectID)
}

// Generate gives a new project its first, active key; it joins the caller's unit of work.
func (s *Service) Generate(ctx context.Context, orgID, projectID uuid.UUID) (*Key, error) {
	ctx = scope(ctx, orgID, projectID)
	k, sealed, err := s.newKey(orgID, projectID)
	if err != nil {
		return nil, s.unexpected(ctx, "keys: generate", err)
	}
	k.Activate(k.CreatedAt)
	if err := s.store.Insert(ctx, k, sealed); err != nil {
		return nil, s.unexpected(ctx, "keys: generate", err)
	}
	s.changed(projectID)
	return k, nil
}

// Rotate publishes a key that signs after the pre-publication delay, or at once when immediate or when none is active.
func (s *Service) Rotate(ctx context.Context, orgID, projectID uuid.UUID, immediate bool) (*Key, error) {
	ctx = scope(ctx, orgID, projectID)
	var out *Key
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		next, active, err := s.store.Live(ctx, projectID)
		if err != nil {
			return err
		}
		now := s.now().UTC()
		switch {
		case next != nil && active != nil && !immediate:
			return &RotationPendingError{ProjectID: projectID.String()}
		case next != nil:
			out = next
			return s.promote(ctx, next, active, now)
		}
		k, sealed, err := s.newKey(orgID, projectID)
		if err != nil {
			return err
		}
		if active == nil {
			k.Activate(now)
			out = k
			return s.store.Insert(ctx, k, sealed)
		}
		activates := now.Add(s.cfg.PrePublish)
		k.Status, k.ActivatesAt = StatusNext, &activates
		if immediate {
			k.ActivatesAt = &now
		}
		if err := s.store.Insert(ctx, k, sealed); err != nil {
			return err
		}
		out = k
		if immediate {
			return s.promote(ctx, k, active, now)
		}
		return nil
	})
	if err != nil {
		return nil, s.errOrReport(ctx, "keys: rotate", err)
	}
	s.changed(projectID)
	return out, nil
}

// Promote makes the project's next key active once its activates_at has passed, and reports whether it did.
func (s *Service) Promote(ctx context.Context, orgID, projectID uuid.UUID) (bool, error) {
	ctx = scope(ctx, orgID, projectID)
	promoted, kid, keyID := false, "", ""
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		next, active, err := s.store.Live(ctx, projectID)
		if err != nil || next == nil {
			return err
		}
		now := s.now().UTC()
		if next.ActivatesAt != nil && next.ActivatesAt.After(now) {
			return nil
		}
		promoted, kid, keyID = true, next.KID, next.PublicID
		return s.promote(ctx, next, active, now)
	})
	if err != nil {
		return false, s.errOrReport(ctx, "keys: promote", err)
	}
	if promoted {
		s.log.LogAttrs(ctx, slog.LevelInfo, "keys: promoted",
			slog.String("project_id", projectID.String()), slog.String("kid", kid), slog.String("key", keyID))
		s.changed(projectID)
	}
	return promoted, nil
}

// Prune unpublishes retiring keys past retire_after and deletes keys retired more than 30 days ago; it returns how many keys changed.
func (s *Service) Prune(ctx context.Context, orgID, projectID uuid.UUID) (int, error) {
	ctx = scope(ctx, orgID, projectID)
	changed := 0
	err := s.uow.RunInTx(ctx, func(ctx context.Context) error {
		now := s.now().UTC()
		due, err := s.store.Retiring(ctx, projectID, now)
		if err != nil {
			return err
		}
		for _, k := range due {
			k.Unpublish(now)
			if err := s.store.Update(ctx, k); err != nil {
				return err
			}
		}
		deleted, err := s.store.DeleteRetired(ctx, projectID, now.Add(-RetiredKeep))
		changed = len(due) + int(deleted)
		return err
	})
	if err != nil {
		return 0, s.errOrReport(ctx, "keys: prune", err)
	}
	if changed > 0 {
		s.changed(projectID)
	}
	return changed, nil
}

// List returns every key of the project, newest first.
func (s *Service) List(ctx context.Context, orgID, projectID uuid.UUID) ([]*Key, error) {
	ctx = scope(ctx, orgID, projectID)
	keys, err := s.store.List(ctx, projectID)
	if err != nil {
		return nil, s.errOrReport(ctx, "keys: list", err)
	}
	return keys, nil
}

// Published returns the keys in the project's JWKS, oldest first; ctx carries the project's tenant scope.
func (s *Service) Published(ctx context.Context, projectID uuid.UUID) ([]*Key, error) {
	keys, err := s.store.Published(ctx, projectID)
	if err != nil {
		return nil, s.errOrReport(ctx, "keys: published", err)
	}
	return keys, nil
}

// JWKS returns the project's key set as fixed bytes for the same keys; ctx carries the project's tenant scope.
func (s *Service) JWKS(ctx context.Context, projectID uuid.UUID) ([]byte, error) {
	now := s.now()
	s.mu.Lock()
	if c := s.cache[projectID]; c != nil && c.jwks != nil && now.Before(c.jwksUntil) {
		body := c.jwks
		s.mu.Unlock()
		return body, nil
	}
	s.mu.Unlock()
	keys, err := s.Published(ctx, projectID)
	if err != nil {
		return nil, err
	}
	set := JWKS{Keys: make([]JWK, 0, len(keys))}
	for _, k := range keys {
		set.Keys = append(set.Keys, k.JWK)
	}
	body, err := json.Marshal(set)
	if err != nil {
		return nil, s.unexpected(ctx, "keys: jwks", err)
	}
	s.mu.Lock()
	c := s.entry(projectID)
	c.jwks, c.jwksUntil = body, now.Add(CacheTTL)
	s.mu.Unlock()
	return body, nil
}

// ActiveKey returns the project's signing key with its private half opened; ctx carries the project's tenant scope.
func (s *Service) ActiveKey(ctx context.Context, projectID uuid.UUID) (*SigningKey, error) {
	now := s.now()
	s.mu.Lock()
	if c := s.cache[projectID]; c != nil && c.signer != nil && now.Before(c.signUntil) {
		sk := c.signer
		s.mu.Unlock()
		return sk, nil
	}
	s.mu.Unlock()
	k, sealed, err := s.store.Active(ctx, projectID)
	if err != nil {
		return nil, s.errOrReport(ctx, "keys: active", err)
	}
	der, err := s.sealer.Open(sealed, sealAAD(projectID, k.KID))
	if err != nil {
		return nil, s.unexpected(ctx, "keys: open "+k.PublicID, err)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	clear(der)
	priv, ok := parsed.(*rsa.PrivateKey)
	if err != nil || !ok {
		return nil, s.unexpected(ctx, "keys: parse "+k.PublicID, errors.New("sealed key is not an RSA PKCS#8 key"))
	}
	sk := &SigningKey{KID: k.KID, Alg: k.Alg, private: priv}
	s.mu.Lock()
	c := s.entry(projectID)
	c.signer, c.signUntil = sk, now.Add(CacheTTL)
	s.mu.Unlock()
	return sk, nil
}

func (s *Service) entry(projectID uuid.UUID) *cached {
	c := s.cache[projectID]
	if c == nil {
		c = &cached{}
		s.cache[projectID] = c
	}
	return c
}

func (s *Service) newKey(orgID, projectID uuid.UUID) (*Key, []byte, error) {
	priv, err := s.generate(s.cfg.RSABits)
	if err != nil {
		return nil, nil, fmt.Errorf("keys: generate rsa: %w", err)
	}
	jwk := PublicJWK(&priv.PublicKey)
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, fmt.Errorf("keys: encode private key: %w", err)
	}
	sealed, err := s.sealer.Seal(der, sealAAD(projectID, jwk.Kid))
	clear(der)
	if err != nil {
		return nil, nil, fmt.Errorf("keys: seal private key: %w", err)
	}
	publicID, err := nanoid.New(publicIDLen)
	if err != nil {
		return nil, nil, err
	}
	return &Key{
		ID: uuid.New(), PublicID: publicID, OrgID: orgID, ProjectID: projectID, KID: jwk.Kid, Alg: AlgRS256,
		JWK: jwk, CreatedAt: s.now().UTC(),
	}, sealed, nil
}

func (s *Service) promote(ctx context.Context, next, active *Key, now time.Time) error {
	if active != nil {
		active.Retire(now, s.cfg.Overlap)
		if err := s.store.Update(ctx, active); err != nil {
			return err
		}
	}
	next.Activate(now)
	return s.store.Update(ctx, next)
}

func (s *Service) changed(projectID uuid.UUID) {
	s.mu.Lock()
	delete(s.cache, projectID)
	s.mu.Unlock()
	if s.onChange != nil {
		s.onChange(projectID)
	}
}

func (s *Service) errOrReport(ctx context.Context, message string, err error) error {
	if IsExpectedError(err) {
		return err
	}
	return s.unexpected(ctx, message, err)
}
