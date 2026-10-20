// Package apiclient is the Connect client the CLI uses to reach a remote Othra; it imports no domain package.
package apiclient

import (
	"context"
	"strings"
	"time"

	"connectrpc.com/connect"

	apikeyv1connect "altalune.id/othra/gen/go/apikey/v1/apikeyv1connect"
	authv1connect "altalune.id/othra/gen/go/auth/v1/authv1connect"
	clientv1connect "altalune.id/othra/gen/go/client/v1/clientv1connect"
	invitev1connect "altalune.id/othra/gen/go/invite/v1/invitev1connect"
	keysv1connect "altalune.id/othra/gen/go/keys/v1/keysv1connect"
	memberv1connect "altalune.id/othra/gen/go/member/v1/memberv1connect"
	orgv1connect "altalune.id/othra/gen/go/org/v1/orgv1connect"
	projectv1connect "altalune.id/othra/gen/go/project/v1/projectv1connect"
	resourceserverv1connect "altalune.id/othra/gen/go/resourceserver/v1/resourceserverv1connect"
	webhookv1connect "altalune.id/othra/gen/go/webhook/v1/webhookv1connect"
	"altalune.id/othra/httpclient"
	"altalune.id/othra/internal/kernel/session"
)

// DefaultTimeout bounds every remote CLI request.
const DefaultTimeout = 30 * time.Second

// Credential is what the CLI presents: an API key (Bearer) or a signed session (cookie); at most one is used.
type Credential struct {
	Token   string
	Session string
}

// Client bundles the Connect clients the CLI uses to reach a remote Othra.
type Client struct {
	Org      orgv1connect.OrgServiceClient
	Projects projectv1connect.ProjectServiceClient
	APIKeys  apikeyv1connect.APIKeyServiceClient
	Webhooks webhookv1connect.WebhookServiceClient
	Auth     authv1connect.AuthServiceClient
	Members  memberv1connect.MemberServiceClient
	Invites  invitev1connect.InviteServiceClient
	Keys     keysv1connect.SigningKeyServiceClient
	Clients  clientv1connect.ClientServiceClient

	ResourceServers resourceserverv1connect.ResourceServerServiceClient
}

// New points every client at baseURL+"/api"; a token becomes the Bearer header, else a session becomes the cookie.
func New(baseURL string, cred Credential) *Client {
	httpClient := httpclient.New(
		httpclient.WithAllowPrivateHosts(true), // the CLI may legitimately target localhost/private hosts
		httpclient.WithTimeout(DefaultTimeout),
	)
	base := strings.TrimRight(baseURL, "/") + "/api"
	opts := []connect.ClientOption{connect.WithInterceptors(credentialInterceptor(cred))}
	return &Client{
		Org:      orgv1connect.NewOrgServiceClient(httpClient, base, opts...),
		Projects: projectv1connect.NewProjectServiceClient(httpClient, base, opts...),
		APIKeys:  apikeyv1connect.NewAPIKeyServiceClient(httpClient, base, opts...),
		Webhooks: webhookv1connect.NewWebhookServiceClient(httpClient, base, opts...),
		Auth:     authv1connect.NewAuthServiceClient(httpClient, base, opts...),
		Members:  memberv1connect.NewMemberServiceClient(httpClient, base, opts...),
		Invites:  invitev1connect.NewInviteServiceClient(httpClient, base, opts...),
		Keys:     keysv1connect.NewSigningKeyServiceClient(httpClient, base, opts...),
		Clients:  clientv1connect.NewClientServiceClient(httpClient, base, opts...),

		ResourceServers: resourceserverv1connect.NewResourceServerServiceClient(httpClient, base, opts...),
	}
}

func credentialInterceptor(cred Credential) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			switch {
			case cred.Token != "":
				req.Header().Set("Authorization", "Bearer "+cred.Token)
			case cred.Session != "":
				req.Header().Set("Cookie", session.CookieName+"="+cred.Session)
			}
			return next(ctx, req)
		}
	}
}
