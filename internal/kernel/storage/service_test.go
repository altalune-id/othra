package storage

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
)

var (
	logoPurpose = Purpose{
		Name: "logo", OwnerKind: OwnerProjects, Visibility: Public,
		Types: []string{"image/png", "image/jpeg", "image/webp"}, MaxBytes: 256 << 10,
	}
	pngBody = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0xab}, 92)...)
)

type harness struct {
	svc    *Service
	store  *fakeStore
	driver *fakeDriver
	owner  Owner
	now    time.Time
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{store: newFakeStore(), driver: newFakeDriver(), now: time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)}
	h.svc = NewService(h.driver, h.store, Config{
		App: "othra", PublicBaseURL: "https://files.altalune.id/", ImageTransform: ImageTransformNone,
		UploadURLTTL: 10 * time.Minute, PendingTTL: time.Hour, PrivateURLTTL: 5 * time.Minute,
	}, apperror.NewReporter(nil, false).Unexpected)
	h.svc.now = func() time.Time { return h.now }
	h.owner = Owner{OrgID: uuid.New(), ID: uuid.New(), PublicID: "prj8h2kq4"}
	return h
}

func (h *harness) begin(t *testing.T) *File {
	t.Helper()
	f, _, err := h.svc.BeginUpload(context.Background(), BeginUploadInput{
		Purpose: logoPurpose, Owner: h.owner, ContentType: "image/png", Size: int64(len(pngBody)), OriginalName: "logo.png",
	})
	require.NoError(t, err)
	return f
}

func TestKey_FollowsTheLayout(t *testing.T) {
	avatar := Purpose{Name: "avatar", OwnerKind: OwnerUsers, Visibility: Public}
	exports := Purpose{Name: "exports", OwnerKind: OwnerOrgs, Visibility: Private}
	for _, tc := range []struct {
		app   string
		p     Purpose
		owner string
		file  string
		ext   string
		want  string
	}{
		{"othra", logoPurpose, "prj_8h2kq4", "fil_3nq9wd", "webp", "public/othra/projects/prj_8h2kq4/logo/fil_3nq9wd.webp"},
		{"yasaku", avatar, "usr_x81mz0", "fil_k2p7aa", "jpg", "public/yasaku/users/usr_x81mz0/avatar/fil_k2p7aa.jpg"},
		{"openwa", exports, "org_p0d9s1", "fil_m4c8tt", "csv", "private/openwa/orgs/org_p0d9s1/exports/fil_m4c8tt.csv"},
	} {
		assert.Equal(t, tc.want, Key(tc.app, tc.p, tc.owner, tc.file, tc.ext))
	}
}

func TestExtension(t *testing.T) {
	assert.Equal(t, "png", Extension("image/png"))
	assert.Equal(t, "jpg", Extension("image/jpeg"))
	assert.Equal(t, "webp", Extension("image/webp"))
	assert.Equal(t, "bin", Extension("application/x-unknown"))
}

func TestSniffMatches(t *testing.T) {
	assert.True(t, sniffMatches("image/png", pngBody))
	assert.True(t, sniffMatches("image/jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0}))
	assert.True(t, sniffMatches("image/webp", []byte("RIFF\x00\x00\x00\x00WEBPVP8 ")))
	assert.False(t, sniffMatches("image/webp", []byte("RIFF")))
	assert.True(t, sniffMatches("image/gif", []byte("GIF89a")))
	assert.True(t, sniffMatches("application/pdf", []byte("%PDF-1.7")))
	assert.False(t, sniffMatches("image/png", []byte{0xFF, 0xD8, 0xFF}))
	assert.True(t, sniffMatches("text/csv", []byte("a,b")), "an unsniffable type always matches")
}

func TestBegin_RecordsAPendingFileAndSignsItsUpload(t *testing.T) {
	h := newHarness(t)
	f, up, err := h.svc.BeginUpload(context.Background(), BeginUploadInput{
		Purpose: logoPurpose, Owner: h.owner, ContentType: "image/png", Size: 100, OriginalName: "logo.png",
	})
	require.NoError(t, err)
	assert.Len(t, f.PublicID, publicIDLen)
	assert.Equal(t, "public/othra/projects/prj8h2kq4/logo/"+f.PublicID+".png", f.ObjectKey)
	assert.Equal(t, StatusPending, f.Status)
	assert.Equal(t, h.now.Add(time.Hour), f.ExpiresAt)
	assert.Equal(t, h.owner.OrgID, f.OrgID)
	assert.Equal(t, h.owner.ID, f.OwnerID)
	assert.Equal(t, OwnerProjects, f.OwnerKind)
	assert.Equal(t, "logo.png", f.OriginalName)
	assert.Equal(t, "https://s3.test/"+f.ObjectKey, up.URL)
	assert.Equal(t, "PUT", up.Method)
	assert.NotNil(t, h.store.get(f.ID))
}

func TestBegin_RefusesWhatThePurposeDoesNotAccept(t *testing.T) {
	h := newHarness(t)
	in := BeginUploadInput{Purpose: logoPurpose, Owner: h.owner, ContentType: "image/gif", Size: 10}
	_, _, err := h.svc.BeginUpload(context.Background(), in)
	assert.True(t, IsTypeNotAllowedError(err), "%v", err)

	in.ContentType, in.Size = "image/png", logoPurpose.MaxBytes+1
	_, _, err = h.svc.BeginUpload(context.Background(), in)
	assert.True(t, IsTooLargeError(err), "%v", err)
	assert.ErrorContains(t, err, "262144")

	in.Size = 0
	_, _, err = h.svc.BeginUpload(context.Background(), in)
	assert.Equal(t, apperror.CodeValidation, codeOf(err))
	assert.Empty(t, h.store.files, "nothing is recorded for a refused upload")
}

func TestBegin_ReportsStoreAndDriverFailures(t *testing.T) {
	for _, tc := range []struct{ store, driver string }{{store: "insert"}, {driver: "presignPut"}} {
		h := newHarness(t)
		h.store.failOn, h.driver.failOn = tc.store, tc.driver
		_, _, err := h.svc.BeginUpload(context.Background(), BeginUploadInput{Purpose: logoPurpose, Owner: h.owner, ContentType: "image/png", Size: 1})
		assert.Equal(t, apperror.CodeUnexpected, codeOf(err), "%+v: %v", tc, err)
	}
}

func TestVerify_AcceptsTheDeclaredFile(t *testing.T) {
	h := newHarness(t)
	f := h.begin(t)
	h.driver.put(f.ObjectKey, "image/png", pngBody)

	got, err := h.svc.Verify(context.Background(), f.PublicID, logoPurpose, h.owner)
	require.NoError(t, err)
	assert.Equal(t, f.ID, got.ID)
	assert.True(t, h.driver.has(f.ObjectKey))
}

func TestVerify_AcceptsAContentTypeWithParameters(t *testing.T) {
	h := newHarness(t)
	f := h.begin(t)
	h.driver.put(f.ObjectKey, "image/png; foo=bar", pngBody)
	_, err := h.svc.Verify(context.Background(), f.PublicID, logoPurpose, h.owner)
	assert.NoError(t, err)
}

func TestVerify_IsIdempotentOnceReady(t *testing.T) {
	h := newHarness(t)
	f := h.begin(t)
	require.NoError(t, h.svc.MarkReady(context.Background(), f.ID))
	h.now = h.now.Add(2 * time.Hour)
	got, err := h.svc.Verify(context.Background(), f.PublicID, logoPurpose, h.owner)
	require.NoError(t, err)
	assert.Equal(t, StatusReady, got.Status, "a retried completion succeeds without storage")
}

// SECURITY: an upload made for another owner or purpose answers the same as an unknown one.
func TestVerify_UnknownForeignExpiredOrDeletingIsNotFound(t *testing.T) {
	h := newHarness(t)
	f := h.begin(t)
	h.driver.put(f.ObjectKey, "image/png", pngBody)
	other := h.owner
	other.ID = uuid.New()
	avatar := logoPurpose
	avatar.Name = "avatar"
	users := logoPurpose
	users.OwnerKind = OwnerUsers

	for name, call := range map[string]func() error{
		"unknown":       func() error { _, err := h.svc.Verify(context.Background(), "nope", logoPurpose, h.owner); return err },
		"other owner":   func() error { _, err := h.svc.Verify(context.Background(), f.PublicID, logoPurpose, other); return err },
		"other purpose": func() error { _, err := h.svc.Verify(context.Background(), f.PublicID, avatar, h.owner); return err },
		"other kind":    func() error { _, err := h.svc.Verify(context.Background(), f.PublicID, users, h.owner); return err },
	} {
		err := call()
		assert.True(t, IsUploadNotFoundError(err), "%s: %v", name, err)
		assert.Equal(t, apperror.CodeStorageUploadNotFound, codeOf(err), name)
	}

	h.now = f.ExpiresAt
	_, err := h.svc.Verify(context.Background(), f.PublicID, logoPurpose, h.owner)
	assert.True(t, IsUploadNotFoundError(err), "expired: %v", err)

	h.now = f.CreatedAt
	require.NoError(t, h.svc.MarkDeleting(context.Background(), f.ID))
	_, err = h.svc.Verify(context.Background(), f.PublicID, logoPurpose, h.owner)
	assert.True(t, IsUploadNotFoundError(err), "deleting: %v", err)
}

func TestVerify_MissingObjectKeepsTheUploadOpen(t *testing.T) {
	h := newHarness(t)
	f := h.begin(t)
	_, err := h.svc.Verify(context.Background(), f.PublicID, logoPurpose, h.owner)
	var mm *UploadMismatchError
	require.ErrorAs(t, err, &mm)
	assert.Equal(t, MismatchMissing, mm.Reason)
	assert.Equal(t, StatusPending, h.store.get(f.ID).Status, "the browser may still finish its PUT")
}

func TestVerify_MismatchDeletesTheObjectAndHandsTheRowToTheSweep(t *testing.T) {
	jpeg := append([]byte{0xFF, 0xD8, 0xFF}, bytes.Repeat([]byte{1}, 97)...)
	for _, tc := range []struct {
		name        string
		contentType string
		body        []byte
		want        MismatchReason
	}{
		{"larger than declared", "image/png", append(append([]byte{}, pngBody...), 0), MismatchSize},
		{"empty", "image/png", []byte{}, MismatchSize},
		{"another type", "image/jpeg", pngBody, MismatchType},
		{"content is not the type", "image/png", jpeg, MismatchContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			f := h.begin(t)
			h.driver.put(f.ObjectKey, tc.contentType, tc.body)

			_, err := h.svc.Verify(context.Background(), f.PublicID, logoPurpose, h.owner)
			var mm *UploadMismatchError
			require.ErrorAs(t, err, &mm)
			assert.Equal(t, tc.want, mm.Reason)
			app := mm.ToAppError()
			assert.Equal(t, apperror.CodeStorageUploadMismatch, app.Code())
			assert.Equal(t, string(tc.want), app.Meta()["reason"])
			assert.False(t, h.driver.has(f.ObjectKey), "the object is deleted")
			assert.Equal(t, StatusDeleting, h.store.get(f.ID).Status)
		})
	}
}

func TestVerify_SmallerThanDeclaredIsAccepted(t *testing.T) {
	h := newHarness(t)
	f := h.begin(t)
	h.driver.put(f.ObjectKey, "image/png", pngBody[:50])
	_, err := h.svc.Verify(context.Background(), f.PublicID, logoPurpose, h.owner)
	assert.NoError(t, err, "the cap is an upper bound; storage enforces the signed length")
}

func TestVerify_ReportsInfrastructureFailures(t *testing.T) {
	for _, tc := range []struct{ store, driver string }{
		{store: "byPublicID"}, {driver: "head"}, {driver: "read"}, {driver: "delete"}, {store: "markDeleting"},
	} {
		h := newHarness(t)
		f := h.begin(t)
		body := pngBody
		if tc.driver == "delete" || tc.store == "markDeleting" {
			body = append(append([]byte{}, pngBody...), 0)
		}
		h.driver.put(f.ObjectKey, "image/png", body)
		h.store.failOn, h.driver.failOn = tc.store, tc.driver
		_, err := h.svc.Verify(context.Background(), f.PublicID, logoPurpose, h.owner)
		assert.Equal(t, apperror.CodeUnexpected, codeOf(err), "%+v: %v", tc, err)
	}
}

func TestMarkReadyAndDeleting(t *testing.T) {
	h := newHarness(t)
	f := h.begin(t)
	require.NoError(t, h.svc.MarkReady(context.Background(), f.ID))
	got := h.store.get(f.ID)
	assert.Equal(t, StatusReady, got.Status)
	assert.Equal(t, h.now, *got.CompletedAt)
	require.NoError(t, h.svc.MarkReady(context.Background(), f.ID), "marking ready twice is fine")

	require.NoError(t, h.svc.MarkDeleting(context.Background(), f.ID))
	assert.Equal(t, StatusDeleting, h.store.get(f.ID).Status)
	assert.True(t, IsUploadNotFoundError(h.svc.MarkReady(context.Background(), f.ID)), "a deleting file is never attached")
	assert.True(t, IsUploadNotFoundError(h.svc.MarkDeleting(context.Background(), uuid.New())))

	h.store.failOn = "markReady"
	assert.Equal(t, apperror.CodeUnexpected, codeOf(h.svc.MarkReady(context.Background(), f.ID)))
}

func TestURLs(t *testing.T) {
	h := newHarness(t)
	key := "public/othra/projects/p/logo/f.png"
	assert.Equal(t, "https://files.altalune.id/"+key, h.svc.PublicURL(key))
	assert.Empty(t, h.svc.PublicURL(""))
	assert.Equal(t, "https://files.altalune.id/"+key, h.svc.ImageURL(key, 128), "no transform serves the original")

	h.svc.cfg.ImageTransform = ImageTransformCloudflare
	assert.Equal(t, "https://files.altalune.id/cdn-cgi/image/width=128,format=auto/"+key, h.svc.ImageURL(key, 128))
	assert.Equal(t, "https://files.altalune.id/"+key, h.svc.ImageURL(key, 0))
	assert.Empty(t, h.svc.ImageURL("", 128))

	u, err := h.svc.PrivateURL(context.Background(), "private/othra/orgs/o/exports/f.csv")
	require.NoError(t, err)
	assert.Equal(t, "https://s3.test/private/othra/orgs/o/exports/f.csv?signed", u)
	h.driver.failOn = "presignGet"
	_, err = h.svc.PrivateURL(context.Background(), "k")
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

func TestDisabled_EveryCallSaysNotConfigured(t *testing.T) {
	svc := NewService(nil, newFakeStore(), Config{}, apperror.NewReporter(nil, false).Unexpected)
	assert.False(t, svc.Enabled())
	assert.False(t, (*Service)(nil).Enabled())
	ctx := context.Background()
	_, _, err := svc.BeginUpload(ctx, BeginUploadInput{Purpose: logoPurpose, ContentType: "image/png", Size: 1})
	assert.True(t, IsNotConfiguredError(err))
	assert.Equal(t, apperror.CodeStorageNotConfigured, codeOf(err))
	_, err = svc.Verify(ctx, "f", logoPurpose, Owner{})
	assert.True(t, IsNotConfiguredError(err))
	assert.True(t, IsNotConfiguredError(svc.MarkReady(ctx, uuid.New())))
	assert.True(t, IsNotConfiguredError(svc.MarkDeleting(ctx, uuid.New())))
	_, err = svc.PrivateURL(ctx, "k")
	assert.True(t, IsNotConfiguredError(err))
	n, err := svc.Sweep(ctx)
	assert.Zero(t, n)
	assert.NoError(t, err)
}

func TestErrors_AreExpectedAndCarryTheirCodes(t *testing.T) {
	for _, err := range []error{
		&TypeNotAllowedError{Purpose: "logo", ContentType: "image/gif"},
		&TooLargeError{Purpose: "logo", MaxBytes: 1},
		&UploadNotFoundError{PublicID: "f"},
		&UploadMismatchError{PublicID: "f", Reason: MismatchSize},
		&NotConfiguredError{},
	} {
		assert.True(t, IsExpectedError(err), "%T", err)
		assert.NotEmpty(t, err.Error())
	}
	assert.False(t, IsExpectedError(errors.New("boom")))
	assert.True(t, IsObjectNotFoundError(&ObjectNotFoundError{Key: "k"}))
	assert.Contains(t, (&ObjectNotFoundError{Key: "k"}).Error(), "k")
	assert.Equal(t, apperror.CodeStorageUploadNotFound, (&UploadNotFoundError{}).ToAppError().Code())
}

func codeOf(err error) apperror.Code {
	if app, ok := apperror.AsAppError(err); ok {
		return app.Code()
	}
	return ""
}
