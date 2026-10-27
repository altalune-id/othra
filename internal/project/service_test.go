package project_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/kernel/storage"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/testutil/fakes"
)

func testUnexpected() apperror.UnexpectedFunc {
	return apperror.NewReporter(nil, false).Unexpected
}

func TestService_Create(t *testing.T) {
	store := &fakes.ProjectStore{}
	svc := project.NewService(store, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs(""))

	orgID := uuid.New()
	p, err := svc.Create(context.Background(), project.CreateRequest{OrgID: orgID, Slug: "acme", Name: "Acme", CreatedBy: uuid.New()})
	require.NoError(t, err)
	assert.Equal(t, p, store.Saved)
	assert.Equal(t, orgID, p.OrgID)
	assert.Equal(t, "acme", p.Slug)
}

func TestService_Create_MissingOrg(t *testing.T) {
	_, err := project.NewService(&fakes.ProjectStore{}, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs("")).Create(context.Background(), project.CreateRequest{Name: "Acme"})
	_, ok := apperror.AsAppError(err)
	assert.True(t, ok)
}

func TestService_Create_GeneratedSlug(t *testing.T) {
	svc := project.NewService(&fakes.ProjectStore{}, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs(""))

	p, err := svc.Create(context.Background(), project.CreateRequest{OrgID: uuid.New(), Name: "Acme"})
	require.NoError(t, err)
	assert.NotEmpty(t, p.Slug)
	assert.NoError(t, project.ValidateSlug(p.Slug))
}

func TestService_Create_GeneratedSlugRetries(t *testing.T) {
	calls := 0
	store := &fakes.ProjectStore{SaveFn: func(*project.Project) error {
		calls++
		if calls < 3 {
			return &project.AlreadyExistsError{}
		}
		return nil
	}}

	p, err := project.NewService(store, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs("")).Create(context.Background(), project.CreateRequest{OrgID: uuid.New(), Name: "Acme"})
	require.NoError(t, err)
	assert.Equal(t, 3, calls)
	assert.NotEmpty(t, p.Slug)
}

func TestService_Create_GeneratedSlugExhausted(t *testing.T) {
	store := &fakes.ProjectStore{SaveFn: func(*project.Project) error { return &project.AlreadyExistsError{} }}

	_, err := project.NewService(store, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs("")).Create(context.Background(), project.CreateRequest{OrgID: uuid.New(), Name: "Acme"})
	assert.True(t, project.IsAlreadyExistsError(err))
}

func TestService_Create_Validation(t *testing.T) {
	store := &fakes.ProjectStore{}
	svc := project.NewService(store, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs(""))

	_, err := svc.Create(context.Background(), project.CreateRequest{OrgID: uuid.New(), Slug: "ab", Name: "Acme"})
	assert.True(t, project.IsInvalidSlugError(err))
	assert.Nil(t, store.Saved)
}

func TestService_Create_StoreError(t *testing.T) {
	store := &fakes.ProjectStore{Err: &project.AlreadyExistsError{Slug: "acme"}}
	svc := project.NewService(store, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs(""))

	_, err := svc.Create(context.Background(), project.CreateRequest{OrgID: uuid.New(), Slug: "acme", Name: "Acme"})
	assert.True(t, project.IsAlreadyExistsError(err))
}

func TestService_GetAndList(t *testing.T) {
	p, err := project.NewProject(uuid.New(), "acme", "Acme", uuid.New())
	require.NoError(t, err)
	store := &fakes.ProjectStore{Project: p, Projects: []*project.Project{p}}
	svc := project.NewService(store, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs(""))

	got, err := svc.Get(context.Background(), p.ID)
	require.NoError(t, err)
	assert.Equal(t, p, got)

	bySlug, err := svc.BySlug(context.Background(), p.OrgID, "acme")
	require.NoError(t, err)
	assert.Equal(t, p, bySlug)

	all, err := svc.List(context.Background(), p.OrgID)
	require.NoError(t, err)
	assert.Equal(t, []*project.Project{p}, all)
}

func TestService_Get_StoreError(t *testing.T) {
	svc := project.NewService(&fakes.ProjectStore{Err: errors.New("boom")}, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs(""))

	_, err := svc.Get(context.Background(), uuid.New())
	assert.Error(t, err)
	_, err = svc.BySlug(context.Background(), uuid.New(), "acme")
	assert.Error(t, err)
	_, err = svc.List(context.Background(), uuid.New())
	assert.Error(t, err)
}

func TestService_Unexpected(t *testing.T) {
	svc := project.NewService(&fakes.ProjectStore{Err: errors.New("boom")}, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs(""))

	_, err := svc.Get(context.Background(), uuid.New())
	_, ok := apperror.AsAppError(err)
	assert.True(t, ok)
}

func TestService_Update(t *testing.T) {
	p, err := project.NewProject(uuid.New(), "acme", "Acme", uuid.New())
	require.NoError(t, err)
	store := &fakes.ProjectStore{Project: p}

	got, err := project.NewService(store, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs("")).Update(context.Background(), p.ID, "Acme Two", "acme-two")
	require.NoError(t, err)
	assert.Equal(t, "Acme Two", got.Name)
	assert.Equal(t, "acme-two", got.Slug)
	assert.Equal(t, got, store.Saved)
}

func TestService_Update_NotFound(t *testing.T) {
	store := &fakes.ProjectStore{Err: &project.NotFoundError{ID: "x"}}

	_, err := project.NewService(store, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs("")).Update(context.Background(), uuid.New(), "Acme Two", "acme-two")
	assert.Error(t, err)
}

func TestService_Update_InvalidName(t *testing.T) {
	p, err := project.NewProject(uuid.New(), "acme", "Acme", uuid.New())
	require.NoError(t, err)

	_, err = project.NewService(&fakes.ProjectStore{Project: p}, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs("")).Update(context.Background(), p.ID, "", "acme-two")
	assert.True(t, project.IsInvalidNameError(err))
}

func TestService_Update_InvalidSlug(t *testing.T) {
	p, err := project.NewProject(uuid.New(), "acme", "Acme", uuid.New())
	require.NoError(t, err)

	_, err = project.NewService(&fakes.ProjectStore{Project: p}, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs("")).Update(context.Background(), p.ID, "Acme", "Bad_Slug")
	assert.True(t, project.IsInvalidSlugError(err))
}

func TestService_ByPublicID(t *testing.T) {
	p, err := project.NewProject(uuid.New(), "web", "Web", uuid.Nil)
	require.NoError(t, err)
	got, err := project.NewService(&fakes.ProjectStore{Project: p}, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs("")).ByPublicID(context.Background(), p.PublicID)
	require.NoError(t, err)
	assert.Same(t, p, got)

	_, err = project.NewService(&fakes.ProjectStore{Project: p}, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs("")).ByPublicID(context.Background(), "nope")
	assert.True(t, project.IsNotFoundError(err))
	_, err = project.NewService(&fakes.ProjectStore{Err: errors.New("down")}, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs("")).ByPublicID(context.Background(), p.PublicID)
	_, isApp := apperror.AsAppError(err)
	assert.True(t, isApp)
}

type orgSlugs map[uuid.UUID]string

func (o orgSlugs) Slug(_ context.Context, id uuid.UUID) (string, error) {
	if s, ok := o[id]; ok {
		return s, nil
	}
	return "", errors.New("unknown org")
}

func TestService_Create_DefaultIssuerHandle(t *testing.T) {
	orgID := uuid.New()
	store := &fakes.ProjectStore{}
	svc := project.NewService(store, fakes.Transactor{}, fakes.NoMemberships{}, nil, "https://id.example.com", testUnexpected()).WithOrgSlugs(orgSlugs{orgID: "acme"})
	p, err := svc.Create(context.Background(), project.CreateRequest{OrgID: orgID, Slug: "web", Name: "Web"})
	require.NoError(t, err)
	assert.Equal(t, "acme-web", p.IssuerHandle)
	assert.Equal(t, "https://id.example.com/p/acme-web", svc.IssuerURL(p.IssuerHandle))

	_, err = svc.Create(context.Background(), project.CreateRequest{OrgID: uuid.New(), Slug: "web", Name: "Web"})
	require.Error(t, err, "the org slug lookup failing fails the create")
}

func TestService_Create_IssuerHandleCollisions(t *testing.T) {
	orgID := uuid.New()
	taken := 0
	store := &fakes.ProjectStore{SaveFn: func(p *project.Project) error {
		if p.IssuerHandle == "acme-web" {
			taken++
			return &project.IssuerHandleTakenError{Handle: p.IssuerHandle}
		}
		return nil
	}}
	svc := project.NewService(store, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(orgSlugs{orgID: "acme"})
	p, err := svc.Create(context.Background(), project.CreateRequest{OrgID: orgID, Slug: "web", Name: "Web"})
	require.NoError(t, err)
	assert.Equal(t, 1, taken)
	assert.Regexp(t, `^acme-web-[a-z0-9]{4}$`, p.IssuerHandle, "a taken default handle gets a random suffix")

	_, err = svc.Create(context.Background(), project.CreateRequest{OrgID: orgID, Slug: "web", Name: "Web", IssuerHandle: "acme-web"})
	assert.True(t, project.IsIssuerHandleTakenError(err), "a chosen handle is never changed: %v", err)
	_, err = svc.Create(context.Background(), project.CreateRequest{OrgID: orgID, Slug: "web", Name: "Web", IssuerHandle: "system"})
	assert.True(t, project.IsInvalidIssuerHandleError(err))

	reserved := &fakes.ProjectStore{}
	well, err := project.NewService(reserved, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(orgSlugs{orgID: "well"}).
		Create(context.Background(), project.CreateRequest{OrgID: orgID, Slug: "known", Name: "Known"})
	require.NoError(t, err)
	assert.Regexp(t, `^well-known-[a-z0-9]{4}$`, well.IssuerHandle, "a reserved default is suffixed")
}

func TestService_UpdateIssuerHandle(t *testing.T) {
	p, err := project.NewProject(uuid.New(), "web", "Web", uuid.Nil)
	require.NoError(t, err)
	store := &fakes.ProjectStore{Project: p}
	var changed []uuid.UUID
	svc := project.NewService(store, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs("")).WithOnIssuerChange(func(id uuid.UUID) { changed = append(changed, id) })
	same, err := svc.UpdateIssuerHandle(context.Background(), p.ID, p.IssuerHandle)
	require.NoError(t, err)
	assert.Equal(t, p.IssuerHandle, same.IssuerHandle)
	assert.Empty(t, changed, "the same handle changes nothing")

	got, err := svc.UpdateIssuerHandle(context.Background(), p.ID, " acme-shop ")
	require.NoError(t, err)
	assert.Equal(t, "acme-shop", got.IssuerHandle)
	assert.Equal(t, []uuid.UUID{p.ID}, changed, "a handle change drops the issuer cache")

	store.HandleSetErr = &project.IssuerHandleTakenError{Handle: "acme-taken"}
	_, err = svc.UpdateIssuerHandle(context.Background(), p.ID, "acme-taken")
	assert.True(t, project.IsIssuerHandleTakenError(err))

	require.NoError(t, svc.LockIssuerHandle(context.Background(), p.ID))
	require.NotNil(t, store.LockedAt)
	p.LockIssuerHandle(*store.LockedAt)
	_, err = svc.UpdateIssuerHandle(context.Background(), p.ID, "acme-other")
	assert.True(t, project.IsIssuerHandleLockedError(err))
	reason, err := svc.CheckIssuerHandle(context.Background(), "acme-other", p)
	require.NoError(t, err)
	assert.Equal(t, project.HandleLocked, reason)
}

func TestService_Settings(t *testing.T) {
	p, err := project.NewProject(uuid.New(), "web", "Web", uuid.Nil)
	require.NoError(t, err)
	store := &fakes.ProjectStore{Project: p}
	svc := project.NewService(store, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs(""))
	ctx := context.Background()

	_, err = svc.UpdateBranding(ctx, p.ID, "", "red", "")
	require.Error(t, err)

	store.Issuer = &project.Issuer{Handle: "acme-web", ProjectID: p.ID}
	i, err := svc.ResolveIssuer(ctx, "acme-web")
	require.NoError(t, err)
	assert.Equal(t, p.ID, i.ProjectID)
	_, err = svc.ResolveIssuer(ctx, "nope-nope")
	assert.True(t, project.IsNotFoundError(err))

	store.Err = errors.New("db down")
	for _, call := range []func() error{
		func() error { _, err := svc.UpdateBranding(ctx, p.ID, "", "", ""); return err },
		func() error { _, err := svc.UpdateTerms(ctx, p.ID, "", ""); return err },
		func() error { _, err := svc.BumpTermsVersion(ctx, p.ID); return err },
		func() error { _, err := svc.UpdateIssuerHandle(ctx, p.ID, "acme-x"); return err },
		func() error { return svc.LockIssuerHandle(ctx, p.ID) },
		func() error { _, err := svc.CheckIssuerHandle(ctx, "acme-x", nil); return err },
		func() error { _, err := svc.ResolveIssuer(ctx, "acme-x"); return err },
	} {
		assert.Error(t, call())
	}
}

func logoService(t *testing.T) (*project.Service, *fakes.ProjectStore, *fakes.FileStorage, *project.Project) {
	t.Helper()
	p, err := project.NewProject(uuid.New(), "web", "Web", uuid.Nil)
	require.NoError(t, err)
	store := &fakes.ProjectStore{Project: p}
	files := &fakes.FileStorage{Files: map[string]*storage.File{}}
	return project.NewService(store, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs("")).WithStorage(files), store, files, p
}

func TestService_CreateLogoUpload(t *testing.T) {
	svc, _, files, p := logoService(t)
	ctx := context.Background()

	f, up, err := svc.CreateLogoUpload(ctx, p.ID, "image/png", 1024)
	require.NoError(t, err)
	assert.NotEmpty(t, f.PublicID)
	assert.Equal(t, "PUT", up.Method)
	require.Len(t, files.Begun, 1)
	assert.Equal(t, storage.BeginUploadInput{
		Purpose: project.LogoPurpose(), Owner: storage.Owner{OrgID: p.OrgID, ID: p.ID, PublicID: p.PublicID},
		ContentType: "image/png", Size: 1024,
	}, files.Begun[0])

	for in, want := range map[error]project.LogoReason{
		&storage.TypeNotAllowedError{Purpose: "logo", ContentType: "image/gif"}: project.LogoType,
		&storage.TooLargeError{Purpose: "logo", MaxBytes: 256 << 10}:            project.LogoSize,
	} {
		files.BeginErr = in
		_, _, err := svc.CreateLogoUpload(ctx, p.ID, "image/gif", 1)
		var e *project.InvalidLogoError
		require.ErrorAs(t, err, &e, "%v", in)
		assert.Equal(t, want, e.Reason)
	}

	files.BeginErr = &storage.NotConfiguredError{}
	_, _, err = svc.CreateLogoUpload(ctx, p.ID, "image/png", 1)
	assert.True(t, storage.IsNotConfiguredError(err))
}

func TestLogoPurpose(t *testing.T) {
	p := project.LogoPurpose()
	assert.Equal(t, storage.OwnerProjects, p.OwnerKind)
	assert.Equal(t, storage.Public, p.Visibility)
	assert.ElementsMatch(t, []string{"image/png", "image/jpeg", "image/webp"}, p.Types)
	assert.Equal(t, int64(256<<10), p.MaxBytes)
}

func TestService_CompleteLogoUploadReplacesTheOldLogo(t *testing.T) {
	svc, store, files, p := logoService(t)
	ctx := context.Background()
	first := &storage.File{ID: uuid.New(), PublicID: "firstfirstfirstfirst2"}
	second := &storage.File{ID: uuid.New(), PublicID: "secondsecondsecondsc2"}
	files.Files[first.PublicID], files.Files[second.PublicID] = first, second

	_, err := svc.CompleteLogoUpload(ctx, p.ID, first.PublicID)
	require.NoError(t, err)
	assert.Equal(t, &first.ID, store.Project.LogoFileID)
	assert.Equal(t, []uuid.UUID{first.ID}, files.Ready)
	assert.Empty(t, files.Deleting, "the first logo replaces nothing")

	_, err = svc.CompleteLogoUpload(ctx, p.ID, second.PublicID)
	require.NoError(t, err)
	assert.Equal(t, &second.ID, store.Project.LogoFileID)
	assert.Equal(t, []uuid.UUID{first.ID}, files.Deleting, "the replaced logo goes to the sweep")

	_, err = svc.CompleteLogoUpload(ctx, p.ID, second.PublicID)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{first.ID}, files.Deleting, "completing the same upload twice never deletes it")

	_, err = svc.CompleteLogoUpload(ctx, p.ID, "unknwnunknwnunknwnabc")
	assert.True(t, storage.IsUploadNotFoundError(err))
	files.VerifyErr = &storage.UploadMismatchError{Reason: storage.MismatchContent}
	_, err = svc.CompleteLogoUpload(ctx, p.ID, second.PublicID)
	assert.True(t, storage.IsUploadMismatchError(err))
	assert.Equal(t, &second.ID, store.Project.LogoFileID, "a refused upload keeps the current logo")
}

func TestService_DeleteLogo(t *testing.T) {
	svc, store, files, p := logoService(t)
	ctx := context.Background()

	_, err := svc.DeleteLogo(ctx, p.ID)
	require.NoError(t, err, "a project without a logo deletes nothing")
	assert.Empty(t, files.Deleting)

	logo := uuid.New()
	store.Project.LogoFileID = &logo
	_, err = svc.DeleteLogo(ctx, p.ID)
	require.NoError(t, err)
	assert.Nil(t, store.Project.LogoFileID)
	assert.Equal(t, []uuid.UUID{logo}, files.Deleting)
}

func TestService_LogoWithoutStorage(t *testing.T) {
	p, err := project.NewProject(uuid.New(), "web", "Web", uuid.Nil)
	require.NoError(t, err)
	svc := project.NewService(&fakes.ProjectStore{Project: p}, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs(""))
	ctx := context.Background()

	_, _, err = svc.CreateLogoUpload(ctx, p.ID, "image/png", 1)
	assert.True(t, storage.IsNotConfiguredError(err))
	_, err = svc.CompleteLogoUpload(ctx, p.ID, "upxdupxdupxdupxdupxd2")
	assert.True(t, storage.IsNotConfiguredError(err))
	_, err = svc.DeleteLogo(ctx, p.ID)
	require.NoError(t, err, "nothing to delete without storage")
	orphan := uuid.New()
	p.LogoFileID = &orphan
	_, err = svc.DeleteLogo(ctx, p.ID)
	assert.True(t, storage.IsNotConfiguredError(err), "a logo left behind after storage was turned off")
	assert.Empty(t, svc.LogoURL("public/othra/projects/x/logo/y.png"))

	withFiles, _, _, _ := logoService(t)
	assert.Equal(t, "https://files.test/public/othra/projects/x/logo/y.png", withFiles.LogoURL("public/othra/projects/x/logo/y.png"))
	assert.Empty(t, withFiles.LogoURL(""))
}

func TestService_LogoStoreFailures(t *testing.T) {
	svc, store, files, p := logoService(t)
	ctx := context.Background()
	f := &storage.File{ID: uuid.New(), PublicID: "firstfirstfirstfirst2"}
	files.Files[f.PublicID] = f

	store.LogoSetErr = errors.New("db down")
	_, err := svc.CompleteLogoUpload(ctx, p.ID, f.PublicID)
	require.Error(t, err)
	store.Project.LogoFileID = &f.ID
	_, err = svc.DeleteLogo(ctx, p.ID)
	require.Error(t, err)
	store.LogoSetErr = nil

	files.MarkErr = errors.New("db down")
	_, err = svc.CompleteLogoUpload(ctx, p.ID, f.PublicID)
	require.Error(t, err)

	store.Err = errors.New("db down")
	_, _, err = svc.CreateLogoUpload(ctx, p.ID, "image/png", 1)
	require.Error(t, err)
	_, err = svc.CompleteLogoUpload(ctx, p.ID, f.PublicID)
	require.Error(t, err)
}

// a terms change drops the issuer cache at once, so the hosted pages never gate on the old version.
func TestService_TermsChangeDropsTheIssuerCache(t *testing.T) {
	p, err := project.NewProject(uuid.New(), "web", "Web", uuid.Nil)
	require.NoError(t, err)
	store := &fakes.ProjectStore{Project: p}
	var changed []uuid.UUID
	svc := project.NewService(store, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs("")).WithOnIssuerChange(func(id uuid.UUID) { changed = append(changed, id) })
	ctx := context.Background()

	_, err = svc.UpdateTerms(ctx, p.ID, "https://acme.example/terms", "")
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{p.ID}, changed, "setting the terms URL publishes version 1")
	_, err = svc.BumpTermsVersion(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{p.ID, p.ID}, changed, "a bump drops the cache too")

	store.Err = errors.New("db down")
	_, err = svc.BumpTermsVersion(ctx, p.ID)
	require.Error(t, err)
	assert.Len(t, changed, 2, "a failed change drops nothing")
}

// A branding or logo change drops the issuer cache at once, so the hosted pages show it right away.
func TestService_BrandingChangeDropsTheIssuerCache(t *testing.T) {
	svc, store, files, p := logoService(t)
	var changed []uuid.UUID
	svc.WithOnIssuerChange(func(id uuid.UUID) { changed = append(changed, id) })
	ctx := context.Background()

	_, err := svc.UpdateBranding(ctx, p.ID, "Acme", "#1e3a8a", "")
	require.NoError(t, err)
	logo := &storage.File{ID: uuid.New(), PublicID: "logologologologologo2"}
	files.Files[logo.PublicID] = logo
	_, err = svc.CompleteLogoUpload(ctx, p.ID, logo.PublicID)
	require.NoError(t, err)
	_, err = svc.DeleteLogo(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{p.ID, p.ID, p.ID}, changed)

	_, err = svc.UpdateBranding(ctx, p.ID, "", "red", "")
	require.Error(t, err)
	store.Err = errors.New("db down")
	_, err = svc.DeleteLogo(ctx, p.ID)
	require.Error(t, err)
	assert.Len(t, changed, 3, "a refused or failed change drops nothing")
}
