//go:build integration

package storage_test

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/kernel/storage"
	storages3 "altalune.id/othra/internal/kernel/storage/s3"
	"altalune.id/othra/internal/testutil/s3test"
	s3conn "altalune.id/othra/s3"
)

var logo = storage.Purpose{
	Name: "logo", OwnerKind: storage.OwnerProjects, Visibility: storage.Public,
	Types: []string{"image/png", "image/jpeg", "image/webp"}, MaxBytes: 256 << 10,
}

func put(t *testing.T, up storage.Upload, contentType string, body []byte) int {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), up.Method, up.URL, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	return resp.StatusCode
}

func TestService_UploadVerifyAttachReplaceSweep_OnGarageAndPostgres(t *testing.T) {
	f := newFixture(t)
	s := f.seed(t)
	h := s3test.New(t)
	h.Client(t)
	conn, err := s3conn.Open(t.Context(), s3conn.Config{
		Endpoint: h.Endpoint, Region: h.Region, Bucket: h.Bucket, AccessKeyID: h.AccessKeyID,
		SecretAccessKey: h.SecretAccessKey, UsePathStyle: true, Timeout: 5 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	svc := storage.NewService(storages3.New(conn), f.store, storage.Config{
		App: h.Prefix + "othra", PublicBaseURL: "https://files.altalune.id",
		UploadURLTTL: time.Minute, PendingTTL: time.Hour, PrivateURLTTL: time.Minute,
	}, apperror.NewReporter(nil, false).Unexpected)
	owner := storage.Owner{OrgID: s.org, ID: s.project, PublicID: "prj1"}
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0xab}, 92)...)

	begin := func() (*storage.File, storage.Upload) {
		file, up, err := svc.BeginUpload(t.Context(), storage.BeginUploadInput{Purpose: logo, Owner: owner, ContentType: "image/png", Size: int64(len(png))})
		require.NoError(t, err)
		return file, up
	}

	first, up := begin()
	assert.Equal(t, http.StatusOK, put(t, up, "image/png", png))
	got, err := svc.Verify(t.Context(), first.PublicID, logo, owner)
	require.NoError(t, err)
	require.NoError(t, db.RunInTx(s.ctx, f.pool, func(ctx context.Context) error { return svc.MarkReady(ctx, got.ID) }))
	assert.Equal(t, "https://files.altalune.id/"+first.ObjectKey, svc.PublicURL(first.ObjectKey))

	second, up := begin()
	jpeg := append([]byte{0xFF, 0xD8, 0xFF}, bytes.Repeat([]byte{1}, 97)...)
	assert.Equal(t, http.StatusOK, put(t, up, "image/png", jpeg), "storage only checks the signed headers")
	_, err = svc.Verify(t.Context(), second.PublicID, logo, owner)
	var mm *storage.UploadMismatchError
	require.ErrorAs(t, err, &mm)
	assert.Equal(t, storage.MismatchContent, mm.Reason)

	third, up := begin()
	assert.Equal(t, http.StatusOK, put(t, up, "image/png", png))
	_, err = svc.Verify(t.Context(), third.PublicID, logo, owner)
	require.NoError(t, err)
	require.NoError(t, db.RunInTx(s.ctx, f.pool, func(ctx context.Context) error {
		if err := svc.MarkReady(ctx, third.ID); err != nil {
			return err
		}
		return svc.MarkDeleting(ctx, first.ID)
	}))

	n, err := svc.Sweep(s.ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, n, "the replaced logo and the rejected upload")
	_, err = svc.Verify(t.Context(), third.PublicID, logo, owner)
	require.NoError(t, err, "the current logo stays")
	d := storages3.New(conn)
	_, err = d.Head(t.Context(), first.ObjectKey)
	assert.True(t, storage.IsObjectNotFoundError(err), "the replaced object is gone")
	_, err = d.Head(t.Context(), second.ObjectKey)
	assert.True(t, storage.IsObjectNotFoundError(err))
	require.NoError(t, d.Delete(t.Context(), third.ObjectKey))
}
