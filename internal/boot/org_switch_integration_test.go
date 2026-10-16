//go:build integration

package boot

import (
	"context"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	apikeyv1 "altalune.id/othra/gen/go/apikey/v1"
	apikeyv1connect "altalune.id/othra/gen/go/apikey/v1/apikeyv1connect"
	authv1 "altalune.id/othra/gen/go/auth/v1"
	authv1connect "altalune.id/othra/gen/go/auth/v1/authv1connect"
	projectv1 "altalune.id/othra/gen/go/project/v1"
	projectv1connect "altalune.id/othra/gen/go/project/v1/projectv1connect"
	"altalune.id/othra/internal/apikey"
	"altalune.id/othra/internal/org"
	"altalune.id/othra/internal/org/member"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/nanoid"
)

func (f *apiFixture) setActiveOrg(cookie, bearer, orgID string) (*authv1.WhoamiResponse, error) {
	req := connect.NewRequest(&authv1.SetActiveOrgRequest{OrgId: orgID})
	if cookie != "" {
		req = withCookie(req, cookie)
	}
	if bearer != "" {
		req.Header().Set("Authorization", "Bearer "+bearer)
	}
	resp, err := authv1connect.NewAuthServiceClient(http.DefaultClient, f.url).SetActiveOrg(context.Background(), req)
	if err != nil {
		return nil, err
	}
	return resp.Msg.GetMe(), nil
}

func (f *apiFixture) listProjects(cookie, orgID string) error {
	_, err := projectv1connect.NewProjectServiceClient(http.DefaultClient, f.url).
		ListProjects(context.Background(), withCookie(connect.NewRequest(&projectv1.ListProjectsRequest{OrgId: orgID}), cookie))
	return err
}

func (f *apiFixture) newOrg(t *testing.T, slug string) *org.Org {
	t.Helper()
	o, err := org.NewOrg(slug, slug, uuid.Nil)
	require.NoError(t, err)
	require.NoError(t, f.saveOrg(t, o))
	_, err = f.svcs.Projects.Create(t.Context(), project.CreateRequest{OrgID: o.ID, Slug: "main", Name: "Main"})
	require.NoError(t, err)
	return o
}

func TestAPI_APersonSwitchesBetweenTheirOrgs(t *testing.T) {
	f := newAPIFixture(t)
	beta, foreign := f.newOrg(t, "beta-org"), f.newOrg(t, "foreign-org")
	cookie, _, userID := f.personCookie(t, "two-orgs@example.com", member.RoleMember)
	_, err := f.svcs.Members.Add(t.Context(), beta.ID, userID, member.RoleAdmin)
	require.NoError(t, err)

	orgs, err := f.listOrgs(t, cookie)
	require.NoError(t, err)
	var listed []string
	for _, o := range orgs {
		listed = append(listed, o.GetId())
	}
	assert.ElementsMatch(t, []string{f.orgID, beta.PublicID}, listed, "every org the person belongs to, and nothing else")

	require.Error(t, f.listProjects(cookie, beta.PublicID), "before the switch the second org is out of reach")

	me, err := f.setActiveOrg(cookie, "", beta.PublicID)
	require.NoError(t, err)
	assert.Equal(t, beta.PublicID, me.GetActiveOrgId())
	assert.Contains(t, me.GetPermissions(), "member:*", "admin scopes in the new org")

	require.NoError(t, f.listProjects(cookie, beta.PublicID), "the switch holds for later requests")
	require.Error(t, f.listProjects(cookie, f.orgID), "the old org is now out of reach")

	_, err = f.setActiveOrg(cookie, "", foreign.PublicID)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "an org you do not belong to is not found: %v", err)
	unknown, err := nanoid.New(21)
	require.NoError(t, err)
	_, err = f.setActiveOrg(cookie, "", unknown)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "an unknown org is not found: %v", err)

	_, err = f.setActiveOrg("", f.mint(t, apikey.KindOrg, "org:read"), beta.PublicID)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), "a key never switches: %v", err)
}

// SECURITY: a person handles their own personal keys in every org they belong to, without switching; never in an org they are not in.
func TestAPI_APersonManagesOwnPersonalKeysInEveryMembership(t *testing.T) {
	f := newAPIFixture(t)
	beta, foreign := f.newOrg(t, "beta-keys"), f.newOrg(t, "foreign-keys")
	cookie, _, userID := f.personCookie(t, "keys@example.com", member.RoleMember)
	_, err := f.svcs.Members.Add(t.Context(), beta.ID, userID, member.RoleMember)
	require.NoError(t, err)
	keys := apikeyv1connect.NewAPIKeyServiceClient(http.DefaultClient, f.url)
	personal := func(orgID string) *connect.Request[apikeyv1.CreateAPIKeyRequest] {
		return withCookie(connect.NewRequest(&apikeyv1.CreateAPIKeyRequest{
			OrgId: orgID, Kind: "personal", Name: "laptop", Scopes: []string{"webhook:read"}, AllProjects: true,
			ExpiresAt: timestamppb.New(time.Now().Add(time.Hour)),
		}), cookie)
	}

	created, err := keys.CreateAPIKey(context.Background(), personal(beta.PublicID))
	require.NoError(t, err, "a non-active org the person belongs to")
	list, err := keys.ListAPIKeys(context.Background(), withCookie(connect.NewRequest(&apikeyv1.ListAPIKeysRequest{OrgId: beta.PublicID}), cookie))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetKeys(), 1)
	assert.Equal(t, created.Msg.GetKey().GetSecretHint(), list.Msg.GetKeys()[0].GetSecretHint())
	_, err = keys.RevokeAPIKey(context.Background(), withCookie(connect.NewRequest(&apikeyv1.RevokeAPIKeyRequest{
		OrgId: beta.PublicID, Hint: created.Msg.GetKey().GetSecretHint(),
	}), cookie))
	require.NoError(t, err)

	_, err = keys.CreateAPIKey(context.Background(), personal(foreign.PublicID))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "an org the person is not in: %v", err)
	_, err = keys.ListAPIKeys(context.Background(), withCookie(connect.NewRequest(&apikeyv1.ListAPIKeysRequest{OrgId: foreign.PublicID}), cookie))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "%v", err)
}
