package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/kernel/tenant"
	slugpkg "altalune.id/othra/slug"
)

// Service is the project application service: the driving port surfaces call.
type Service struct {
	store      Store
	unexpected apperror.UnexpectedFunc
	orgSlugs   OrgSlugs
	issuerBase string
	now        func() time.Time
}

// NewService builds a Service over store.
func NewService(store Store, unexpected apperror.UnexpectedFunc) *Service {
	return &Service{store: store, unexpected: unexpected, now: time.Now}
}

// WithOrgSlugs names the org slugs a new project's default issuer handle is built from.
func (s *Service) WithOrgSlugs(o OrgSlugs) *Service {
	s.orgSlugs = o
	return s
}

// WithIssuerBase sets the public base URL every issuer URL starts with.
func (s *Service) WithIssuerBase(base string) *Service {
	s.issuerBase = base
	return s
}

// IssuerURL is the exact issuer of the project with handle.
func (s *Service) IssuerURL(handle string) string {
	return IssuerURL(s.issuerBase, handle)
}

// CreateRequest is the input to Create.
type CreateRequest struct {
	OrgID        uuid.UUID
	Slug         string
	Name         string
	IssuerHandle string
	CreatedBy    uuid.UUID
}

// Create validates and persists a new project, filling an empty slug or issuer handle with a free default.
func (s *Service) Create(ctx context.Context, req CreateRequest) (*Project, error) {
	if req.OrgID == uuid.Nil {
		return nil, apperror.Invalid("organization_id", apperror.FieldRequired)
	}
	ctx = tenant.WithOrg(ctx, req.OrgID)
	handle := strings.TrimSpace(req.IssuerHandle)
	if handle != "" {
		if err := ValidateIssuerHandle(handle); err != nil {
			return nil, err
		}
	}
	orgSlug, err := s.orgSlug(ctx, req.OrgID)
	if err != nil {
		return nil, s.errOrReport(ctx, "project: create", err)
	}
	generated, suffixed := strings.TrimSpace(req.Slug) == "", false
	var last error
	for range slugpkg.MaxAttempts {
		slug := req.Slug
		if generated {
			slug = slugpkg.Generate()
		}
		p, err := NewProject(req.OrgID, slug, req.Name, req.CreatedBy)
		if err != nil {
			return nil, err
		}
		p.IssuerHandle = handle
		if handle == "" {
			p.IssuerHandle = defaultHandle(orgSlug, p.Slug, suffixed)
		}
		last = s.store.Save(ctx, p)
		switch {
		case last == nil:
			return p, nil
		case generated && IsAlreadyExistsError(last):
		case handle == "" && IsIssuerHandleTakenError(last):
			suffixed = true
		default:
			return nil, s.errOrReport(ctx, "project: create", last)
		}
	}
	return nil, s.errOrReport(ctx, "project: create", last)
}

func defaultHandle(orgSlug, projectSlug string, suffixed bool) string {
	h := DefaultIssuerHandle(orgSlug, projectSlug)
	if suffixed || ValidateIssuerHandle(h) != nil {
		return withIssuerSuffix(h)
	}
	return h
}

func (s *Service) orgSlug(ctx context.Context, orgID uuid.UUID) (string, error) {
	if s.orgSlugs == nil {
		return "", nil
	}
	return s.orgSlugs.Slug(ctx, orgID)
}

// Get loads a project by id.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Project, error) {
	p, err := s.store.ByID(ctx, id)
	if err != nil {
		return nil, s.errOrReport(ctx, "project: get", err)
	}
	return p, nil
}

// ByPublicID loads a project by its public id.
func (s *Service) ByPublicID(ctx context.Context, publicID string) (*Project, error) {
	p, err := s.store.ByPublicID(ctx, publicID)
	if err != nil {
		return nil, s.errOrReport(ctx, "project: by public id", err)
	}
	return p, nil
}

// BySlug loads a project by slug within an org.
func (s *Service) BySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Project, error) {
	ctx = tenant.WithOrg(ctx, orgID)
	p, err := s.store.BySlug(ctx, orgID, slug)
	if err != nil {
		return nil, s.errOrReport(ctx, "project: by slug", err)
	}
	return p, nil
}

// List returns an organization's projects.
func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]*Project, error) {
	ctx = tenant.WithOrg(ctx, orgID)
	projects, err := s.store.List(ctx, orgID)
	if err != nil {
		return nil, s.errOrReport(ctx, "project: list", err)
	}
	return projects, nil
}

// Update changes a project's display name and slug.
func (s *Service) Update(ctx context.Context, id uuid.UUID, name, slug string) (*Project, error) {
	p, err := s.store.ByID(ctx, id)
	if err != nil {
		return nil, s.errOrReport(ctx, "project: update", err)
	}
	if err := p.Update(name, slug); err != nil {
		return nil, err
	}
	if err := s.store.Save(ctx, p); err != nil {
		return nil, s.errOrReport(ctx, "project: update", err)
	}
	return p, nil
}

// UpdateIssuerHandle changes a project's issuer handle; it is refused once the project has OAuth clients.
func (s *Service) UpdateIssuerHandle(ctx context.Context, id uuid.UUID, handle string) (*Project, error) {
	p, err := s.store.ByID(ctx, id)
	if err != nil {
		return nil, s.errOrReport(ctx, "project: update issuer", err)
	}
	h := strings.TrimSpace(handle)
	if h == p.IssuerHandle {
		return p, nil
	}
	if err := p.SetIssuerHandle(h); err != nil {
		return nil, err
	}
	if err := s.store.SetIssuerHandle(ctx, p.ID, h); err != nil {
		return nil, s.errOrReport(ctx, "project: update issuer", err)
	}
	return s.reload(ctx, "project: update issuer", p.ID)
}

// LockIssuerHandle freezes a project's issuer handle; the first OAuth client's creation calls it in the same unit of work.
func (s *Service) LockIssuerHandle(ctx context.Context, id uuid.UUID) error {
	if err := s.store.LockIssuerHandle(ctx, id, s.now()); err != nil {
		return s.errOrReport(ctx, "project: lock issuer", err)
	}
	return nil
}

// UpdateBranding sets a project's display name, primary colour and support email.
func (s *Service) UpdateBranding(ctx context.Context, id uuid.UUID, displayName, primaryColor, supportEmail string) (*Project, error) {
	return s.change(ctx, "project: update branding", id, func(p *Project) error {
		return p.UpdateBranding(displayName, primaryColor, supportEmail)
	})
}

// UpdateTerms sets a project's terms and privacy links; a first terms link publishes version 1.
func (s *Service) UpdateTerms(ctx context.Context, id uuid.UUID, termsURL, privacyURL string) (*Project, error) {
	return s.change(ctx, "project: update terms", id, func(p *Project) error {
		return p.UpdateTerms(termsURL, privacyURL, s.now())
	})
}

// BumpTermsVersion publishes a new terms version, so every end user accepts again.
func (s *Service) BumpTermsVersion(ctx context.Context, id uuid.UUID) (*Project, error) {
	return s.change(ctx, "project: bump terms", id, func(p *Project) error {
		_, err := p.BumpTermsVersion(s.now())
		return err
	})
}

// UploadLogo replaces a project's logo with content, a PNG, JPEG or WebP of at most 256 KB.
func (s *Service) UploadLogo(ctx context.Context, id uuid.UUID, content []byte) (*Project, error) {
	contentType, err := ValidateLogo(content)
	if err != nil {
		return nil, err
	}
	p, err := s.store.ByID(ctx, id)
	if err != nil {
		return nil, s.errOrReport(ctx, "project: upload logo", err)
	}
	sum := sha256.Sum256(content)
	logo := &Logo{
		ProjectID: p.ID, OrgID: p.OrgID, ContentType: contentType, Content: content,
		SHA256: hex.EncodeToString(sum[:]), UpdatedAt: s.now().UTC(),
	}
	if err := s.store.SaveLogo(ctx, logo); err != nil {
		return nil, s.errOrReport(ctx, "project: upload logo", err)
	}
	return s.reload(ctx, "project: upload logo", p.ID)
}

// DeleteLogo removes a project's logo.
func (s *Service) DeleteLogo(ctx context.Context, id uuid.UUID) (*Project, error) {
	if err := s.store.DeleteLogo(ctx, id); err != nil {
		return nil, s.errOrReport(ctx, "project: delete logo", err)
	}
	return s.reload(ctx, "project: delete logo", id)
}

// Logo returns a project's logo, or nil when it has none.
func (s *Service) Logo(ctx context.Context, id uuid.UUID) (*Logo, error) {
	l, err := s.store.Logo(ctx, id)
	if err != nil {
		return nil, s.errOrReport(ctx, "project: logo", err)
	}
	return l, nil
}

// The answers of CheckIssuerHandle.
const (
	HandleOK       = "ok"
	HandleTaken    = "taken"
	HandleReserved = "reserved"
	HandleInvalid  = "invalid"
	HandleLocked   = "locked"
)

// CheckIssuerHandle reports whether handle is free for a new project, or for current when it is set.
func (s *Service) CheckIssuerHandle(ctx context.Context, handle string, current *Project) (string, error) {
	if err := ValidateIssuerHandle(handle); err != nil {
		if e, ok := errors.AsType[*InvalidIssuerHandleError](err); ok && e.Reason == IssuerHandleReserved {
			return HandleReserved, nil
		}
		return HandleInvalid, nil
	}
	if current != nil && current.IssuerHandle == handle {
		return HandleOK, nil
	}
	if current != nil && current.IssuerHandleLockedAt != nil {
		return HandleLocked, nil
	}
	taken, err := s.store.IssuerHandleExists(ctx, handle)
	if err != nil {
		return "", s.errOrReport(ctx, "project: check issuer", err)
	}
	if taken {
		return HandleTaken, nil
	}
	return HandleOK, nil
}

// ResolveIssuer finds the project behind handle before sign-in; the result is never saved.
func (s *Service) ResolveIssuer(ctx context.Context, handle string) (*Issuer, error) {
	i, err := s.store.ResolveIssuer(ctx, handle)
	if err != nil {
		return nil, s.errOrReport(ctx, "project: resolve issuer", err)
	}
	return i, nil
}

func (s *Service) change(ctx context.Context, op string, id uuid.UUID, apply func(*Project) error) (*Project, error) {
	p, err := s.store.ByID(ctx, id)
	if err != nil {
		return nil, s.errOrReport(ctx, op, err)
	}
	if err := apply(p); err != nil {
		return nil, err
	}
	if err := s.store.Save(ctx, p); err != nil {
		return nil, s.errOrReport(ctx, op, err)
	}
	return s.reload(ctx, op, p.ID)
}

func (s *Service) reload(ctx context.Context, op string, id uuid.UUID) (*Project, error) {
	p, err := s.store.ByID(ctx, id)
	if err != nil {
		return nil, s.errOrReport(ctx, op, err)
	}
	return p, nil
}

func (s *Service) errOrReport(ctx context.Context, message string, err error) error {
	if IsExpectedError(err) {
		return err
	}
	return s.unexpected(ctx, message, err)
}
