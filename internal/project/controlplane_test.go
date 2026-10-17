package project_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	projectv1 "altalune.id/othra/gen/go/project/v1"
	projectv1connect "altalune.id/othra/gen/go/project/v1/projectv1connect"
	"altalune.id/othra/internal/kernel/storage"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/testutil/fakes"
)

func newProjectServiceClient(t *testing.T, store project.Store, refs project.OrgRefs) projectv1connect.ProjectServiceClient {
	t.Helper()
	_, handler := projectv1connect.NewProjectServiceHandler(project.NewControlPlane(project.NewService(store, nil), refs),
		connect.WithInterceptors(fakes.OperatorInterceptor()))
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return projectv1connect.NewProjectServiceClient(http.DefaultClient, ts.URL)
}

func TestProjectService_CreateProject(t *testing.T) {
	refs := fakes.NewPublicIDs()
	orgPub := refs.Org(uuid.New())
	store := &fakes.ProjectStore{}
	c := newProjectServiceClient(t, store, refs)

	resp, err := c.CreateProject(context.Background(), connect.NewRequest(&projectv1.CreateProjectRequest{OrgId: orgPub, Slug: "acme", Name: "Acme"}))
	require.NoError(t, err)
	require.NotNil(t, store.Saved)
	assert.Equal(t, "acme", resp.Msg.GetProject().GetSlug())
	assert.Equal(t, "Acme", resp.Msg.GetProject().GetName())
	assert.Equal(t, store.Saved.PublicID, resp.Msg.GetProject().GetId())
	assert.Equal(t, orgPub, resp.Msg.GetProject().GetOrgId())
}

func TestProjectService_CreateProject_UnknownOrgID(t *testing.T) {
	refs := fakes.NewPublicIDs()
	orgID := uuid.New()
	refs.Org(orgID)
	c := newProjectServiceClient(t, &fakes.ProjectStore{}, refs)

	for _, id := range []string{"nope", orgID.String()} {
		_, err := c.CreateProject(context.Background(), connect.NewRequest(&projectv1.CreateProjectRequest{OrgId: id, Name: "Acme"}))
		assert.Error(t, err, id)
	}
}

func TestProjectService_GetProject(t *testing.T) {
	refs := fakes.NewPublicIDs()
	orgID := uuid.New()
	orgPub := refs.Org(orgID)
	p, err := project.NewProject(orgID, "acme", "Acme", uuid.New())
	require.NoError(t, err)
	c := newProjectServiceClient(t, &fakes.ProjectStore{Project: p}, refs)

	resp, err := c.GetProject(context.Background(), connect.NewRequest(&projectv1.GetProjectRequest{Id: p.PublicID}))
	require.NoError(t, err)
	assert.Equal(t, "acme", resp.Msg.GetProject().GetSlug())
	assert.Equal(t, p.PublicID, resp.Msg.GetProject().GetId())
	assert.Equal(t, orgPub, resp.Msg.GetProject().GetOrgId())
}

func TestProjectService_GetProject_UnknownID(t *testing.T) {
	p, err := project.NewProject(uuid.New(), "acme", "Acme", uuid.New())
	require.NoError(t, err)
	c := newProjectServiceClient(t, &fakes.ProjectStore{Project: p}, fakes.NewPublicIDs())

	for _, id := range []string{"not-a-public-id", p.ID.String()} {
		_, err := c.GetProject(context.Background(), connect.NewRequest(&projectv1.GetProjectRequest{Id: id}))
		assert.Error(t, err, id)
	}
}

func TestProjectService_ListProjects(t *testing.T) {
	refs := fakes.NewPublicIDs()
	orgID := uuid.New()
	orgPub := refs.Org(orgID)
	p1, err := project.NewProject(orgID, "aaa", "A", uuid.New())
	require.NoError(t, err)
	p2, err := project.NewProject(orgID, "bbb", "B", uuid.New())
	require.NoError(t, err)
	c := newProjectServiceClient(t, &fakes.ProjectStore{Projects: []*project.Project{p1, p2}}, refs)

	resp, err := c.ListProjects(context.Background(), connect.NewRequest(&projectv1.ListProjectsRequest{OrgId: orgPub}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetProjects(), 2)
	assert.Equal(t, p1.PublicID, resp.Msg.GetProjects()[0].GetId())
	assert.Equal(t, orgPub, resp.Msg.GetProjects()[0].GetOrgId())
}

func TestProjectService_ListProjects_UnknownOrgID(t *testing.T) {
	c := newProjectServiceClient(t, &fakes.ProjectStore{}, fakes.NewPublicIDs())

	_, err := c.ListProjects(context.Background(), connect.NewRequest(&projectv1.ListProjectsRequest{OrgId: "not-a-public-id"}))
	assert.Error(t, err)
}

func TestProjectService_ListProjects_StoreError(t *testing.T) {
	refs := fakes.NewPublicIDs()
	c := newProjectServiceClient(t, &fakes.ProjectStore{Err: &project.NotFoundError{Slug: "acme"}}, refs)

	_, err := c.ListProjects(context.Background(), connect.NewRequest(&projectv1.ListProjectsRequest{OrgId: refs.Org(uuid.New())}))
	assert.Error(t, err)
}

func TestProjectService_IssuerRPCs(t *testing.T) {
	refs := fakes.NewPublicIDs()
	orgID := uuid.New()
	orgPub := refs.Org(orgID)
	p, err := project.NewProject(orgID, "web", "Web", uuid.New())
	require.NoError(t, err)
	store := &fakes.ProjectStore{Project: p, TakenHandles: []string{"taken-one"}}
	c := newProjectServiceClient(t, store, refs)
	ctx := context.Background()

	issuer, err := c.UpdateProjectIssuer(ctx, connect.NewRequest(&projectv1.UpdateProjectIssuerRequest{Id: p.PublicID, IssuerHandle: "acme-web"}))
	require.NoError(t, err)
	assert.Equal(t, "acme-web", issuer.Msg.GetProject().GetIssuerHandle())
	assert.Equal(t, "/p/acme-web", issuer.Msg.GetProject().GetIssuerUrl())

	branding, err := c.UpdateProjectBranding(ctx, connect.NewRequest(&projectv1.UpdateProjectBrandingRequest{
		Id: p.PublicID, DisplayName: "Acme", PrimaryColor: "#4F46E5", SupportEmail: "help@acme.test",
	}))
	require.NoError(t, err)
	assert.Equal(t, "Acme", branding.Msg.GetProject().GetDisplayName())

	_, err = c.BumpTermsVersion(ctx, connect.NewRequest(&projectv1.BumpTermsVersionRequest{Id: p.PublicID}))
	require.Error(t, err, "no terms link yet")
	terms, err := c.UpdateProjectTerms(ctx, connect.NewRequest(&projectv1.UpdateProjectTermsRequest{Id: p.PublicID, TermsUrl: "https://acme.test/terms"}))
	require.NoError(t, err)
	assert.Equal(t, int32(1), terms.Msg.GetProject().GetTermsVersion())
	bumped, err := c.BumpTermsVersion(ctx, connect.NewRequest(&projectv1.BumpTermsVersionRequest{Id: p.PublicID}))
	require.NoError(t, err)
	assert.Equal(t, int32(2), bumped.Msg.GetVersion())
	assert.NotNil(t, bumped.Msg.GetPublishedAt())

	deleted, err := c.DeleteProjectLogo(ctx, connect.NewRequest(&projectv1.DeleteProjectLogoRequest{Id: p.PublicID}))
	require.NoError(t, err)
	assert.Empty(t, deleted.Msg.GetProject().GetLogoUrl(), "no logo answers an empty URL")

	for handle, want := range map[string]string{"free-one": "ok", "taken-one": "taken", "system": "reserved", "Bad": "invalid", "acme-web": "ok"} {
		got, err := c.CheckIssuerHandle(ctx, connect.NewRequest(&projectv1.CheckIssuerHandleRequest{OrgId: orgPub, ProjectId: p.PublicID, IssuerHandle: handle}))
		require.NoError(t, err, handle)
		assert.Equal(t, want, got.Msg.GetReason(), handle)
		assert.Equal(t, want == "ok", got.Msg.GetAvailable(), handle)
	}

	for _, call := range []func() error{
		func() error {
			_, err := c.UpdateProjectIssuer(ctx, connect.NewRequest(&projectv1.UpdateProjectIssuerRequest{Id: "unknwnunknwnunknwnabc", IssuerHandle: "acme-web"}))
			return err
		},
		func() error {
			_, err := c.CreateProjectLogoUpload(ctx, connect.NewRequest(&projectv1.CreateProjectLogoUploadRequest{
				Id: "unknwnunknwnunknwnabc", ContentType: "image/png", SizeBytes: 10,
			}))
			return err
		},
		func() error {
			_, err := c.CompleteProjectLogoUpload(ctx, connect.NewRequest(&projectv1.CompleteProjectLogoUploadRequest{
				Id: "unknwnunknwnunknwnabc", UploadId: "unknwnunknwnunknwnabc",
			}))
			return err
		},
		func() error {
			_, err := c.CheckIssuerHandle(ctx, connect.NewRequest(&projectv1.CheckIssuerHandleRequest{OrgId: "unknwnunknwnunknwnabc", IssuerHandle: "acme-web"}))
			return err
		},
		func() error {
			_, err := c.CheckIssuerHandle(ctx, connect.NewRequest(&projectv1.CheckIssuerHandleRequest{OrgId: orgPub, ProjectId: "unknwnunknwnunknwnabc", IssuerHandle: "acme-web"}))
			return err
		},
	} {
		require.Error(t, call())
	}
}

func TestProjectService_LogoUpload(t *testing.T) {
	refs := fakes.NewPublicIDs()
	orgID := uuid.New()
	refs.Org(orgID)
	p, err := project.NewProject(orgID, "web", "Web", uuid.New())
	require.NoError(t, err)
	store := &fakes.ProjectStore{Project: p}
	files := &fakes.FileStorage{Files: map[string]*storage.File{}}
	_, handler := projectv1connect.NewProjectServiceHandler(
		project.NewControlPlane(project.NewService(store, nil).WithStorage(files, fakes.Transactor{}), refs),
		connect.WithInterceptors(fakes.OperatorInterceptor()))
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	c := projectv1connect.NewProjectServiceClient(http.DefaultClient, ts.URL)
	ctx := context.Background()

	up, err := c.CreateProjectLogoUpload(ctx, connect.NewRequest(&projectv1.CreateProjectLogoUploadRequest{
		Id: p.PublicID, ContentType: "image/webp", SizeBytes: 2048,
	}))
	require.NoError(t, err)
	assert.Equal(t, "upxdupxdupxdupxdupxd2", up.Msg.GetUploadId())
	assert.Equal(t, "https://files.test/put", up.Msg.GetUrl())
	assert.Equal(t, "PUT", up.Msg.GetMethod())
	assert.Equal(t, map[string]string{"Content-Type": "image/webp"}, up.Msg.GetHeaders())
	assert.NotNil(t, up.Msg.GetExpiresAt())
	require.Len(t, files.Begun, 1)
	assert.Equal(t, int64(2048), files.Begun[0].Size)

	logo := &storage.File{ID: uuid.New(), PublicID: up.Msg.GetUploadId(), ObjectKey: "public/othra/projects/" + p.PublicID + "/logo/x.webp"}
	files.Files[logo.PublicID] = logo
	p.LogoKey = logo.ObjectKey
	done, err := c.CompleteProjectLogoUpload(ctx, connect.NewRequest(&projectv1.CompleteProjectLogoUploadRequest{Id: p.PublicID, UploadId: logo.PublicID}))
	require.NoError(t, err)
	assert.Equal(t, "https://files.test/"+logo.ObjectKey, done.Msg.GetProject().GetLogoUrl())
	assert.Equal(t, &logo.ID, store.Project.LogoFileID)

	files.BeginErr = &storage.TooLargeError{Purpose: "logo", MaxBytes: 256 << 10}
	_, err = c.CreateProjectLogoUpload(ctx, connect.NewRequest(&projectv1.CreateProjectLogoUploadRequest{Id: p.PublicID, ContentType: "image/png", SizeBytes: 300 << 10}))
	require.Error(t, err)
}
