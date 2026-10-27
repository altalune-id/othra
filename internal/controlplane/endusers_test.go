package controlplane

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accountv1 "github.com/altalune-id/othra/gen/go/account/v1"
	enduserv1 "github.com/altalune-id/othra/gen/go/enduser/v1"
	"github.com/altalune-id/othra/internal/enduser"
	"github.com/altalune-id/othra/internal/federation"
	"github.com/altalune-id/othra/internal/kernel/session"
	"github.com/altalune-id/othra/internal/project"
)

type fakeProjectRefs struct {
	id, org uuid.UUID
	err     error
}

func (f fakeProjectRefs) ProjectID(_ context.Context, publicID string) (uuid.UUID, uuid.UUID, bool, error) {
	return f.id, f.org, publicID == "prj", f.err
}

type fakeEndUsers struct {
	user  *enduser.EndUser
	live  []enduser.LiveSession
	ended []string
	err   error
}

func (f *fakeEndUsers) ByPublicID(_ context.Context, _ enduser.Project, publicID string) (*enduser.EndUser, error) {
	if f.user == nil || publicID != f.user.Subject {
		return nil, &enduser.EndUserNotFoundError{}
	}
	return f.user, nil
}

func (f *fakeEndUsers) ListEndUsers(context.Context, enduser.Project, string, string, int) ([]*enduser.EndUser, string, error) {
	return []*enduser.EndUser{f.user}, "next", f.err
}

func (f *fakeEndUsers) Sessions(context.Context, enduser.Project, uuid.UUID) ([]enduser.LiveSession, error) {
	return f.live, f.err
}

func (f *fakeEndUsers) EndUserSession(_ context.Context, _ enduser.Project, _ uuid.UUID, publicID string) error {
	f.ended = append(f.ended, publicID)
	return f.err
}

type fakeMethods struct{ err error }

func (f fakeMethods) SignInMethods(context.Context, federation.Project, uuid.UUID) ([]federation.SignInMethod, error) {
	return []federation.SignInMethod{{Linked: true, LastMethod: true}}, f.err
}

type adminFixture struct {
	svc    *EndUserService
	users  *fakeEndUsers
	apps   *fakeApps
	tokens *fakeTokens
	org    uuid.UUID
}

func newAdminFixture() *adminFixture {
	f := &adminFixture{
		users: &fakeEndUsers{
			user: &enduser.EndUser{ID: uuid.New(), Subject: "eu-ana", Email: "ana@example.com", CreatedAt: time.Now()},
			live: []enduser.LiveSession{{PublicID: "us-1", Method: enduser.MethodMagicLink}},
		},
		apps:   &fakeApps{apps: []*accountv1.ConnectedApp{{ClientId: "web"}}},
		tokens: &fakeTokens{tokens: []*accountv1.PersonalToken{{Id: "pt-1"}}},
		org:    uuid.New(),
	}
	slices := AccountSlices{Profile: fakeProfile{}, Apps: f.apps, Tokens: f.tokens}
	f.svc = NewEndUserService(f.users, fakeMethods{}, slices, fakeProjectRefs{id: uuid.New(), org: f.org})
	return f
}

func (f *adminFixture) as(person bool) context.Context {
	if person {
		return session.PrincipalInto(context.Background(), session.Principal{Operator: true})
	}
	return session.PrincipalInto(context.Background(), session.Principal{UserID: uuid.New(), ActiveOrgID: uuid.New()})
}

// the admin view reads an end user through the same slices as the portal.
func TestEndUserService_Reads(t *testing.T) {
	f := newAdminFixture()
	ctx := f.as(true)
	list, err := f.svc.ListEndUsers(ctx, connect.NewRequest(&enduserv1.ListEndUsersRequest{ProjectId: "prj"}))
	require.NoError(t, err)
	assert.Equal(t, "eu-ana", list.Msg.GetEndUsers()[0].GetId())
	assert.Equal(t, "next", list.Msg.GetNext())

	got, err := f.svc.GetEndUser(ctx, connect.NewRequest(&enduserv1.GetEndUserRequest{ProjectId: "prj", EndUserId: "eu-ana"}))
	require.NoError(t, err)
	assert.Equal(t, "ana@example.com", got.Msg.GetEndUser().GetEmail())
	assert.Len(t, got.Msg.GetMethods(), 1)
	assert.False(t, got.Msg.GetSessions()[0].GetCurrent(), "an admin has no current session here")
	assert.Equal(t, "web", got.Msg.GetApps()[0].GetClientId())
	assert.Equal(t, "pt-1", got.Msg.GetTokens()[0].GetId())

	_, err = f.svc.GetEndUser(ctx, connect.NewRequest(&enduserv1.GetEndUserRequest{ProjectId: "prj", EndUserId: "eu-bob"}))
	assert.True(t, enduser.IsEndUserNotFoundError(err))
}

func TestEndUserService_Revokes(t *testing.T) {
	f := newAdminFixture()
	ctx := f.as(true)
	_, err := f.svc.RevokeEndUserSession(ctx, connect.NewRequest(&enduserv1.RevokeEndUserSessionRequest{
		ProjectId: "prj", EndUserId: "eu-ana", SessionId: "us-1",
	}))
	require.NoError(t, err, "an admin may end the current session too")
	assert.Equal(t, []string{"us-1"}, f.users.ended)
	_, err = f.svc.DisconnectEndUserApp(ctx, connect.NewRequest(&enduserv1.DisconnectEndUserAppRequest{
		ProjectId: "prj", EndUserId: "eu-ana", ClientId: "web",
	}))
	require.NoError(t, err)
	assert.Equal(t, []string{"web"}, f.apps.gone)
	_, err = f.svc.RevokeEndUserToken(ctx, connect.NewRequest(&enduserv1.RevokeEndUserTokenRequest{
		ProjectId: "prj", EndUserId: "eu-ana", TokenId: "pt-1",
	}))
	require.NoError(t, err)
	assert.Equal(t, []string{"pt-1"}, f.tokens.revoked)
}

// SECURITY: an unknown project and one out of reach answer the same not-found, before anything is read or revoked.
func TestEndUserService_Reach(t *testing.T) {
	f := newAdminFixture()
	outside := f.as(false)
	_, err := f.svc.ListEndUsers(outside, connect.NewRequest(&enduserv1.ListEndUsersRequest{ProjectId: "prj"}))
	assert.True(t, project.IsNotFoundError(err))
	_, err = f.svc.RevokeEndUserSession(outside, connect.NewRequest(&enduserv1.RevokeEndUserSessionRequest{
		ProjectId: "prj", EndUserId: "eu-ana", SessionId: "us-1",
	}))
	assert.True(t, project.IsNotFoundError(err))
	assert.Empty(t, f.users.ended)
	_, err = f.svc.GetEndUser(f.as(true), connect.NewRequest(&enduserv1.GetEndUserRequest{ProjectId: "other", EndUserId: "eu-ana"}))
	assert.True(t, project.IsNotFoundError(err))
}

func TestEndUserService_Failures(t *testing.T) {
	boom := errors.New("boom")
	f := newAdminFixture()
	ctx := f.as(true)
	get := func() error {
		_, err := f.svc.GetEndUser(ctx, connect.NewRequest(&enduserv1.GetEndUserRequest{ProjectId: "prj", EndUserId: "eu-ana"}))
		return err
	}
	f.tokens.err = boom
	assert.ErrorIs(t, get(), boom)
	f.apps.err = boom
	assert.ErrorIs(t, get(), boom)
	f.users.err = boom
	assert.ErrorIs(t, get(), boom)
	_, err := f.svc.ListEndUsers(ctx, connect.NewRequest(&enduserv1.ListEndUsersRequest{ProjectId: "prj"}))
	assert.ErrorIs(t, err, boom)
	for _, call := range []func() error{
		func() error {
			_, err := f.svc.RevokeEndUserSession(ctx, connect.NewRequest(&enduserv1.RevokeEndUserSessionRequest{ProjectId: "prj", EndUserId: "eu-ana"}))
			return err
		},
		func() error {
			_, err := f.svc.DisconnectEndUserApp(ctx, connect.NewRequest(&enduserv1.DisconnectEndUserAppRequest{ProjectId: "prj", EndUserId: "eu-ana"}))
			return err
		},
		func() error {
			_, err := f.svc.RevokeEndUserToken(ctx, connect.NewRequest(&enduserv1.RevokeEndUserTokenRequest{ProjectId: "prj", EndUserId: "eu-ana"}))
			return err
		},
	} {
		assert.ErrorIs(t, call(), boom)
	}
	f.svc.methods = fakeMethods{err: boom}
	f.users.err = nil
	assert.ErrorIs(t, get(), boom)
	f.svc.refs = fakeProjectRefs{err: boom}
	assert.ErrorIs(t, get(), boom)
}
