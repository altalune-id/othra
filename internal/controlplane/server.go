// Package controlplane wires the Connect-RPC handlers into an http.Handler.
package controlplane

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"

	"github.com/altalune-id/othra/apperror"
	apikeyv1connect "github.com/altalune-id/othra/gen/go/apikey/v1/apikeyv1connect"
	authv1connect "github.com/altalune-id/othra/gen/go/auth/v1/authv1connect"
	clientv1connect "github.com/altalune-id/othra/gen/go/client/v1/clientv1connect"
	enduserv1connect "github.com/altalune-id/othra/gen/go/enduser/v1/enduserv1connect"
	invitev1connect "github.com/altalune-id/othra/gen/go/invite/v1/invitev1connect"
	keysv1connect "github.com/altalune-id/othra/gen/go/keys/v1/keysv1connect"
	memberv1connect "github.com/altalune-id/othra/gen/go/member/v1/memberv1connect"
	orgv1connect "github.com/altalune-id/othra/gen/go/org/v1/orgv1connect"
	projectv1connect "github.com/altalune-id/othra/gen/go/project/v1/projectv1connect"
	resourceserverv1connect "github.com/altalune-id/othra/gen/go/resourceserver/v1/resourceserverv1connect"
	upstreamv1connect "github.com/altalune-id/othra/gen/go/upstream/v1/upstreamv1connect"
	webhookv1connect "github.com/altalune-id/othra/gen/go/webhook/v1/webhookv1connect"
	"github.com/altalune-id/othra/internal/apikey"
	"github.com/altalune-id/othra/internal/auth"
	"github.com/altalune-id/othra/internal/client"
	"github.com/altalune-id/othra/internal/enduser"
	"github.com/altalune-id/othra/internal/federation"
	"github.com/altalune-id/othra/internal/kernel/authn"
	"github.com/altalune-id/othra/internal/keys"
	"github.com/altalune-id/othra/internal/org"
	"github.com/altalune-id/othra/internal/org/invite"
	"github.com/altalune-id/othra/internal/org/member"
	"github.com/altalune-id/othra/internal/project"
	"github.com/altalune-id/othra/internal/resourceserver"
	"github.com/altalune-id/othra/internal/upstream"
	"github.com/altalune-id/othra/internal/user"
	"github.com/altalune-id/othra/internal/webhook"
)

// Options configures the optional parts of the controlplane surface.
type Options struct {
	OpenAPIEnabled   bool
	OpenAPIBasicAuth *BasicAuth
	Authn            authn.InterceptorOptions
	// Validator checks every request after authn; nil uses protovalidate's shared one.
	Validator *Validator
}

// Services are the domain services the controlplane exposes.
type Services struct {
	Orgs     *org.Service
	Projects *project.Service
	APIKeys  *apikey.Service
	Webhooks *webhook.Service
	Users    *user.Service
	Members  *member.Service
	Invites  *invite.Service
	Auth     *auth.Service
	Keys     *keys.Service
	Clients  *client.Service

	ResourceServers *resourceserver.Service
	Upstreams       *upstream.Service
	EndUsers        *enduser.Service
	Federation      *federation.Service
	// Account is the slices of account.v1 the admin end-user view reuses.
	Account AccountSlices
	Refs    Refs
	// FileUploads is true when the deployment has file storage.
	FileUploads bool
	// Passwords is true when console password sign-in is on.
	Passwords bool
}

// Refs turns the public ids every surface carries into ids and back, for every domain's handler.
type Refs interface {
	invite.OrgIDs
	project.OrgRefs
	apikey.Refs
	webhook.Refs
	member.Refs
	auth.PublicIDs
	keys.Refs
	client.Refs
	resourceserver.Refs
	upstream.Refs
}

// Server holds the wired Connect handlers.
type Server struct {
	Orgs     *org.Service
	Projects *project.Service
	APIKeys  *apikey.Service
	Webhooks *webhook.Service

	OrgSvc      *org.OrgService
	ProjectSvc  *project.ProjectService
	APIKeySvc   *apikey.APIKeyService
	WebhookSvc  *webhook.WebhookControlPlane
	AuthSvc     *auth.AuthService
	MemberSvc   *member.MemberService
	InviteSvc   *invite.InviteService
	KeySvc      *keys.SigningKeyService
	ClientSvc   *client.ClientService
	RSSvc       *resourceserver.ResourceServerService
	UpstreamSvc *upstream.UpstreamService
	EndUserSvc  *EndUserService

	OpenAPIEnabled   bool
	OpenAPIBasicAuth *BasicAuth
	authn            authn.InterceptorOptions
	validator        *Validator

	unexpected apperror.UnexpectedFunc
}

// New builds a Server.
func New(svcs Services, unexpected apperror.UnexpectedFunc, opts Options) *Server {
	s := &Server{
		Orgs:             svcs.Orgs,
		Projects:         svcs.Projects,
		APIKeys:          svcs.APIKeys,
		Webhooks:         svcs.Webhooks,
		OrgSvc:           org.NewControlPlane(svcs.Orgs),
		OpenAPIEnabled:   opts.OpenAPIEnabled,
		OpenAPIBasicAuth: opts.OpenAPIBasicAuth,
		authn:            opts.Authn,
		validator:        opts.Validator,
		unexpected:       unexpected,
	}
	if svcs.Projects != nil {
		s.ProjectSvc = project.NewControlPlane(svcs.Projects, svcs.Refs)
	}
	if svcs.APIKeys != nil {
		s.APIKeySvc = apikey.NewControlPlane(svcs.APIKeys, svcs.Refs)
	}
	if svcs.Webhooks != nil {
		s.WebhookSvc = webhook.NewControlPlane(svcs.Webhooks, svcs.Refs)
	}
	if svcs.Keys != nil {
		s.KeySvc = keys.NewControlPlane(svcs.Keys, svcs.Refs)
	}
	if svcs.Clients != nil {
		s.ClientSvc = client.NewControlPlane(svcs.Clients, svcs.Refs)
	}
	if svcs.ResourceServers != nil {
		s.RSSvc = resourceserver.NewControlPlane(svcs.ResourceServers, svcs.Refs)
	}
	if svcs.Upstreams != nil {
		s.UpstreamSvc = upstream.NewControlPlane(svcs.Upstreams, svcs.Refs)
	}
	if svcs.EndUsers != nil && svcs.Federation != nil && svcs.Account.Complete() {
		s.EndUserSvc = NewEndUserService(svcs.EndUsers, svcs.Federation, svcs.Account, svcs.Refs)
	}
	if svcs.Members != nil {
		s.MemberSvc = member.NewControlPlane(svcs.Members, svcs.Refs)
	}
	if svcs.Invites != nil {
		s.InviteSvc = invite.NewControlPlane(svcs.Invites, svcs.Refs)
	}
	if svcs.Orgs != nil && svcs.Projects != nil && svcs.Users != nil {
		s.AuthSvc = auth.NewControlPlane(svcs.Orgs, svcs.Projects, svcs.Users, svcs.Auth).
			WithFileUploads(svcs.FileUploads).WithPasswords(svcs.Passwords)
	}
	return s
}

// Handler mounts the Connect handlers and OpenAPI endpoints under basePath+"/api".
func (s *Server) Handler(basePath string) http.Handler {
	inner := http.NewServeMux()
	orgPath, orgHandler := orgv1connect.NewOrgServiceHandler(s.OrgSvc, s.handlerOptions()...)
	inner.Handle(orgPath, orgHandler)
	if s.ProjectSvc != nil {
		projectPath, projectHandler := projectv1connect.NewProjectServiceHandler(s.ProjectSvc, s.handlerOptions()...)
		inner.Handle(projectPath, projectHandler)
	}
	if s.APIKeySvc != nil {
		keyPath, keyHandler := apikeyv1connect.NewAPIKeyServiceHandler(s.APIKeySvc, s.handlerOptions()...)
		inner.Handle(keyPath, keyHandler)
	}
	if s.WebhookSvc != nil {
		hookPath, hookHandler := webhookv1connect.NewWebhookServiceHandler(s.WebhookSvc, s.handlerOptions()...)
		inner.Handle(hookPath, hookHandler)
	}
	if s.MemberSvc != nil {
		memberPath, memberHandler := memberv1connect.NewMemberServiceHandler(s.MemberSvc, s.handlerOptions()...)
		inner.Handle(memberPath, memberHandler)
	}
	if s.InviteSvc != nil {
		invitePath, inviteHandler := invitev1connect.NewInviteServiceHandler(s.InviteSvc, s.handlerOptions()...)
		inner.Handle(invitePath, inviteHandler)
	}
	if s.KeySvc != nil {
		keysPath, keysHandler := keysv1connect.NewSigningKeyServiceHandler(s.KeySvc, s.handlerOptions()...)
		inner.Handle(keysPath, keysHandler)
	}
	if s.ClientSvc != nil {
		clientPath, clientHandler := clientv1connect.NewClientServiceHandler(s.ClientSvc, s.handlerOptions()...)
		inner.Handle(clientPath, clientHandler)
	}
	if s.RSSvc != nil {
		rsPath, rsHandler := resourceserverv1connect.NewResourceServerServiceHandler(s.RSSvc, s.handlerOptions()...)
		inner.Handle(rsPath, rsHandler)
	}
	if s.UpstreamSvc != nil {
		upstreamPath, upstreamHandler := upstreamv1connect.NewUpstreamServiceHandler(s.UpstreamSvc, s.handlerOptions()...)
		inner.Handle(upstreamPath, upstreamHandler)
	}
	if s.EndUserSvc != nil {
		endUserPath, endUserHandler := enduserv1connect.NewEndUserServiceHandler(s.EndUserSvc, s.handlerOptions()...)
		inner.Handle(endUserPath, endUserHandler)
	}

	if s.AuthSvc != nil {
		authPath, authHandler := authv1connect.NewAuthServiceHandler(s.AuthSvc, s.handlerOptions()...)
		inner.Handle(authPath, authHandler)
	}

	if s.OpenAPIEnabled {
		yamlBody, jsonBody := openAPI()
		if len(yamlBody) > 0 {
			guard := openAPIGuard(s.OpenAPIBasicAuth)
			inner.Handle("/openapi.yaml", s.recoverHTTP(guard(openAPIHandler(yamlBody, "application/yaml"))))
			inner.Handle("/openapi.json", s.recoverHTTP(guard(openAPIHandler(jsonBody, "application/json"))))
			inner.Handle("/docs", s.recoverHTTP(guard(docsHandler(basePath+"/api/openapi.yaml"))))
		}
	}

	mount := basePath + "/api"
	outer := http.NewServeMux()
	outer.Handle(mount+"/", http.StripPrefix(mount, inner))
	return outer
}

// NOTE: no RPC takes more than a few KiB; uploads go to presigned storage URLs, never through Connect.
const maxRequestBytes = 1 << 20

func (s *Server) handlerOptions() []connect.HandlerOption {
	ics := []connect.Interceptor{
		RequestID(),
	}
	if otel, err := OTel(nil, nil); err == nil && otel != nil {
		ics = append(ics, otel)
	}
	ics = append(ics, Wrap(s.unexpected), authn.Interceptor(s.authn), tenantFromPrincipal(), s.validator.interceptor())
	return []connect.HandlerOption{
		connect.WithRecover(s.recoverPanic),
		connect.WithReadMaxBytes(maxRequestBytes),
		connect.WithInterceptors(ics...),
	}
}

func (s *Server) recoverPanic(ctx context.Context, _ connect.Spec, _ http.Header, p any) error {
	return Translate(ctx, fmt.Errorf("panic: %v", p), s.unexpected)
}

func (s *Server) recoverHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			p := recover()
			if p == nil {
				return
			}
			err := fmt.Errorf("panic: %v", p)
			if s.unexpected != nil {
				_ = s.unexpected(r.Context(), "api: unexpected", err)
			}
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}()
		next.ServeHTTP(w, r)
	})
}
