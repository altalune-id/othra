package consent

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	accountv1 "altalune.id/othra/gen/go/account/v1"
	"altalune.id/othra/internal/kernel/session"
)

type fakeDirectory struct {
	clients map[uuid.UUID]ClientInfo
	servers map[uuid.UUID]ServerInfo
	err     error
}

func (d *fakeDirectory) ClientID(_ context.Context, _ Project, oauthID string) (uuid.UUID, bool, error) {
	for id, c := range d.clients {
		if c.OAuthID == oauthID {
			return id, true, d.err
		}
	}
	return uuid.Nil, false, d.err
}

func (d *fakeDirectory) Clients(context.Context, Project, []uuid.UUID) (map[uuid.UUID]ClientInfo, error) {
	return d.clients, d.err
}

func (d *fakeDirectory) Servers(context.Context, Project) (map[uuid.UUID]ServerInfo, error) {
	return d.servers, d.err
}

// C1, P8: the slice names each app and server and disconnects by client_id, a public id or a metadata document URL.
func TestAccountService_Apps(t *testing.T) {
	store := newFakeStore()
	s, now := newTestService(store)
	p, user := Project{OrgID: uuid.New(), ID: uuid.New()}, uuid.New()
	web, cimd, rs, gone := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	grants := &fakeGrants{families: []Family{
		{ClientID: cimd, Scopes: []string{"openid"}, CreatedAt: *now, LastUsedAt: *now},
		{ClientID: gone, Scopes: []string{"openid"}, CreatedAt: *now},
	}}
	s = s.WithGrants(grants, fakeTx{})
	_, err := s.Grant(context.Background(), p, GrantRequest{Key: Key{EndUserID: user, ClientID: web, ResourceServerID: &rs}, Scopes: []string{"read", "write"}})
	require.NoError(t, err)
	dir := &fakeDirectory{
		clients: map[uuid.UUID]ClientInfo{
			web:  {OAuthID: "cl-web", Name: "Web"},
			cimd: {OAuthID: "https://app.example/client.json", Name: "MCP app", MetadataHost: "app.example"},
		},
		servers: map[uuid.UUID]ServerInfo{rs: {PublicID: "rs-1", Name: "API", Identifier: "https://api.example", Descriptions: map[string]string{"read": "Read things"}}},
	}
	a := NewAccountService(s, dir)
	ctx := session.EndUserInto(context.Background(), session.EndUser{OrgID: p.OrgID, ProjectID: p.ID, EndUserID: user})

	list, err := a.ListConnectedApps(ctx, connect.NewRequest(&accountv1.ListConnectedAppsRequest{}))
	require.NoError(t, err)
	apps := list.Msg.GetApps()
	require.Len(t, apps, 2, "an app whose client is gone is left out")
	byID := map[string]*accountv1.ConnectedApp{}
	for _, app := range apps {
		byID[app.GetClientId()] = app
	}
	w := byID["cl-web"]
	require.NotNil(t, w)
	assert.True(t, w.GetConsented())
	assert.Nil(t, w.GetLastUsedAt(), "never used through a live grant")
	assert.Equal(t, "API", w.GetResources()[0].GetName())
	assert.Equal(t, map[string]string{"read": "Read things"}, w.GetResources()[0].GetScopeDescriptions())
	m := byID["https://app.example/client.json"]
	require.NotNil(t, m)
	assert.Equal(t, "app.example", m.GetMetadataHost())
	assert.Empty(t, m.GetResources()[0].GetId(), "OIDC-only: no resource server")

	_, err = a.DisconnectApp(ctx, connect.NewRequest(&accountv1.DisconnectAppRequest{ClientId: "https://app.example/client.json"}))
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{cimd}, grants.revoked)
	_, err = a.DisconnectApp(ctx, connect.NewRequest(&accountv1.DisconnectAppRequest{ClientId: "unknown"}))
	assert.Equal(t, apperror.CodeAccountAppNotConnected, codeOf(err), "an unknown client answers like a disconnected one")

	empty, err := a.Apps(context.Background(), p, uuid.New())
	require.NoError(t, err)
	assert.Empty(t, empty)

	dir.err = errFake
	_, err = a.ListConnectedApps(ctx, connect.NewRequest(&accountv1.ListConnectedAppsRequest{}))
	assert.ErrorIs(t, err, errFake)
	_, err = a.DisconnectApp(ctx, connect.NewRequest(&accountv1.DisconnectAppRequest{ClientId: "cl-web"}))
	assert.ErrorIs(t, err, errFake)

	bare := context.Background()
	_, err = a.ListConnectedApps(bare, connect.NewRequest(&accountv1.ListConnectedAppsRequest{}))
	assert.Error(t, err)
	_, err = a.DisconnectApp(bare, connect.NewRequest(&accountv1.DisconnectAppRequest{}))
	assert.Error(t, err)
}

func TestAccountService_ServersFail(t *testing.T) {
	store := newFakeStore()
	s, _ := newTestService(store)
	p, user, web := Project{OrgID: uuid.New(), ID: uuid.New()}, uuid.New(), uuid.New()
	s = s.WithGrants(&fakeGrants{families: []Family{{ClientID: web}}}, fakeTx{})
	dir := &fakeDirectory{clients: map[uuid.UUID]ClientInfo{web: {OAuthID: "w"}}}
	a := NewAccountService(s, &serversDown{dir})
	_, err := a.Apps(context.Background(), p, user)
	assert.ErrorIs(t, err, errFake)
}

type serversDown struct{ *fakeDirectory }

func (serversDown) Servers(context.Context, Project) (map[uuid.UUID]ServerInfo, error) {
	return nil, errFake
}
