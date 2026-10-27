package permission

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"

	accountv1 "altalune.id/othra/gen/go/account/v1"
	apikeyv1 "altalune.id/othra/gen/go/apikey/v1"
	authv1 "altalune.id/othra/gen/go/auth/v1"
	clientv1 "altalune.id/othra/gen/go/client/v1"
	enduserv1 "altalune.id/othra/gen/go/enduser/v1"
	invitev1 "altalune.id/othra/gen/go/invite/v1"
	keysv1 "altalune.id/othra/gen/go/keys/v1"
	memberv1 "altalune.id/othra/gen/go/member/v1"
	orgv1 "altalune.id/othra/gen/go/org/v1"
	projectv1 "altalune.id/othra/gen/go/project/v1"
	resourceserverv1 "altalune.id/othra/gen/go/resourceserver/v1"
	upstreamv1 "altalune.id/othra/gen/go/upstream/v1"
	webhookv1 "altalune.id/othra/gen/go/webhook/v1"
	"altalune.id/othra/internal/kernel/authz"
	"altalune.id/othra/scope"
)

func allProcedures() []string {
	return procedures(
		apikeyv1.File_apikey_v1_apikey_proto,
		authv1.File_auth_v1_auth_proto,
		clientv1.File_client_v1_client_proto,
		enduserv1.File_enduser_v1_enduser_proto,
		invitev1.File_invite_v1_invite_proto,
		keysv1.File_keys_v1_keys_proto,
		memberv1.File_member_v1_member_proto,
		orgv1.File_org_v1_org_proto,
		projectv1.File_project_v1_project_proto,
		upstreamv1.File_upstream_v1_upstream_proto,
		resourceserverv1.File_resourceserver_v1_resourceserver_proto,
		webhookv1.File_webhook_v1_webhook_proto,
	)
}

func procedures(files ...protoreflect.FileDescriptor) []string {
	var out []string
	for _, f := range files {
		svcs := f.Services()
		for i := range svcs.Len() {
			s := svcs.Get(i)
			for j := range s.Methods().Len() {
				out = append(out, "/"+string(s.FullName())+"/"+string(s.Methods().Get(j).Name()))
			}
		}
	}
	return out
}

func TestEveryRPCHasAPermission(t *testing.T) {
	procs := allProcedures()
	require.Len(t, procs, 85,
		"Org 4, Project 13, APIKey 3, Webhook 11, Auth 6, Member 4, Invite 7, SigningKey 2, Client 8, ResourceServer 16, Upstream 6, EndUser 5")
	table := Table()
	for _, proc := range procs {
		perms := table[proc]
		assert.NotEmpty(t, perms, "%s has no permission", proc)
		for _, perm := range perms {
			if perm == authz.AnyAuthenticated || perm == authz.PersonOnly {
				assert.Len(t, perms, 1, "%s: %s is only ever the single entry", proc, perm)
				continue
			}
			assert.True(t, scope.Valid(perm), "%s maps to unknown permission %q", proc, perm)
		}
	}
	assert.Len(t, table, len(procs), "the table names no procedure that does not exist")
}

// SECURITY: every procedure is in exactly one plane; the end user's own are all in EndUserTable, none in Table.
func TestEndUserTable_IsItsOwnPlane(t *testing.T) {
	account := procedures(accountv1.File_account_v1_account_proto)
	require.Len(t, account, 13)
	table, own := Table(), EndUserTable()
	for _, proc := range account {
		assert.Contains(t, []string{Self, SelfRecent}, own[proc], "%s is missing from EndUserTable", proc)
		assert.NotContains(t, table, proc, "%s must never be routed by /api or MCP", proc)
	}
	assert.Len(t, own, len(account), "EndUserTable names no procedure that does not exist")
	for _, proc := range allProcedures() {
		assert.NotContains(t, own, proc, "%s is an admin procedure", proc)
	}
	assert.Equal(t, SelfRecent, own["/account.v1.AccountService/CreatePersonalToken"], "creating a token is the only step-up")
}

func TestPermissionTable_Spec15(t *testing.T) {
	table := Table()
	for proc, want := range map[string]string{
		"/org.v1.OrgService/CreateOrg":                                        "org:write",
		"/org.v1.OrgService/ListOrgs":                                         "org:read",
		"/org.v1.OrgService/UpdateOrg":                                        "org:write",
		"/project.v1.ProjectService/UpdateProject":                            "project:write",
		"/project.v1.ProjectService/UpdateProjectSignIn":                      "project:write",
		"/upstream.v1.UpstreamService/ListUpstreams":                          "project:read",
		"/upstream.v1.UpstreamService/GetUpstream":                            "project:read",
		"/upstream.v1.UpstreamService/CreateUpstream":                         "project:write",
		"/upstream.v1.UpstreamService/UpdateUpstream":                         "project:write",
		"/upstream.v1.UpstreamService/SetUpstreamEnabled":                     "project:write",
		"/upstream.v1.UpstreamService/DeleteUpstream":                         "project:write",
		"/project.v1.ProjectService/ListProjects":                             "project:read",
		"/keys.v1.SigningKeyService/ListSigningKeys":                          "project:read",
		"/keys.v1.SigningKeyService/RotateSigningKey":                         "project:write",
		"/client.v1.ClientService/ListClients":                                "project:read",
		"/client.v1.ClientService/GetClient":                                  "project:read",
		"/client.v1.ClientService/CreateClient":                               "project:write",
		"/client.v1.ClientService/UpdateClient":                               "project:write",
		"/client.v1.ClientService/DeleteClient":                               "project:write",
		"/client.v1.ClientService/GenerateClientSecret":                       "project:write",
		"/client.v1.ClientService/RotateClientSecret":                         "project:write",
		"/client.v1.ClientService/RevokeClientSecret":                         "project:write",
		"/resourceserver.v1.ResourceServerService/ListResourceServers":        "project:read",
		"/resourceserver.v1.ResourceServerService/GetResourceServer":          "project:read",
		"/resourceserver.v1.ResourceServerService/CreateResourceServer":       "project:write",
		"/resourceserver.v1.ResourceServerService/UpdateResourceServer":       "project:write",
		"/resourceserver.v1.ResourceServerService/DeleteResourceServer":       "project:write",
		"/resourceserver.v1.ResourceServerService/VerifyResourceServer":       "project:write",
		"/resourceserver.v1.ResourceServerService/SetResourceServerTrust":     "project:write",
		"/resourceserver.v1.ResourceServerService/SetResourceServerAdmission": "project:write",
		"/resourceserver.v1.ResourceServerService/ListLinks":                  "project:read",
		"/resourceserver.v1.ResourceServerService/LinkClient":                 "project:write",
		"/resourceserver.v1.ResourceServerService/UpdateLink":                 "project:write",
		"/resourceserver.v1.ResourceServerService/UnlinkClient":               "project:write",
		"/webhook.v1.WebhookService/ListWebhookDeliveryAttempts":              "webhook:read",
		"/webhook.v1.WebhookService/RotateWebhookSecret":                      "webhook:write",
		"/webhook.v1.WebhookService/RetryFailedWebhookDeliveries":             "webhook:write",
		"/apikey.v1.APIKeyService/ListAPIKeys":                                "apikey:read",
		"/apikey.v1.APIKeyService/CreateAPIKey":                               authz.AnyAuthenticated,
		"/apikey.v1.APIKeyService/RevokeAPIKey":                               authz.AnyAuthenticated,
		"/auth.v1.AuthService/Whoami":                                         authz.AnyAuthenticated,
		"/auth.v1.AuthService/SetActiveOrg":                                   authz.PersonOnly,
		"/auth.v1.AuthService/CreateOwnOrg":                                   authz.PersonOnly,
		"/auth.v1.AuthService/UpdateProfile":                                  authz.PersonOnly,
		"/auth.v1.AuthService/ChangePassword":                                 authz.PersonOnly,
		"/auth.v1.AuthService/SignOutEverywhere":                              authz.PersonOnly,
		"/member.v1.MemberService/ListMembers":                                "member:read",
		"/member.v1.MemberService/SetMemberRole":                              "member:write",
		"/member.v1.MemberService/RemoveMember":                               "member:write",
		"/member.v1.MemberService/LeaveOrg":                                   authz.PersonOnly,
		"/invite.v1.InviteService/ListInvites":                                "member:read",
		"/invite.v1.InviteService/CreateInvite":                               "member:write",
		"/invite.v1.InviteService/ResendInvite":                               "member:write",
		"/invite.v1.InviteService/RevokeInvite":                               "member:write",
		"/invite.v1.InviteService/DraftInvite":                                "member:write",
		"/invite.v1.InviteService/ConfirmDraftInvite":                         "member:write",
		"/invite.v1.InviteService/DiscardDraftInvite":                         "member:write",
	} {
		assert.Equal(t, []string{want}, table[proc], proc)
	}
}

func TestFor(t *testing.T) {
	assert.Equal(t, []string{scope.ProjectRead}, For("/project.v1.ProjectService/GetProject"))
	assert.Nil(t, For("/nope.v1.Nope/Nothing"), "an unknown procedure has no permission, so it is denied")
}
