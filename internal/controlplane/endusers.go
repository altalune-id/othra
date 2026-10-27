package controlplane

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	accountv1 "altalune.id/othra/gen/go/account/v1"
	enduserv1 "altalune.id/othra/gen/go/enduser/v1"
	"altalune.id/othra/gen/go/enduser/v1/enduserv1connect"
	"altalune.id/othra/internal/consent"
	"altalune.id/othra/internal/enduser"
	"altalune.id/othra/internal/federation"
	"altalune.id/othra/internal/kernel/authz"
	"altalune.id/othra/internal/pat"
	"altalune.id/othra/internal/project"
)

// ProjectRefs turns a project public id into its id and org.
type ProjectRefs interface {
	ProjectID(ctx context.Context, publicID string) (id, orgID uuid.UUID, ok bool, err error)
}

// EndUsers is what the admin view reads and ends of a project's end users; enduser.Service is it.
type EndUsers interface {
	ByPublicID(ctx context.Context, p enduser.Project, publicID string) (*enduser.EndUser, error)
	ListEndUsers(ctx context.Context, p enduser.Project, query, after string, limit int) ([]*enduser.EndUser, string, error)
	Sessions(ctx context.Context, p enduser.Project, endUserID uuid.UUID) ([]enduser.LiveSession, error)
	EndUserSession(ctx context.Context, p enduser.Project, endUserID uuid.UUID, publicID string) error
}

// SignInMethods lists an end user's sign-in methods; federation.Service is it.
type SignInMethods interface {
	SignInMethods(ctx context.Context, p federation.Project, endUserID uuid.UUID) ([]federation.SignInMethod, error)
}

// EndUserService is the admin view of a project's end users: read and revoke, through the same services as the portal.
type EndUserService struct {
	enduserv1connect.UnimplementedEndUserServiceHandler
	users   EndUsers
	methods SignInMethods
	slices  AccountSlices
	refs    ProjectRefs
}

var _ enduserv1connect.EndUserServiceHandler = (*EndUserService)(nil)

// NewEndUserService builds the admin end-user handler.
func NewEndUserService(users EndUsers, methods SignInMethods, slices AccountSlices, refs ProjectRefs) *EndUserService {
	return &EndUserService{users: users, methods: methods, slices: slices, refs: refs}
}

type adminTarget struct {
	orgID, projectID uuid.UUID
	projectPublicID  string
	user             *enduser.EndUser
}

func (t adminTarget) enduser() enduser.Project {
	return enduser.Project{OrgID: t.orgID, ID: t.projectID, PublicID: t.projectPublicID}
}

// SECURITY: an unknown project and one out of reach answer the same not-found; reads need ReachProject, revokes ReachWholeProject.
func (s *EndUserService) project(ctx context.Context, publicID string, write bool) (adminTarget, error) {
	id, orgID, ok, err := s.refs.ProjectID(ctx, publicID)
	reach := authz.ReachProject
	if write {
		reach = authz.ReachWholeProject
	}
	switch {
	case err != nil:
		return adminTarget{}, err
	case !ok || !reach(ctx, orgID, id):
		return adminTarget{}, &project.NotFoundError{}
	}
	return adminTarget{orgID: orgID, projectID: id, projectPublicID: publicID}, nil
}

func (s *EndUserService) target(ctx context.Context, projectPublicID, endUserPublicID string, write bool) (adminTarget, error) {
	t, err := s.project(ctx, projectPublicID, write)
	if err != nil {
		return t, err
	}
	t.user, err = s.users.ByPublicID(ctx, t.enduser(), endUserPublicID)
	return t, err
}

// ListEndUsers lists a project's end users, newest first, optionally by email prefix.
func (s *EndUserService) ListEndUsers(
	ctx context.Context, req *connect.Request[enduserv1.ListEndUsersRequest],
) (*connect.Response[enduserv1.ListEndUsersResponse], error) {
	t, err := s.project(ctx, req.Msg.GetProjectId(), false)
	if err != nil {
		return nil, err
	}
	users, next, err := s.users.ListEndUsers(ctx, t.enduser(), req.Msg.GetQuery(), req.Msg.GetAfter(), int(req.Msg.GetLimit()))
	if err != nil {
		return nil, err
	}
	out := make([]*enduserv1.EndUser, 0, len(users))
	for _, u := range users {
		out = append(out, enduser.EndUserProto(u))
	}
	return connect.NewResponse(&enduserv1.ListEndUsersResponse{EndUsers: out, Next: next}), nil
}

// GetEndUser returns one end user with their sign-in methods, live sessions, connected apps and personal tokens.
func (s *EndUserService) GetEndUser(
	ctx context.Context, req *connect.Request[enduserv1.GetEndUserRequest],
) (*connect.Response[enduserv1.GetEndUserResponse], error) {
	t, err := s.target(ctx, req.Msg.GetProjectId(), req.Msg.GetEndUserId(), false)
	if err != nil {
		return nil, err
	}
	methods, err := s.methods.SignInMethods(ctx, federation.Project{OrgID: t.orgID, ID: t.projectID, PublicID: t.projectPublicID}, t.user.ID)
	if err != nil {
		return nil, err
	}
	live, err := s.users.Sessions(ctx, t.enduser(), t.user.ID)
	if err != nil {
		return nil, err
	}
	apps, err := s.slices.Apps.Apps(ctx, consent.Project{OrgID: t.orgID, ID: t.projectID, PublicID: t.projectPublicID}, t.user.ID)
	if err != nil {
		return nil, err
	}
	tokens, err := s.tokens(ctx, t)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&enduserv1.GetEndUserResponse{
		EndUser: enduser.EndUserProto(t.user), Methods: federation.MethodsProto(methods), Sessions: enduser.SessionsProto(live, ""),
		Apps: apps, Tokens: tokens,
	}), nil
}

func (s *EndUserService) tokens(ctx context.Context, t adminTarget) ([]*accountv1.PersonalToken, error) {
	return s.slices.Tokens.Tokens(ctx, pat.Project{OrgID: t.orgID, ID: t.projectID, PublicID: t.projectPublicID}, t.user.ID)
}

// RevokeEndUserSession signs an end user out of one session with its grants; unlike the portal, their current one may go.
func (s *EndUserService) RevokeEndUserSession(
	ctx context.Context, req *connect.Request[enduserv1.RevokeEndUserSessionRequest],
) (*connect.Response[enduserv1.RevokeEndUserSessionResponse], error) {
	t, err := s.target(ctx, req.Msg.GetProjectId(), req.Msg.GetEndUserId(), true)
	if err != nil {
		return nil, err
	}
	if err := s.users.EndUserSession(ctx, t.enduser(), t.user.ID, req.Msg.GetSessionId()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&enduserv1.RevokeEndUserSessionResponse{}), nil
}

// DisconnectEndUserApp withdraws an end user's consents for an app and revokes its grants.
func (s *EndUserService) DisconnectEndUserApp(
	ctx context.Context, req *connect.Request[enduserv1.DisconnectEndUserAppRequest],
) (*connect.Response[enduserv1.DisconnectEndUserAppResponse], error) {
	t, err := s.target(ctx, req.Msg.GetProjectId(), req.Msg.GetEndUserId(), true)
	if err != nil {
		return nil, err
	}
	p := consent.Project{OrgID: t.orgID, ID: t.projectID, PublicID: t.projectPublicID}
	if err := s.slices.Apps.Disconnect(ctx, p, t.user.ID, t.user.Subject, req.Msg.GetClientId()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&enduserv1.DisconnectEndUserAppResponse{}), nil
}

// RevokeEndUserToken revokes one of an end user's personal tokens; it stops working at once.
func (s *EndUserService) RevokeEndUserToken(
	ctx context.Context, req *connect.Request[enduserv1.RevokeEndUserTokenRequest],
) (*connect.Response[enduserv1.RevokeEndUserTokenResponse], error) {
	t, err := s.target(ctx, req.Msg.GetProjectId(), req.Msg.GetEndUserId(), true)
	if err != nil {
		return nil, err
	}
	if err := s.slices.Tokens.Revoke(ctx, pat.Project{OrgID: t.orgID, ID: t.projectID, PublicID: t.projectPublicID}, t.user.ID,
		req.Msg.GetTokenId()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&enduserv1.RevokeEndUserTokenResponse{}), nil
}
