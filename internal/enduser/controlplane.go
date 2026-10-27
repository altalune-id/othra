package enduser

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	accountv1 "github.com/altalune-id/othra/gen/go/account/v1"
	enduserv1 "github.com/altalune-id/othra/gen/go/enduser/v1"
	"github.com/altalune-id/othra/internal/kernel/session"
	"github.com/altalune-id/othra/internal/upstream"
)

// AccountService is enduser's slice of account.v1: the signed-in end user's profile and sessions.
type AccountService struct{ svc *Service }

// NewAccountService builds enduser's account.v1 slice over svc.
func NewAccountService(svc *Service) *AccountService { return &AccountService{svc: svc} }

var errNoEndUser = errors.New("enduser: no end user on the account plane")

func signedIn(ctx context.Context) (session.EndUser, Project, error) {
	eu, ok := session.EndUserFrom(ctx)
	if !ok {
		return session.EndUser{}, Project{}, errNoEndUser
	}
	return eu, Project{OrgID: eu.OrgID, ID: eu.ProjectID, PublicID: eu.ProjectPublicID}, nil
}

// GetProfile returns the signed-in end user's profile and the hosted sign-out form's token.
func (s *AccountService) GetProfile(
	ctx context.Context, _ *connect.Request[accountv1.GetProfileRequest],
) (*connect.Response[accountv1.GetProfileResponse], error) {
	eu, p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	u, err := s.svc.Profile(ctx, p, eu.EndUserID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&accountv1.GetProfileResponse{
		Profile: ProfileProto(u), SignOutToken: eu.SignOutToken, CurrentSessionId: eu.UserSession,
	}), nil
}

// UpdateProfile changes the name the signed-in end user goes by.
func (s *AccountService) UpdateProfile(
	ctx context.Context, req *connect.Request[accountv1.UpdateProfileRequest],
) (*connect.Response[accountv1.UpdateProfileResponse], error) {
	eu, p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	u, err := s.svc.UpdateName(ctx, p, eu.EndUserID, req.Msg.GetName())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&accountv1.UpdateProfileResponse{Profile: ProfileProto(u)}), nil
}

// ListSessions lists the signed-in end user's live sessions on every browser, marking this one.
func (s *AccountService) ListSessions(
	ctx context.Context, _ *connect.Request[accountv1.ListSessionsRequest],
) (*connect.Response[accountv1.ListSessionsResponse], error) {
	eu, p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	live, err := s.svc.Sessions(ctx, p, eu.EndUserID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&accountv1.ListSessionsResponse{Sessions: SessionsProto(live, eu.UserSession)}), nil
}

// EndSession signs the end user out of another of their sessions, with its grants; this browser's own is ACC008 (use Sign out).
func (s *AccountService) EndSession(
	ctx context.Context, req *connect.Request[accountv1.EndSessionRequest],
) (*connect.Response[accountv1.EndSessionResponse], error) {
	eu, p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.GetSessionId() == eu.UserSession {
		return nil, &CurrentSessionError{}
	}
	if err := s.svc.EndUserSession(ctx, p, eu.EndUserID, req.Msg.GetSessionId()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&accountv1.EndSessionResponse{}), nil
}

// EndOtherSessions signs the end user out everywhere but this browser.
func (s *AccountService) EndOtherSessions(
	ctx context.Context, _ *connect.Request[accountv1.EndOtherSessionsRequest],
) (*connect.Response[accountv1.EndOtherSessionsResponse], error) {
	eu, p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	n, err := s.svc.EndOtherSessions(ctx, p, eu.EndUserID, eu.UserSession)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&accountv1.EndOtherSessionsResponse{Ended: int32(n)}), nil //nolint:gosec // bounded by live sessions.
}

// ProfileProto is the end user as account.v1 shows them.
func ProfileProto(u *EndUser) *accountv1.Profile {
	return &accountv1.Profile{Id: u.Subject, Email: u.Email, Name: u.Name, CreatedAt: timestamppb.New(u.CreatedAt)}
}

// EndUserProto is the end user as the admin enduser.v1 shows them.
func EndUserProto(u *EndUser) *enduserv1.EndUser {
	out := &enduserv1.EndUser{Id: u.Subject, Email: u.Email, Name: u.Name, CreatedAt: timestamppb.New(u.CreatedAt)}
	if u.LastSignInAt != nil {
		out.LastSignInAt = timestamppb.New(*u.LastSignInAt)
	}
	return out
}

// SessionsProto are the live sessions as account.v1 shows them, current marking the caller's own.
func SessionsProto(live []LiveSession, current string) []*accountv1.Session {
	out := make([]*accountv1.Session, 0, len(live))
	for _, ls := range live {
		out = append(out, &accountv1.Session{
			Id: ls.PublicID, Method: MethodProto(ls.Method), UserAgent: ls.UserAgent, AuthTime: timestamppb.New(ls.AuthTime),
			LastSeenAt: timestamppb.New(ls.LastSeenAt), Current: ls.PublicID == current,
		})
	}
	return out
}

// MethodProto is how a user session signed in: the email link, or the upstream IdP of that kind.
func MethodProto(method string) accountv1.SignInMethodKind {
	if method == MethodMagicLink {
		return accountv1.SignInMethodKind_SIGN_IN_METHOD_KIND_EMAIL_LINK
	}
	return upstream.MethodProto(upstream.Kind(method))
}
