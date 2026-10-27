package install

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/legal"
	"altalune.id/othra/internal/org"
	"altalune.id/othra/internal/org/member"
	"altalune.id/othra/internal/testutil/fakes"
)

type acceptedTerms struct{}

func (acceptedTerms) Accept(context.Context, uuid.UUID, legal.Source) (time.Time, error) {
	return time.Now(), nil
}

func newRequiredService(t *testing.T, sys *org.Org) (*Service, *member.Service) {
	t.Helper()
	unexpected := apperror.NewReporter(nil, false).Unexpected
	members := member.NewService(&fakes.MemberStore{}, fakes.Transactor{}, unexpected)
	orgs := org.NewService(&fakes.OrgStore{Org: sys}, fakes.Transactor{}, fakes.NoMemberRoles{}, nil, unexpected)
	return NewService(fakes.Transactor{}, nil, orgs, members, nil, acceptedTerms{}, nil), members
}

func TestRequired(t *testing.T) {
	ctx := context.Background()

	svc, _ := newRequiredService(t, nil)
	for _, basic := range []bool{false, true} {
		got, err := svc.Required(ctx, basic)
		require.NoError(t, err)
		assert.True(t, got, "no system org: basic=%v", basic)
	}

	sys, err := org.NewSystemOrg("default", "Default", uuid.Nil)
	require.NoError(t, err)
	svc, members := newRequiredService(t, sys)
	got, err := svc.Required(ctx, false)
	require.NoError(t, err)
	assert.True(t, got, "local: an ownerless system org still needs install")
	got, err = svc.Required(ctx, true)
	require.NoError(t, err)
	assert.False(t, got, "basic: the system org alone is enough")

	_, err = members.Add(ctx, sys.ID, uuid.New(), member.RoleOwner)
	require.NoError(t, err)
	got, err = svc.Required(ctx, false)
	require.NoError(t, err)
	assert.False(t, got)
}

func TestAlreadyInstalledError(t *testing.T) {
	err := &AlreadyInstalledError{}
	assert.Equal(t, "install: already installed", err.Error())
	assert.True(t, IsAlreadyInstalledError(err))
	assert.False(t, IsAlreadyInstalledError(assert.AnError))
	ae := err.ToAppError()
	assert.Equal(t, apperror.CodeInstallAlreadyDone, ae.Code())
	assert.Equal(t, "This instance is already installed", ae.Message())
}

func TestRequired_PropagatesErrors(t *testing.T) {
	ctx := context.Background()
	unexpected := apperror.NewReporter(nil, false).Unexpected
	sys, err := org.NewSystemOrg("default", "Default", uuid.Nil)
	require.NoError(t, err)

	orgs := org.NewService(&fakes.OrgStore{Err: errors.New("org store down")}, fakes.Transactor{}, fakes.NoMemberRoles{}, nil, unexpected)
	svc := NewService(fakes.Transactor{}, nil, orgs, member.NewService(&fakes.MemberStore{}, fakes.Transactor{}, unexpected), nil, acceptedTerms{}, nil)
	_, err = svc.Required(ctx, true)
	require.Error(t, err)

	orgs = org.NewService(&fakes.OrgStore{Org: sys}, fakes.Transactor{}, fakes.NoMemberRoles{}, nil, unexpected)
	svc = NewService(fakes.Transactor{}, nil, orgs, member.NewService(&fakes.MemberStore{Err: errors.New("member store down")}, fakes.Transactor{}, unexpected), nil, acceptedTerms{}, nil)
	_, err = svc.Required(ctx, true)
	require.NoError(t, err, "basic mode never counts owners")
	_, err = svc.Required(ctx, false)
	require.Error(t, err)
}
