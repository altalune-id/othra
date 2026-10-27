package storage

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/internal/kernel/tenant"
	"github.com/altalune-id/othra/nanoid"
)

const publicIDLen = 21

// Service signs uploads, verifies them and builds URLs; with no driver it is disabled and every call returns *NotConfiguredError.
type Service struct {
	driver     Driver
	store      Store
	cfg        Config
	unexpected apperror.UnexpectedFunc
	now        func() time.Time
}

// NewService builds the feature over driver and store; a nil driver disables it.
func NewService(driver Driver, store Store, cfg Config, unexpected apperror.UnexpectedFunc) *Service {
	return &Service{driver: driver, store: store, cfg: cfg, unexpected: unexpected, now: time.Now}
}

// Enabled reports whether this server has file storage.
func (s *Service) Enabled() bool { return s != nil && s.driver != nil }

// BeginUploadInput is what a domain declares about an upload before the browser sends it.
type BeginUploadInput struct {
	Purpose      Purpose
	Owner        Owner
	ContentType  string
	Size         int64
	OriginalName string
}

// BeginUpload checks the declaration against the purpose, records a pending file and signs its upload.
func (s *Service) BeginUpload(ctx context.Context, in BeginUploadInput) (*File, Upload, error) {
	if !s.Enabled() {
		return nil, Upload{}, &NotConfiguredError{}
	}
	p := in.Purpose
	if !p.Allows(in.ContentType) {
		return nil, Upload{}, &TypeNotAllowedError{Purpose: p.Name, ContentType: in.ContentType}
	}
	if in.Size < 1 {
		return nil, Upload{}, apperror.Invalid("size_bytes", apperror.FieldRequired)
	}
	if in.Size > p.MaxBytes {
		return nil, Upload{}, &TooLargeError{Purpose: p.Name, MaxBytes: p.MaxBytes}
	}
	ctx = tenant.WithOrg(ctx, in.Owner.OrgID)
	publicID, err := nanoid.New(publicIDLen)
	if err != nil {
		return nil, Upload{}, s.unexpected(ctx, "storage: begin upload", err)
	}
	now := s.now().UTC()
	f := &File{
		ID: uuid.New(), PublicID: publicID, OrgID: in.Owner.OrgID, OwnerKind: p.OwnerKind, OwnerID: in.Owner.ID,
		Purpose: p.Name, Visibility: p.Visibility,
		ObjectKey:   Key(s.cfg.App, p, in.Owner.PublicID, publicID, Extension(in.ContentType)),
		ContentType: in.ContentType, SizeBytes: in.Size, OriginalName: in.OriginalName,
		Status: StatusPending, CreatedAt: now, ExpiresAt: now.Add(s.cfg.PendingTTL),
	}
	if err := s.store.Insert(ctx, f); err != nil {
		return nil, Upload{}, s.unexpected(ctx, "storage: begin upload", err)
	}
	up, err := s.driver.PresignPut(ctx, f.ObjectKey, f.ContentType, f.SizeBytes, s.cfg.UploadURLTTL)
	if err != nil {
		return nil, Upload{}, s.unexpected(ctx, "storage: begin upload", err)
	}
	return f, up, nil
}

// Verify checks a finished upload against its declaration, outside any transaction; a mismatch deletes the object.
func (s *Service) Verify(ctx context.Context, filePublicID string, p Purpose, owner Owner) (*File, error) {
	if !s.Enabled() {
		return nil, &NotConfiguredError{}
	}
	ctx = tenant.WithOrg(ctx, owner.OrgID)
	f, err := s.store.ByPublicID(ctx, filePublicID)
	if err != nil {
		return nil, s.errOrReport(ctx, "storage: verify", err)
	}
	// SECURITY: an upload made for another owner or purpose answers the same as an unknown one.
	if f.OwnerID != owner.ID || f.OwnerKind != p.OwnerKind || f.Purpose != p.Name {
		return nil, &UploadNotFoundError{PublicID: filePublicID}
	}
	switch {
	case f.Status == StatusReady:
		return f, nil
	case f.Status != StatusPending, !s.now().Before(f.ExpiresAt):
		return nil, &UploadNotFoundError{PublicID: filePublicID}
	}
	obj, err := s.driver.Head(ctx, f.ObjectKey)
	if IsObjectNotFoundError(err) {
		return nil, &UploadMismatchError{PublicID: filePublicID, Reason: MismatchMissing}
	}
	if err != nil {
		return nil, s.unexpected(ctx, "storage: verify", err)
	}
	reason := mismatch(f, obj)
	if reason == "" {
		head, err := s.driver.ReadPrefix(ctx, f.ObjectKey, sniffLen)
		if err != nil {
			return nil, s.unexpected(ctx, "storage: verify", err)
		}
		if !sniffMatches(f.ContentType, head) {
			reason = MismatchContent
		}
	}
	if reason != "" {
		return nil, s.reject(ctx, f, reason)
	}
	return f, nil
}

func mismatch(f *File, obj Object) MismatchReason {
	switch {
	case obj.Size > f.SizeBytes || obj.Size < 1:
		return MismatchSize
	case !strings.EqualFold(mediaType(obj.ContentType), f.ContentType):
		return MismatchType
	}
	return ""
}

func mediaType(contentType string) string {
	mt, _, _ := strings.Cut(contentType, ";")
	return strings.TrimSpace(mt)
}

func (s *Service) reject(ctx context.Context, f *File, reason MismatchReason) error {
	if err := s.driver.Delete(ctx, f.ObjectKey); err != nil {
		return s.unexpected(ctx, "storage: verify", err)
	}
	if err := s.store.MarkDeleting(ctx, f.ID); err != nil {
		return s.unexpected(ctx, "storage: verify", err)
	}
	return &UploadMismatchError{PublicID: f.PublicID, Reason: reason}
}

// MarkReady attaches a verified file; it joins the caller's unit of work.
func (s *Service) MarkReady(ctx context.Context, fileID uuid.UUID) error {
	if !s.Enabled() {
		return &NotConfiguredError{}
	}
	return s.errOrReport(ctx, "storage: mark ready", s.store.MarkReady(ctx, fileID, s.now()))
}

// MarkDeleting hands a replaced or removed file to the sweep; it joins the caller's unit of work.
func (s *Service) MarkDeleting(ctx context.Context, fileID uuid.UUID) error {
	if !s.Enabled() {
		return &NotConfiguredError{}
	}
	return s.errOrReport(ctx, "storage: mark deleting", s.store.MarkDeleting(ctx, fileID))
}

// PublicURL is the address of a public object, under storage.publicBaseURL.
func (s *Service) PublicURL(key string) string {
	if key == "" {
		return ""
	}
	return strings.TrimSuffix(s.cfg.PublicBaseURL, "/") + "/" + key
}

// ImageURL is a public image resized to width when an image transform is on, else the original.
func (s *Service) ImageURL(key string, width int) string {
	if key == "" || s.cfg.ImageTransform != ImageTransformCloudflare || width < 1 {
		return s.PublicURL(key)
	}
	return strings.TrimSuffix(s.cfg.PublicBaseURL, "/") + "/cdn-cgi/image/width=" + strconv.Itoa(width) + ",format=auto/" + key
}

// PrivateURL signs a short-lived GET of a private object.
func (s *Service) PrivateURL(ctx context.Context, key string) (string, error) {
	if !s.Enabled() {
		return "", &NotConfiguredError{}
	}
	u, err := s.driver.PresignGet(ctx, key, s.cfg.PrivateURLTTL)
	if err != nil {
		return "", s.unexpected(ctx, "storage: private url", err)
	}
	return u, nil
}

// UploadMethod is the HTTP method every signed upload uses.
const UploadMethod = "PUT"

func (s *Service) errOrReport(ctx context.Context, message string, err error) error {
	if err == nil || IsExpectedError(err) {
		return err
	}
	return s.unexpected(ctx, message, err)
}
