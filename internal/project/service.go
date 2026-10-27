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
	methods     SignInMethods
	now         func() time.Time
}

// SignInMethods says what else a project's end users can sign in with, for the email-link guard.
type SignInMethods interface {
	UpstreamEnabled(ctx context.Context, orgID, projectID uuid.UUID) (bool, error)
	AnyEndUserUnlinked(ctx context.Context, orgID, projectID uuid.UUID) (bool, error)
}

// WithSignInMethods lets the email link be turned off; without it, turning it off is always refused.
func (s *Service) WithSignInMethods(m SignInMethods) *Service {
	s.methods = m
	return s
}

// NewService builds a Service over store, issuing under issuerBase; Create runs afterCreate, unless nil, in the new project's unit of work.
func NewService(
	store Store, tx db.Transactor, members Members, afterCreate func(ctx context.Context, p *Project) error, issuerBase string,
	unexpected apperror.UnexpectedFunc,
) *Service {
	return &Service{
		store: store, tx: tx, members: members, afterCreate: afterCreate, issuerBase: issuerBase, unexpected: unexpected,
		files: noStorage{}, now: time.Now,
	}
}

// WithStorage keeps project files such as the logo in files; without it, uploads are off.
func (s *Service) WithStorage(files FileStorage) *Service {
	s.files = files
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
		return "", errNoOrgSlugs
	}
	return s.orgSlugs.Slug(ctx, orgID)
}

// CreateSystem creates the system org's console project, keys included through the after-create hook; install calls it once.
func (s *Service) CreateSystem(ctx context.Context, orgID uuid.UUID) (*Project, error) {
	p, err := NewSystemProject(orgID)
	if err != nil {
		return nil, err
	}
	hookErr, err := s.save(tenant.WithOrg(ctx, orgID), p)
	switch {
	case hookErr != nil:
		return nil, s.errOrReport(ctx, "project: after create", hookErr)
	case err != nil:
		return nil, s.errOrReport(ctx, "project: create system", err)
	}
	return p, nil
}

// Get loads a project by id.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Project, error) {
	p, err := s.store.ByID(ctx, id)
	if err != nil {
		return nil, s.errOrReport(ctx, "project: get", err)
	}
	return p, nil
}

// IsSystem reports whether project id is the console project, where only Othra adds clients and resource servers.
func (s *Service) IsSystem(ctx context.Context, id uuid.UUID) (bool, error) {
	p, err := s.Get(ctx, id)
	if err != nil {
		return false, err
	}
	return p.IsSystem(), nil
}

// ByPublicID loads a project by its public id.
func (s *Service) ByPublicID(ctx context.Context, publicID string) (*Project, error) {
	p, err := s.store.ByPublicID(ctx, publicID)
	if err != nil {
		return nil, s.errOrReport(ctx, "project: by public id", err)
	}
	return p, nil
}

// RefByPublicID loads a project's ids and slugs, with its org's slug, in one read.
func (s *Service) RefByPublicID(ctx context.Context, publicID string) (*Ref, error) {
	r, err := s.store.RefByPublicID(ctx, publicID)
	if err != nil {
		return nil, s.errOrReport(ctx, "project: ref by public id", err)
	}
	return r, nil
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
	if p.IsSystem() {
		return nil, &SystemManagedError{}
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
	if p.IsSystem() {
		return nil, &SystemManagedError{}
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

// SECURITY: with the project row locked, the email link goes off only while every end user can sign in another way.
func (s *Service) SetSignInEmailLink(ctx context.Context, id uuid.UUID, on bool) (*Project, error) {
	var out *Project
	err := s.inTx(ctx, func(ctx context.Context) error {
		if _, err := s.store.LockSignIn(ctx, id); err != nil {
			return err
		}
		p, err := s.store.ByID(ctx, id)
		switch {
		case err != nil:
			return err
		case !on && p.IsSystem():
			return &SystemManagedError{}
		}
		if !on {
			if err := s.othersRemain(ctx, p); err != nil {
				return err
			}
		}
		p.SignInEmailLink = on
		out = p
		return s.store.Save(ctx, p)
	})
	if err != nil {
		return nil, s.errOrReport(ctx, "project: sign-in", err)
	}
	return s.issuerChanged(out, nil)
}

func (s *Service) othersRemain(ctx context.Context, p *Project) error {
	if s.methods == nil {
		return &SignInLastMethodError{}
	}
	scoped := tenant.WithProject(tenant.WithOrg(ctx, p.OrgID), p.ID)
	enabled, err := s.methods.UpstreamEnabled(scoped, p.OrgID, p.ID)
	if err != nil {
		return err
	}
	unlinked, err := s.methods.AnyEndUserUnlinked(scoped, p.OrgID, p.ID)
	switch {
	case err != nil:
		return err
	case !enabled || unlinked:
		return &SignInLastMethodError{}
	}
	return nil
}

// EmailLink reports whether the project id's email link is on, without a lock, for a display.
func (s *Service) EmailLink(ctx context.Context, id uuid.UUID) (bool, error) {
	p, err := s.Get(ctx, id)
	if err != nil {
		return false, err
	}
	return p.SignInEmailLink, nil
}

// LockEmailLink locks the project id's row until the caller's unit of work ends and reports whether its email link is on.
func (s *Service) LockEmailLink(ctx context.Context, id uuid.UUID) (bool, error) {
	on, err := s.store.LockSignIn(ctx, id)
	if err != nil {
		return false, s.errOrReport(ctx, "project: lock sign-in", err)
	}
	return on, nil
}

func (s *Service) inTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return s.tx.RunInTx(ctx, fn)
}

// UpdateBranding sets a project's display name, primary colour and support email.
func (s *Service) UpdateBranding(ctx context.Context, id uuid.UUID, displayName, primaryColor, supportEmail string) (*Project, error) {
	return s.issuerChanged(s.change(ctx, "project: update branding", id, func(p *Project) error {
		return p.UpdateBranding(displayName, primaryColor, supportEmail)
	}))
}

// UpdateTerms sets a project's terms and privacy links; a first terms link publishes version 1.
func (s *Service) UpdateTerms(ctx context.Context, id uuid.UUID, termsURL, privacyURL string) (*Project, error) {
	return s.issuerChanged(s.change(ctx, "project: update terms", id, func(p *Project) error {
		return p.UpdateTerms(termsURL, privacyURL, s.now())
	}))
}

// BumpTermsVersion publishes a new terms version, so every end user accepts again.
func (s *Service) BumpTermsVersion(ctx context.Context, id uuid.UUID) (*Project, error) {
	return s.issuerChanged(s.change(ctx, "project: bump terms", id, func(p *Project) error {
		_, err := p.BumpTermsVersion(s.now())
		return err
	}))
}

// NOTE: the issuer cache holds the terms and branding the hosted pages show, so a change to either drops it at once.
func (s *Service) issuerChanged(p *Project, err error) (*Project, error) {
	if err == nil && s.onIssuer != nil {
		s.onIssuer(p.ID)
	}
	return p, err
}

// IsMember reports whether userID belongs to orgID.
func (s *Service) IsMember(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
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
	return s.issuerChanged(s.reload(ctx, "project: complete logo upload", p.ID))
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
	return s.issuerChanged(s.reload(ctx, "project: delete logo", p.ID))
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
