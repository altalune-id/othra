package project_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/testutil/fakes"
)

func testUnexpected() apperror.UnexpectedFunc {
	return apperror.NewReporter(nil, false).Unexpected
}

func TestService_Create(t *testing.T) {
	store := &fakes.ProjectStore{}
	svc := project.NewService(store, testUnexpected())

	orgID := uuid.New()
	p, err := svc.Create(context.Background(), project.CreateRequest{OrgID: orgID, Slug: "acme", Name: "Acme", CreatedBy: uuid.New()})
	require.NoError(t, err)
	assert.Equal(t, p, store.Saved)
	assert.Equal(t, orgID, p.OrgID)
	assert.Equal(t, "acme", p.Slug)
}

func TestService_Create_MissingOrg(t *testing.T) {
	_, err := project.NewService(&fakes.ProjectStore{}, testUnexpected()).Create(context.Background(), project.CreateRequest{Name: "Acme"})
	_, ok := apperror.AsAppError(err)
	assert.True(t, ok)
}

func TestService_Create_GeneratedSlug(t *testing.T) {
	svc := project.NewService(&fakes.ProjectStore{}, testUnexpected())

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

	p, err := project.NewService(store, testUnexpected()).Create(context.Background(), project.CreateRequest{OrgID: uuid.New(), Name: "Acme"})
	require.NoError(t, err)
	assert.Equal(t, 3, calls)
	assert.NotEmpty(t, p.Slug)
}

func TestService_Create_GeneratedSlugExhausted(t *testing.T) {
	store := &fakes.ProjectStore{SaveFn: func(*project.Project) error { return &project.AlreadyExistsError{} }}

	_, err := project.NewService(store, testUnexpected()).Create(context.Background(), project.CreateRequest{OrgID: uuid.New(), Name: "Acme"})
	assert.True(t, project.IsAlreadyExistsError(err))
}

func TestService_Create_Validation(t *testing.T) {
	store := &fakes.ProjectStore{}
	svc := project.NewService(store, testUnexpected())

	_, err := svc.Create(context.Background(), project.CreateRequest{OrgID: uuid.New(), Slug: "ab", Name: "Acme"})
	assert.True(t, project.IsInvalidSlugError(err))
	assert.Nil(t, store.Saved)
}

func TestService_Create_StoreError(t *testing.T) {
	store := &fakes.ProjectStore{Err: &project.AlreadyExistsError{Slug: "acme"}}
	svc := project.NewService(store, testUnexpected())

	_, err := svc.Create(context.Background(), project.CreateRequest{OrgID: uuid.New(), Slug: "acme", Name: "Acme"})
	assert.True(t, project.IsAlreadyExistsError(err))
}

func TestService_GetAndList(t *testing.T) {
	p, err := project.NewProject(uuid.New(), "acme", "Acme", uuid.New())
	require.NoError(t, err)
	store := &fakes.ProjectStore{Project: p, Projects: []*project.Project{p}}
	svc := project.NewService(store, testUnexpected())

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
	svc := project.NewService(&fakes.ProjectStore{Err: errors.New("boom")}, testUnexpected())

	_, err := svc.Get(context.Background(), uuid.New())
	assert.Error(t, err)
	_, err = svc.BySlug(context.Background(), uuid.New(), "acme")
	assert.Error(t, err)
	_, err = svc.List(context.Background(), uuid.New())
	assert.Error(t, err)
}

func TestService_Unexpected(t *testing.T) {
	svc := project.NewService(&fakes.ProjectStore{Err: errors.New("boom")}, testUnexpected())

	_, err := svc.Get(context.Background(), uuid.New())
	_, ok := apperror.AsAppError(err)
	assert.True(t, ok)
}

func TestService_Update(t *testing.T) {
	p, err := project.NewProject(uuid.New(), "acme", "Acme", uuid.New())
	require.NoError(t, err)
	store := &fakes.ProjectStore{Project: p}

	got, err := project.NewService(store, testUnexpected()).Update(context.Background(), p.ID, "Acme Two", "acme-two")
	require.NoError(t, err)
	assert.Equal(t, "Acme Two", got.Name)
	assert.Equal(t, "acme-two", got.Slug)
	assert.Equal(t, got, store.Saved)
}

func TestService_Update_NotFound(t *testing.T) {
	store := &fakes.ProjectStore{Err: &project.NotFoundError{ID: "x"}}

	_, err := project.NewService(store, testUnexpected()).Update(context.Background(), uuid.New(), "Acme Two", "acme-two")
	assert.Error(t, err)
}

func TestService_Update_InvalidName(t *testing.T) {
	p, err := project.NewProject(uuid.New(), "acme", "Acme", uuid.New())
	require.NoError(t, err)

	_, err = project.NewService(&fakes.ProjectStore{Project: p}, testUnexpected()).Update(context.Background(), p.ID, "", "acme-two")
	assert.True(t, project.IsInvalidNameError(err))
}

func TestService_Update_InvalidSlug(t *testing.T) {
	p, err := project.NewProject(uuid.New(), "acme", "Acme", uuid.New())
	require.NoError(t, err)

	_, err = project.NewService(&fakes.ProjectStore{Project: p}, testUnexpected()).Update(context.Background(), p.ID, "Acme", "Bad_Slug")
	assert.True(t, project.IsInvalidSlugError(err))
}

func TestService_ByPublicID(t *testing.T) {
	p, err := project.NewProject(uuid.New(), "web", "Web", uuid.Nil)
	require.NoError(t, err)
	got, err := project.NewService(&fakes.ProjectStore{Project: p}, testUnexpected()).ByPublicID(context.Background(), p.PublicID)
	require.NoError(t, err)
	assert.Same(t, p, got)

	_, err = project.NewService(&fakes.ProjectStore{Project: p}, testUnexpected()).ByPublicID(context.Background(), "nope")
	assert.True(t, project.IsNotFoundError(err))
	_, err = project.NewService(&fakes.ProjectStore{Err: errors.New("down")}, testUnexpected()).ByPublicID(context.Background(), p.PublicID)
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
	svc := project.NewService(store, testUnexpected()).WithOrgSlugs(orgSlugs{orgID: "acme"}).WithIssuerBase("https://id.example.com")
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
	svc := project.NewService(store, testUnexpected()).WithOrgSlugs(orgSlugs{orgID: "acme"})
	p, err := svc.Create(context.Background(), project.CreateRequest{OrgID: orgID, Slug: "web", Name: "Web"})
	require.NoError(t, err)
	assert.Equal(t, 1, taken)
	assert.Regexp(t, `^acme-web-[a-z0-9]{4}$`, p.IssuerHandle, "a taken default handle gets a random suffix")

	_, err = svc.Create(context.Background(), project.CreateRequest{OrgID: orgID, Slug: "web", Name: "Web", IssuerHandle: "acme-web"})
	assert.True(t, project.IsIssuerHandleTakenError(err), "a chosen handle is never changed: %v", err)
	_, err = svc.Create(context.Background(), project.CreateRequest{OrgID: orgID, Slug: "web", Name: "Web", IssuerHandle: "system"})
	assert.True(t, project.IsInvalidIssuerHandleError(err))

	reserved := &fakes.ProjectStore{}
	well, err := project.NewService(reserved, testUnexpected()).WithOrgSlugs(orgSlugs{orgID: "well"}).
		Create(context.Background(), project.CreateRequest{OrgID: orgID, Slug: "known", Name: "Known"})
	require.NoError(t, err)
	assert.Regexp(t, `^well-known-[a-z0-9]{4}$`, well.IssuerHandle, "a reserved default is suffixed")
}

func TestService_UpdateIssuerHandle(t *testing.T) {
	p, err := project.NewProject(uuid.New(), "web", "Web", uuid.Nil)
	require.NoError(t, err)
	store := &fakes.ProjectStore{Project: p}
	svc := project.NewService(store, testUnexpected())
	same, err := svc.UpdateIssuerHandle(context.Background(), p.ID, p.IssuerHandle)
	require.NoError(t, err)
	assert.Equal(t, p.IssuerHandle, same.IssuerHandle)

	got, err := svc.UpdateIssuerHandle(context.Background(), p.ID, " acme-shop ")
	require.NoError(t, err)
	assert.Equal(t, "acme-shop", got.IssuerHandle)

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

func TestService_SettingsAndLogo(t *testing.T) {
	p, err := project.NewProject(uuid.New(), "web", "Web", uuid.Nil)
	require.NoError(t, err)
	store := &fakes.ProjectStore{Project: p}
	svc := project.NewService(store, testUnexpected())
	ctx := context.Background()

	_, err = svc.UpdateBranding(ctx, p.ID, "", "red", "")
	require.Error(t, err)
	_, err = svc.UploadLogo(ctx, p.ID, []byte("not an image"))
	assert.True(t, project.IsInvalidLogoError(err))
	_, err = svc.UploadLogo(ctx, p.ID, []byte{0xFF, 0xD8, 0xFF, 0xE0})
	require.NoError(t, err)
	require.NotNil(t, store.LogoContent)
	assert.Equal(t, "image/jpeg", store.LogoContent.ContentType)
	assert.Len(t, store.LogoContent.SHA256, 64)
	l, err := svc.Logo(ctx, p.ID)
	require.NoError(t, err)
	assert.NotNil(t, l)

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
		func() error { _, err := svc.UploadLogo(ctx, p.ID, []byte{0xFF, 0xD8, 0xFF}); return err },
		func() error { _, err := svc.DeleteLogo(ctx, p.ID); return err },
		func() error { _, err := svc.Logo(ctx, p.ID); return err },
		func() error { _, err := svc.UpdateIssuerHandle(ctx, p.ID, "acme-x"); return err },
		func() error { return svc.LockIssuerHandle(ctx, p.ID) },
		func() error { _, err := svc.CheckIssuerHandle(ctx, "acme-x", nil); return err },
		func() error { _, err := svc.ResolveIssuer(ctx, "acme-x"); return err },
	} {
		assert.Error(t, call())
	}
}
