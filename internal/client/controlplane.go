package client

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	clientv1 "github.com/altalune-id/othra/gen/go/client/v1"
	clientv1connect "github.com/altalune-id/othra/gen/go/client/v1/clientv1connect"
	"github.com/altalune-id/othra/internal/kernel/authz"
	"github.com/altalune-id/othra/internal/kernel/secret"
	"github.com/altalune-id/othra/internal/kernel/session"
)

// ClientService is the ClientService Connect handler.
type ClientService struct {
	clientv1connect.UnimplementedClientServiceHandler
	clients *Service
	refs    Refs
}

var _ clientv1connect.ClientServiceHandler = (*ClientService)(nil)

// NewControlPlane builds the ClientService handler.
func NewControlPlane(clients *Service, refs Refs) *ClientService {
	return &ClientService{clients: clients, refs: refs}
}

// ListClients lists a project's clients, oldest first.
func (s *ClientService) ListClients(
	ctx context.Context, req *connect.Request[clientv1.ListClientsRequest],
) (*connect.Response[clientv1.ListClientsResponse], error) {
	p, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachProject)
	if err != nil {
		return nil, err
	}
	list, err := s.clients.List(ctx, p.OrgID, p.ID)
	if err != nil {
		return nil, err
	}
	out := make([]*clientv1.Client, 0, len(list))
	for _, c := range list {
		out = append(out, p.result(c))
	}
	return connect.NewResponse(&clientv1.ListClientsResponse{Clients: out}), nil
}

// GetClient loads one client by its client_id.
func (s *ClientService) GetClient(
	ctx context.Context, req *connect.Request[clientv1.GetClientRequest],
) (*connect.Response[clientv1.GetClientResponse], error) {
	p, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachProject)
	if err != nil {
		return nil, err
	}
	c, err := s.clients.Get(ctx, p.OrgID, p.ID, req.Msg.GetClientId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&clientv1.GetClientResponse{Client: p.result(c)}), nil
}

// CreateClient registers a client; it never returns a secret.
func (s *ClientService) CreateClient(
	ctx context.Context, req *connect.Request[clientv1.CreateClientRequest],
) (*connect.Response[clientv1.CreateClientResponse], error) {
	p, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachWholeProject)
	if err != nil {
		return nil, err
	}
	kind := KindWeb
	if req.Msg.GetKind() == clientv1.ClientKind_CLIENT_KIND_CLI {
		kind = KindCLI
	}
	c, err := s.clients.Create(ctx, p.OrgID, p.ID, kind, Settings{
		Name:                   req.Msg.GetName(),
		RedirectURIs:           req.Msg.GetRedirectUris(),
		PostLogoutRedirectURIs: req.Msg.GetPostLogoutRedirectUris(),
		RefreshTokens:          req.Msg.GetRefreshTokens(),
		ConsentPolicy:          consentFromProto(req.Msg.GetConsentPolicy()),
		AuthMethod:             authMethodFromProto(req.Msg.GetAuthMethod()),
		PKCERequired:           req.Msg.PkceRequired,
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&clientv1.CreateClientResponse{Client: p.result(c)}), nil
}

// UpdateClient replaces a client's settings.
func (s *ClientService) UpdateClient(
	ctx context.Context, req *connect.Request[clientv1.UpdateClientRequest],
) (*connect.Response[clientv1.UpdateClientResponse], error) {
	p, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachWholeProject)
	if err != nil {
		return nil, err
	}
	c, err := s.clients.Update(ctx, p.OrgID, p.ID, req.Msg.GetClientId(), Settings{
		Name:                   req.Msg.GetName(),
		RedirectURIs:           req.Msg.GetRedirectUris(),
		PostLogoutRedirectURIs: req.Msg.GetPostLogoutRedirectUris(),
		RefreshTokens:          req.Msg.GetRefreshTokens(),
		ConsentPolicy:          consentFromProto(req.Msg.GetConsentPolicy()),
		AuthMethod:             authMethodFromProto(req.Msg.GetAuthMethod()),
		PKCERequired:           req.Msg.PkceRequired,
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&clientv1.UpdateClientResponse{Client: p.result(c)}), nil
}

// DeleteClient removes a client with its secrets and links.
func (s *ClientService) DeleteClient(
	ctx context.Context, req *connect.Request[clientv1.DeleteClientRequest],
) (*connect.Response[clientv1.DeleteClientResponse], error) {
	p, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachWholeProject)
	if err != nil {
		return nil, err
	}
	if err := s.clients.Delete(ctx, p.OrgID, p.ID, req.Msg.GetClientId()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&clientv1.DeleteClientResponse{}), nil
}

// GenerateClientSecret gives a web client without a current secret a new one, shown once.
func (s *ClientService) GenerateClientSecret(
	ctx context.Context, req *connect.Request[clientv1.GenerateClientSecretRequest],
) (*connect.Response[clientv1.GenerateClientSecretResponse], error) {
	p, err := s.personProject(ctx, req.Msg.GetProjectId())
	if err != nil {
		return nil, err
	}
	c, plaintext, err := s.clients.GenerateSecret(ctx, p.OrgID, p.ID, req.Msg.GetClientId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&clientv1.GenerateClientSecretResponse{Client: p.result(c), Secret: plaintext}), nil
}

// RotateClientSecret makes a new current secret, shown once; the old one keeps working for the grace.
func (s *ClientService) RotateClientSecret(
	ctx context.Context, req *connect.Request[clientv1.RotateClientSecretRequest],
) (*connect.Response[clientv1.RotateClientSecretResponse], error) {
	p, err := s.personProject(ctx, req.Msg.GetProjectId())
	if err != nil {
		return nil, err
	}
	c, plaintext, err := s.clients.RotateSecret(ctx, p.OrgID, p.ID, req.Msg.GetClientId(), graceFromProto(req.Msg.GetGrace()))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&clientv1.RotateClientSecretResponse{Client: p.result(c), Secret: plaintext}), nil
}

// RevokeClientSecret ends one of a client's secrets at once.
func (s *ClientService) RevokeClientSecret(
	ctx context.Context, req *connect.Request[clientv1.RevokeClientSecretRequest],
) (*connect.Response[clientv1.RevokeClientSecretResponse], error) {
	p, err := s.personProject(ctx, req.Msg.GetProjectId())
	if err != nil {
		return nil, err
	}
	c, err := s.clients.RevokeSecret(ctx, p.OrgID, p.ID, req.Msg.GetClientId(), req.Msg.GetSecretId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&clientv1.RevokeClientSecretResponse{Client: p.result(c)}), nil
}

// SECURITY: secrets are created, rotated and revoked by a signed-in person only, never a key or the system.
func (s *ClientService) personProject(ctx context.Context, publicID string) (ProjectRef, error) {
	p, err := s.project(ctx, publicID, authz.ReachWholeProject)
	if err != nil {
		return ProjectRef{}, err
	}
	if !session.PrincipalFrom(ctx).IsPerson() {
		return ProjectRef{}, &SecretPersonOnlyError{}
	}
	return p, nil
}

// SECURITY: an unknown project and a project outside the caller's reach answer the same not-found.
func (s *ClientService) project(
	ctx context.Context, publicID string, reach func(ctx context.Context, orgID, projectID uuid.UUID) bool,
) (ProjectRef, error) {
	p, ok, err := s.refs.ClientProject(ctx, publicID)
	if err != nil {
		return ProjectRef{}, err
	}
	if !ok || !reach(ctx, p.OrgID, p.ID) {
		return ProjectRef{}, &ProjectNotFoundError{}
	}
	return p, nil
}

func (p ProjectRef) result(c *Client) *clientv1.Client {
	out := toProto(c)
	out.ConsoleUrl = p.ConsoleURL(c.PublicID)
	return out
}

func toProto(c *Client) *clientv1.Client {
	kind := clientv1.ClientKind_CLIENT_KIND_WEB
	if c.Kind() == KindCLI {
		kind = clientv1.ClientKind_CLIENT_KIND_CLI
	}
	consent := clientv1.ConsentPolicy_CONSENT_POLICY_REQUIRE
	if c.ConsentPolicy == ConsentImplicit {
		consent = clientv1.ConsentPolicy_CONSENT_POLICY_IMPLICIT
	}
	secrets := make([]*clientv1.ClientSecret, 0, len(c.Secrets))
	for _, sec := range c.Secrets {
		out := &clientv1.ClientSecret{Id: sec.PublicID, Hint: sec.Hint, CreatedAt: timestamppb.New(sec.CreatedAt)}
		if sec.ExpiresAt != nil {
			out.ExpiresAt = timestamppb.New(*sec.ExpiresAt)
		}
		secrets = append(secrets, out)
	}
	out := &clientv1.Client{
		Id:                      c.PublicID,
		Name:                    c.Name,
		Kind:                    kind,
		RedirectUris:            c.RedirectURIs,
		PostLogoutRedirectUris:  c.PostLogoutRedirectURIs,
		RefreshTokens:           c.RefreshTokens(),
		ConsentPolicy:           consent,
		GrantTypes:              c.GrantTypes,
		AllowedScopes:           c.AllowedScopes,
		TokenEndpointAuthMethod: c.TokenEndpointAuthMethod,
		Secrets:                 secrets,
		IssuerHandleLocked:      true,
		PkceRequired:            c.PKCERequired,
		CreatedAt:               timestamppb.New(c.CreatedAt),
		UpdatedAt:               timestamppb.New(c.UpdatedAt),
		Registration:            clientv1.ClientRegistration_CLIENT_REGISTRATION_REGISTERED,
	}
	if c.System {
		out.Registration = clientv1.ClientRegistration_CLIENT_REGISTRATION_SYSTEM
	}
	if c.Managed() {
		out.Registration = clientv1.ClientRegistration_CLIENT_REGISTRATION_CIMD
		out.MetadataUrl = c.MetadataURL
		if c.MetadataFetchedAt != nil {
			out.MetadataFetchedAt = timestamppb.New(*c.MetadataFetchedAt)
		}
	}
	return out
}

func authMethodFromProto(m clientv1.AuthMethod) string {
	switch m {
	case clientv1.AuthMethod_AUTH_METHOD_CLIENT_SECRET_BASIC:
		return AuthClientSecretBasic
	case clientv1.AuthMethod_AUTH_METHOD_CLIENT_SECRET_POST:
		return AuthClientSecretPost
	default:
		return ""
	}
}

func consentFromProto(p clientv1.ConsentPolicy) ConsentPolicy {
	if p == clientv1.ConsentPolicy_CONSENT_POLICY_IMPLICIT {
		return ConsentImplicit
	}
	return ConsentRequire
}

func graceFromProto(g clientv1.SecretGrace) time.Duration {
	switch g {
	case clientv1.SecretGrace_SECRET_GRACE_NOW:
		return secret.GraceNow
	case clientv1.SecretGrace_SECRET_GRACE_HOUR:
		return secret.GraceHour
	case clientv1.SecretGrace_SECRET_GRACE_WEEK:
		return secret.MaxGrace
	default:
		return secret.GraceDay
	}
}
