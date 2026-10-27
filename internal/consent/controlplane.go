package consent

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	accountv1 "altalune.id/othra/gen/go/account/v1"
	"altalune.id/othra/internal/kernel/session"
)

// AccountService is consent's slice of account.v1: the signed-in end user's connected apps.
type AccountService struct {
	svc *Service
	dir Directory
}

// NewAccountService builds consent's account.v1 slice over svc, naming clients and servers through dir.
func NewAccountService(svc *Service, dir Directory) *AccountService {
	return &AccountService{svc: svc, dir: dir}
}

var errNoEndUser = errors.New("consent: no end user on the account plane")

func signedIn(ctx context.Context) (session.EndUser, Project, error) {
	eu, ok := session.EndUserFrom(ctx)
	if !ok {
		return session.EndUser{}, Project{}, errNoEndUser
	}
	return eu, Project{OrgID: eu.OrgID, ID: eu.ProjectID, PublicID: eu.ProjectPublicID}, nil
}

// ListConnectedApps lists every app the signed-in end user has a consent or a live grant with (C1).
func (s *AccountService) ListConnectedApps(
	ctx context.Context, _ *connect.Request[accountv1.ListConnectedAppsRequest],
) (*connect.Response[accountv1.ListConnectedAppsResponse], error) {
	eu, p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	apps, err := s.Apps(ctx, p, eu.EndUserID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&accountv1.ListConnectedAppsResponse{Apps: apps}), nil
}

// DisconnectApp withdraws the end user's consents for an app and revokes its grants (R-5); access already held ends within 15 min.
func (s *AccountService) DisconnectApp(
	ctx context.Context, req *connect.Request[accountv1.DisconnectAppRequest],
) (*connect.Response[accountv1.DisconnectAppResponse], error) {
	eu, p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.Disconnect(ctx, p, eu.EndUserID, eu.Subject, req.Msg.GetClientId()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&accountv1.DisconnectAppResponse{}), nil
}

// Apps are the end user's connected apps as account.v1 shows them; the admin view reuses it.
func (s *AccountService) Apps(ctx context.Context, p Project, endUserID uuid.UUID) ([]*accountv1.ConnectedApp, error) {
	apps, err := s.svc.ConnectedApps(ctx, p, endUserID)
	if err != nil || len(apps) == 0 {
		return []*accountv1.ConnectedApp{}, err
	}
	ids := make([]uuid.UUID, 0, len(apps))
	for _, a := range apps {
		ids = append(ids, a.ClientID)
	}
	clients, err := s.dir.Clients(ctx, p, ids)
	if err != nil {
		return nil, err
	}
	servers, err := s.dir.Servers(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make([]*accountv1.ConnectedApp, 0, len(apps))
	for _, a := range apps {
		c, ok := clients[a.ClientID]
		if !ok {
			continue
		}
		out = append(out, appProto(a, c, servers))
	}
	return out, nil
}

// Disconnect resolves oauthID and disconnects that app from the end user, or returns *AppNotConnectedError (ACC003).
func (s *AccountService) Disconnect(ctx context.Context, p Project, endUserID uuid.UUID, endUserPublicID, oauthID string) error {
	clientID, ok, err := s.dir.ClientID(ctx, p, oauthID)
	switch {
	case err != nil:
		return err
	case !ok:
		return &AppNotConnectedError{}
	}
	return s.svc.Disconnect(ctx, p, Connection{
		EndUserID: endUserID, EndUserPublicID: endUserPublicID, ClientID: clientID, ClientOAuthID: oauthID,
	})
}

func appProto(a App, c ClientInfo, servers map[uuid.UUID]ServerInfo) *accountv1.ConnectedApp {
	out := &accountv1.ConnectedApp{
		ClientId: c.OAuthID, Name: c.Name, MetadataHost: c.MetadataHost, Consented: a.Consented, Offline: a.Offline,
		ConnectedAt: timestamppb.New(a.ConnectedAt),
	}
	if !a.LastUsedAt.IsZero() {
		out.LastUsedAt = timestamppb.New(a.LastUsedAt)
	}
	for _, r := range a.Resources {
		res := &accountv1.AppResource{Scopes: r.Scopes}
		if r.ResourceServerID != nil {
			srv := servers[*r.ResourceServerID]
			res.Id, res.Name, res.Identifier = srv.PublicID, srv.Name, srv.Identifier
			res.ScopeDescriptions = described(srv.Descriptions, r.Scopes)
		}
		out.Resources = append(out.Resources, res)
	}
	return out
}

func described(all map[string]string, scopes []string) map[string]string {
	out := map[string]string{}
	for _, sc := range scopes {
		if d, ok := all[sc]; ok {
			out[sc] = d
		}
	}
	return out
}
