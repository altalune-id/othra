package project

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/kernel/storage"
	"altalune.id/othra/internal/kernel/tenant"
	slugpkg "altalune.id/othra/slug"
)

// Service is the project application service: the driving port surfaces call.
type Service struct {
	store       Store
	unexpected  apperror.UnexpectedFunc
	orgSlugs    OrgSlugs
	issuerBase  string
	files       FileStorage
	tx          db.Transactor
	members     Members
	afterCreate func(ctx context.Context, p *Project) error
	onIssuer    func(projectID uuid.UUID)
	now         func() time.Time
}

// NewService builds a Service over store, without file storage until WithStorage.
func NewService(store Store, unexpected apperror.UnexpectedFunc) *Service {
	return &Service{store: store, unexpected: unexpected, files: noStorage{}, now: time.Now}
}

// WithStorage keeps project files such as the logo in files, attaching them in units of work run by tx.
func (s *Service) WithStorage(files FileStorage, tx db.Transactor) *Service {
	s.files, s.tx = files, tx
	return s
}

// WithAfterCreate runs hook in the new project's unit of work, so a failing hook rolls the project back.
func (s *Service) WithAfterCreate(hook func(ctx context.Context, p *Project) error) *Service {
	s.afterCreate = hook
	return s
}

// WithOnIssuerChange calls fn with a project's id after its issuer handle changes.
func (s *Service) WithOnIssuerChange(fn func(projectID uuid.UUID)) *Service {
	s.onIssuer = fn
	return s
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
		hookErr, saveErr := s.save(ctx, p)
		last = saveErr
		switch {
		case hookErr != nil:
			return nil, s.errOrReport(ctx, "project: after create", hookErr)
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

func (s *Service) save(ctx context.Context, p *Project) (hookErr, err error) {
	run := func(ctx context.Context) error {
		if err := s.store.Save(ctx, p); err != nil {
			return err
		}
		if s.afterCreate == nil {
			return nil
		}
		hookErr = s.afterCreate(ctx, p)
		return hookErr
	}
	if s.tx == nil {
		return hookErr, run(ctx)
	}
	// SECURITY: a new project's unit of work is scoped to its org only, never the caller's active project.
	scoped := tenant.Into(ctx, tenant.Context{OrgID: p.OrgID, UserID: tenant.Peek(ctx).UserID})
	err = s.tx.RunInTx(scoped, run)
	return hookErr, err
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
	if s.onIssuer != nil {
		s.onIssuer(p.ID)
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

// WithMembers lets a person list the projects of every org they belong to, not only the active one.
func (s *Service) WithMembers(m Members) *Service {
	s.members = m
	return s
}

// IsMember reports whether userID belongs to orgID; without WithMembers nobody does.
func (s *Service) IsMember(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	if s.members == nil {
		return false, nil
	}
	ok, err := s.members.IsMember(ctx, orgID, userID)
	if err != nil {
		return false, s.errOrReport(ctx, "project: membership", err)
	}
	return ok, nil
}

// LogoPurpose is where a project's logo lives in file storage: a public PNG, JPEG or WebP of at most 256 KB.
func LogoPurpose() storage.Purpose {
	return storage.Purpose{
		Name: "logo", OwnerKind: storage.OwnerProjects, Visibility: storage.Public,
		Types: []string{"image/png", "image/jpeg", "image/webp"}, MaxBytes: logoMaxBytes,
	}
}

// LogoURL is the public address of a logo stored under key, or "" without one or without file storage.
func (s *Service) LogoURL(key string) string {
	if key == "" || !s.files.Enabled() {
		return ""
	}
	return s.files.PublicURL(key)
}

// CreateLogoUpload checks a new logo's declared type and size and signs its direct upload to file storage.
func (s *Service) CreateLogoUpload(
	ctx context.Context, id uuid.UUID, contentType string, size int64,
) (*storage.File, storage.Upload, error) {
	if !s.files.Enabled() {
		return nil, storage.Upload{}, &storage.NotConfiguredError{}
	}
	p, err := s.store.ByID(ctx, id)
	if err != nil {
		return nil, storage.Upload{}, s.errOrReport(ctx, "project: create logo upload", err)
	}
	f, up, err := s.files.BeginUpload(ctx, storage.BeginUploadInput{
		Purpose: LogoPurpose(), Owner: logoOwner(p), ContentType: contentType, Size: size,
	})
	switch {
	case storage.IsTypeNotAllowedError(err):
		return nil, storage.Upload{}, &InvalidLogoError{Reason: LogoType}
	case storage.IsTooLargeError(err):
		return nil, storage.Upload{}, &InvalidLogoError{Reason: LogoSize}
	case err != nil:
		return nil, storage.Upload{}, err
	}
	return f, up, nil
}

// CompleteLogoUpload verifies a finished upload and makes it the project's logo; the logo it replaces goes to the sweep.
func (s *Service) CompleteLogoUpload(ctx context.Context, id uuid.UUID, uploadID string) (*Project, error) {
	if !s.files.Enabled() {
		return nil, &storage.NotConfiguredError{}
	}
	p, err := s.store.ByID(ctx, id)
	if err != nil {
		return nil, s.errOrReport(ctx, "project: complete logo upload", err)
	}
	f, err := s.files.Verify(ctx, uploadID, LogoPurpose(), logoOwner(p))
	if err != nil {
		return nil, err
	}
	err = s.tx.RunInTx(tenant.WithOrg(ctx, p.OrgID), func(ctx context.Context) error {
		if err := s.files.MarkReady(ctx, f.ID); err != nil {
			return err
		}
		return s.swapLogo(ctx, p.ID, &f.ID)
	})
	if err != nil {
		return nil, s.errOrReport(ctx, "project: complete logo upload", err)
	}
	return s.reload(ctx, "project: complete logo upload", p.ID)
}

// DeleteLogo removes a project's logo and hands its file to the sweep.
func (s *Service) DeleteLogo(ctx context.Context, id uuid.UUID) (*Project, error) {
	p, err := s.store.ByID(ctx, id)
	if err != nil {
		return nil, s.errOrReport(ctx, "project: delete logo", err)
	}
	if p.LogoFileID == nil {
		return p, nil
	}
	if !s.files.Enabled() {
		return nil, &storage.NotConfiguredError{}
	}
	err = s.tx.RunInTx(tenant.WithOrg(ctx, p.OrgID), func(ctx context.Context) error {
		return s.swapLogo(ctx, p.ID, nil)
	})
	if err != nil {
		return nil, s.errOrReport(ctx, "project: delete logo", err)
	}
	return s.reload(ctx, "project: delete logo", p.ID)
}

func (s *Service) swapLogo(ctx context.Context, projectID uuid.UUID, fileID *uuid.UUID) error {
	previous, err := s.store.SetLogoFile(ctx, projectID, fileID)
	if err != nil {
		return err
	}
	if previous == nil || (fileID != nil && *previous == *fileID) {
		return nil
	}
	return s.files.MarkDeleting(ctx, *previous)
}

func logoOwner(p *Project) storage.Owner {
	return storage.Owner{OrgID: p.OrgID, ID: p.ID, PublicID: p.PublicID}
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
	if IsExpectedError(err) || storage.IsExpectedError(err) {
		return err
	}
	return s.unexpected(ctx, message, err)
}

type noStorage struct{}

func (noStorage) Enabled() bool { return false }

func (noStorage) BeginUpload(context.Context, storage.BeginUploadInput) (*storage.File, storage.Upload, error) {
	return nil, storage.Upload{}, &storage.NotConfiguredError{}
}

func (noStorage) Verify(context.Context, string, storage.Purpose, storage.Owner) (*storage.File, error) {
	return nil, &storage.NotConfiguredError{}
}

func (noStorage) MarkReady(context.Context, uuid.UUID) error { return &storage.NotConfiguredError{} }

func (noStorage) MarkDeleting(context.Context, uuid.UUID) error { return &storage.NotConfiguredError{} }

func (noStorage) PublicURL(string) string { return "" }
