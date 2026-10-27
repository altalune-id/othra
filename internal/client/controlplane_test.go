package client_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"altalune.id/othra/apperror"
	clientv1 "altalune.id/othra/gen/go/client/v1"
	"altalune.id/othra/internal/client"
	"altalune.id/othra/internal/kernel/session"
	"altalune.id/othra/internal/testutil/fakes"
)

type cpFixture struct {
	h          *harness
	refs       *fakes.PublicIDs
	cp         *client.ClientService
	projectPub string
	foreignPub string
	person     context.Context
	key        context.Context
}

func newCPFixture(t *testing.T) *cpFixture {
	t.Helper()
	h := newHarness(t)
	refs := fakes.NewPublicIDs()
	refs.Org(h.org)
	f := &cpFixture{h: h, refs: refs, cp: client.NewControlPlane(h.svc, refs), projectPub: refs.Project(h.project, h.org)}
	foreignOrg := uuid.New()
	refs.Org(foreignOrg)
	f.foreignPub = refs.Project(uuid.New(), foreignOrg)
	f.person = session.PrincipalInto(context.Background(), session.Principal{
		Source: session.SourceLocal, UserID: uuid.New(), ActiveOrgID: h.org, Scopes: []string{"*"},
	})
	f.key = session.PrincipalInto(context.Background(), session.Principal{
		Source: session.SourceAPIKey, ActiveOrgID: h.org, AllProjects: true, Scopes: []string{"project:write"},
	})
	return f
}

func (f *cpFixture) createWeb(t *testing.T, ctx context.Context) *clientv1.Client {
	t.Helper()
	resp, err := f.cp.CreateClient(ctx, connect.NewRequest(&clientv1.CreateClientRequest{
		ProjectId: f.projectPub, Name: "Web", Kind: clientv1.ClientKind_CLIENT_KIND_WEB,
		RedirectUris: []string{"https://app.example/callback"}, ConsentPolicy: clientv1.ConsentPolicy_CONSENT_POLICY_IMPLICIT,
	}))
	require.NoError(t, err)
	return resp.Msg.GetClient()
}

func TestClientLifecycle(t *testing.T) {
	f := newCPFixture(t)
	c := f.createWeb(t, f.key)
	assert.Len(t, c.GetId(), 21)
	assert.Equal(t, clientv1.ClientKind_CLIENT_KIND_WEB, c.GetKind())
	assert.Equal(t, clientv1.ConsentPolicy_CONSENT_POLICY_IMPLICIT, c.GetConsentPolicy())
	assert.Equal(t, "client_secret_basic", c.GetTokenEndpointAuthMethod())
	assert.Empty(t, c.GetSecrets())
	assert.True(t, c.GetIssuerHandleLocked())

	cli, err := f.cp.CreateClient(f.key, connect.NewRequest(&clientv1.CreateClientRequest{
		ProjectId: f.projectPub, Name: "CLI", Kind: clientv1.ClientKind_CLIENT_KIND_CLI,
		RedirectUris: []string{"http://127.0.0.1/callback"}, RefreshTokens: true,
	}))
	require.NoError(t, err)
	assert.Equal(t, clientv1.ClientKind_CLIENT_KIND_CLI, cli.Msg.GetClient().GetKind())
	assert.Equal(t, clientv1.ConsentPolicy_CONSENT_POLICY_REQUIRE, cli.Msg.GetClient().GetConsentPolicy())
	assert.Equal(t, []string{"authorization_code", "refresh_token"}, cli.Msg.GetClient().GetGrantTypes())

	upd, err := f.cp.UpdateClient(f.key, connect.NewRequest(&clientv1.UpdateClientRequest{
		ProjectId: f.projectPub, ClientId: c.GetId(), Name: "Renamed", RedirectUris: []string{"https://app.example/cb2"},
		PostLogoutRedirectUris: []string{"https://app.example/"}, ConsentPolicy: clientv1.ConsentPolicy_CONSENT_POLICY_REQUIRE,
	}))
	require.NoError(t, err)
	assert.Equal(t, "Renamed", upd.Msg.GetClient().GetName())
	assert.Equal(t, []string{"https://app.example/"}, upd.Msg.GetClient().GetPostLogoutRedirectUris())

	list, err := f.cp.ListClients(f.key, connect.NewRequest(&clientv1.ListClientsRequest{ProjectId: f.projectPub}))
	require.NoError(t, err)
	assert.Len(t, list.Msg.GetClients(), 2)

	got, err := f.cp.GetClient(f.key, connect.NewRequest(&clientv1.GetClientRequest{ProjectId: f.projectPub, ClientId: c.GetId()}))
	require.NoError(t, err)
	assert.Equal(t, "Renamed", got.Msg.GetClient().GetName())

	_, err = f.cp.DeleteClient(f.key, connect.NewRequest(&clientv1.DeleteClientRequest{ProjectId: f.projectPub, ClientId: c.GetId()}))
	require.NoError(t, err)
	_, err = f.cp.GetClient(f.key, connect.NewRequest(&clientv1.GetClientRequest{ProjectId: f.projectPub, ClientId: c.GetId()}))
	assert.Equal(t, apperror.CodeClientNotFound, codeOf(err))
}

func TestSecretRPCs_ForAPersonOnly(t *testing.T) {
	f := newCPFixture(t)
	c := f.createWeb(t, f.person)
	_, err := f.cp.GenerateClientSecret(f.key, connect.NewRequest(&clientv1.GenerateClientSecretRequest{ProjectId: f.projectPub, ClientId: c.GetId()}))
	assert.Equal(t, apperror.CodeClientSecretPersonOnly, codeOf(err))
	_, err = f.cp.RotateClientSecret(f.key, connect.NewRequest(&clientv1.RotateClientSecretRequest{ProjectId: f.projectPub, ClientId: c.GetId()}))
	assert.Equal(t, apperror.CodeClientSecretPersonOnly, codeOf(err))
	_, err = f.cp.RevokeClientSecret(f.key, connect.NewRequest(&clientv1.RevokeClientSecretRequest{ProjectId: f.projectPub, ClientId: c.GetId(), SecretId: c.GetId()}))
	assert.Equal(t, apperror.CodeClientSecretPersonOnly, codeOf(err))
	_, err = f.cp.GenerateClientSecret(session.PrincipalInto(context.Background(), session.System()),
		connect.NewRequest(&clientv1.GenerateClientSecretRequest{ProjectId: f.projectPub, ClientId: c.GetId()}))
	assert.Equal(t, apperror.CodeClientSecretPersonOnly, codeOf(err), "the stdio operator is not a person")
	assert.Zero(t, f.h.store.SecretCount())

	gen, err := f.cp.GenerateClientSecret(f.person, connect.NewRequest(&clientv1.GenerateClientSecretRequest{ProjectId: f.projectPub, ClientId: c.GetId()}))
	require.NoError(t, err)
	assert.Contains(t, gen.Msg.GetSecret(), client.SecretPrefix)
	require.Len(t, gen.Msg.GetClient().GetSecrets(), 1)
	assert.Nil(t, gen.Msg.GetClient().GetSecrets()[0].GetExpiresAt())

	for grace, want := range map[clientv1.SecretGrace]int{
		clientv1.SecretGrace_SECRET_GRACE_HOUR:        2,
		clientv1.SecretGrace_SECRET_GRACE_UNSPECIFIED: 2,
		clientv1.SecretGrace_SECRET_GRACE_WEEK:        2,
		clientv1.SecretGrace_SECRET_GRACE_NOW:         1,
	} {
		rot, err := f.cp.RotateClientSecret(f.person, connect.NewRequest(&clientv1.RotateClientSecretRequest{ProjectId: f.projectPub, ClientId: c.GetId(), Grace: grace}))
		require.NoError(t, err, grace)
		assert.Len(t, rot.Msg.GetClient().GetSecrets(), want, grace)
		assert.NotEqual(t, gen.Msg.GetSecret(), rot.Msg.GetSecret())
	}
	rot, err := f.cp.RotateClientSecret(f.person, connect.NewRequest(&clientv1.RotateClientSecretRequest{ProjectId: f.projectPub, ClientId: c.GetId(), Grace: clientv1.SecretGrace_SECRET_GRACE_DAY}))
	require.NoError(t, err)
	old := rot.Msg.GetClient().GetSecrets()[1]
	assert.NotNil(t, old.GetExpiresAt())
	rev, err := f.cp.RevokeClientSecret(f.person, connect.NewRequest(&clientv1.RevokeClientSecretRequest{ProjectId: f.projectPub, ClientId: c.GetId(), SecretId: old.GetId()}))
	require.NoError(t, err)
	assert.Len(t, rev.Msg.GetClient().GetSecrets(), 1)
}

// SECURITY: an MCP result is the JSON of the response; no read or create response carries a secret.
func TestClientResponses_CarryNoSecret(t *testing.T) {
	f := newCPFixture(t)
	c := f.createWeb(t, f.person)
	gen, err := f.cp.GenerateClientSecret(f.person, connect.NewRequest(&clientv1.GenerateClientSecretRequest{ProjectId: f.projectPub, ClientId: c.GetId()}))
	require.NoError(t, err)
	secret := gen.Msg.GetSecret()

	created, err := f.cp.CreateClient(f.key, connect.NewRequest(&clientv1.CreateClientRequest{
		ProjectId: f.projectPub, Name: "Web 2", Kind: clientv1.ClientKind_CLIENT_KIND_WEB, RedirectUris: []string{"https://b.example/cb"},
	}))
	require.NoError(t, err)
	got, err := f.cp.GetClient(f.key, connect.NewRequest(&clientv1.GetClientRequest{ProjectId: f.projectPub, ClientId: c.GetId()}))
	require.NoError(t, err)
	list, err := f.cp.ListClients(f.key, connect.NewRequest(&clientv1.ListClientsRequest{ProjectId: f.projectPub}))
	require.NoError(t, err)
	for name, msg := range map[string]proto.Message{"create": created.Msg, "get": got.Msg, "list": list.Msg} {
		body, err := protojson.Marshal(msg)
		require.NoError(t, err)
		assert.NotContains(t, string(body), secret, name)
		assert.NotContains(t, string(body), client.SecretPrefix, name)
		assert.NotContains(t, string(body), `"secret"`, name)
	}
}

// SECURITY: an unknown project, a UUID, and a project of another org all answer the same not-found, and nothing is written.
func TestClients_OutOfReachIsNotFound(t *testing.T) {
	f := newCPFixture(t)
	limited := session.PrincipalInto(context.Background(), session.Principal{
		Source: session.SourceAPIKey, ActiveOrgID: f.h.org, ProjectIDs: []uuid.UUID{uuid.New()}, Scopes: []string{"project:write"},
	})
	for name, ctx := range map[string]context.Context{"person": f.person, "org key": f.key, "key for another project": limited} {
		ids := []string{f.foreignPub, "abcdefghjkmnpqrstuvwx", f.h.project.String()}
		if name == "key for another project" {
			ids = append(ids, f.projectPub)
		}
		for _, id := range ids {
			_, err := f.cp.ListClients(ctx, connect.NewRequest(&clientv1.ListClientsRequest{ProjectId: id}))
			assert.Equal(t, apperror.CodeProjectNotFound, codeOf(err), "%s list %s", name, id)
			_, err = f.cp.GetClient(ctx, connect.NewRequest(&clientv1.GetClientRequest{ProjectId: id, ClientId: "abcdefghjkmnpqrstuvwx"}))
			assert.Equal(t, apperror.CodeProjectNotFound, codeOf(err), "%s get %s", name, id)
			_, err = f.cp.CreateClient(ctx, connect.NewRequest(&clientv1.CreateClientRequest{ProjectId: id, Name: "x", Kind: clientv1.ClientKind_CLIENT_KIND_WEB, RedirectUris: []string{"https://a.example/"}}))
			assert.Equal(t, apperror.CodeProjectNotFound, codeOf(err), "%s create %s", name, id)
			_, err = f.cp.UpdateClient(ctx, connect.NewRequest(&clientv1.UpdateClientRequest{ProjectId: id, ClientId: "abcdefghjkmnpqrstuvwx"}))
			assert.Equal(t, apperror.CodeProjectNotFound, codeOf(err), "%s update %s", name, id)
			_, err = f.cp.DeleteClient(ctx, connect.NewRequest(&clientv1.DeleteClientRequest{ProjectId: id, ClientId: "abcdefghjkmnpqrstuvwx"}))
			assert.Equal(t, apperror.CodeProjectNotFound, codeOf(err), "%s delete %s", name, id)
			_, err = f.cp.GenerateClientSecret(ctx, connect.NewRequest(&clientv1.GenerateClientSecretRequest{ProjectId: id, ClientId: "abcdefghjkmnpqrstuvwx"}))
			assert.Equal(t, apperror.CodeProjectNotFound, codeOf(err), "%s generate %s", name, id)
		}
	}
	assert.Empty(t, f.h.issuer.locked, "nothing was created")

	c := f.createWeb(t, f.person)
	other := newCPFixture(t)
	_, err := other.cp.GetClient(other.person, connect.NewRequest(&clientv1.GetClientRequest{ProjectId: other.projectPub, ClientId: c.GetId()}))
	assert.Equal(t, apperror.CodeClientNotFound, codeOf(err), "another project's client is not found")
}

func TestRefsFailure(t *testing.T) {
	h := newHarness(t)
	refs := fakes.NewPublicIDs()
	refs.Err = assert.AnError
	cp := client.NewControlPlane(h.svc, refs)
	_, err := cp.ListClients(context.Background(), connect.NewRequest(&clientv1.ListClientsRequest{ProjectId: "abcdefghjkmnpqrstuvwx"}))
	assert.ErrorIs(t, err, assert.AnError)
}

// The console's kind badge: GetClient names a CIMD shadow with its metadata URL and fetch time, and a registered client as registered.
func TestGetClient_CIMDFields(t *testing.T) {
	f := newCPFixture(t)
	fetched := time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)
	shadow := &client.Client{
		ID: uuid.New(), PublicID: "shadow0000000000000aa", OrgID: f.h.org, ProjectID: f.h.project, Name: "Claude Code",
		ClientType: "public", ApplicationType: client.AppNative, TokenEndpointAuthMethod: client.AuthNone,
		GrantTypes: []string{"authorization_code", "refresh_token"}, ConsentPolicy: client.ConsentRequire,
		MetadataURL: "https://claude.ai/oauth/claude-code-client-metadata", MetadataFetchedAt: &fetched,
	}
	require.NoError(t, f.h.store.Insert(context.Background(), shadow))

	got, err := f.cp.GetClient(f.key, connect.NewRequest(&clientv1.GetClientRequest{ProjectId: f.projectPub, ClientId: shadow.PublicID}))
	require.NoError(t, err)
	c := got.Msg.GetClient()
	assert.Equal(t, clientv1.ClientRegistration_CLIENT_REGISTRATION_CIMD, c.GetRegistration())
	assert.Equal(t, shadow.MetadataURL, c.GetMetadataUrl())
	assert.Equal(t, fetched, c.GetMetadataFetchedAt().AsTime())

	web := f.createWeb(t, f.key)
	assert.Equal(t, clientv1.ClientRegistration_CLIENT_REGISTRATION_REGISTERED, web.GetRegistration())
	assert.Empty(t, web.GetMetadataUrl())
	assert.Nil(t, web.GetMetadataFetchedAt())
}

// Every client result links to the client's console page, where a signed-in person manages its secrets; lists look the route up once.
func TestClientResults_CarryTheConsoleURL(t *testing.T) {
	f := newCPFixture(t)
	created := f.createWeb(t, f.person)
	want := "https://othra.test/console/o/" + f.refs.SlugOf(f.h.org) + "/projects/" + f.refs.SlugOf(f.h.project) + "/clients/" + created.GetId()
	assert.Equal(t, want, created.GetConsoleUrl())

	got, err := f.cp.GetClient(f.key, connect.NewRequest(&clientv1.GetClientRequest{ProjectId: f.projectPub, ClientId: created.GetId()}))
	require.NoError(t, err)
	assert.Equal(t, created.GetConsoleUrl(), got.Msg.GetClient().GetConsoleUrl())
	list, err := f.cp.ListClients(f.key, connect.NewRequest(&clientv1.ListClientsRequest{ProjectId: f.projectPub}))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetClients(), 1)
	assert.Equal(t, created.GetConsoleUrl(), list.Msg.GetClients()[0].GetConsoleUrl())
}

// A failed project lookup stops a write before it starts, and its code reaches the caller.
func TestClientResults_ConsoleURLLookupFails(t *testing.T) {
	f := newCPFixture(t)
	created := f.createWeb(t, f.person)
	before, err := f.h.svc.Get(context.Background(), f.h.org, f.h.project, created.GetId())
	require.NoError(t, err)
	f.refs.Err = apperror.New(apperror.CodeUnexpected, "lookup failed")

	_, err = f.cp.CreateClient(f.person, connect.NewRequest(&clientv1.CreateClientRequest{
		ProjectId: f.projectPub, Name: "Second", Kind: clientv1.ClientKind_CLIENT_KIND_WEB,
		RedirectUris: []string{"https://app.example/callback"},
	}))
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	_, err = f.cp.GenerateClientSecret(f.person, connect.NewRequest(&clientv1.GenerateClientSecretRequest{ProjectId: f.projectPub, ClientId: created.GetId()}))
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	_, err = f.cp.UpdateClient(f.person, connect.NewRequest(&clientv1.UpdateClientRequest{
		ProjectId: f.projectPub, ClientId: created.GetId(), Name: "Renamed", RedirectUris: []string{"https://app.example/callback"},
	}))
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))

	list, err := f.h.svc.List(context.Background(), f.h.org, f.h.project)
	require.NoError(t, err)
	require.Len(t, list, 1, "no client was created")
	after := list[0]
	assert.Equal(t, before.Name, after.Name, "the client was not updated")
	assert.Equal(t, before.UpdatedAt, after.UpdatedAt)
	assert.Equal(t, len(before.Secrets), len(after.Secrets), "no secret was generated")
}
