package org_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/apperror"
	orgv1 "github.com/altalune-id/othra/gen/go/org/v1"
	"github.com/altalune-id/othra/internal/kernel/session"
	"github.com/altalune-id/othra/internal/org"
	"github.com/altalune-id/othra/internal/testutil/fakes"
)

type memberRoles struct {
	roles map[uuid.UUID]string
	err   error
}

func (m memberRoles) Role(_ context.Context, orgID, _ uuid.UUID) (string, []string, bool, error) {
	if m.err != nil {
		return "", nil, false, m.err
	}
	r, ok := m.roles[orgID]
	if !ok {
		return "", nil, false, nil
	}
	return r, []string{r + ":perm"}, true, nil
}

func TestListOrgs_APersonSeesTheirRoleInEachOrg(t *testing.T) {
	orgA, orgB := mustOrg(t, "org-a"), mustOrg(t, "org-b")
	person := uuid.New()
	store := &fakes.OrgStore{Orgs: []*org.Org{orgA, orgB}, ForUser: map[uuid.UUID][]*org.Org{person: {orgA, orgB}}}
	roles := memberRoles{roles: map[uuid.UUID]string{orgA.ID: "admin", orgB.ID: "member"}}
	cp := org.NewControlPlane(org.NewService(store, fakes.Transactor{}, roles, nil, nil))
	list := func(p session.Principal) []*orgv1.Org {
		resp, err := cp.ListOrgs(session.PrincipalInto(context.Background(), p), connect.NewRequest(&orgv1.ListOrgsRequest{}))
		require.NoError(t, err)
		return resp.Msg.GetOrgs()
	}

	got := list(session.Principal{Source: session.SourceLocal, UserID: person, ActiveOrgID: orgA.ID})
	require.Len(t, got, 2)
	assert.Equal(t, "admin", got[0].GetRole())
	assert.Equal(t, []string{"admin:perm"}, got[0].GetPermissions())
	assert.Equal(t, "member", got[1].GetRole(), "a membership other than the active org")
	assert.Equal(t, []string{"member:perm"}, got[1].GetPermissions())

	for _, o := range list(session.Principal{Source: session.SourceAPIKey, ActiveOrgID: orgA.ID}) {
		assert.Empty(t, o.GetRole(), "a key has no role")
		assert.Empty(t, o.GetPermissions())
	}
	for _, o := range list(session.Principal{Source: session.SourceBasic, Operator: true}) {
		assert.Empty(t, o.GetRole(), "a Basic operator is no member of any org")
	}

	staff := uuid.New()
	store.ForUser[staff] = []*org.Org{orgB}
	got = list(session.Principal{Source: session.SourceLocal, UserID: staff, Operator: true, ActiveOrgID: orgA.ID})
	require.Len(t, got, 2, "an operator lists every org")
	assert.Equal(t, orgA.PublicID, got[0].GetId())
	assert.Empty(t, got[0].GetRole(), "an org the operator reaches without a membership has no role")
	assert.Empty(t, got[0].GetPermissions())
	assert.Equal(t, "member", got[1].GetRole(), "an operator's own membership carries its role")
	assert.Equal(t, []string{"member:perm"}, got[1].GetPermissions())

	reporter := apperror.NewReporter(nil, false).Unexpected
	cp = org.NewControlPlane(org.NewService(store, fakes.Transactor{}, memberRoles{err: errors.New("db down")}, nil, reporter))
	_, err := cp.ListOrgs(session.PrincipalInto(context.Background(), session.Principal{Source: session.SourceLocal, UserID: person}),
		connect.NewRequest(&orgv1.ListOrgsRequest{}))
	require.Error(t, err)

	cp = org.NewControlPlane(org.NewService(store, fakes.Transactor{}, fakes.NoMemberRoles{}, nil, nil))
	for _, o := range list(session.Principal{Source: session.SourceLocal, UserID: person}) {
		assert.Empty(t, o.GetRole(), "without a role port nothing is filled")
	}
}
