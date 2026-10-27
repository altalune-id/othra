// Package permission holds the one table of the permissions each Connect procedure needs, for /api and MCP alike.
package permission

import (
	"time"

	accountv1connect "altalune.id/othra/gen/go/account/v1/accountv1connect"
	apikeyv1connect "altalune.id/othra/gen/go/apikey/v1/apikeyv1connect"
	authv1connect "altalune.id/othra/gen/go/auth/v1/authv1connect"
	clientv1connect "altalune.id/othra/gen/go/client/v1/clientv1connect"
	enduserv1connect "altalune.id/othra/gen/go/enduser/v1/enduserv1connect"
	invitev1connect "altalune.id/othra/gen/go/invite/v1/invitev1connect"
	keysv1connect "altalune.id/othra/gen/go/keys/v1/keysv1connect"
	memberv1connect "altalune.id/othra/gen/go/member/v1/memberv1connect"
	orgv1connect "altalune.id/othra/gen/go/org/v1/orgv1connect"
	projectv1connect "altalune.id/othra/gen/go/project/v1/projectv1connect"
	resourceserverv1connect "altalune.id/othra/gen/go/resourceserver/v1/resourceserverv1connect"
	upstreamv1connect "altalune.id/othra/gen/go/upstream/v1/upstreamv1connect"
	webhookv1connect "altalune.id/othra/gen/go/webhook/v1/webhookv1connect"
	"altalune.id/othra/internal/kernel/authz"
	"altalune.id/othra/scope"
)

// Table maps every procedure to the permissions its caller must hold, all of them. SECURITY: a missing or empty entry is denied.
func Table() map[string][]string {
	return map[string][]string{
		orgv1connect.OrgServiceCreateOrgProcedure: {scope.OrgWrite},
		orgv1connect.OrgServiceGetOrgProcedure:    {scope.OrgRead},
		orgv1connect.OrgServiceListOrgsProcedure:  {scope.OrgRead},
		orgv1connect.OrgServiceUpdateOrgProcedure: {scope.OrgWrite},

		projectv1connect.ProjectServiceCreateProjectProcedure:             {scope.ProjectWrite},
		projectv1connect.ProjectServiceGetProjectProcedure:                {scope.ProjectRead},
		projectv1connect.ProjectServiceListProjectsProcedure:              {scope.ProjectRead},
		projectv1connect.ProjectServiceUpdateProjectProcedure:             {scope.ProjectWrite},
		projectv1connect.ProjectServiceUpdateProjectIssuerProcedure:       {scope.ProjectWrite},
		projectv1connect.ProjectServiceUpdateProjectBrandingProcedure:     {scope.ProjectWrite},
		projectv1connect.ProjectServiceUpdateProjectTermsProcedure:        {scope.ProjectWrite},
		projectv1connect.ProjectServiceUpdateProjectSignInProcedure:       {scope.ProjectWrite},
		projectv1connect.ProjectServiceBumpTermsVersionProcedure:          {scope.ProjectWrite},
		projectv1connect.ProjectServiceCreateProjectLogoUploadProcedure:   {scope.ProjectWrite},
		projectv1connect.ProjectServiceCompleteProjectLogoUploadProcedure: {scope.ProjectWrite},
		projectv1connect.ProjectServiceDeleteProjectLogoProcedure:         {scope.ProjectWrite},
		projectv1connect.ProjectServiceCheckIssuerHandleProcedure:         {scope.ProjectRead},

		keysv1connect.SigningKeyServiceListSigningKeysProcedure:  {scope.ProjectRead},
		keysv1connect.SigningKeyServiceRotateSigningKeyProcedure: {scope.ProjectWrite},

		clientv1connect.ClientServiceListClientsProcedure:          {scope.ProjectRead},
		clientv1connect.ClientServiceGetClientProcedure:            {scope.ProjectRead},
		clientv1connect.ClientServiceCreateClientProcedure:         {scope.ProjectWrite},
		clientv1connect.ClientServiceUpdateClientProcedure:         {scope.ProjectWrite},
		clientv1connect.ClientServiceDeleteClientProcedure:         {scope.ProjectWrite},
		clientv1connect.ClientServiceGenerateClientSecretProcedure: {scope.ProjectWrite},
		clientv1connect.ClientServiceRotateClientSecretProcedure:   {scope.ProjectWrite},
		clientv1connect.ClientServiceRevokeClientSecretProcedure:   {scope.ProjectWrite},

		resourceserverv1connect.ResourceServerServiceListResourceServersProcedure:         {scope.ProjectRead},
		resourceserverv1connect.ResourceServerServiceGetResourceServerProcedure:           {scope.ProjectRead},
		resourceserverv1connect.ResourceServerServiceCreateResourceServerProcedure:        {scope.ProjectWrite},
		resourceserverv1connect.ResourceServerServiceUpdateResourceServerProcedure:        {scope.ProjectWrite},
		resourceserverv1connect.ResourceServerServiceDeleteResourceServerProcedure:        {scope.ProjectWrite},
		resourceserverv1connect.ResourceServerServiceVerifyResourceServerProcedure:        {scope.ProjectWrite},
		resourceserverv1connect.ResourceServerServiceSetResourceServerTrustProcedure:      {scope.ProjectWrite},
		resourceserverv1connect.ResourceServerServiceSetResourceServerAdmissionProcedure:  {scope.ProjectWrite},
		resourceserverv1connect.ResourceServerServiceSetPersonalTokenAccessProcedure:      {scope.ProjectWrite},
		resourceserverv1connect.ResourceServerServiceGenerateIntrospectionSecretProcedure: {scope.ProjectWrite},
		resourceserverv1connect.ResourceServerServiceRotateIntrospectionSecretProcedure:   {scope.ProjectWrite},
		resourceserverv1connect.ResourceServerServiceRevokeIntrospectionSecretProcedure:   {scope.ProjectWrite},
		upstreamv1connect.UpstreamServiceListUpstreamsProcedure:                           {scope.ProjectRead},
		upstreamv1connect.UpstreamServiceGetUpstreamProcedure:                             {scope.ProjectRead},
		upstreamv1connect.UpstreamServiceCreateUpstreamProcedure:                          {scope.ProjectWrite},
		upstreamv1connect.UpstreamServiceUpdateUpstreamProcedure:                          {scope.ProjectWrite},
		upstreamv1connect.UpstreamServiceSetUpstreamEnabledProcedure:                      {scope.ProjectWrite},
		upstreamv1connect.UpstreamServiceDeleteUpstreamProcedure:                          {scope.ProjectWrite},
		resourceserverv1connect.ResourceServerServiceListLinksProcedure:                   {scope.ProjectRead},
		resourceserverv1connect.ResourceServerServiceLinkClientProcedure:                  {scope.ProjectWrite},
		resourceserverv1connect.ResourceServerServiceUpdateLinkProcedure:                  {scope.ProjectWrite},
		resourceserverv1connect.ResourceServerServiceUnlinkClientProcedure:                {scope.ProjectWrite},

		enduserv1connect.EndUserServiceListEndUsersProcedure:         {scope.EndUserRead},
		enduserv1connect.EndUserServiceGetEndUserProcedure:           {scope.EndUserRead},
		enduserv1connect.EndUserServiceRevokeEndUserSessionProcedure: {scope.EndUserRevoke},
		enduserv1connect.EndUserServiceDisconnectEndUserAppProcedure: {scope.EndUserRevoke},
		enduserv1connect.EndUserServiceRevokeEndUserTokenProcedure:   {scope.EndUserRevoke},

		apikeyv1connect.APIKeyServiceListAPIKeysProcedure:  {scope.ApikeyRead},
		apikeyv1connect.APIKeyServiceCreateAPIKeyProcedure: {authz.AnyAuthenticated},
		apikeyv1connect.APIKeyServiceRevokeAPIKeyProcedure: {authz.AnyAuthenticated},

		webhookv1connect.WebhookServiceGetWebhookProcedure:                   {scope.WebhookRead},
		webhookv1connect.WebhookServiceListWebhooksProcedure:                 {scope.WebhookRead},
		webhookv1connect.WebhookServiceListWebhookDeliveriesProcedure:        {scope.WebhookRead},
		webhookv1connect.WebhookServiceListWebhookDeliveryAttemptsProcedure:  {scope.WebhookRead},
		webhookv1connect.WebhookServiceCreateWebhookProcedure:                {scope.WebhookWrite},
		webhookv1connect.WebhookServiceUpdateWebhookProcedure:                {scope.WebhookWrite},
		webhookv1connect.WebhookServiceDeleteWebhookProcedure:                {scope.WebhookWrite},
		webhookv1connect.WebhookServiceRotateWebhookSecretProcedure:          {scope.WebhookWrite},
		webhookv1connect.WebhookServicePingWebhookProcedure:                  {scope.WebhookWrite},
		webhookv1connect.WebhookServiceRetryWebhookDeliveryProcedure:         {scope.WebhookWrite},
		webhookv1connect.WebhookServiceRetryFailedWebhookDeliveriesProcedure: {scope.WebhookWrite},

		memberv1connect.MemberServiceListMembersProcedure:   {scope.MemberRead},
		memberv1connect.MemberServiceSetMemberRoleProcedure: {scope.MemberWrite},
		memberv1connect.MemberServiceRemoveMemberProcedure:  {scope.MemberWrite},
		memberv1connect.MemberServiceLeaveOrgProcedure:      {authz.PersonOnly},

		invitev1connect.InviteServiceListInvitesProcedure:        {scope.MemberRead},
		invitev1connect.InviteServiceCreateInviteProcedure:       {scope.MemberWrite},
		invitev1connect.InviteServiceResendInviteProcedure:       {scope.MemberWrite},
		invitev1connect.InviteServiceRevokeInviteProcedure:       {scope.MemberWrite},
		invitev1connect.InviteServiceDraftInviteProcedure:        {scope.MemberWrite},
		invitev1connect.InviteServiceConfirmDraftInviteProcedure: {scope.MemberWrite},
		invitev1connect.InviteServiceDiscardDraftInviteProcedure: {scope.MemberWrite},

		authv1connect.AuthServiceWhoamiProcedure:            {authz.AnyAuthenticated},
		authv1connect.AuthServiceSetActiveOrgProcedure:      {authz.PersonOnly},
		authv1connect.AuthServiceCreateOwnOrgProcedure:      {authz.PersonOnly},
		authv1connect.AuthServiceUpdateProfileProcedure:     {authz.PersonOnly},
		authv1connect.AuthServiceChangePasswordProcedure:    {authz.PersonOnly},
		authv1connect.AuthServiceSignOutEverywhereProcedure: {authz.PersonOnly},
	}
}

// The end-user plane's markers: Self is the signed-in end user on their own rows; SelfRecent also needs a sign-in in the last 10 minutes.
const (
	Self       = "self"
	SelfRecent = "self:recent"
)

// RecentSignIn is how fresh the sign-in behind a SelfRecent call must be.
const RecentSignIn = 10 * time.Minute

// EndUserTable maps each end user's own procedure at /p/<handle>/api/ to Self or SelfRecent. SECURITY: Table never names them.
func EndUserTable() map[string]string {
	return map[string]string{
		accountv1connect.AccountServiceGetProfileProcedure:          Self,
		accountv1connect.AccountServiceUpdateProfileProcedure:       Self,
		accountv1connect.AccountServiceGetSignInMethodsProcedure:    Self,
		accountv1connect.AccountServiceUnlinkIdentityProcedure:      Self,
		accountv1connect.AccountServiceListSessionsProcedure:        Self,
		accountv1connect.AccountServiceEndSessionProcedure:          Self,
		accountv1connect.AccountServiceEndOtherSessionsProcedure:    Self,
		accountv1connect.AccountServiceListConnectedAppsProcedure:   Self,
		accountv1connect.AccountServiceDisconnectAppProcedure:       Self,
		accountv1connect.AccountServiceListTokenServersProcedure:    Self,
		accountv1connect.AccountServiceListPersonalTokensProcedure:  Self,
		accountv1connect.AccountServiceCreatePersonalTokenProcedure: SelfRecent,
		accountv1connect.AccountServiceRevokePersonalTokenProcedure: Self,
	}
}

// For returns the permissions procedure needs, or nil for an unknown procedure (which is denied).
func For(procedure string) []string { return Table()[procedure] }
