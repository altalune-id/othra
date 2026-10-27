package resourceserver

import (
	"context"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	resourceserverv1 "altalune.id/othra/gen/go/resourceserver/v1"
	resourceserverv1connect "altalune.id/othra/gen/go/resourceserver/v1/resourceserverv1connect"
	"altalune.id/othra/internal/kernel/authz"
	"altalune.id/othra/internal/kernel/session"
	"altalune.id/othra/internal/org/member"
	"altalune.id/othra/internal/permission"
	"altalune.id/othra/internal/testutil/fakes"
)

type cpFixture struct {
	h          *harness
	cp         *ResourceServerService
	projectPub string
	foreignPub string
	siblingPub string
	owner      context.Context
	key        context.Context
}

func newCPFixture(t *testing.T, trust bool) *cpFixture {
	t.Helper()
	h := newHarness(t, trust)
	refs := fakes.NewPublicIDs()
	refs.Org(h.org)
	f := &cpFixture{h: h, cp: NewControlPlane(h.svc, refs), projectPub: refs.Project(h.project, h.org)}
	f.siblingPub = refs.Project(uuid.New(), h.org)
	foreignOrg := uuid.New()
	refs.Org(foreignOrg)
	f.foreignPub = refs.Project(uuid.New(), foreignOrg)
	f.owner = session.PrincipalInto(context.Background(), session.Principal{
		Source: session.SourceLocal, UserID: h.person, ActiveOrgID: h.org, Scopes: []string{"*"},
	})
	f.key = session.PrincipalInto(context.Background(), session.Principal{
		Source: session.SourceAPIKey, UserID: uuid.New(), ActiveOrgID: h.org, AllProjects: true, Scopes: []string{"project:*"},
	})
	return f
}

func (f *cpFixture) create(t *testing.T, ctx context.Context, identifier string) *resourceserverv1.ResourceServer {
	t.Helper()
	resp, err := f.cp.CreateResourceServer(ctx, connect.NewRequest(&resourceserverv1.CreateResourceServerRequest{
		ProjectId: f.projectPub, Name: "API", Identifier: identifier, Scopes: []string{"read", "write"},
		Trusted: IsLoopbackHTTP(identifier),
	}))
	require.NoError(t, err)
	return resp.Msg.GetResourceServer()
}

func TestControlPlane_Lifecycle(t *testing.T) {
	f := newCPFixture(t, false)
	rs := f.create(t, f.owner, "https://api.example.com/mcp")
	assert.Len(t, rs.GetId(), publicIDLen)
	assert.Equal(t, resourceserverv1.ResourceServerState_RESOURCE_SERVER_STATE_PENDING, rs.GetState())
	assert.Equal(t, "https://api.example.com/.well-known/oauth-protected-resource/mcp", rs.GetMetadataUrl())
	assert.False(t, rs.GetTrustAvailable())
	assert.Nil(t, rs.GetVerifiedAt())

	list, err := f.cp.ListResourceServers(f.owner, connect.NewRequest(&resourceserverv1.ListResourceServersRequest{ProjectId: f.projectPub}))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetResourceServers(), 1)
	assert.False(t, list.Msg.GetTrustAvailable())

	f.h.checker.set(&Failure{Reason: FailureResourceMismatch, Resource: "https://elsewhere", CheckedAt: f.h.now})
	verified, err := f.cp.VerifyResourceServer(f.owner, connect.NewRequest(&resourceserverv1.VerifyResourceServerRequest{
		ProjectId: f.projectPub, ResourceServerId: rs.GetId(),
	}))
	require.NoError(t, err)
	assert.False(t, verified.Msg.GetPassed())
	assert.Equal(t, "resource_mismatch", verified.Msg.GetError().GetReason())
	assert.Equal(t, "https://elsewhere", verified.Msg.GetError().GetResource())
	assert.Equal(t, resourceserverv1.ResourceServerState_RESOURCE_SERVER_STATE_PENDING, verified.Msg.GetResourceServer().GetState())

	f.h.checker.set(nil)
	f.h.now = f.h.now.Add(VerifyCooldown)
	verified, err = f.cp.VerifyResourceServer(f.owner, connect.NewRequest(&resourceserverv1.VerifyResourceServerRequest{
		ProjectId: f.projectPub, ResourceServerId: rs.GetId(),
	}))
	require.NoError(t, err)
	assert.True(t, verified.Msg.GetPassed())
	assert.Nil(t, verified.Msg.GetError())
	assert.Equal(t, resourceserverv1.ResourceServerState_RESOURCE_SERVER_STATE_VERIFIED, verified.Msg.GetResourceServer().GetState())
	assert.NotNil(t, verified.Msg.GetResourceServer().GetVerifiedAt())

	web := f.h.clients.add("Web")
	link, err := f.cp.LinkClient(f.owner, connect.NewRequest(&resourceserverv1.LinkClientRequest{
		ProjectId: f.projectPub, ClientId: web.PublicID, ResourceServerId: rs.GetId(), Scopes: []string{"read"},
	}))
	require.NoError(t, err)
	assert.Equal(t, web.PublicID, link.Msg.GetLink().GetClientId())
	assert.Equal(t, "Web", link.Msg.GetLink().GetClientName())
	assert.Equal(t, rs.GetId(), link.Msg.GetLink().GetResourceServerId())
	assert.Equal(t, "https://api.example.com/mcp", link.Msg.GetLink().GetResourceServerIdentifier())
	assert.Equal(t, resourceserverv1.ResourceServerState_RESOURCE_SERVER_STATE_VERIFIED, link.Msg.GetLink().GetResourceServerState())

	updated, err := f.cp.UpdateLink(f.owner, connect.NewRequest(&resourceserverv1.UpdateLinkRequest{
		ProjectId: f.projectPub, ClientId: web.PublicID, ResourceServerId: rs.GetId(), Scopes: []string{"read", "write"},
	}))
	require.NoError(t, err)
	assert.Equal(t, []string{"read", "write"}, updated.Msg.GetLink().GetScopes())

	links, err := f.cp.ListLinks(f.owner, connect.NewRequest(&resourceserverv1.ListLinksRequest{
		ProjectId: f.projectPub, Target: &resourceserverv1.ListLinksRequest_ClientId{ClientId: web.PublicID},
	}))
	require.NoError(t, err)
	require.Len(t, links.Msg.GetLinks(), 1)

	_, err = f.cp.UpdateResourceServer(f.owner, connect.NewRequest(&resourceserverv1.UpdateResourceServerRequest{
		ProjectId: f.projectPub, ResourceServerId: rs.GetId(), Name: "API", Scopes: []string{"read"},
	}))
	assert.True(t, IsScopePinnedError(err), "%v", err)
	_, err = f.cp.DeleteResourceServer(f.owner, connect.NewRequest(&resourceserverv1.DeleteResourceServerRequest{
		ProjectId: f.projectPub, ResourceServerId: rs.GetId(),
	}))
	assert.True(t, IsLinkedError(err), "%v", err)

	_, err = f.cp.UnlinkClient(f.owner, connect.NewRequest(&resourceserverv1.UnlinkClientRequest{
		ProjectId: f.projectPub, ClientId: web.PublicID, ResourceServerId: rs.GetId(),
	}))
	require.NoError(t, err)
	got, err := f.cp.UpdateResourceServer(f.owner, connect.NewRequest(&resourceserverv1.UpdateResourceServerRequest{
		ProjectId: f.projectPub, ResourceServerId: rs.GetId(), Name: "Renamed", Scopes: []string{"read"},
	}))
	require.NoError(t, err)
	assert.Equal(t, "Renamed", got.Msg.GetResourceServer().GetName())
	_, err = f.cp.DeleteResourceServer(f.owner, connect.NewRequest(&resourceserverv1.DeleteResourceServerRequest{
		ProjectId: f.projectPub, ResourceServerId: rs.GetId(),
	}))
	require.NoError(t, err)
	_, err = f.cp.GetResourceServer(f.owner, connect.NewRequest(&resourceserverv1.GetResourceServerRequest{
		ProjectId: f.projectPub, ResourceServerId: rs.GetId(),
	}))
	assert.True(t, IsNotFoundError(err))
}

// trust is person-only and self-hosted only; a key is refused before anything changes.
func TestControlPlane_Trust(t *testing.T) {
	f := newCPFixture(t, true)
	rs := f.create(t, f.owner, "https://api.example.com")
	assert.True(t, rs.GetTrustAvailable())
	req := func(on bool) *connect.Request[resourceserverv1.SetResourceServerTrustRequest] {
		return connect.NewRequest(&resourceserverv1.SetResourceServerTrustRequest{ProjectId: f.projectPub, ResourceServerId: rs.GetId(), Trusted: on})
	}
	_, err := f.cp.SetResourceServerTrust(f.key, req(true))
	assert.True(t, authz.IsForbiddenError(err), "%v", err)
	_, err = f.cp.SetResourceServerTrust(context.Background(), req(true))
	assert.True(t, authz.IsForbiddenError(err), "%v", err)

	resp, err := f.cp.SetResourceServerTrust(f.owner, req(true))
	require.NoError(t, err)
	assert.True(t, resp.Msg.GetResourceServer().GetTrusted())

	loop := f.create(t, f.owner, "http://127.0.0.1:8080/mcp")
	assert.True(t, loop.GetTrusted(), "a person's http loopback server is created trusted")
	assert.Empty(t, loop.GetMetadataUrl(), "it has no metadata to fetch")
	_, err = f.cp.CreateResourceServer(f.key, connect.NewRequest(&resourceserverv1.CreateResourceServerRequest{
		ProjectId: f.projectPub, Name: "API", Identifier: "http://localhost:9000", Scopes: []string{"read"}, Trusted: true,
	}))
	assert.True(t, authz.IsForbiddenError(err), "a key can't create a trusted server: %v", err)
	list, err := f.cp.ListResourceServers(f.owner, connect.NewRequest(&resourceserverv1.ListResourceServersRequest{ProjectId: f.projectPub}))
	require.NoError(t, err)
	assert.True(t, list.Msg.GetTrustAvailable())

	cloud := newCPFixture(t, false)
	crs := cloud.create(t, cloud.owner, "https://api.example.com")
	_, err = cloud.cp.SetResourceServerTrust(cloud.owner, connect.NewRequest(&resourceserverv1.SetResourceServerTrustRequest{
		ProjectId: cloud.projectPub, ResourceServerId: crs.GetId(), Trusted: true,
	}))
	assert.True(t, IsTrustSelfHostedOnlyError(err), "%v", err)
}

// SECURITY: another org's project, a sibling project out of a key's reach, an unknown id and a UUID all answer not-found.
func TestControlPlane_OutOfReachIsNotFound(t *testing.T) {
	f := newCPFixture(t, true)
	rs := f.create(t, f.owner, "https://api.example.com")
	web := f.h.clients.add("Web")
	limited := session.PrincipalInto(context.Background(), session.Principal{
		Source: session.SourceAPIKey, ActiveOrgID: f.h.org, ProjectIDs: []uuid.UUID{uuid.New()}, Scopes: []string{"project:*"},
	})
	calls := map[string]func(ctx context.Context, project string) error{
		"list": func(ctx context.Context, p string) error {
			_, err := f.cp.ListResourceServers(ctx, connect.NewRequest(&resourceserverv1.ListResourceServersRequest{ProjectId: p}))
			return err
		},
		"get": func(ctx context.Context, p string) error {
			_, err := f.cp.GetResourceServer(ctx, connect.NewRequest(&resourceserverv1.GetResourceServerRequest{ProjectId: p, ResourceServerId: rs.GetId()}))
			return err
		},
		"create": func(ctx context.Context, p string) error {
			_, err := f.cp.CreateResourceServer(ctx, connect.NewRequest(&resourceserverv1.CreateResourceServerRequest{
				ProjectId: p, Name: "X", Identifier: "https://x.example", Scopes: []string{"r"},
			}))
			return err
		},
		"update": func(ctx context.Context, p string) error {
			_, err := f.cp.UpdateResourceServer(ctx, connect.NewRequest(&resourceserverv1.UpdateResourceServerRequest{
				ProjectId: p, ResourceServerId: rs.GetId(), Name: "X", Scopes: []string{"r"},
			}))
			return err
		},
		"delete": func(ctx context.Context, p string) error {
			_, err := f.cp.DeleteResourceServer(ctx, connect.NewRequest(&resourceserverv1.DeleteResourceServerRequest{ProjectId: p, ResourceServerId: rs.GetId()}))
			return err
		},
		"verify": func(ctx context.Context, p string) error {
			_, err := f.cp.VerifyResourceServer(ctx, connect.NewRequest(&resourceserverv1.VerifyResourceServerRequest{ProjectId: p, ResourceServerId: rs.GetId()}))
			return err
		},
		"trust": func(ctx context.Context, p string) error {
			_, err := f.cp.SetResourceServerTrust(ctx, connect.NewRequest(&resourceserverv1.SetResourceServerTrustRequest{
				ProjectId: p, ResourceServerId: rs.GetId(), Trusted: true,
			}))
			return err
		},
		"list links": func(ctx context.Context, p string) error {
			_, err := f.cp.ListLinks(ctx, connect.NewRequest(&resourceserverv1.ListLinksRequest{
				ProjectId: p, Target: &resourceserverv1.ListLinksRequest_ResourceServerId{ResourceServerId: rs.GetId()},
			}))
			return err
		},
		"link": func(ctx context.Context, p string) error {
			_, err := f.cp.LinkClient(ctx, connect.NewRequest(&resourceserverv1.LinkClientRequest{
				ProjectId: p, ClientId: web.PublicID, ResourceServerId: rs.GetId(), Scopes: []string{"read"},
			}))
			return err
		},
		"update link": func(ctx context.Context, p string) error {
			_, err := f.cp.UpdateLink(ctx, connect.NewRequest(&resourceserverv1.UpdateLinkRequest{
				ProjectId: p, ClientId: web.PublicID, ResourceServerId: rs.GetId(), Scopes: []string{"read"},
			}))
			return err
		},
		"unlink": func(ctx context.Context, p string) error {
			_, err := f.cp.UnlinkClient(ctx, connect.NewRequest(&resourceserverv1.UnlinkClientRequest{
				ProjectId: p, ClientId: web.PublicID, ResourceServerId: rs.GetId(),
			}))
			return err
		},
	}
	for name, call := range calls {
		for _, project := range []string{f.foreignPub, "abcdefghjkmnpqrstuvwx", f.h.project.String()} {
			assert.True(t, IsProjectNotFoundError(call(f.owner, project)), "%s on %s", name, project)
		}
		if name == "trust" {
			assert.True(t, authz.IsForbiddenError(call(limited, f.projectPub)), "a key never sets trust")
			continue
		}
		assert.True(t, IsProjectNotFoundError(call(limited, f.projectPub)), "%s by a key for another project", name)
		assert.True(t, IsProjectNotFoundError(call(f.key, f.foreignPub)), "%s by a key of another org", name)
	}
	assert.Len(t, f.h.store.servers, 1, "nothing was created")
	assert.Empty(t, f.h.store.links, "nothing was linked")
	assert.False(t, f.h.store.server(uuidOf(t, f, rs.GetId())).Trusted, "nothing was trusted")

	_, err := f.cp.GetResourceServer(f.owner, connect.NewRequest(&resourceserverv1.GetResourceServerRequest{
		ProjectId: f.siblingPub, ResourceServerId: rs.GetId(),
	}))
	assert.True(t, IsNotFoundError(err), "a server is not found through a sibling project: %v", err)
}

func uuidOf(t *testing.T, f *cpFixture, publicID string) uuid.UUID {
	t.Helper()
	r, err := f.h.store.ByPublicID(context.Background(), f.h.project, publicID)
	require.NoError(t, err)
	return r.ID
}

// a member reads resource servers and links but writes nothing; a key never sets trust.
func TestPermissions_MemberReadsOnly(t *testing.T) {
	memberP := session.Principal{Source: session.SourceLocal, UserID: uuid.New(), Scopes: member.BuiltinPermissions(member.RoleMember)}
	adminP := session.Principal{Source: session.SourceLocal, UserID: uuid.New(), Scopes: member.BuiltinPermissions(member.RoleAdmin)}
	keyP := session.Principal{Source: session.SourceAPIKey, Scopes: []string{"project:*"}}
	reads := map[string]bool{
		resourceserverv1connect.ResourceServerServiceListResourceServersProcedure: true,
		resourceserverv1connect.ResourceServerServiceGetResourceServerProcedure:   true,
		resourceserverv1connect.ResourceServerServiceListLinksProcedure:           true,
	}
	allowed := func(p session.Principal, proc string) bool {
		perms := permission.For(proc)
		for _, perm := range perms {
			if !authz.Allowed(p, perm) {
				return false
			}
		}
		return len(perms) > 0
	}
	methods := resourceserverv1.File_resourceserver_v1_resourceserver_proto.Services().Get(0).Methods()
	for i := range methods.Len() {
		proc := "/resourceserver.v1.ResourceServerService/" + string(methods.Get(i).Name())
		assert.Equal(t, reads[proc], allowed(memberP, proc), "member on %s", proc)
		assert.True(t, allowed(adminP, proc), "admin on %s", proc)
		if proc != resourceserverv1connect.ResourceServerServiceSetResourceServerTrustProcedure {
			assert.True(t, allowed(keyP, proc), "a project key on %s", proc)
		}
	}
}

func TestToProto_MapsEveryState(t *testing.T) {
	for s, want := range map[State]resourceserverv1.ResourceServerState{
		StatePending:  resourceserverv1.ResourceServerState_RESOURCE_SERVER_STATE_PENDING,
		StateVerified: resourceserverv1.ResourceServerState_RESOURCE_SERVER_STATE_VERIFIED,
		StateFailed:   resourceserverv1.ResourceServerState_RESOURCE_SERVER_STATE_FAILED,
		"bogus":       resourceserverv1.ResourceServerState_RESOURCE_SERVER_STATE_UNSPECIFIED,
	} {
		assert.Equal(t, want, stateToProto(s))
	}
	now := time.Now()
	assert.NotNil(t, ts(&now))
	assert.Nil(t, ts(nil))
}

func TestControlPlane_RefsFailure(t *testing.T) {
	h := newHarness(t, false)
	refs := fakes.NewPublicIDs()
	refs.Err = errFake
	cp := NewControlPlane(h.svc, refs)
	_, err := cp.ListResourceServers(context.Background(), connect.NewRequest(&resourceserverv1.ListResourceServersRequest{ProjectId: "abcdefghjkmnpqrstuvwx"}))
	assert.ErrorIs(t, err, errFake)
}

// SetResourceServerAdmission saves admission; the server and its links show it; out of reach is not-found.
func TestControlPlane_Admission(t *testing.T) {
	f := newCPFixture(t, true)
	rs := f.create(t, f.owner, "http://127.0.0.1:8080/mcp")
	req := &resourceserverv1.SetResourceServerAdmissionRequest{
		ProjectId: f.projectPub, ResourceServerId: rs.GetId(), Accept: true, Hosts: []string{"claude.ai"}, Scopes: []string{"read"},
	}
	resp, err := f.cp.SetResourceServerAdmission(f.key, connect.NewRequest(req))
	require.NoError(t, err)
	cimd := resp.Msg.GetResourceServer().GetCimd()
	assert.True(t, cimd.GetAccept())
	assert.Equal(t, []string{"claude.ai"}, cimd.GetHosts())
	assert.False(t, cimd.GetAllScopes())
	assert.Equal(t, []string{"read"}, cimd.GetScopes())

	req.AllScopes, req.Scopes = true, nil
	resp, err = f.cp.SetResourceServerAdmission(f.key, connect.NewRequest(req))
	require.NoError(t, err)
	assert.True(t, resp.Msg.GetResourceServer().GetCimd().GetAllScopes())

	managed := f.h.clients.addManaged("Claude Code", "https://claude.ai/oauth/claude-code-client-metadata")
	server, err := f.h.svc.Get(context.Background(), f.h.org, f.h.project, rs.GetId())
	require.NoError(t, err)
	require.NoError(t, f.h.svc.LinkMetadataClient(context.Background(), f.h.org, f.h.project, managed.ID, server.ID, []string{"read"}))
	links, err := f.cp.ListLinks(f.owner, connect.NewRequest(&resourceserverv1.ListLinksRequest{
		ProjectId: f.projectPub, Target: &resourceserverv1.ListLinksRequest_ResourceServerId{ResourceServerId: rs.GetId()},
	}))
	require.NoError(t, err)
	require.Len(t, links.Msg.GetLinks(), 1)
	assert.Equal(t, managed.MetadataURL, links.Msg.GetLinks()[0].GetClientMetadataUrl())
	assert.NotNil(t, links.Msg.GetLinks()[0].GetMetadataFetchedAt())

	req.ProjectId = f.foreignPub
	_, err = f.cp.SetResourceServerAdmission(f.owner, connect.NewRequest(req))
	assert.True(t, IsProjectNotFoundError(err), "%v", err)
	req.ProjectId, req.Hosts = f.projectPub, []string{"203.0.113.9"}
	_, err = f.cp.SetResourceServerAdmission(f.owner, connect.NewRequest(req))
	assert.True(t, IsInvalidClientHostError(err), "%v", err)

	updated, err := f.cp.UpdateResourceServer(f.owner, connect.NewRequest(&resourceserverv1.UpdateResourceServerRequest{
		ProjectId: f.projectPub, ResourceServerId: rs.GetId(), Name: "MCP", Scopes: []string{"read", "write"},
		ScopeDescriptions: map[string]string{"read": "Read your orders"},
	}))
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"read": "Read your orders"}, updated.Msg.GetResourceServer().GetScopeDescriptions())
}

// SECURITY: introspection secrets, like client secrets, and the personal-token switch are set only by a signed-in person.
func TestControlPlane_IntrospectionSecretsArePersonOnly(t *testing.T) {
	f := newCPFixture(t, true)
	rs := f.create(t, f.owner, "https://api.example.com")
	generate := connect.NewRequest(&resourceserverv1.GenerateIntrospectionSecretRequest{ProjectId: f.projectPub, ResourceServerId: rs.GetId()})
	rotate := connect.NewRequest(&resourceserverv1.RotateIntrospectionSecretRequest{ProjectId: f.projectPub, ResourceServerId: rs.GetId()})
	access := connect.NewRequest(&resourceserverv1.SetPersonalTokenAccessRequest{ProjectId: f.projectPub, ResourceServerId: rs.GetId()})
	for _, ctx := range []context.Context{f.key, context.Background()} {
		_, err := f.cp.SetPersonalTokenAccess(ctx, access)
		assert.True(t, authz.IsForbiddenError(err), "%v", err)
		_, err = f.cp.GenerateIntrospectionSecret(ctx, generate)
		assert.True(t, authz.IsForbiddenError(err), "%v", err)
		_, err = f.cp.RotateIntrospectionSecret(ctx, rotate)
		assert.True(t, authz.IsForbiddenError(err), "%v", err)
		_, err = f.cp.RevokeIntrospectionSecret(ctx, connect.NewRequest(&resourceserverv1.RevokeIntrospectionSecretRequest{
			ProjectId: f.projectPub, ResourceServerId: rs.GetId(), SecretId: "s-any",
		}))
		assert.True(t, authz.IsForbiddenError(err), "%v", err)
	}
	assert.Empty(t, f.h.store.server(uuidOf(t, f, rs.GetId())).IntrospectionSecrets, "an API key minted nothing")

	resp, err := f.cp.GenerateIntrospectionSecret(f.owner, generate)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resp.Msg.GetSecret(), IntrospectionSecretPrefix))
	rotated, err := f.cp.RotateIntrospectionSecret(f.owner, rotate)
	require.NoError(t, err)
	assert.NotEqual(t, resp.Msg.GetSecret(), rotated.Msg.GetSecret())
	_, err = f.cp.SetPersonalTokenAccess(f.owner, access)
	require.NoError(t, err, "a person may turn personal tokens off")
	secrets := rotated.Msg.GetResourceServer().GetIntrospectionSecrets()
	require.NotEmpty(t, secrets)
	_, err = f.cp.RevokeIntrospectionSecret(f.owner, connect.NewRequest(&resourceserverv1.RevokeIntrospectionSecretRequest{
		ProjectId: f.projectPub, ResourceServerId: rs.GetId(), SecretId: secrets[0].GetId(),
	}))
	require.NoError(t, err)
}
