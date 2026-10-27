package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"

	"github.com/altalune-id/othra/internal/kernel/db"
	"github.com/altalune-id/othra/schema/entity"
)

// Insert records a new file.
func (s *postgresStore) Insert(ctx context.Context, f *File) error {
	t := s.files
	completed := entity.NullTimestampz()
	if f.CompletedAt != nil {
		completed = postgres.TimestampzT(f.CompletedAt.UTC())
	}
	stmt := t.INSERT(t.AllColumns).VALUES(
		f.ID, f.PublicID, f.OrgID, string(f.OwnerKind), f.OwnerID, f.Purpose, string(f.Visibility), f.ObjectKey,
		f.ContentType, f.SizeBytes, f.OriginalName, string(f.Status), f.CreatedAt.UTC(), completed, f.ExpiresAt.UTC(),
	)
	if err := db.Write(ctx, s.pool, func(q qrm.DB) error { _, err := stmt.ExecContext(ctx, q); return err }); err != nil {
		return fmt.Errorf("storage.postgres: Insert: %w", err)
	}
	return nil
}

// MarkReady moves a pending file to ready at at; a file already ready is left as is.
func (s *postgresStore) MarkReady(ctx context.Context, id uuid.UUID, at time.Time) error {
	t := s.files
	stmt := t.UPDATE(t.Status, t.CompletedAt).
		SET(postgres.String(string(StatusReady)), postgres.TimestampzT(at.UTC())).
		WHERE(t.ID.EQ(postgres.UUID(id)).AND(t.Status.EQ(postgres.String(string(StatusPending)))))
	n, err := s.exec(ctx, stmt)
	if err != nil {
		return fmt.Errorf("storage.postgres: MarkReady: %w", err)
	}
	if n == 0 {
		return s.expectStatus(ctx, id, StatusReady)
	}
	return nil
}

// MarkDeleting hands a file to the sweep.
func (s *postgresStore) MarkDeleting(ctx context.Context, id uuid.UUID) error {
	t := s.files
	stmt := t.UPDATE(t.Status).SET(postgres.String(string(StatusDeleting))).WHERE(t.ID.EQ(postgres.UUID(id)))
	n, err := s.exec(ctx, stmt)
	if err != nil {
		return fmt.Errorf("storage.postgres: MarkDeleting: %w", err)
	}
	if n == 0 {
		return &UploadNotFoundError{PublicID: id.String()}
	}
	return nil
}

// Delete removes the file row id.
func (s *postgresStore) Delete(ctx context.Context, id uuid.UUID) error {
	t := s.files
	if _, err := s.exec(ctx, t.DELETE().WHERE(t.ID.EQ(postgres.UUID(id)))); err != nil {
		return fmt.Errorf("storage.postgres: Delete: %w", err)
	}
	return nil
}

func (s *postgresStore) exec(ctx context.Context, stmt postgres.Statement) (int64, error) {
	var n int64
	err := db.Write(ctx, s.pool, func(q qrm.DB) error {
		res, err := stmt.ExecContext(ctx, q)
		if err != nil {
			return err
		}
		n, err = res.RowsAffected()
		return err
	})
	return n, err
}

func (s *postgresStore) expectStatus(ctx context.Context, id uuid.UUID, want Status) error {
	t := s.files
	stmt := postgres.SELECT(t.Status).FROM(t).WHERE(t.ID.EQ(postgres.UUID(id)))
	var row struct {
		Status string `alias:"files.status"`
	}
	err := db.Write(ctx, s.pool, func(q qrm.DB) error { return stmt.QueryContext(ctx, q, &row) })
	switch {
	case db.IsNoRows(err):
		return &UploadNotFoundError{PublicID: id.String()}
	case err != nil:
		return fmt.Errorf("storage.postgres: status: %w", err)
	case Status(row.Status) != want:
		return &UploadNotFoundError{PublicID: id.String()}
	}
	return nil
}
