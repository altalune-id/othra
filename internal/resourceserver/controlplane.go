package resourceserver

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	clientv1 "altalune.id/othra/gen/go/client/v1"
	resourceserverv1 "altalune.id/othra/gen/go/resourceserver/v1"
	resourceserverv1connect "altalune.id/othra/gen/go/resourceserver/v1/resourceserverv1connect"
	"altalune.id/othra/internal/kernel/authz"
	"altalune.id/othra/internal/kernel/secret"
	"altalune.id/othra/internal/kernel/session"
)

// ResourceServerService is the ResourceServerService Connect handler.
type ResourceServerService struct {
	resourceserverv1connect.UnimplementedResourceServerServiceHandler
	svc  *Service
	refs Refs
}

var _ resourceserverv1connect.ResourceServerServiceHandler = (*ResourceServerService)(nil)

// NewControlPlane builds the ResourceServerService handler.
func NewControlPlane(svc *Service, refs Refs) *ResourceServerService {
	return &ResourceServerService{svc: svc, refs: refs}
}

// ListResourceServers lists a project's resource servers.
func (s *ResourceServerService) ListResourceServers(
	ctx context.Context, req *connect.Request[resourceserverv1.ListResourceServersRequest],
) (*connect.Response[resourceserverv1.ListResourceServersResponse], error) {
	orgID, projectID, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachProject)
	if err != nil {
		return nil, err
	}
	list, err := s.svc.List(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]*resourceserverv1.ResourceServer, 0, len(list))
	for _, r := range list {
		out = append(out, s.toProto(r))
	}
	return connect.NewResponse(&resourceserverv1.ListResourceServersResponse{
		ResourceServers: out, TrustAvailable: s.svc.TrustAvailable(),
	}), nil
}

// GetResourceServer returns one resource server.
func (s *ResourceServerService) GetResourceServer(
	ctx context.Context, req *connect.Request[resourceserverv1.GetResourceServerRequest],
) (*connect.Response[resourceserverv1.GetResourceServerResponse], error) {
	orgID, projectID, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachProject)
	if err != nil {
		return nil, err
	}
	r, err := s.svc.Get(ctx, orgID, projectID, req.Msg.GetResourceServerId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&resourceserverv1.GetResourceServerResponse{ResourceServer: s.toProto(r)}), nil
}

// CreateResourceServer registers a resource server.
func (s *ResourceServerService) CreateResourceServer(
	ctx context.Context, req *connect.Request[resourceserverv1.CreateResourceServerRequest],
) (*connect.Response[resourceserverv1.CreateResourceServerResponse], error) {
	orgID, projectID, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachWholeProject)
	if err != nil {
		return nil, err
	}
	r, err := s.svc.Create(ctx, CreateRequest{
		OrgID: orgID, ProjectID: projectID, Name: req.Msg.GetName(), Identifier: req.Msg.GetIdentifier(),
		Scopes: req.Msg.GetScopes(), Trusted: req.Msg.GetTrusted(), Person: person(ctx), Descriptions: req.Msg.GetScopeDescriptions(),
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&resourceserverv1.CreateResourceServerResponse{ResourceServer: s.toProto(r)}), nil
}

// UpdateResourceServer replaces a resource server's name and scopes.
func (s *ResourceServerService) UpdateResourceServer(
	ctx context.Context, req *connect.Request[resourceserverv1.UpdateResourceServerRequest],
) (*connect.Response[resourceserverv1.UpdateResourceServerResponse], error) {
	orgID, projectID, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachWholeProject)
	if err != nil {
		return nil, err
	}
	r, err := s.svc.Update(
		ctx, orgID, projectID, req.Msg.GetResourceServerId(), req.Msg.GetName(), req.Msg.GetScopes(), req.Msg.GetScopeDescriptions(),
	)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&resourceserverv1.UpdateResourceServerResponse{ResourceServer: s.toProto(r)}), nil
}

// DeleteResourceServer deletes a resource server no client is linked to.
func (s *ResourceServerService) DeleteResourceServer(
	ctx context.Context, req *connect.Request[resourceserverv1.DeleteResourceServerRequest],
) (*connect.Response[resourceserverv1.DeleteResourceServerResponse], error) {
	orgID, projectID, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachWholeProject)
	if err != nil {
		return nil, err
	}
	if err := s.svc.Delete(ctx, orgID, projectID, req.Msg.GetResourceServerId()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&resourceserverv1.DeleteResourceServerResponse{}), nil
}

// VerifyResourceServer checks a resource server's RFC 9728 metadata now.
func (s *ResourceServerService) VerifyResourceServer(
	ctx context.Context, req *connect.Request[resourceserverv1.VerifyResourceServerRequest],
) (*connect.Response[resourceserverv1.VerifyResourceServerResponse], error) {
	orgID, projectID, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachWholeProject)
	if err != nil {
		return nil, err
	}
	r, f, err := s.svc.Verify(ctx, orgID, projectID, req.Msg.GetResourceServerId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&resourceserverv1.VerifyResourceServerResponse{
		ResourceServer: s.toProto(r), Passed: f == nil, Error: failureToProto(f),
	}), nil
}

// SetResourceServerTrust turns trust without verification on or off; only a signed-in person may.
func (s *ResourceServerService) SetResourceServerTrust(
	ctx context.Context, req *connect.Request[resourceserverv1.SetResourceServerTrustRequest],
) (*connect.Response[resourceserverv1.SetResourceServerTrustResponse], error) {
	orgID, projectID, p, err := s.personProject(ctx, req.Msg.GetProjectId())
	if err != nil {
		return nil, err
	}
	r, err := s.svc.SetTrust(ctx, orgID, projectID, req.Msg.GetResourceServerId(), req.Msg.GetTrusted(), p)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&resourceserverv1.SetResourceServerTrustResponse{ResourceServer: s.toProto(r)}), nil
}

// SetResourceServerAdmission sets whether a resource server admits apps without registration (CIMD), from which hosts, with which scopes.
func (s *ResourceServerService) SetResourceServerAdmission(
	ctx context.Context, req *connect.Request[resourceserverv1.SetResourceServerAdmissionRequest],
) (*connect.Response[resourceserverv1.SetResourceServerAdmissionResponse], error) {
	orgID, projectID, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachWholeProject)
	if err != nil {
		return nil, err
	}
	a := Admission{Accept: req.Msg.GetAccept(), Hosts: req.Msg.GetHosts()}
	if !req.Msg.GetAllScopes() {
		a.Scopes = append([]string{}, req.Msg.GetScopes()...)
	}
	r, err := s.svc.SetAdmission(ctx, orgID, projectID, req.Msg.GetResourceServerId(), a)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&resourceserverv1.SetResourceServerAdmissionResponse{ResourceServer: s.toProto(r)}), nil
}

// SetPersonalTokenAccess turns personal tokens for a resource server on or off; only a signed-in person may.
func (s *ResourceServerService) SetPersonalTokenAccess(
	ctx context.Context, req *connect.Request[resourceserverv1.SetPersonalTokenAccessRequest],
) (*connect.Response[resourceserverv1.SetPersonalTokenAccessResponse], error) {
	orgID, projectID, _, err := s.personProject(ctx, req.Msg.GetProjectId())
	if err != nil {
		return nil, err
	}
	r, err := s.svc.SetPersonalTokenAccess(ctx, orgID, projectID, req.Msg.GetResourceServerId(), req.Msg.GetAccept())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&resourceserverv1.SetPersonalTokenAccessResponse{ResourceServer: s.toProto(r)}), nil
}

// GenerateIntrospectionSecret gives a resource server its first introspection secret, shown once; only a signed-in person may.
func (s *ResourceServerService) GenerateIntrospectionSecret(
	ctx context.Context, req *connect.Request[resourceserverv1.GenerateIntrospectionSecretRequest],
) (*connect.Response[resourceserverv1.GenerateIntrospectionSecretResponse], error) {
	orgID, projectID, _, err := s.personProject(ctx, req.Msg.GetProjectId())
	if err != nil {
		return nil, err
	}
	r, plaintext, err := s.svc.GenerateIntrospectionSecret(ctx, orgID, projectID, req.Msg.GetResourceServerId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&resourceserverv1.GenerateIntrospectionSecretResponse{ResourceServer: s.toProto(r), Secret: plaintext}), nil
}

// RotateIntrospectionSecret replaces a resource server's introspection secret, keeping the old one for the grace; only a person may.
func (s *ResourceServerService) RotateIntrospectionSecret(
	ctx context.Context, req *connect.Request[resourceserverv1.RotateIntrospectionSecretRequest],
) (*connect.Response[resourceserverv1.RotateIntrospectionSecretResponse], error) {
	orgID, projectID, _, err := s.personProject(ctx, req.Msg.GetProjectId())
	if err != nil {
		return nil, err
	}
	grace := graceFromProto(req.Msg.GetGrace())
	r, plaintext, err := s.svc.RotateIntrospectionSecret(ctx, orgID, projectID, req.Msg.GetResourceServerId(), grace)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&resourceserverv1.RotateIntrospectionSecretResponse{ResourceServer: s.toProto(r), Secret: plaintext}), nil
}

// RevokeIntrospectionSecret ends one of a resource server's introspection secrets at once; only a signed-in person may.
func (s *ResourceServerService) RevokeIntrospectionSecret(
	ctx context.Context, req *connect.Request[resourceserverv1.RevokeIntrospectionSecretRequest],
) (*connect.Response[resourceserverv1.RevokeIntrospectionSecretResponse], error) {
	orgID, projectID, _, err := s.personProject(ctx, req.Msg.GetProjectId())
	if err != nil {
		return nil, err
	}
	r, err := s.svc.RevokeIntrospectionSecret(ctx, orgID, projectID, req.Msg.GetResourceServerId(), req.Msg.GetSecretId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&resourceserverv1.RevokeIntrospectionSecretResponse{ResourceServer: s.toProto(r)}), nil
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

// ListLinks lists the links of one client or of one resource server.
func (s *ResourceServerService) ListLinks(
	ctx context.Context, req *connect.Request[resourceserverv1.ListLinksRequest],
) (*connect.Response[resourceserverv1.ListLinksResponse], error) {
	orgID, projectID, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachProject)
	if err != nil {
		return nil, err
	}
	links, err := s.svc.ListLinks(ctx, orgID, projectID, req.Msg.GetClientId(), req.Msg.GetResourceServerId())
	if err != nil {
		return nil, err
	}
	out := make([]*resourceserverv1.Link, 0, len(links))
	for _, l := range links {
		out = append(out, linkToProto(l))
	}
	return connect.NewResponse(&resourceserverv1.ListLinksResponse{Links: out}), nil
}

// LinkClient links a client to a resource server with some of its scopes.
func (s *ResourceServerService) LinkClient(
	ctx context.Context, req *connect.Request[resourceserverv1.LinkClientRequest],
) (*connect.Response[resourceserverv1.LinkClientResponse], error) {
	orgID, projectID, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachWholeProject)
	if err != nil {
		return nil, err
	}
	l, err := s.svc.LinkClient(ctx, orgID, projectID, req.Msg.GetClientId(), req.Msg.GetResourceServerId(), req.Msg.GetScopes())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&resourceserverv1.LinkClientResponse{Link: linkToProto(l)}), nil
}

// UpdateLink replaces the scopes a link allows.
func (s *ResourceServerService) UpdateLink(
	ctx context.Context, req *connect.Request[resourceserverv1.UpdateLinkRequest],
) (*connect.Response[resourceserverv1.UpdateLinkResponse], error) {
	orgID, projectID, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachWholeProject)
	if err != nil {
		return nil, err
	}
	l, err := s.svc.UpdateLink(ctx, orgID, projectID, req.Msg.GetClientId(), req.Msg.GetResourceServerId(), req.Msg.GetScopes())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&resourceserverv1.UpdateLinkResponse{Link: linkToProto(l)}), nil
}

// UnlinkClient removes a link.
func (s *ResourceServerService) UnlinkClient(
	ctx context.Context, req *connect.Request[resourceserverv1.UnlinkClientRequest],
) (*connect.Response[resourceserverv1.UnlinkClientResponse], error) {
	orgID, projectID, err := s.project(ctx, req.Msg.GetProjectId(), authz.ReachWholeProject)
	if err != nil {
		return nil, err
	}
	if err := s.svc.UnlinkClient(ctx, orgID, projectID, req.Msg.GetClientId(), req.Msg.GetResourceServerId()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&resourceserverv1.UnlinkClientResponse{}), nil
}

// SECURITY: an unknown project and a project outside the caller's reach answer the same not-found.
func (s *ResourceServerService) project(
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

// SECURITY: trust and introspection secrets are a signed-in person's acts, never an API key's (as for client secrets).
func (s *ResourceServerService) personProject(ctx context.Context, publicID string) (orgID, projectID, p uuid.UUID, err error) {
	if p = person(ctx); p == uuid.Nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, &authz.ForbiddenError{Permission: authz.PersonOnly}
	}
	orgID, projectID, err = s.project(ctx, publicID, authz.ReachWholeProject)
	return orgID, projectID, p, err
}

func person(ctx context.Context) uuid.UUID {
	if p := session.PrincipalFrom(ctx); p.IsPerson() {
		return p.UserID
	}
	return uuid.Nil
}

func (s *ResourceServerService) toProto(r *ResourceServer) *resourceserverv1.ResourceServer {
	out := &resourceserverv1.ResourceServer{
		Id: r.PublicID, Name: r.Name, Identifier: r.Identifier, Scopes: r.Scopes, State: stateToProto(r.State),
		Trusted: r.Trusted, VerifiedAt: ts(r.VerifiedAt), LastCheckedAt: ts(r.LastCheckedAt), LastError: failureToProto(r.LastError),
		ConsecutiveFailures: r.ConsecutiveFailures, TrustAvailable: s.svc.TrustAvailable(),
		CreatedAt: timestamppb.New(r.CreatedAt), UpdatedAt: timestamppb.New(r.UpdatedAt), ScopeDescriptions: r.Descriptions,
		Cimd: &resourceserverv1.CIMDAdmission{
			Accept: r.Admission.Accept, Hosts: r.Admission.Hosts, AllScopes: r.Admission.Scopes == nil, Scopes: r.Admission.Scopes,
		},
		AcceptPersonalTokens: r.AcceptPersonalTokens,
	}
	for _, st := range r.IntrospectionSecrets {
		out.IntrospectionSecrets = append(out.IntrospectionSecrets, &clientv1.ClientSecret{
			Id: st.PublicID, Hint: st.Hint, CreatedAt: timestamppb.New(st.CreatedAt), ExpiresAt: ts(st.ExpiresAt),
		})
	}
	if !IsLoopbackHTTP(r.Identifier) {
		out.MetadataUrl = MetadataURL(r.Identifier)
	}
	return out
}

func linkToProto(l *Link) *resourceserverv1.Link {
	return &resourceserverv1.Link{
		ClientId: l.Client.PublicID, ClientName: l.Client.Name, ResourceServerId: l.ResourceServer.PublicID,
		ResourceServerName: l.ResourceServer.Name, ResourceServerIdentifier: l.ResourceServer.Identifier,
		ResourceServerState: stateToProto(l.ResourceServer.State), ResourceServerTrusted: l.ResourceServer.Trusted,
		Scopes: l.Scopes, CreatedAt: timestamppb.New(l.CreatedAt), ClientMetadataUrl: l.Client.MetadataURL,
		MetadataFetchedAt: ts(l.Client.MetadataFetchedAt),
	}
}

func failureToProto(f *Failure) *resourceserverv1.VerificationFailure {
	if f == nil {
		return nil
	}
	return &resourceserverv1.VerificationFailure{
		Reason: string(f.Reason), Status: f.Status, Resource: f.Resource, CheckedAt: timestamppb.New(f.CheckedAt),
	}
}

func stateToProto(s State) resourceserverv1.ResourceServerState {
	switch s {
	case StatePending:
		return resourceserverv1.ResourceServerState_RESOURCE_SERVER_STATE_PENDING
	case StateVerified:
		return resourceserverv1.ResourceServerState_RESOURCE_SERVER_STATE_VERIFIED
	case StateFailed:
		return resourceserverv1.ResourceServerState_RESOURCE_SERVER_STATE_FAILED
	}
	return resourceserverv1.ResourceServerState_RESOURCE_SERVER_STATE_UNSPECIFIED
}

func ts(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}
