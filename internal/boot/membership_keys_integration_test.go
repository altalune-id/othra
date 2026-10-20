//go:build integration

package boot

import (
	"context"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	apikeyv1 "altalune.id/othra/gen/go/apikey/v1"
	apikeyv1connect "altalune.id/othra/gen/go/apikey/v1/apikeyv1connect"
	projectv1 "altalune.id/othra/gen/go/project/v1"
	projectv1connect "altalune.id/othra/gen/go/project/v1/projectv1connect"
	"altalune.id/othra/internal/org/member"
	"altalune.id/othra/internal/project"
)

// A person who is admin in their active org and a member of another lists that org's projects and sees what they may grant there.
func TestAPI_APersonPreparesAPersonalKeyInAnotherMembership(t *testing.T) {
	f := newAPIFixture(t)
	alpha, beta := f.newOrg(t, "alpha-scopes"), f.newOrg(t, "beta-scopes")
	cookie, p, userID := f.personCookieIn(t, alpha.ID, "two-roles@example.com", member.RoleAdmin)
	require.False(t, p.Operator, "a person outside the system org")
	_, err := f.svcs.Members.Add(t.Context(), beta.ID, userID, member.RoleMember)
	require.NoError(t, err)

	projects, err := projectv1connect.NewProjectServiceClient(http.DefaultClient, f.url).
		ListProjects(context.Background(), withCookie(connect.NewRequest(&projectv1.ListProjectsRequest{OrgId: beta.PublicID}), cookie))
	require.NoError(t, err, "a member lists the projects of every org they belong to")
	require.Len(t, projects.Msg.GetProjects(), 1)
	assert.Equal(t, "main", projects.Msg.GetProjects()[0].GetSlug())

	keys := apikeyv1connect.NewAPIKeyServiceClient(http.DefaultClient, f.url)
	personal := func(scopes ...string) error {
		_, err := keys.CreateAPIKey(context.Background(), withCookie(connect.NewRequest(&apikeyv1.CreateAPIKeyRequest{
			OrgId: beta.PublicID, Kind: "personal", Name: "laptop", Scopes: scopes,
			ProjectIds: []string{projects.Msg.GetProjects()[0].GetId()}, ExpiresAt: timestamppb.New(time.Now().Add(time.Hour)),
		}), cookie))
		return err
	}
	require.NoError(t, personal("webhook:read", "project:read"), "a scope the member role grants")
	err = personal("project:write")
	require.Error(t, err)
	assert.Equal(t, "KEY007", connectCode(t, err), "the admin scope of the active org does not carry over")

	orgs, err := f.listOrgs(t, cookie)
	require.NoError(t, err)
	byID := map[string][]string{}
	roles := map[string]string{}
	for _, o := range orgs {
		byID[o.GetId()], roles[o.GetId()] = o.GetPermissions(), o.GetRole()
	}
	assert.Equal(t, "member", roles[beta.PublicID])
	assert.Equal(t, member.BuiltinPermissions(member.RoleMember), byID[beta.PublicID])
	assert.Equal(t, "admin", roles[alpha.PublicID])
	assert.Equal(t, member.BuiltinPermissions(member.RoleAdmin), byID[alpha.PublicID])
}

// An operator whose active org is another one lists and revokes the project-limited personal key they made in an org they belong to.
func TestAPI_AnOperatorManagesAProjectKeyInAnotherOrg(t *testing.T) {
	f := newAPIFixture(t)
	beta := f.newOrg(t, "beta-operator")
	cookie, p, userID := f.personCookie(t, "operator@example.com", member.RoleAdmin)
	require.True(t, p.Operator, "an admin of the system org is an operator")
	_, err := f.svcs.Members.Add(t.Context(), beta.ID, userID, member.RoleOwner)
	require.NoError(t, err)

	projects, err := projectv1connect.NewProjectServiceClient(http.DefaultClient, f.url).
		ListProjects(context.Background(), withCookie(connect.NewRequest(&projectv1.ListProjectsRequest{OrgId: beta.PublicID}), cookie))
	require.NoError(t, err)
	require.NotEmpty(t, projects.Msg.GetProjects())

	keys := apikeyv1connect.NewAPIKeyServiceClient(http.DefaultClient, f.url)
	created, err := keys.CreateAPIKey(context.Background(), withCookie(connect.NewRequest(&apikeyv1.CreateAPIKeyRequest{
		OrgId: beta.PublicID, Kind: "personal", Name: "laptop", Scopes: []string{"project:read"},
		ProjectIds: []string{projects.Msg.GetProjects()[0].GetId()}, ExpiresAt: timestamppb.New(time.Now().Add(time.Hour)),
	}), cookie))
	require.NoError(t, err)

	list, err := keys.ListAPIKeys(context.Background(), withCookie(connect.NewRequest(&apikeyv1.ListAPIKeysRequest{OrgId: beta.PublicID}), cookie))
	require.NoError(t, err, "the key just made in the other org lists")
	require.Len(t, list.Msg.GetKeys(), 1)
	assert.Equal(t, created.Msg.GetKey().GetSecretHint(), list.Msg.GetKeys()[0].GetSecretHint())
	assert.Equal(t, []string{projects.Msg.GetProjects()[0].GetId()}, list.Msg.GetKeys()[0].GetProjectIds())

	_, err = keys.RevokeAPIKey(context.Background(), withCookie(connect.NewRequest(&apikeyv1.RevokeAPIKeyRequest{
		OrgId: beta.PublicID, Hint: created.Msg.GetKey().GetSecretHint(),
	}), cookie))
	require.NoError(t, err, "and revokes it")
}

// SECURITY: listing by membership reads only that org, under its own scope, and never reaches an org the person is not in; keys keep their own reach.
func TestAPI_ListProjectsByMembershipStopsAtTheMembership(t *testing.T) {
	f := newAPIFixture(t)
	home, joined, foreign := f.newOrg(t, "home-scopes"), f.newOrg(t, "joined-scopes"), f.newOrg(t, "foreign-scopes")
	_, err := f.svcs.Projects.Create(t.Context(), project.CreateRequest{OrgID: foreign.ID, Slug: "secret", Name: "Secret"})
	require.NoError(t, err)
	cookie, _, userID := f.personCookieIn(t, home.ID, "one-org@example.com", member.RoleOwner)
	_, err = f.svcs.Members.Add(t.Context(), joined.ID, userID, member.RoleMember)
	require.NoError(t, err)
	client := projectv1connect.NewProjectServiceClient(http.DefaultClient, f.url)

	list, err := client.ListProjects(context.Background(), withCookie(connect.NewRequest(&projectv1.ListProjectsRequest{OrgId: joined.PublicID}), cookie))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetProjects(), 1)
	for _, p := range list.Msg.GetProjects() {
		assert.Equal(t, joined.PublicID, p.GetOrgId(), "only the joined org's projects")
		assert.NotEqual(t, "secret", p.GetSlug())
	}

	_, err = client.ListProjects(context.Background(), withCookie(connect.NewRequest(&projectv1.ListProjectsRequest{OrgId: foreign.PublicID}), cookie))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "%v", err)

	req := connect.NewRequest(&projectv1.ListProjectsRequest{OrgId: foreign.PublicID})
	req.Header().Set("Authorization", "Bearer "+f.mint(t, "org", "project:read"))
	_, err = client.ListProjects(context.Background(), req)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "an org key reaches only its own org: %v", err)
}

// An operator sees their role and permissions in each org they belong to, and none in an org they only reach as staff.
func TestAPI_AnOperatorSeesTheirRoleInTheirMemberships(t *testing.T) {
	f := newAPIFixture(t)
	beta, gamma := f.newOrg(t, "beta-staff"), f.newOrg(t, "gamma-staff")
	cookie, p, userID := f.personCookie(t, "staff@example.com", member.RoleAdmin)
	require.True(t, p.Operator, "a system org admin is an operator in selfhosted")
	_, err := f.svcs.Members.Add(t.Context(), beta.ID, userID, member.RoleMember)
	require.NoError(t, err)

	orgs, err := f.listOrgs(t, cookie)
	require.NoError(t, err)
	roles, permissions := map[string]string{}, map[string][]string{}
	for _, o := range orgs {
		roles[o.GetId()], permissions[o.GetId()] = o.GetRole(), o.GetPermissions()
	}
	require.Contains(t, roles, gamma.PublicID, "an operator lists every org")
	assert.Equal(t, "admin", roles[f.orgID])
	assert.Equal(t, member.BuiltinPermissions(member.RoleAdmin), permissions[f.orgID])
	assert.Equal(t, "member", roles[beta.PublicID])
	assert.Equal(t, member.BuiltinPermissions(member.RoleMember), permissions[beta.PublicID])
	assert.Empty(t, roles[gamma.PublicID], "no membership, no role")
	assert.Empty(t, permissions[gamma.PublicID])
}
