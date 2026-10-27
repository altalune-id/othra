package apikey_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apikeyv1 "github.com/altalune-id/othra/gen/go/apikey/v1"
	"github.com/altalune-id/othra/internal/apikey"
	"github.com/altalune-id/othra/internal/org/member"
	"github.com/altalune-id/othra/internal/testutil/fakes"
)

type membershipsFixture struct {
	refs                 *fakes.PublicIDs
	owners               *fakeOwners
	store                *fakes.APIKeyStore
	me, other            uuid.UUID
	active, beta, gamma  uuid.UUID
	activePub, betaPub   string
	gammaPub             string
	mine, theirs, orgKey *apikey.APIKey
}

// NOTE: the person owns their active org, is a plain member of beta, and is not in gamma.
func newMembershipsFixture() *membershipsFixture {
	f := &membershipsFixture{refs: fakes.NewPublicIDs(), owners: &fakeOwners{}, me: uuid.New(), other: uuid.New(),
		active: uuid.New(), beta: uuid.New(), gamma: uuid.New()}
	f.activePub, f.betaPub, f.gammaPub = f.refs.Org(f.active), f.refs.Org(f.beta), f.refs.Org(f.gamma)
	f.owners.set(f.active, f.me, member.RoleOwner)
	f.owners.set(f.beta, f.me, member.RoleMember)
	f.owners.set(f.beta, f.other, member.RoleMember)
	at := time.Now().UTC()
	f.mine = &apikey.APIKey{ID: uuid.New(), OrgID: f.beta, Kind: apikey.KindPersonal, CreatedBy: f.me, Name: "mine", SecretHint: "aaaa", CreatedAt: at}
	f.theirs = &apikey.APIKey{ID: uuid.New(), OrgID: f.beta, Kind: apikey.KindPersonal, CreatedBy: f.other, Name: "theirs", SecretHint: "bbbb", CreatedAt: at}
	f.orgKey = &apikey.APIKey{ID: uuid.New(), OrgID: f.beta, Kind: apikey.KindOrg, Name: "ci", SecretHint: "cccc", CreatedAt: at}
	f.store = &fakes.APIKeyStore{Keys: []*apikey.APIKey{f.mine, f.theirs, f.orgKey}}
	return f
}

func (f *membershipsFixture) client(t *testing.T) interface {
	CreateAPIKey(context.Context, *connect.Request[apikeyv1.CreateAPIKeyRequest]) (*connect.Response[apikeyv1.CreateAPIKeyResponse], error)
	ListAPIKeys(context.Context, *connect.Request[apikeyv1.ListAPIKeysRequest]) (*connect.Response[apikeyv1.ListAPIKeysResponse], error)
	RevokeAPIKey(context.Context, *connect.Request[apikeyv1.RevokeAPIKeyRequest]) (*connect.Response[apikeyv1.RevokeAPIKeyResponse], error)
} {
	t.Helper()
	return newPersonalClient(t, f.refs, f.store, f.owners, withRole(person(f.me, f.active), member.RoleOwner))
}

// SECURITY: a person handles their own personal keys in every org they belong to, whatever org is active.
func TestPersonalKeysInANonActiveOrg_CreateListRevoke(t *testing.T) {
	f := newMembershipsFixture()
	c := f.client(t)

	resp, err := c.CreateAPIKey(context.Background(), personalCreate(f.betaPub))
	require.NoError(t, err)
	assert.Equal(t, f.betaPub, resp.Msg.GetKey().GetOrgId())
	require.NotNil(t, f.store.Saved)
	assert.Equal(t, f.beta, f.store.Saved.OrgID)
	assert.Equal(t, f.me, f.store.Saved.CreatedBy)

	list, err := c.ListAPIKeys(context.Background(), connect.NewRequest(&apikeyv1.ListAPIKeysRequest{OrgId: f.betaPub}))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetKeys(), 1, "only my own personal keys there: never another person's, never the org's")
	assert.Equal(t, "aaaa", list.Msg.GetKeys()[0].GetSecretHint())

	_, err = c.RevokeAPIKey(context.Background(), connect.NewRequest(&apikeyv1.RevokeAPIKeyRequest{OrgId: f.betaPub, Hint: "aaaa"}))
	require.NoError(t, err)
	assert.NotNil(t, f.mine.RevokedAt)
}

func TestPersonalKeysInANonActiveOrg_ScopesAreCappedByThatOrgsRole(t *testing.T) {
	f := newMembershipsFixture()
	req := personalCreate(f.betaPub)
	req.Msg.Scopes = []string{"member:write"}

	_, err := f.client(t).CreateAPIKey(context.Background(), req)
	require.Error(t, err, "owner of the active org, but a plain member in beta")
	assert.Nil(t, f.store.Saved)
}

// SECURITY: an org you do not belong to answers the same not-found as an unknown one, on every personal key call.
func TestPersonalKeysInAnOrgYouDoNotBelongTo_AreNotFound(t *testing.T) {
	f := newMembershipsFixture()
	c := f.client(t)

	_, err := c.CreateAPIKey(context.Background(), personalCreate(f.gammaPub))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "create: %v", err)
	_, err = c.ListAPIKeys(context.Background(), connect.NewRequest(&apikeyv1.ListAPIKeysRequest{OrgId: f.gammaPub}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "list: %v", err)
	_, err = c.RevokeAPIKey(context.Background(), connect.NewRequest(&apikeyv1.RevokeAPIKeyRequest{OrgId: f.gammaPub, Hint: "aaaa"}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "revoke: %v", err)
	assert.Nil(t, f.store.Saved)
}

func TestPersonalKeysInANonActiveOrg_NeverReachAnotherPersonsOrTheOrgsKeys(t *testing.T) {
	f := newMembershipsFixture()
	c := f.client(t)

	_, err := c.RevokeAPIKey(context.Background(), connect.NewRequest(&apikeyv1.RevokeAPIKeyRequest{OrgId: f.betaPub, Hint: "bbbb"}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "another person's personal key: %v", err)
	assert.Nil(t, f.theirs.RevokedAt)

	_, err = c.RevokeAPIKey(context.Background(), connect.NewRequest(&apikeyv1.RevokeAPIKeyRequest{OrgId: f.betaPub, Hint: "cccc"}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "the org's key outside the active org: %v", err)
	assert.Nil(t, f.orgKey.RevokedAt)

	orgKey := personalCreate(f.betaPub)
	orgKey.Msg.Kind = "org"
	_, err = c.CreateAPIKey(context.Background(), orgKey)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "org keys stay active-org only: %v", err)
	assert.Nil(t, f.store.Saved)
}
