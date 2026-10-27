package controlplane

import (
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/altalune-id/othra/apperror"
	apikeyv1 "github.com/altalune-id/othra/gen/go/apikey/v1"
	authv1 "github.com/altalune-id/othra/gen/go/auth/v1"
	clientv1 "github.com/altalune-id/othra/gen/go/client/v1"
	invitev1 "github.com/altalune-id/othra/gen/go/invite/v1"
	keysv1 "github.com/altalune-id/othra/gen/go/keys/v1"
	memberv1 "github.com/altalune-id/othra/gen/go/member/v1"
	orgv1 "github.com/altalune-id/othra/gen/go/org/v1"
	projectv1 "github.com/altalune-id/othra/gen/go/project/v1"
	resourceserverv1 "github.com/altalune-id/othra/gen/go/resourceserver/v1"
	webhookv1 "github.com/altalune-id/othra/gen/go/webhook/v1"
	"github.com/altalune-id/othra/internal/permission"
	"github.com/altalune-id/othra/nanoid"
	"github.com/altalune-id/othra/scope"
)

//nolint:gochecknoglobals // a fixed allowlist.
var unruled = map[string]string{
	"auth.v1.WhoamiRequest":            "empty request",
	"auth.v1.SignOutEverywhereRequest": "empty request",
	"org.v1.ListOrgsRequest":           "empty request",

	"apikey.v1.CreateAPIKeyRequest.all_projects":                     "a bool; the domain checks it against project_ids (KEY005)",
	"webhook.v1.UpdateWebhookRequest.active":                         "a bool; false deactivates",
	"keys.v1.RotateSigningKeyRequest.immediate":                      "a bool; true promotes the new key at once",
	"client.v1.CreateClientRequest.refresh_tokens":                   "a bool; true adds the refresh_token grant",
	"client.v1.UpdateClientRequest.refresh_tokens":                   "a bool; true adds the refresh_token grant",
	"client.v1.CreateClientRequest.pkce_required":                    "an optional bool; unset is true, false only for a WEB client (CLT003)",
	"client.v1.UpdateClientRequest.pkce_required":                    "an optional bool; unset is true, false only for a WEB client (CLT003)",
	"project.v1.UpdateProjectSignInRequest.email_link":               "a bool; the domain refuses off while it would leave no sign-in method (PRJ010)",
	"upstream.v1.CreateUpstreamRequest.enabled":                      "a bool; any value is valid",
	"upstream.v1.SetUpstreamEnabledRequest.enabled":                  "a bool; the domain refuses off while it would leave no sign-in method (PRJ010)",
	"resourceserver.v1.SetPersonalTokenAccessRequest.accept":         "a bool; the domain refuses on without an introspection secret (RSV012)",
	"resourceserver.v1.SetResourceServerTrustRequest.trusted":        "a bool; the domain refuses it outside self-hosted (RSV007)",
	"resourceserver.v1.SetResourceServerAdmissionRequest.accept":     "a bool; the domain refuses it while the server isn't admissible (RSV011)",
	"resourceserver.v1.SetResourceServerAdmissionRequest.all_scopes": "a bool; false needs scopes from the catalog (RSV009, RSV005)",
	"resourceserver.v1.CreateResourceServerRequest.trusted":          "a bool; the domain refuses it outside self-hosted (RSV007) and from a key",
	"webhook.v1.ListWebhookDeliveriesRequest.limit":                  "any int32 works today: clamped to 1..50",
}

// SECURITY: every RPC request is checked before its handler; a field with no rule must be allowlisted with a reason.
func TestEveryRequestFieldHasARule(t *testing.T) {
	var missing []string
	for procedure := range permission.Table() {
		service, method, ok := strings.Cut(strings.TrimPrefix(procedure, "/"), "/")
		require.True(t, ok, procedure)
		desc, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(service + "." + method))
		require.NoError(t, err, procedure)
		md, ok := desc.(protoreflect.MethodDescriptor)
		require.True(t, ok, procedure)

		input := md.Input()
		name := string(input.FullName())
		if _, ok := unruled[name]; ok {
			assert.Zero(t, input.Fields().Len(), "%s is allowlisted as empty but has fields", name)
			continue
		}
		for i := range input.Fields().Len() {
			field := input.Fields().Get(i)
			key := name + "." + string(field.Name())
			if _, ok := unruled[key]; ok {
				continue
			}
			if !hasRule(field) {
				missing = append(missing, key)
			}
		}
	}
	slices.Sort(missing)
	assert.Empty(t, missing, "request fields with no buf.validate rule; add one or allowlist it with a reason")
}

func TestUnruledListsOnlyRealFields(t *testing.T) {
	for key := range unruled {
		if desc, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(key)); err == nil {
			_, isMsg := desc.(protoreflect.MessageDescriptor)
			_, isField := desc.(protoreflect.FieldDescriptor)
			assert.True(t, isMsg || isField, key)
			continue
		}
		t.Errorf("%s is allowlisted but does not exist", key)
	}
}

func customRuleIDs(t *testing.T) []string {
	t.Helper()
	seen := map[string]bool{}
	for procedure := range permission.Table() {
		service, method, _ := strings.Cut(strings.TrimPrefix(procedure, "/"), "/")
		desc, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(service + "." + method))
		require.NoError(t, err, procedure)
		md, ok := desc.(protoreflect.MethodDescriptor)
		require.True(t, ok, procedure)
		for i := range md.Input().Fields().Len() {
			rules := fieldRules(md.Input().Fields().Get(i))
			for _, r := range [...]*validate.FieldRules{rules, rules.GetRepeated().GetItems()} {
				for _, cel := range r.GetCel() {
					seen[cel.GetId()] = true
				}
				for _, cel := range predefinedRules(r.GetString()) {
					seen[cel.GetId()] = true
				}
			}
		}
	}
	return slices.Sorted(maps.Keys(seen))
}

func predefinedRules(r *validate.StringRules) []*validate.Rule {
	var out []*validate.Rule
	if r == nil {
		return nil
	}
	r.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if !fd.IsExtension() || fd.Kind() != protoreflect.BoolKind || !v.Bool() {
			return true
		}
		if p, ok := proto.GetExtension(fd.Options(), validate.E_Predefined).(*validate.PredefinedRules); ok {
			out = append(out, p.GetCel()...)
		}
		return true
	})
	return out
}

func fieldRules(field protoreflect.FieldDescriptor) *validate.FieldRules {
	opts := field.Options()
	if opts == nil || !proto.HasExtension(opts, validate.E_Field) {
		return nil
	}
	rules, _ := proto.GetExtension(opts, validate.E_Field).(*validate.FieldRules)
	return rules
}

func hasRule(field protoreflect.FieldDescriptor) bool {
	opts := field.Options()
	if opts == nil || !proto.HasExtension(opts, validate.E_Field) {
		return false
	}
	rules, ok := proto.GetExtension(opts, validate.E_Field).(*validate.FieldRules)
	if !ok || rules == nil {
		return false
	}
	return rules.GetRequired() || len(rules.GetCel()) > 0 || rules.GetType() != nil
}

const goodID = "abcdefghjkmnpqrstuvwx"

type ruleCase struct {
	name   string
	msg    proto.Message
	field  string
	reason apperror.FieldReason
}

func TestRequestRules_Accept(t *testing.T) {
	v := newTestValidator(t)
	future := timestamppb.New(time.Now().Add(time.Hour))
	accepted := []struct {
		name string
		msg  proto.Message
	}{
		{"public id", &orgv1.GetOrgRequest{Id: goodID}},
		{"padded name the domain trims", &orgv1.UpdateOrgRequest{Id: goodID, Name: "  " + strings.Repeat("é", 200) + "  ", Slug: "acme"}},
		{"uppercase email", &invitev1.DraftInviteRequest{OrgId: goodID, Email: "  Dina@Example.COM "}},
		{"unicode email", &invitev1.DraftInviteRequest{OrgId: goodID, Email: "dïna@exämple.com"}},
		{"dotless email", &invitev1.DraftInviteRequest{OrgId: goodID, Email: "dina@localhost"}},
		{"domain-literal email", &invitev1.DraftInviteRequest{OrgId: goodID, Email: "dina@[1.2.3.4]"}},
		{"254-character email", &invitev1.DraftInviteRequest{OrgId: goodID, Email: strings.Repeat("a", 242) + "@example.com"}},
		{"each role", &invitev1.CreateInviteRequest{OrgId: goodID, Email: "a@b.c", Role: "admin"}},
		{"org key", &apikeyv1.CreateAPIKeyRequest{OrgId: goodID, Kind: "org", Name: " ci ", Scopes: []string{"webhook:read", "webhook:read"}, ProjectIds: []string{goodID}, ExpiresAt: future}},
		{"project key", &apikeyv1.CreateAPIKeyRequest{OrgId: goodID, ProjectId: goodID, Kind: "project", Name: strings.Repeat("k", 100), ResourceIds: []string{goodID}, ExpiresAt: future}},
		{"bare hint", &apikeyv1.RevokeAPIKeyRequest{OrgId: goodID, Hint: "a_9-"}},
		{"hint as shown", &apikeyv1.RevokeAPIKeyRequest{OrgId: goodID, Hint: " …...Zz09 "}},
		{"hint with dots", &apikeyv1.RevokeAPIKeyRequest{OrgId: goodID, Hint: "...Zz09"}},
		{"blank profile name clears it", &authv1.UpdateProfileRequest{Name: "   "}},
		{"256-character password", &authv1.ChangePasswordRequest{CurrentPassword: "x", NewPassword: strings.Repeat("ü", 256)}},
		{"12-character password counted in runes", &authv1.ChangePasswordRequest{CurrentPassword: "x", NewPassword: strings.Repeat("ü", 12)}},
		{"common password is the domain's", &authv1.ChangePasswordRequest{CurrentPassword: "x", NewPassword: "password1234"}},
		{"padded description", &webhookv1.CreateWebhookRequest{OrgId: goodID, ProjectId: goodID, Url: "https://e.com", Description: " " + strings.Repeat("d", 200) + " ", EventTypes: []string{"member.added"}}},
		{"webhook url url.Parse accepts", &webhookv1.CreateWebhookRequest{OrgId: goodID, ProjectId: goodID, Url: " HTTPS://example.com/a b/ü ", EventTypes: []string{"invite.created"}}},
		{"duplicate events are the domain's", &webhookv1.UpdateWebhookRequest{OrgId: goodID, ProjectId: goodID, Id: goodID, Url: "http://[::1]:8080/x", EventTypes: []string{"invite.created", "invite.created"}}},
		{"any delivery limit", &webhookv1.ListWebhookDeliveriesRequest{OrgId: goodID, ProjectId: goodID, Id: goodID, Limit: -5}},
		{"webhook description of 200", &webhookv1.CreateWebhookRequest{OrgId: goodID, ProjectId: goodID, Url: "https://e.com", Description: strings.Repeat("ß", 200), EventTypes: []string{"member.added"}}},
		{"empty slug on create mints one", &orgv1.CreateOrgRequest{Name: "Acme"}},
		{"shortest slug", &projectv1.CreateProjectRequest{OrgId: goodID, Name: "Web", Slug: "a-1"}},
		{"longest slug", &projectv1.CreateProjectRequest{OrgId: goodID, Name: "Web", Slug: strings.Repeat("a", 64)}},
		{"largest logo upload", &projectv1.CreateProjectLogoUploadRequest{Id: goodID, ContentType: strings.Repeat("t", 100), SizeBytes: 10 << 20}},
		{"any declared type is the domain's", &projectv1.CreateProjectLogoUploadRequest{Id: goodID, ContentType: "image/gif", SizeBytes: 1}},
		{"complete an upload", &projectv1.CompleteProjectLogoUploadRequest{Id: goodID, UploadId: goodID}},
		{"reserved slug is the domain's", &orgv1.UpdateOrgRequest{Id: goodID, Name: "Acme", Slug: "new"}},
		{"list signing keys", &keysv1.ListSigningKeysRequest{ProjectId: goodID}},
		{"rotate at once", &keysv1.RotateSigningKeyRequest{ProjectId: goodID, Immediate: true}},
		{"web client", &clientv1.CreateClientRequest{ProjectId: goodID, Name: strings.Repeat("c", 100), Kind: clientv1.ClientKind_CLIENT_KIND_WEB, RedirectUris: []string{"https://a.example/cb"}}},
		{"redirect URI shape is the domain's", &clientv1.CreateClientRequest{ProjectId: goodID, Name: "CLI", Kind: clientv1.ClientKind_CLIENT_KIND_CLI, RedirectUris: []string{"not a url", strings.Repeat("u", 2000)}, ConsentPolicy: clientv1.ConsentPolicy_CONSENT_POLICY_IMPLICIT}},
		{"twenty redirect URIs", &clientv1.UpdateClientRequest{ProjectId: goodID, ClientId: goodID, Name: "Web", RedirectUris: make([]string, 20), PostLogoutRedirectUris: make([]string, 20), ConsentPolicy: clientv1.ConsentPolicy_CONSENT_POLICY_REQUIRE}},
		{"rotate with the default grace", &clientv1.RotateClientSecretRequest{ProjectId: goodID, ClientId: goodID}},
		{"revoke a secret", &clientv1.RevokeClientSecretRequest{ProjectId: goodID, ClientId: goodID, SecretId: goodID}},
		{"identifier shape is the domain's", &resourceserverv1.CreateResourceServerRequest{ProjectId: goodID, Name: "API", Identifier: strings.Repeat("i", 512), Scopes: make([]string, 64)}},
		{"links of a client", &resourceserverv1.ListLinksRequest{ProjectId: goodID, Target: &resourceserverv1.ListLinksRequest_ClientId{ClientId: goodID}}},
		{"links of a resource server", &resourceserverv1.ListLinksRequest{ProjectId: goodID, Target: &resourceserverv1.ListLinksRequest_ResourceServerId{ResourceServerId: goodID}}},
		{"link with a 128-character scope", &resourceserverv1.LinkClientRequest{ProjectId: goodID, ClientId: goodID, ResourceServerId: goodID, Scopes: []string{strings.Repeat("s", 128)}}},
	}
	for _, tt := range accepted {
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, v.Validate(tt.msg))
		})
	}
}

func TestRequestRules_Refuse(t *testing.T) {
	v := newTestValidator(t)
	future := timestamppb.New(time.Now().Add(time.Hour))
	refused := []ruleCase{
		{"missing id", &orgv1.GetOrgRequest{}, "id", apperror.FieldRequired},
		{"short id", &orgv1.GetOrgRequest{Id: "abc"}, "id", apperror.FieldInvalidFormat},
		{"uuid id", &orgv1.GetOrgRequest{Id: "4b0b0a0e-1d2c-4f7a-9a6b-1f0e2d3c4b5a"}, "id", apperror.FieldInvalidFormat},
		{"id outside the alphabet", &projectv1.GetProjectRequest{Id: "abcdefghjkmnpqrstuv-_"}, "id", apperror.FieldInvalidFormat},
		{"blank org id", &invitev1.ListInvitesRequest{OrgId: "   "}, "org_id", apperror.FieldInvalidFormat},
		{"missing org id", &invitev1.ListInvitesRequest{}, "org_id", apperror.FieldRequired},
		{"missing invite id", &invitev1.RevokeInviteRequest{OrgId: goodID}, "invite_id", apperror.FieldRequired},
		{"missing name", &projectv1.CreateProjectRequest{OrgId: goodID}, "name", apperror.FieldRequired},
		{"blank name", &projectv1.CreateProjectRequest{OrgId: goodID, Name: " \t "}, "name", apperror.FieldRequired},
		{"201-character name", &orgv1.UpdateOrgRequest{Id: goodID, Name: strings.Repeat("a", 201), Slug: "acme"}, "name", apperror.FieldTooLong},
		{"missing email", &invitev1.DraftInviteRequest{OrgId: goodID}, "email", apperror.FieldRequired},
		{"not an email", &invitev1.DraftInviteRequest{OrgId: goodID, Email: "dina"}, "email", apperror.FieldInvalidFormat},
		{"spaced email", &invitev1.DraftInviteRequest{OrgId: goodID, Email: "di na@x.io"}, "email", apperror.FieldInvalidFormat},
		{"255-character email", &invitev1.DraftInviteRequest{OrgId: goodID, Email: strings.Repeat("a", 243) + "@example.com"}, "email", apperror.FieldTooLong},
		{"unknown role", &memberv1.SetMemberRoleRequest{OrgId: goodID, UserId: goodID, Role: "Owner"}, "role", apperror.FieldNotAllowed},
		{"missing role", &invitev1.CreateInviteRequest{OrgId: goodID, Email: "a@b.c"}, "role", apperror.FieldRequired},
		{"unknown kind", &apikeyv1.CreateAPIKeyRequest{OrgId: goodID, Kind: "admin", Name: "ci", ExpiresAt: future}, "kind", apperror.FieldNotAllowed},
		{"blank key name", &apikeyv1.CreateAPIKeyRequest{OrgId: goodID, Kind: "org", Name: "  ", ExpiresAt: future}, "name", apperror.FieldRequired},
		{"101-character key name", &apikeyv1.CreateAPIKeyRequest{OrgId: goodID, Kind: "org", Name: strings.Repeat("k", 101), ExpiresAt: future}, "name", apperror.FieldTooLong},
		{"scope shape", &apikeyv1.CreateAPIKeyRequest{OrgId: goodID, Kind: "org", Name: "ci", Scopes: []string{"webhook:*"}, ExpiresAt: future}, "scopes[0]", apperror.FieldInvalidFormat},
		{"project ids are public ids", &apikeyv1.CreateAPIKeyRequest{OrgId: goodID, Kind: "org", Name: "ci", ProjectIds: []string{goodID, "x"}, ExpiresAt: future}, "project_ids[1]", apperror.FieldInvalidFormat},
		{"missing expiry", &apikeyv1.CreateAPIKeyRequest{OrgId: goodID, Kind: "org", Name: "ci"}, "expires_at", apperror.FieldRequired},
		{"hint shape", &apikeyv1.RevokeAPIKeyRequest{OrgId: goodID, Hint: "abcde"}, "hint", apperror.FieldInvalidFormat},
		{"201-character profile name", &authv1.UpdateProfileRequest{Name: strings.Repeat("n", 201)}, "name", apperror.FieldTooLong},
		{"missing current password", &authv1.ChangePasswordRequest{NewPassword: "a long enough one"}, "current_password", apperror.FieldRequired},
		{"257-character password", &authv1.ChangePasswordRequest{CurrentPassword: "x", NewPassword: strings.Repeat("p", 257)}, "new_password", apperror.FieldTooLong},
		{"missing url", &webhookv1.CreateWebhookRequest{OrgId: goodID, ProjectId: goodID, EventTypes: []string{"invite.created"}}, "url", apperror.FieldRequired},
		{"relative url", &webhookv1.CreateWebhookRequest{OrgId: goodID, ProjectId: goodID, Url: "/hook", EventTypes: []string{"invite.created"}}, "url", apperror.FieldInvalidFormat},
		{"url without host", &webhookv1.CreateWebhookRequest{OrgId: goodID, ProjectId: goodID, Url: "https:///x", EventTypes: []string{"invite.created"}}, "url", apperror.FieldInvalidFormat},
		{"2049-character url", &webhookv1.CreateWebhookRequest{OrgId: goodID, ProjectId: goodID, Url: "https://e.com/" + strings.Repeat("u", 2035), EventTypes: []string{"invite.created"}}, "url", apperror.FieldTooLong},
		{"201-character webhook description", &webhookv1.CreateWebhookRequest{OrgId: goodID, ProjectId: goodID, Url: "https://e.com", Description: strings.Repeat("d", 201), EventTypes: []string{"invite.created"}}, "description", apperror.FieldTooLong},
		{"no event types", &webhookv1.CreateWebhookRequest{OrgId: goodID, ProjectId: goodID, Url: "https://e.com"}, "event_types", apperror.FieldRequired},
		{"event type shape", &webhookv1.UpdateWebhookRequest{OrgId: goodID, ProjectId: goodID, Id: goodID, Url: "https://e.com", EventTypes: []string{"Invite Created"}}, "event_types[0]", apperror.FieldInvalidFormat},
		{"short new password", &authv1.ChangePasswordRequest{CurrentPassword: "x", NewPassword: "short"}, "new_password", apperror.FieldTooShort},
		{"11-character password counted in runes", &authv1.ChangePasswordRequest{CurrentPassword: "x", NewPassword: strings.Repeat("ü", 11)}, "new_password", apperror.FieldTooShort},
		{"slug outside the alphabet", &orgv1.UpdateOrgRequest{Id: goodID, Name: "Acme", Slug: "NOT A SLUG"}, "slug", apperror.FieldInvalidFormat},
		{"slug ending in a dash", &projectv1.UpdateProjectRequest{Id: goodID, Name: "Web", Slug: "web-"}, "slug", apperror.FieldInvalidFormat},
		{"padded slug", &orgv1.CreateOrgRequest{Name: "Acme", Slug: " acme "}, "slug", apperror.FieldInvalidFormat},
		{"2-character slug", &projectv1.CreateProjectRequest{OrgId: goodID, Name: "Web", Slug: "ab"}, "slug", apperror.FieldTooShort},
		{"65-character slug", &projectv1.CreateProjectRequest{OrgId: goodID, Name: "Web", Slug: strings.Repeat("a", 65)}, "slug", apperror.FieldTooLong},
		{"logo without a type", &projectv1.CreateProjectLogoUploadRequest{Id: goodID, SizeBytes: 1}, "content_type", apperror.FieldRequired},
		{"101-character logo type", &projectv1.CreateProjectLogoUploadRequest{Id: goodID, ContentType: strings.Repeat("t", 101), SizeBytes: 1}, "content_type", apperror.FieldTooLong},
		{"empty logo", &projectv1.CreateProjectLogoUploadRequest{Id: goodID, ContentType: "image/png"}, "size_bytes", apperror.FieldInvalid},
		{"logo past the transport bound", &projectv1.CreateProjectLogoUploadRequest{Id: goodID, ContentType: "image/png", SizeBytes: 10<<20 + 1}, "size_bytes", apperror.FieldInvalid},
		{"upload id", &projectv1.CompleteProjectLogoUploadRequest{Id: goodID, UploadId: "x"}, "upload_id", apperror.FieldInvalidFormat},
		{"missing upload id", &projectv1.CompleteProjectLogoUploadRequest{Id: goodID}, "upload_id", apperror.FieldRequired},
		{"missing slug on update", &orgv1.UpdateOrgRequest{Id: goodID, Name: "Acme"}, "slug", apperror.FieldRequired},
		{"delivery id", &webhookv1.RetryWebhookDeliveryRequest{OrgId: goodID, ProjectId: goodID, Id: goodID, DeliveryId: "x"}, "delivery_id", apperror.FieldInvalidFormat},
		{"client without a kind", &clientv1.CreateClientRequest{ProjectId: goodID, Name: "Web", RedirectUris: []string{"https://a.example/cb"}}, "kind", apperror.FieldRequired},
		{"unknown client kind", &clientv1.CreateClientRequest{ProjectId: goodID, Name: "Web", Kind: 9, RedirectUris: []string{"https://a.example/cb"}}, "kind", apperror.FieldNotAllowed},
		{"101-character client name", &clientv1.CreateClientRequest{ProjectId: goodID, Name: strings.Repeat("c", 101), Kind: clientv1.ClientKind_CLIENT_KIND_WEB, RedirectUris: []string{"https://a.example/cb"}}, "name", apperror.FieldTooLong},
		{"blank client name", &clientv1.UpdateClientRequest{ProjectId: goodID, ClientId: goodID, Name: "  ", RedirectUris: []string{"https://a.example/cb"}, ConsentPolicy: clientv1.ConsentPolicy_CONSENT_POLICY_REQUIRE}, "name", apperror.FieldRequired},
		{"no redirect URI", &clientv1.CreateClientRequest{ProjectId: goodID, Name: "Web", Kind: clientv1.ClientKind_CLIENT_KIND_WEB}, "redirect_uris", apperror.FieldRequired},
		{"21 redirect URIs", &clientv1.CreateClientRequest{ProjectId: goodID, Name: "Web", Kind: clientv1.ClientKind_CLIENT_KIND_WEB, RedirectUris: make([]string, 21)}, "redirect_uris", apperror.FieldTooMany},
		{"2001-character redirect URI", &clientv1.CreateClientRequest{ProjectId: goodID, Name: "Web", Kind: clientv1.ClientKind_CLIENT_KIND_WEB, RedirectUris: []string{strings.Repeat("u", 2001)}}, "redirect_uris[0]", apperror.FieldTooLong},
		{"21 sign-out URIs", &clientv1.UpdateClientRequest{ProjectId: goodID, ClientId: goodID, Name: "Web", RedirectUris: []string{"https://a.example/cb"}, PostLogoutRedirectUris: make([]string, 21), ConsentPolicy: clientv1.ConsentPolicy_CONSENT_POLICY_REQUIRE}, "post_logout_redirect_uris", apperror.FieldTooMany},
		{"update without a consent policy", &clientv1.UpdateClientRequest{ProjectId: goodID, ClientId: goodID, Name: "Web", RedirectUris: []string{"https://a.example/cb"}}, "consent_policy", apperror.FieldRequired},
		{"unknown grace", &clientv1.RotateClientSecretRequest{ProjectId: goodID, ClientId: goodID, Grace: 9}, "grace", apperror.FieldNotAllowed},
		{"missing secret id", &clientv1.RevokeClientSecretRequest{ProjectId: goodID, ClientId: goodID}, "secret_id", apperror.FieldRequired},
		{"missing client id", &clientv1.GenerateClientSecretRequest{ProjectId: goodID}, "client_id", apperror.FieldRequired},
		{"missing identifier", &resourceserverv1.CreateResourceServerRequest{ProjectId: goodID, Name: "API", Scopes: []string{"read"}}, "identifier", apperror.FieldRequired},
		{"513-character identifier", &resourceserverv1.CreateResourceServerRequest{ProjectId: goodID, Name: "API", Identifier: strings.Repeat("i", 513), Scopes: []string{"read"}}, "identifier", apperror.FieldTooLong},
		{"no scopes", &resourceserverv1.UpdateResourceServerRequest{ProjectId: goodID, ResourceServerId: goodID, Name: "API"}, "scopes", apperror.FieldRequired},
		{"65 scopes", &resourceserverv1.CreateResourceServerRequest{ProjectId: goodID, Name: "API", Identifier: "https://a.example", Scopes: make([]string, 65)}, "scopes", apperror.FieldTooMany},
		{"129-character scope", &resourceserverv1.UpdateLinkRequest{ProjectId: goodID, ClientId: goodID, ResourceServerId: goodID, Scopes: []string{strings.Repeat("s", 129)}}, "scopes[0]", apperror.FieldTooLong},
		{"101-character resource server name", &resourceserverv1.CreateResourceServerRequest{ProjectId: goodID, Name: strings.Repeat("r", 101), Identifier: "https://a.example", Scopes: []string{"read"}}, "name", apperror.FieldTooLong},
		{"link without scopes", &resourceserverv1.LinkClientRequest{ProjectId: goodID, ClientId: goodID, ResourceServerId: goodID}, "scopes", apperror.FieldRequired},
		{"links of nothing", &resourceserverv1.ListLinksRequest{ProjectId: goodID}, "target", apperror.FieldRequired},
		{"links of a bad client id", &resourceserverv1.ListLinksRequest{ProjectId: goodID, Target: &resourceserverv1.ListLinksRequest_ClientId{ClientId: "x"}}, "client_id", apperror.FieldInvalidFormat},
		{"missing resource server id", &resourceserverv1.UnlinkClientRequest{ProjectId: goodID, ClientId: goodID}, "resource_server_id", apperror.FieldRequired},
	}
	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			ae, ok := apperror.AsAppError(v.Validate(tt.msg))
			require.True(t, ok, "want a refusal")
			assert.Contains(t, ae.Violations(), apperror.Violation{Field: tt.field, Reason: tt.reason, Param: paramOf(ae.Violations(), tt.field)})
			assert.Len(t, ae.Violations(), 1, "%v", ae.Violations())
		})
	}
}

func paramOf(vs []apperror.Violation, field string) string {
	for _, v := range vs {
		if v.Field == field {
			return v.Param
		}
	}
	return ""
}

func TestRequestRules_ParamsCarryTheLimit(t *testing.T) {
	v := newTestValidator(t)
	limits := map[string]proto.Message{
		"200": &orgv1.UpdateOrgRequest{Id: goodID, Name: strings.Repeat("a", 201), Slug: "acme"},
		"254": &invitev1.DraftInviteRequest{OrgId: goodID, Email: strings.Repeat("a", 243) + "@example.com"},
		"100": &apikeyv1.CreateAPIKeyRequest{OrgId: goodID, Kind: "org", Name: strings.Repeat("k", 101), ExpiresAt: timestamppb.Now()},
		"256": &authv1.ChangePasswordRequest{CurrentPassword: "x", NewPassword: strings.Repeat("p", 257)},
		"12":  &authv1.ChangePasswordRequest{CurrentPassword: "x", NewPassword: "short"},
		"3":   &projectv1.CreateProjectRequest{OrgId: goodID, Name: "Web", Slug: "ab"},
		"64":  &orgv1.UpdateOrgRequest{Id: goodID, Name: "Acme", Slug: strings.Repeat("a", 65)},
	}
	for want, msg := range limits {
		ae, ok := apperror.AsAppError(v.Validate(msg))
		require.True(t, ok, want)
		assert.Equal(t, want, ae.Violations()[0].Param)
	}
}

// the API-key scope rule takes every catalog scope, so a new scope needs no proto edit; the service checks membership.
func TestAPIKeyScopeRule_AcceptsEveryCatalogScope(t *testing.T) {
	v := newTestValidator(t)
	future := timestamppb.New(time.Now().Add(time.Hour))
	for _, name := range scope.Names() {
		req := &apikeyv1.CreateAPIKeyRequest{OrgId: goodID, ProjectId: goodID, Kind: "project", Name: "ci", Scopes: []string{name}, ExpiresAt: future}
		require.NoError(t, v.Validate(req), name)
	}
	for _, bad := range []string{"Project:read", "project", ":read", "project:read write", "end-user:read"} {
		req := &apikeyv1.CreateAPIKeyRequest{OrgId: goodID, ProjectId: goodID, Kind: "project", Name: "ci", Scopes: []string{bad}, ExpiresAt: future}
		assert.Error(t, v.Validate(req), bad)
	}
}

// SECURITY: othra.public_id must accept every id nanoid mints, or a real record could never be named.
func TestPublicIDRule_AcceptsEveryMintedID(t *testing.T) {
	v := newTestValidator(t)
	for range 2000 {
		id, err := nanoid.New(21)
		require.NoError(t, err)
		require.NoError(t, v.Validate(&orgv1.GetOrgRequest{Id: id}), id)
	}
	for _, outside := range []string{"0", "1", "i", "l", "o", "I", "L", "O", "-", "_"} {
		ae, ok := apperror.AsAppError(v.Validate(&orgv1.GetOrgRequest{Id: goodID[:20] + outside}))
		require.True(t, ok, outside)
		assert.Equal(t, apperror.FieldInvalidFormat, ae.Violations()[0].Reason, outside)
	}
}
