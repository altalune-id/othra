package storage

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Store is the driven port for the files table; every call is scoped to the org on ctx.
type Store interface {
	// Insert records a new file.
	Insert(ctx context.Context, f *File) error
	// ByPublicID loads a file, or *UploadNotFoundError when the org has none with that public id.
	ByPublicID(ctx context.Context, publicID string) (*File, error)
	// MarkReady moves a pending file to ready at at; a file already ready is left as is.
	MarkReady(ctx context.Context, id uuid.UUID, at time.Time) error
	// MarkDeleting hands a file to the sweep.
	MarkDeleting(ctx context.Context, id uuid.UUID) error
	// Sweepable returns up to limit files that are deleting, or pending past their expiry at now.
	Sweepable(ctx context.Context, now time.Time, limit int) ([]*File, error)
	// Delete removes the file row id.
	Delete(ctx context.Context, id uuid.UUID) error
}
