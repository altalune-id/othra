package project_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/testutil/fakes"
)

type methods struct {
	upstream bool
	unlinked bool
	err      error
}

func (m methods) UpstreamEnabled(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return m.upstream, m.err
}
func (m methods) AnyEndUserUnlinked(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return m.unlinked, nil
}

// PRJ010: the email link goes off only while Google is on and every end user has linked it; on is always allowed.
func TestService_SetSignInEmailLink(t *testing.T) {
	newSvc := func(m project.SignInMethods) (*project.Service, *fakes.ProjectStore) {
		store := &fakes.ProjectStore{Project: &project.Project{ID: uuid.New(), OrgID: uuid.New(), SignInEmailLink: true}}
		svc := project.NewService(store, fakes.Transactor{}, fakes.NoMemberships{}, nil, "", testUnexpected()).WithOrgSlugs(fakes.OrgSlugs(""))
		if m != nil {
			svc.WithSignInMethods(m)
		}
		return svc, store
	}
	for name, tc := range map[string]struct {
		m    project.SignInMethods
		want bool
	}{
		"no upstream":           {methods{upstream: false}, false},
		"someone not linked":    {methods{upstream: true, unlinked: true}, false},
		"no methods wired":      {nil, false},
		"google on, all linked": {methods{upstream: true}, true},
	} {
		svc, store := newSvc(tc.m)
		p, err := svc.SetSignInEmailLink(context.Background(), store.Project.ID, false)
		if !tc.want {
			assert.True(t, project.IsSignInLastMethodError(err), "%s: %v", name, err)
			continue
		}
		require.NoError(t, err, name)
		assert.False(t, p.SignInEmailLink, name)
	}

	svc, store := newSvc(methods{})
	var forgotten []uuid.UUID
	svc.WithOnIssuerChange(func(id uuid.UUID) { forgotten = append(forgotten, id) })
	p, err := svc.SetSignInEmailLink(context.Background(), store.Project.ID, true)
	require.NoError(t, err, "turning it on needs nothing")
	assert.True(t, p.SignInEmailLink)
	assert.Equal(t, []uuid.UUID{store.Project.ID}, forgotten, "the issuer cache drops the old switch")

	on, err := svc.LockEmailLink(context.Background(), store.Project.ID)
	require.NoError(t, err)
	assert.True(t, on)
	on, err = svc.EmailLink(context.Background(), store.Project.ID)
	require.NoError(t, err)
	assert.True(t, on, "the unlocked read sees the same switch")

	svc, store = newSvc(methods{err: errors.New("down")})
	_, err = svc.SetSignInEmailLink(context.Background(), store.Project.ID, false)
	assert.Error(t, err)
	store.Err = errors.New("db down")
	_, err = svc.LockEmailLink(context.Background(), store.Project.ID)
	assert.Error(t, err)
	_, err = svc.EmailLink(context.Background(), store.Project.ID)
	assert.Error(t, err)
}
