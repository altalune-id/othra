package upstream

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	accountv1 "altalune.id/othra/gen/go/account/v1"
	upstreamv1 "altalune.id/othra/gen/go/upstream/v1"
	upstreamv1connect "altalune.id/othra/gen/go/upstream/v1/upstreamv1connect"
	"altalune.id/othra/internal/kernel/authz"
	"altalune.id/othra/internal/kernel/session"
)

// UpstreamService is the UpstreamService Connect handler.
type UpstreamService struct {
	upstreamv1connect.UnimplementedUpstreamServiceHandler
	svc  *Service
	refs Refs
}

var _ upstreamv1connect.UpstreamServiceHandler = (*UpstreamService)(nil)

// NewControlPlane builds the UpstreamService handler.
func NewControlPlane(svc *Service, refs Refs) *UpstreamService {
	return &UpstreamService{svc: svc, refs: refs}
}

// ListUpstreams lists a project's upstream IdPs.
func (s *UpstreamService) ListUpstreams(
	ctx context.Context, req *connect.Request[upstreamv1.ListUpstreamsRequest],
) (*connect.Response[upstreamv1.ListUpstreamsResponse], error) {
	orgID, projectID, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachProject)
	if err != nil {
		return nil, err
	}
	list, err := s.svc.List(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]*upstreamv1.Upstream, 0, len(list))
	for _, u := range list {
		p, err := s.toProto(ctx, u)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return connect.NewResponse(&upstreamv1.ListUpstreamsResponse{Upstreams: out}), nil
}

// GetUpstream returns one upstream IdP.
func (s *UpstreamService) GetUpstream(
	ctx context.Context, req *connect.Request[upstreamv1.GetUpstreamRequest],
) (*connect.Response[upstreamv1.GetUpstreamResponse], error) {
	orgID, projectID, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachProject)
	if err != nil {
		return nil, err
	}
	u, err := s.svc.Get(ctx, orgID, projectID, req.Msg.GetUpstreamId())
	if err != nil {
		return nil, err
	}
	p, err := s.toProto(ctx, u)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&upstreamv1.GetUpstreamResponse{Upstream: p}), nil
}

// CreateUpstream sets up an upstream IdP; only a signed-in person may, as it carries a client secret.
func (s *UpstreamService) CreateUpstream(
	ctx context.Context, req *connect.Request[upstreamv1.CreateUpstreamRequest],
) (*connect.Response[upstreamv1.CreateUpstreamResponse], error) {
	orgID, projectID, err := s.personProject(ctx, req.Msg.GetProjectId())
	if err != nil {
		return nil, err
	}
	u, err := s.svc.Create(ctx, orgID, projectID, CreateRequest{
		Kind: kindOf(req.Msg.GetKind()), ClientID: req.Msg.GetClientId(), ClientSecret: req.Msg.GetClientSecret(), Enabled: req.Msg.GetEnabled(),
	})
	if err != nil {
		return nil, err
	}
	p, err := s.toProto(ctx, u)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&upstreamv1.CreateUpstreamResponse{Upstream: p}), nil
}

// UpdateUpstream replaces an upstream IdP's client id, and its secret when one is sent; only a signed-in person may.
func (s *UpstreamService) UpdateUpstream(
	ctx context.Context, req *connect.Request[upstreamv1.UpdateUpstreamRequest],
) (*connect.Response[upstreamv1.UpdateUpstreamResponse], error) {
	orgID, projectID, err := s.personProject(ctx, req.Msg.GetProjectId())
	if err != nil {
		return nil, err
	}
	u, err := s.svc.Update(ctx, orgID, projectID, req.Msg.GetUpstreamId(), req.Msg.GetClientId(), req.Msg.GetClientSecret())
	if err != nil {
		return nil, err
	}
	p, err := s.toProto(ctx, u)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&upstreamv1.UpdateUpstreamResponse{Upstream: p}), nil
}

// SetUpstreamEnabled turns an upstream IdP on or off.
func (s *UpstreamService) SetUpstreamEnabled(
	ctx context.Context, req *connect.Request[upstreamv1.SetUpstreamEnabledRequest],
) (*connect.Response[upstreamv1.SetUpstreamEnabledResponse], error) {
	orgID, projectID, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachWholeProject)
	if err != nil {
		return nil, err
	}
	u, err := s.svc.SetEnabled(ctx, orgID, projectID, req.Msg.GetUpstreamId(), req.Msg.GetEnabled())
	if err != nil {
		return nil, err
	}
	p, err := s.toProto(ctx, u)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&upstreamv1.SetUpstreamEnabledResponse{Upstream: p}), nil
}

// DeleteUpstream removes an upstream IdP no end user has linked.
func (s *UpstreamService) DeleteUpstream(
	ctx context.Context, req *connect.Request[upstreamv1.DeleteUpstreamRequest],
) (*connect.Response[upstreamv1.DeleteUpstreamResponse], error) {
	orgID, projectID, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachWholeProject)
	if err != nil {
		return nil, err
	}
	if err := s.svc.Delete(ctx, orgID, projectID, req.Msg.GetUpstreamId()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&upstreamv1.DeleteUpstreamResponse{}), nil
}

// SECURITY: a client secret is written only by a signed-in person, never by an API key, as for OAuth client secrets.
func (s *UpstreamService) personProject(ctx context.Context, publicID string) (orgID, projectID uuid.UUID, err error) {
	if p := session.PrincipalFrom(ctx); !p.IsPerson() {
		return uuid.Nil, uuid.Nil, &authz.ForbiddenError{Permission: authz.PersonOnly}
	}
	return s.project(ctx, publicID, authz.ReachWholeProject)
}

// SECURITY: an unknown project and a project outside the caller's reach answer the same not-found.
func (s *UpstreamService) project(
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

func kindOf(k upstreamv1.UpstreamKind) Kind {
	if k == upstreamv1.UpstreamKind_UPSTREAM_KIND_GOOGLE {
		return KindGoogle
	}
	return ""
}

func (s *UpstreamService) toProto(ctx context.Context, u *Upstream) (*upstreamv1.Upstream, error) {
	redirect, err := s.svc.RedirectURI(ctx, u.OrgID, u.ProjectID, u.Kind)
	if err != nil {
		return nil, err
	}
	kind := upstreamv1.UpstreamKind_UPSTREAM_KIND_UNSPECIFIED
	if u.Kind == KindGoogle {
		kind = upstreamv1.UpstreamKind_UPSTREAM_KIND_GOOGLE
	}
	return &upstreamv1.Upstream{
		Id: u.PublicID, Kind: kind, ClientId: u.ClientID, SecretSet: true, Enabled: u.Enabled, RedirectUri: redirect,
		IdentitiesLinked: u.IdentitiesLinked, CreatedAt: timestamppb.New(u.CreatedAt), UpdatedAt: timestamppb.New(u.UpdatedAt),
	}, nil
}

// MethodProto is the account.v1 sign-in method of an upstream IdP of kind.
func MethodProto(k Kind) accountv1.SignInMethodKind {
	if k == KindGoogle {
		return accountv1.SignInMethodKind_SIGN_IN_METHOD_KIND_GOOGLE
	}
	return accountv1.SignInMethodKind_SIGN_IN_METHOD_KIND_UNSPECIFIED
}
