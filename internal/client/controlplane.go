package client

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	clientv1 "altalune.id/othra/gen/go/client/v1"
	clientv1connect "altalune.id/othra/gen/go/client/v1/clientv1connect"
	"altalune.id/othra/internal/kernel/authz"
	"altalune.id/othra/internal/kernel/session"
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
	orgID, projectID, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachProject)
	if err != nil {
		return nil, err
	}
	list, err := s.clients.List(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]*clientv1.Client, 0, len(list))
	for _, c := range list {
		out = append(out, toProto(c))
	}
	return connect.NewResponse(&clientv1.ListClientsResponse{Clients: out}), nil
}

// GetClient loads one client by its client_id.
func (s *ClientService) GetClient(
	ctx context.Context, req *connect.Request[clientv1.GetClientRequest],
) (*connect.Response[clientv1.GetClientResponse], error) {
	orgID, projectID, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachProject)
	if err != nil {
		return nil, err
	}
	c, err := s.clients.Get(ctx, orgID, projectID, req.Msg.GetClientId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&clientv1.GetClientResponse{Client: toProto(c)}), nil
}

// CreateClient registers a client; it never returns a secret.
func (s *ClientService) CreateClient(
	ctx context.Context, req *connect.Request[clientv1.CreateClientRequest],
) (*connect.Response[clientv1.CreateClientResponse], error) {
	orgID, projectID, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachWholeProject)
	if err != nil {
		return nil, err
	}
	kind := KindWeb
	if req.Msg.GetKind() == clientv1.ClientKind_CLIENT_KIND_CLI {
		kind = KindCLI
	}
	c, err := s.clients.Create(ctx, orgID, projectID, kind, Settings{
		Name:                   req.Msg.GetName(),
		RedirectURIs:           req.Msg.GetRedirectUris(),
		PostLogoutRedirectURIs: req.Msg.GetPostLogoutRedirectUris(),
		RefreshTokens:          req.Msg.GetRefreshTokens(),
		ConsentPolicy:          consentFromProto(req.Msg.GetConsentPolicy()),
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&clientv1.CreateClientResponse{Client: toProto(c)}), nil
}

// UpdateClient replaces a client's settings.
func (s *ClientService) UpdateClient(
	ctx context.Context, req *connect.Request[clientv1.UpdateClientRequest],
) (*connect.Response[clientv1.UpdateClientResponse], error) {
	orgID, projectID, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachWholeProject)
	if err != nil {
		return nil, err
	}
	c, err := s.clients.Update(ctx, orgID, projectID, req.Msg.GetClientId(), Settings{
		Name:                   req.Msg.GetName(),
		RedirectURIs:           req.Msg.GetRedirectUris(),
		PostLogoutRedirectURIs: req.Msg.GetPostLogoutRedirectUris(),
		RefreshTokens:          req.Msg.GetRefreshTokens(),
		ConsentPolicy:          consentFromProto(req.Msg.GetConsentPolicy()),
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&clientv1.UpdateClientResponse{Client: toProto(c)}), nil
}

// DeleteClient removes a client with its secrets and links.
func (s *ClientService) DeleteClient(
	ctx context.Context, req *connect.Request[clientv1.DeleteClientRequest],
) (*connect.Response[clientv1.DeleteClientResponse], error) {
	orgID, projectID, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachWholeProject)
	if err != nil {
		return nil, err
	}
	if err := s.clients.Delete(ctx, orgID, projectID, req.Msg.GetClientId()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&clientv1.DeleteClientResponse{}), nil
}

// GenerateClientSecret gives a web client without a current secret a new one, shown once.
func (s *ClientService) GenerateClientSecret(
	ctx context.Context, req *connect.Request[clientv1.GenerateClientSecretRequest],
) (*connect.Response[clientv1.GenerateClientSecretResponse], error) {
	orgID, projectID, err := s.personProject(ctx, req.Msg.GetProjectId())
	if err != nil {
		return nil, err
	}
	c, secret, err := s.clients.GenerateSecret(ctx, orgID, projectID, req.Msg.GetClientId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&clientv1.GenerateClientSecretResponse{Client: toProto(c), Secret: secret}), nil
}

// RotateClientSecret makes a new current secret, shown once; the old one keeps working for the grace.
func (s *ClientService) RotateClientSecret(
	ctx context.Context, req *connect.Request[clientv1.RotateClientSecretRequest],
) (*connect.Response[clientv1.RotateClientSecretResponse], error) {
	orgID, projectID, err := s.personProject(ctx, req.Msg.GetProjectId())
	if err != nil {
		return nil, err
	}
	c, secret, err := s.clients.RotateSecret(ctx, orgID, projectID, req.Msg.GetClientId(), graceFromProto(req.Msg.GetGrace()))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&clientv1.RotateClientSecretResponse{Client: toProto(c), Secret: secret}), nil
}

// RevokeClientSecret ends one of a client's secrets at once.
func (s *ClientService) RevokeClientSecret(
	ctx context.Context, req *connect.Request[clientv1.RevokeClientSecretRequest],
) (*connect.Response[clientv1.RevokeClientSecretResponse], error) {
	orgID, projectID, err := s.personProject(ctx, req.Msg.GetProjectId())
	if err != nil {
		return nil, err
	}
	c, err := s.clients.RevokeSecret(ctx, orgID, projectID, req.Msg.GetClientId(), req.Msg.GetSecretId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&clientv1.RevokeClientSecretResponse{Client: toProto(c)}), nil
}

// SECURITY: secrets are created, rotated and revoked by a signed-in person only, never a key or the system.
func (s *ClientService) personProject(ctx context.Context, publicID string) (orgID, projectID uuid.UUID, err error) {
	orgID, projectID, err = s.project(ctx, publicID, authz.ReachWholeProject)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if !session.PrincipalFrom(ctx).IsPerson() {
		return uuid.Nil, uuid.Nil, &SecretPersonOnlyError{}
	}
	return orgID, projectID, nil
}

// SECURITY: an unknown project and a project outside the caller's reach answer the same not-found.
func (s *ClientService) project(
	ctx context.Context, publicID string, reach func(ctx context.Context, orgID, projectID uuid.UUID) bool,
) (orgID, projectID uuid.UUID, err error) {
	projectID, orgID, ok, err := s.refs.ProjectID(ctx, publicID)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if !ok || !reach(ctx, orgID, projectID) {
		return uuid.Nil, uuid.Nil, &ProjectNotFoundError{}
	}
	return orgID, projectID, nil
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
	return &clientv1.Client{
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
		CreatedAt:               timestamppb.New(c.CreatedAt),
		UpdatedAt:               timestamppb.New(c.UpdatedAt),
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
		return 0
	case clientv1.SecretGrace_SECRET_GRACE_HOUR:
		return time.Hour
	case clientv1.SecretGrace_SECRET_GRACE_WEEK:
		return MaxGrace
	default:
		return 24 * time.Hour
	}
}
