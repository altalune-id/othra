package project

import (
	"context"

	"github.com/google/uuid"

	"github.com/altalune-id/othra/internal/kernel/storage"
)

// OrgRefs turns an org's public id into its id and back; ok is false for an unknown public id.
type OrgRefs interface {
	OrgID(ctx context.Context, publicID string) (id uuid.UUID, ok bool, err error)
	OrgPublicID(ctx context.Context, id uuid.UUID) (string, error)
}

// FileStorage holds the files a project owns, such as its logo; MarkReady and MarkDeleting join the caller's unit of work.
type FileStorage interface {
	Enabled() bool
	BeginUpload(ctx context.Context, in storage.BeginUploadInput) (*storage.File, storage.Upload, error)
	Verify(ctx context.Context, filePublicID string, p storage.Purpose, owner storage.Owner) (*storage.File, error)
	MarkReady(ctx context.Context, fileID uuid.UUID) error
	MarkDeleting(ctx context.Context, fileID uuid.UUID) error
	PublicURL(key string) string
}

// Members reports whether a person belongs to an org.
type Members interface {
	IsMember(ctx context.Context, orgID, userID uuid.UUID) (bool, error)
}

// OrgSlugs gives an org's slug, from which a new project's default issuer handle is built.
type OrgSlugs interface {
	Slug(ctx context.Context, orgID uuid.UUID) (string, error)
}
