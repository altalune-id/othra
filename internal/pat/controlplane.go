package pat

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	accountv1 "altalune.id/othra/gen/go/account/v1"
	"altalune.id/othra/internal/kernel/session"
)

// AccountService is pat's slice of account.v1: the signed-in end user's personal tokens.
type AccountService struct {
	svc *Service
	now func() time.Time
}

// NewAccountService builds pat's account.v1 slice over svc.
func NewAccountService(svc *Service) *AccountService {
	return &AccountService{svc: svc, now: func() time.Time { return time.Now().UTC() }}
}

var errNoEndUser = errors.New("pat: no end user on the account plane")

func signedIn(ctx context.Context) (session.EndUser, Project, error) {
	eu, ok := session.EndUserFrom(ctx)
	if !ok {
		return session.EndUser{}, Project{}, errNoEndUser
	}
	return eu, Project{OrgID: eu.OrgID, ID: eu.ProjectID, PublicID: eu.ProjectPublicID, Name: eu.ProjectName}, nil
}

// ListTokenServers lists the resource servers the signed-in end user can mint a token for.
func (s *AccountService) ListTokenServers(
	ctx context.Context, _ *connect.Request[accountv1.ListTokenServersRequest],
) (*connect.Response[accountv1.ListTokenServersResponse], error) {
	_, p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	servers, err := s.svc.TokenServers(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make([]*accountv1.TokenServer, 0, len(servers))
	for i := range servers {
		out = append(out, serverProto(servers[i]))
	}
	return connect.NewResponse(&accountv1.ListTokenServersResponse{Servers: out}), nil
}

// ListPersonalTokens lists the signed-in end user's personal tokens.
func (s *AccountService) ListPersonalTokens(
	ctx context.Context, _ *connect.Request[accountv1.ListPersonalTokensRequest],
) (*connect.Response[accountv1.ListPersonalTokensResponse], error) {
	eu, p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	tokens, err := s.Tokens(ctx, p, eu.EndUserID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&accountv1.ListPersonalTokensResponse{Tokens: tokens}), nil
}

// CreatePersonalToken mints a token for the signed-in end user and returns its secret once.
func (s *AccountService) CreatePersonalToken(
	ctx context.Context, req *connect.Request[accountv1.CreatePersonalTokenRequest],
) (*connect.Response[accountv1.CreatePersonalTokenResponse], error) {
	eu, p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	got, err := s.svc.Create(ctx, p, CreateRequest{
		EndUserID: eu.EndUserID, Name: m.GetName(), ServerPublicID: m.GetServerId(), Scopes: m.GetScopes(),
		Expiry: expiryOf(m.GetExpiry()),
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&accountv1.CreatePersonalTokenResponse{
		Token: tokenProto(got.Token, got.Server, s.now()), Secret: got.Secret,
	}), nil
}

// RevokePersonalToken revokes one of the signed-in end user's tokens; it stops working at once.
func (s *AccountService) RevokePersonalToken(
	ctx context.Context, req *connect.Request[accountv1.RevokePersonalTokenRequest],
) (*connect.Response[accountv1.RevokePersonalTokenResponse], error) {
	eu, p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.svc.Revoke(ctx, p, eu.EndUserID, req.Msg.GetTokenId()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&accountv1.RevokePersonalTokenResponse{}), nil
}

// Tokens are the end user's personal tokens as account.v1 shows them; the admin view reuses it.
func (s *AccountService) Tokens(ctx context.Context, p Project, endUserID uuid.UUID) ([]*accountv1.PersonalToken, error) {
	listed, err := s.svc.List(ctx, p, endUserID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	out := make([]*accountv1.PersonalToken, 0, len(listed))
	for _, l := range listed {
		out = append(out, tokenProto(l.Token, l.Server, now))
	}
	return out, nil
}

// Revoke revokes the end user's token publicID; the admin view uses it.
func (s *AccountService) Revoke(ctx context.Context, p Project, endUserID uuid.UUID, publicID string) error {
	return s.svc.Revoke(ctx, p, endUserID, publicID)
}

func expiryOf(e accountv1.TokenExpiry) Expiry {
	switch e {
	case accountv1.TokenExpiry_TOKEN_EXPIRY_7_DAYS:
		return Expiry7Days
	case accountv1.TokenExpiry_TOKEN_EXPIRY_30_DAYS:
		return Expiry30Days
	case accountv1.TokenExpiry_TOKEN_EXPIRY_90_DAYS:
		return Expiry90Days
	}
	return 0
}

func serverProto(s Server) *accountv1.TokenServer {
	return &accountv1.TokenServer{
		Id: s.PublicID, Name: s.Name, Identifier: s.Identifier, Scopes: s.Scopes, ScopeDescriptions: s.Descriptions,
	}
}

func tokenProto(t *Token, srv Server, now time.Time) *accountv1.PersonalToken {
	out := &accountv1.PersonalToken{
		Id: t.PublicID, Name: t.Name, Hint: t.Hint, Server: serverProto(srv), Scopes: t.Scopes, State: stateProto(t.State(now)),
		CreatedAt: timestamppb.New(t.CreatedAt), ExpiresAt: timestamppb.New(t.ExpiresAt),
	}
	if t.LastUsedAt != nil {
		out.LastUsedAt = timestamppb.New(*t.LastUsedAt)
	}
	if t.RevokedAt != nil {
		out.RevokedAt = timestamppb.New(*t.RevokedAt)
	}
	return out
}

func stateProto(s State) accountv1.PersonalTokenState {
	switch s {
	case StateActive:
		return accountv1.PersonalTokenState_PERSONAL_TOKEN_STATE_ACTIVE
	case StateExpired:
		return accountv1.PersonalTokenState_PERSONAL_TOKEN_STATE_EXPIRED
	case StateRevoked:
		return accountv1.PersonalTokenState_PERSONAL_TOKEN_STATE_REVOKED
	}
	return accountv1.PersonalTokenState_PERSONAL_TOKEN_STATE_UNSPECIFIED
}
