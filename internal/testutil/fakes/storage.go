package fakes

import (
	"context"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/storage"
	"altalune.id/othra/internal/project"
)

// FileStorage is a project.FileStorage test double that answers from its fields and records the files it marks.
type FileStorage struct {
	Disabled  bool
	BeginErr  error
	VerifyErr error
	MarkErr   error
	Files     map[string]*storage.File
	Begun     []storage.BeginUploadInput
	Ready     []uuid.UUID
	Deleting  []uuid.UUID
}

var _ project.FileStorage = (*FileStorage)(nil)

// Enabled implements project.FileStorage.
func (f *FileStorage) Enabled() bool { return !f.Disabled }

// BeginUpload implements project.FileStorage; it records in and returns a pending file with a signed PUT.
func (f *FileStorage) BeginUpload(_ context.Context, in storage.BeginUploadInput) (*storage.File, storage.Upload, error) {
	if f.Disabled {
		return nil, storage.Upload{}, &storage.NotConfiguredError{}
	}
	if f.BeginErr != nil {
		return nil, storage.Upload{}, f.BeginErr
	}
	f.Begun = append(f.Begun, in)
	file := &storage.File{
		ID: uuid.New(), PublicID: "upxdupxdupxdupxdupxd2", ContentType: in.ContentType, SizeBytes: in.Size, Status: storage.StatusPending,
	}
	return file, storage.Upload{
		URL: "https://files.test/put", Method: storage.UploadMethod,
		Headers:   map[string]string{"Content-Type": in.ContentType},
		ExpiresAt: time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC),
	}, nil
}

// Verify implements project.FileStorage; it returns the file in Files with that public id.
func (f *FileStorage) Verify(_ context.Context, filePublicID string, _ storage.Purpose, _ storage.Owner) (*storage.File, error) {
	if f.Disabled {
		return nil, &storage.NotConfiguredError{}
	}
	if f.VerifyErr != nil {
		return nil, f.VerifyErr
	}
	file, ok := f.Files[filePublicID]
	if !ok {
		return nil, &storage.UploadNotFoundError{PublicID: filePublicID}
	}
	return file, nil
}

// MarkReady implements project.FileStorage and records the file id.
func (f *FileStorage) MarkReady(_ context.Context, fileID uuid.UUID) error {
	if f.MarkErr != nil {
		return f.MarkErr
	}
	f.Ready = append(f.Ready, fileID)
	return nil
}

// MarkDeleting implements project.FileStorage and records the file id.
func (f *FileStorage) MarkDeleting(_ context.Context, fileID uuid.UUID) error {
	if f.MarkErr != nil {
		return f.MarkErr
	}
	f.Deleting = append(f.Deleting, fileID)
	return nil
}

// PublicURL implements project.FileStorage under https://files.test/.
func (f *FileStorage) PublicURL(key string) string {
	if key == "" {
		return ""
	}
	return "https://files.test/" + key
}
