package project_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	projectv1 "altalune.id/othra/gen/go/project/v1"
	"altalune.id/othra/internal/kernel/session"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/testutil/fakes"
)

type members struct {
	of  map[uuid.UUID]uuid.UUID
	err error
}

func (m members) IsMember(_ context.Context, orgID, userID uuid.UUID) (bool, error) {
	return m.of[userID] == orgID, m.err
}

// SECURITY: a person lists the projects of any org they belong to; anyone else, and any other org, stays not found.
func TestProjectReach_APersonListsAnotherMembershipsProjects(t *testing.T) {
	refs := fakes.NewPublicIDs()
	active, other, foreign := uuid.New(), uuid.New(), uuid.New()
	refs.Org(active)
	otherPub, foreignPub := refs.Org(other), refs.Org(foreign)
	person := uuid.New()
	p := mustProject(t, other)
	svc := project.NewService(&fakes.ProjectStore{Projects: []*project.Project{p}}, fakes.Transactor{}, members{of: map[uuid.UUID]uuid.UUID{person: other}}, nil, "", nil).WithOrgSlugs(fakes.OrgSlugs(""))
	cp := project.NewControlPlane(svc, refs)
	list := func(pr session.Principal, orgPub string) (*connect.Response[projectv1.ListProjectsResponse], error) {
		return cp.ListProjects(session.PrincipalInto(context.Background(), pr), connect.NewRequest(&projectv1.ListProjectsRequest{OrgId: orgPub}))
	}

	resp, err := list(session.Principal{Source: session.SourceLocal, UserID: person, ActiveOrgID: active}, otherPub)
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetProjects(), 1)
	assert.Equal(t, p.PublicID, resp.Msg.GetProjects()[0].GetId())
	assert.Equal(t, otherPub, resp.Msg.GetProjects()[0].GetOrgId())

	_, err = list(session.Principal{Source: session.SourceLocal, UserID: person, ActiveOrgID: active}, foreignPub)
	assert.True(t, project.IsNotFoundError(err), "an org the person is not in: %v", err)
	_, err = list(session.Principal{Source: session.SourceAPIKey, UserID: person, ActiveOrgID: active, AllProjects: true}, otherPub)
	assert.True(t, project.IsNotFoundError(err), "a personal key keeps the reach of its own org: %v", err)

	failing := project.NewControlPlane(project.NewService(&fakes.ProjectStore{}, fakes.Transactor{}, members{err: errors.New("db down")}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs("")), refs)
	_, err = failing.ListProjects(session.PrincipalInto(context.Background(), session.Principal{Source: session.SourceLocal, UserID: person, ActiveOrgID: active}),
		connect.NewRequest(&projectv1.ListProjectsRequest{OrgId: otherPub}))
	require.Error(t, err)
	assert.False(t, project.IsNotFoundError(err))

	without := project.NewControlPlane(project.NewService(&fakes.ProjectStore{}, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", nil).WithOrgSlugs(fakes.OrgSlugs("")), refs)
	_, err = without.ListProjects(session.PrincipalInto(context.Background(), session.Principal{Source: session.SourceLocal, UserID: person, ActiveOrgID: active}),
		connect.NewRequest(&projectv1.ListProjectsRequest{OrgId: otherPub}))
	assert.True(t, project.IsNotFoundError(err), "without a membership port only the active org is reached")
}
