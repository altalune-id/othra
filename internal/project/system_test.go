package project_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/testutil/fakes"
)

func codeOf(err error) apperror.Code {
	if ae, ok := apperror.AsAppError(err); ok {
		return ae.Code()
	}
	return ""
}

// The console project is /p/system in the system org, branded "Othra", with the email link on; no other project can take the handle.
func TestNewSystemProject(t *testing.T) {
	orgID := uuid.New()
	p, err := project.NewSystemProject(orgID)
	require.NoError(t, err)
	assert.True(t, p.IsSystem())
	assert.Equal(t, project.SystemHandle, p.IssuerHandle)
	assert.Equal(t, project.SystemSlug, p.Slug)
	assert.Equal(t, "Console", p.Name)
	assert.Equal(t, "Othra", p.DisplayName)
	assert.True(t, p.SignInEmailLink)
	assert.Equal(t, orgID, p.OrgID)

	other, err := project.NewProject(orgID, "app", "App", uuid.Nil)
	require.NoError(t, err)
	assert.False(t, other.IsSystem())
	assert.Error(t, project.ValidateIssuerHandle(project.SystemHandle), "reserved")
}

// CreateSystem saves the console project through the after-create hook, which generates its signing keys.
func TestService_CreateSystem(t *testing.T) {
	store := &fakes.ProjectStore{}
	var hooked *project.Project
	svc := project.NewService(store, fakes.Transactor{}, fakes.NoMemberships{}, func(_ context.Context, p *project.Project) error {
		hooked = p
		return nil
	}, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs(""))
	p, err := svc.CreateSystem(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, p, store.Saved)
	assert.Equal(t, p, hooked)

	store.Err = assert.AnError
	_, err = svc.CreateSystem(context.Background(), uuid.New())
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

// PRJ011: the console project's name, slug, issuer handle and email-link switch can't change; branding and terms can.
func TestService_SystemProjectGuards(t *testing.T) {
	p, err := project.NewSystemProject(uuid.New())
	require.NoError(t, err)
	store := &fakes.ProjectStore{Project: p}
	svc := project.NewService(store, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs(""))
	ctx := context.Background()

	_, err = svc.Update(ctx, p.ID, "Renamed", "renamed")
	assert.Equal(t, apperror.CodeProjectSystemManaged, codeOf(err))
	_, err = svc.UpdateIssuerHandle(ctx, p.ID, "console-two")
	assert.Equal(t, apperror.CodeProjectSystemManaged, codeOf(err))
	_, err = svc.SetSignInEmailLink(ctx, p.ID, false)
	assert.Equal(t, apperror.CodeProjectSystemManaged, codeOf(err))
	assert.True(t, project.IsSystemManagedError(err))

	on, err := svc.SetSignInEmailLink(ctx, p.ID, true)
	require.NoError(t, err, "turning the email link on stays allowed")
	assert.True(t, on.SignInEmailLink)
	branded, err := svc.UpdateBranding(ctx, p.ID, "Acme Admin", "#112233", "")
	require.NoError(t, err)
	assert.Equal(t, "Acme Admin", branded.DisplayName)
	_, err = svc.UpdateTerms(ctx, p.ID, "https://acme.example/terms", "https://acme.example/privacy")
	require.NoError(t, err)
	assert.Equal(t, project.SystemHandle, p.IssuerHandle)
}
