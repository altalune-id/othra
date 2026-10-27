package org_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	orgv1 "github.com/altalune-id/othra/gen/go/org/v1"
	"github.com/altalune-id/othra/internal/kernel/session"
	"github.com/altalune-id/othra/internal/org"
	"github.com/altalune-id/othra/internal/testutil/fakes"
)

func mustOrg(t *testing.T, slug string) *org.Org {
	t.Helper()
	o, err := org.NewOrg(slug, "Org", uuid.Nil)
	require.NoError(t, err)
	return o
}

func TestOrgReach_IDOnlyRPCs(t *testing.T) {
	orgA, orgB := mustOrg(t, "org-a"), mustOrg(t, "org-b")
	cp := org.NewControlPlane(org.NewService(&fakes.OrgStore{Org: orgB}, fakes.Transactor{}, fakes.NoMemberRoles{}, nil, nil))
	for name, p := range map[string]session.Principal{
		"member of A": {Source: session.SourceLocal, UserID: uuid.New(), ActiveOrgID: orgA.ID, Scopes: []string{"*"}},
		"key of A":    {Source: session.SourceAPIKey, ActiveOrgID: orgA.ID, AllProjects: true, Scopes: []string{"org:read", "org:write"}},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := session.PrincipalInto(context.Background(), p)
			_, err := cp.GetOrg(ctx, connect.NewRequest(&orgv1.GetOrgRequest{Id: orgB.PublicID}))
			assert.True(t, org.IsNotFoundError(err), "GetOrg: %v", err)
			_, err = cp.UpdateOrg(ctx, connect.NewRequest(&orgv1.UpdateOrgRequest{Id: orgB.PublicID, Name: "Hacked"}))
			assert.True(t, org.IsNotFoundError(err), "UpdateOrg: %v", err)
		})
	}
}

func TestOrgReach_UnknownOrUUIDIDIsNotFound(t *testing.T) {
	orgA := mustOrg(t, "org-a")
	cp := org.NewControlPlane(org.NewService(&fakes.OrgStore{Err: &org.NotFoundError{}}, fakes.Transactor{}, fakes.NoMemberRoles{}, nil, nil))
	ctx := session.PrincipalInto(context.Background(), session.Principal{Source: session.SourceLocal, UserID: uuid.New(), ActiveOrgID: orgA.ID, Scopes: []string{"*"}})
	for _, id := range []string{"not-a-public-id", orgA.ID.String()} {
		_, err := cp.GetOrg(ctx, connect.NewRequest(&orgv1.GetOrgRequest{Id: id}))
		assert.True(t, org.IsNotFoundError(err), "GetOrg %s: %v", id, err)
		_, err = cp.UpdateOrg(ctx, connect.NewRequest(&orgv1.UpdateOrgRequest{Id: id, Name: "Hacked"}))
		assert.True(t, org.IsNotFoundError(err), "UpdateOrg %s: %v", id, err)
	}
}

func TestOrgReach_OwnOrgAndTrustedCallers(t *testing.T) {
	o := mustOrg(t, "org-a")
	cp := org.NewControlPlane(org.NewService(&fakes.OrgStore{Org: o}, fakes.Transactor{}, fakes.NoMemberRoles{}, nil, nil))
	for name, ctx := range map[string]context.Context{
		"system":   fakes.OperatorContext(context.Background()),
		"member":   session.PrincipalInto(context.Background(), session.Principal{Source: session.SourceLocal, UserID: uuid.New(), ActiveOrgID: o.ID}),
		"operator": session.PrincipalInto(context.Background(), session.Principal{Source: session.SourceBasic, Operator: true}),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := cp.GetOrg(ctx, connect.NewRequest(&orgv1.GetOrgRequest{Id: o.PublicID}))
			require.NoError(t, err)
		})
	}

	_, err := cp.GetOrg(context.Background(), connect.NewRequest(&orgv1.GetOrgRequest{Id: o.PublicID}))
	assert.True(t, org.IsNotFoundError(err), "no principal reaches nothing: %v", err)
}

func TestOrgReach_ListOrgs(t *testing.T) {
	orgA, orgB, orgC := mustOrg(t, "org-a"), mustOrg(t, "org-b"), mustOrg(t, "org-c")
	cp := org.NewControlPlane(org.NewService(&fakes.OrgStore{Orgs: []*org.Org{orgA, orgB}}, fakes.Transactor{}, fakes.NoMemberRoles{}, nil, nil))
	ids := func(ctx context.Context) []string {
		resp, err := cp.ListOrgs(ctx, connect.NewRequest(&orgv1.ListOrgsRequest{}))
		require.NoError(t, err)
		var out []string
		for _, o := range resp.Msg.GetOrgs() {
			out = append(out, o.GetId())
		}
		return out
	}
	assert.Empty(t, ids(context.Background()), "no principal sees nothing")
	assert.Len(t, ids(fakes.OperatorContext(context.Background())), 2, "system sees all")
	assert.Len(t, ids(session.PrincipalInto(context.Background(), session.Principal{Source: session.SourceBasic, Operator: true})), 2, "operator sees all")
	person := uuid.New()
	cp = org.NewControlPlane(org.NewService(&fakes.OrgStore{
		Orgs:    []*org.Org{orgA, orgB, orgC},
		ForUser: map[uuid.UUID][]*org.Org{person: {orgA, orgC}},
	}, fakes.Transactor{}, fakes.NoMemberRoles{}, nil, nil))
	assert.Equal(t, []string{orgA.PublicID, orgC.PublicID}, ids(session.PrincipalInto(context.Background(),
		session.Principal{Source: session.SourceLocal, UserID: person, ActiveOrgID: orgA.ID})),
		"a person sees every org they belong to, and nothing else")
	assert.Equal(t, []string{orgB.PublicID}, ids(session.PrincipalInto(context.Background(),
		session.Principal{Source: session.SourceAPIKey, ActiveOrgID: orgB.ID})), "key sees its org")
}
