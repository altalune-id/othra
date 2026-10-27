package federation

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	accountv1 "altalune.id/othra/gen/go/account/v1"
	"altalune.id/othra/internal/kernel/session"
	"altalune.id/othra/internal/upstream"
)

// AccountService is federation's slice of account.v1: the signed-in end user's sign-in methods.
type AccountService struct{ svc *Service }

// NewAccountService builds federation's account.v1 slice over svc.
func NewAccountService(svc *Service) *AccountService { return &AccountService{svc: svc} }

var errNoEndUser = errors.New("federation: no end user on the account plane")

func signedIn(ctx context.Context) (session.EndUser, Project, error) {
	eu, ok := session.EndUserFrom(ctx)
	if !ok {
		return session.EndUser{}, Project{}, errNoEndUser
	}
	return eu, Project{OrgID: eu.OrgID, ID: eu.ProjectID, PublicID: eu.ProjectPublicID}, nil
}

// GetSignInMethods lists how the signed-in end user signs in, with the token of the hosted connect form.
func (s *AccountService) GetSignInMethods(
	ctx context.Context, _ *connect.Request[accountv1.GetSignInMethodsRequest],
) (*connect.Response[accountv1.GetSignInMethodsResponse], error) {
	eu, p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	ms, err := s.svc.SignInMethods(ctx, p, eu.EndUserID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&accountv1.GetSignInMethodsResponse{Methods: MethodsProto(ms), LinkToken: eu.LinkToken}), nil
}

// UnlinkIdentity removes the end user's identity unless it is the last sign-in method (SIG019); one not theirs or gone is NotFound.
func (s *AccountService) UnlinkIdentity(
	ctx context.Context, req *connect.Request[accountv1.UnlinkIdentityRequest],
) (*connect.Response[accountv1.UnlinkIdentityResponse], error) {
	eu, p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.svc.Unlink(ctx, p, eu.EndUserID, req.Msg.GetIdentityId()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&accountv1.UnlinkIdentityResponse{}), nil
}

// MethodsProto are the sign-in methods as account.v1 shows them.
func MethodsProto(ms []SignInMethod) []*accountv1.SignInMethod {
	out := make([]*accountv1.SignInMethod, 0, len(ms))
	for _, m := range ms {
		pm := &accountv1.SignInMethod{
			Kind: accountv1.SignInMethodKind_SIGN_IN_METHOD_KIND_EMAIL_LINK, Linked: m.Linked, CanRemove: m.CanRemove,
		}
		if m.Kind != "" {
			pm.Kind = upstream.MethodProto(m.Kind)
		}
		if m.LastMethod {
			pm.Lock = accountv1.SignInMethodLock_SIGN_IN_METHOD_LOCK_LAST_METHOD
		}
		if m.Identity.PublicID != "" {
			pm.IdentityId, pm.Email = m.Identity.PublicID, m.Identity.Email
			pm.LinkedAt, pm.LastUsedAt = timestamppb.New(m.Identity.LinkedAt), timestamppb.New(m.Identity.LastUsedAt)
		}
		out = append(out, pm)
	}
	return out
}
