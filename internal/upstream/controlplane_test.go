package upstream

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	accountv1 "altalune.id/othra/gen/go/account/v1"
	upstreamv1 "altalune.id/othra/gen/go/upstream/v1"
	"altalune.id/othra/internal/kernel/authz"
	"altalune.id/othra/internal/kernel/session"
	"altalune.id/othra/internal/testutil/fakes"
)

type fakeIssuers struct{ url string }

func (f fakeIssuers) IssuerURL(context.Context, uuid.UUID) (string, error) { return f.url, nil }

type cpFixture struct {
	h          *harness
	cp         *UpstreamService
	projectPub string
	foreignPub string
	owner      context.Context
	key        context.Context
}

func newCPFixture(t *testing.T) *cpFixture {
	t.Helper()
	h := newHarnessWith(t, fakeIssuers{url: "https://id.example.com/p/acme"})
	refs := fakes.NewPublicIDs()
	refs.Org(h.org)
	f := &cpFixture{h: h, cp: NewControlPlane(h.svc, refs), projectPub: refs.Project(h.project, h.org)}
	foreignOrg := uuid.New()
	refs.Org(foreignOrg)
	f.foreignPub = refs.Project(uuid.New(), foreignOrg)
	f.owner = session.PrincipalInto(context.Background(), session.Principal{
		Source: session.SourceLocal, UserID: uuid.New(), ActiveOrgID: h.org, Scopes: []string{"*"},
	})
	f.key = session.PrincipalInto(context.Background(), session.Principal{
		Source: session.SourceAPIKey, UserID: uuid.New(), ActiveOrgID: h.org, AllProjects: true, Scopes: []string{"project:*"},
	})
	return f
}

// a person sets the secret; no response ever carries it; the redirect URI is the project's callback.
func TestControlPlane_Lifecycle(t *testing.T) {
	f := newCPFixture(t)
	create := &upstreamv1.CreateUpstreamRequest{
		ProjectId: f.projectPub, Kind: upstreamv1.UpstreamKind_UPSTREAM_KIND_GOOGLE, ClientId: "c", ClientSecret: "GOCSPX-x", Enabled: true,
	}
	_, err := f.cp.CreateUpstream(f.key, connect.NewRequest(create))
	assert.True(t, authz.IsForbiddenError(err), "an API key can't write a client secret: %v", err)
	resp, err := f.cp.CreateUpstream(f.owner, connect.NewRequest(create))
	require.NoError(t, err)
	u := resp.Msg.GetUpstream()
	assert.Equal(t, "https://id.example.com/p/acme/federation/google/callback", u.GetRedirectUri())
	assert.True(t, u.GetSecretSet())
	raw, err := protojson.Marshal(u)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "GOCSPX")

	list, err := f.cp.ListUpstreams(f.key, connect.NewRequest(&upstreamv1.ListUpstreamsRequest{ProjectId: f.projectPub}))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetUpstreams(), 1)
	got, err := f.cp.GetUpstream(f.key, connect.NewRequest(&upstreamv1.GetUpstreamRequest{ProjectId: f.projectPub, UpstreamId: u.GetId()}))
	require.NoError(t, err)
	assert.Equal(t, "c", got.Msg.GetUpstream().GetClientId())

	_, err = f.cp.UpdateUpstream(f.key, connect.NewRequest(&upstreamv1.UpdateUpstreamRequest{ProjectId: f.projectPub, UpstreamId: u.GetId(), ClientId: "c2"}))
	assert.True(t, authz.IsForbiddenError(err))
	upd, err := f.cp.UpdateUpstream(f.owner, connect.NewRequest(&upstreamv1.UpdateUpstreamRequest{ProjectId: f.projectPub, UpstreamId: u.GetId(), ClientId: "c2"}))
	require.NoError(t, err)
	assert.Equal(t, "c2", upd.Msg.GetUpstream().GetClientId())

	off, err := f.cp.SetUpstreamEnabled(f.key, connect.NewRequest(&upstreamv1.SetUpstreamEnabledRequest{ProjectId: f.projectPub, UpstreamId: u.GetId()}))
	require.NoError(t, err, "an API key may switch it")
	assert.False(t, off.Msg.GetUpstream().GetEnabled())
	_, err = f.cp.DeleteUpstream(f.key, connect.NewRequest(&upstreamv1.DeleteUpstreamRequest{ProjectId: f.projectPub, UpstreamId: u.GetId()}))
	require.NoError(t, err)
}

// SECURITY: another org's project and an unknown project answer the same not-found, before any upstream is read.
func TestControlPlane_OutOfReach(t *testing.T) {
	f := newCPFixture(t)
	for _, pub := range []string{f.foreignPub, "nope"} {
		_, err := f.cp.ListUpstreams(f.owner, connect.NewRequest(&upstreamv1.ListUpstreamsRequest{ProjectId: pub}))
		assert.True(t, IsProjectNotFoundError(err), "%s: %v", pub, err)
		_, err = f.cp.CreateUpstream(f.owner, connect.NewRequest(&upstreamv1.CreateUpstreamRequest{ProjectId: pub, ClientId: "c", ClientSecret: "s"}))
		assert.True(t, IsProjectNotFoundError(err), pub)
	}
	_, err := f.cp.GetUpstream(f.owner, connect.NewRequest(&upstreamv1.GetUpstreamRequest{ProjectId: f.projectPub, UpstreamId: "nope"}))
	assert.True(t, IsNotFoundError(err))
	_, err = f.cp.CreateUpstream(f.owner, connect.NewRequest(&upstreamv1.CreateUpstreamRequest{ProjectId: f.projectPub, ClientId: "c", ClientSecret: "s"}))
	assert.True(t, IsInvalidConfigError(err), "an unspecified kind")
}

func TestMethodProto(t *testing.T) {
	assert.Equal(t, accountv1.SignInMethodKind_SIGN_IN_METHOD_KIND_GOOGLE, MethodProto(KindGoogle))
	assert.Equal(t, accountv1.SignInMethodKind_SIGN_IN_METHOD_KIND_UNSPECIFIED, MethodProto("other"))
}
