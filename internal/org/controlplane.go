package org

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	orgv1 "github.com/altalune-id/othra/gen/go/org/v1"
	orgv1connect "github.com/altalune-id/othra/gen/go/org/v1/orgv1connect"
	"github.com/altalune-id/othra/internal/kernel/authz"
	"github.com/altalune-id/othra/internal/kernel/session"
)

// OrgService implements org.v1.OrgService.
type OrgService struct {
	orgs *Service
}

// NewControlPlane binds the handler to the org service.
func NewControlPlane(orgs *Service) *OrgService {
	return &OrgService{orgs: orgs}
}

var _ orgv1connect.OrgServiceHandler = (*OrgService)(nil)

// CreateOrg creates an organization, minting a slug when one is not supplied.
func (s *OrgService) CreateOrg(
	ctx context.Context,
	req *connect.Request[orgv1.CreateOrgRequest],
) (*connect.Response[orgv1.CreateOrgResponse], error) {
	in := CreateRequest{Slug: req.Msg.GetSlug(), Name: req.Msg.GetName()}
	if p := session.PrincipalFrom(ctx); p.IsPerson() && !p.Operator {
		in.CreatedBy = p.UserID
	}
	o, err := s.orgs.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&orgv1.CreateOrgResponse{Org: OrgProto(o)}), nil
}

// GetOrg loads an organization by id.
func (s *OrgService) GetOrg(
	ctx context.Context,
	req *connect.Request[orgv1.GetOrgRequest],
) (*connect.Response[orgv1.GetOrgResponse], error) {
	o, err := s.reachable(ctx, req.Msg.GetId(), authz.ReachOrg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&orgv1.GetOrgResponse{Org: OrgProto(o)}), nil
}

// ListOrgs lists the organizations the caller reaches, ordered by creation time.
func (s *OrgService) ListOrgs(
	ctx context.Context,
	_ *connect.Request[orgv1.ListOrgsRequest],
) (*connect.Response[orgv1.ListOrgsResponse], error) {
	orgs, err := s.listable(ctx)
	if err != nil {
		return nil, err
	}
	member, err := s.memberships(ctx, orgs)
	if err != nil {
		return nil, err
	}
	p := session.PrincipalFrom(ctx)
	out := make([]*orgv1.Org, 0, len(orgs))
	for _, o := range orgs {
		pb := OrgProto(o)
		if member[o.ID] {
			role, permissions, ok, err := s.orgs.MemberRole(ctx, o.ID, p.UserID)
			if err != nil {
				return nil, err
			}
			if ok {
				pb.Role, pb.Permissions = role, permissions
			}
		}
		out = append(out, pb)
	}
	return connect.NewResponse(&orgv1.ListOrgsResponse{Orgs: out}), nil
}

// SECURITY: a person sees a role only in orgs they belong to; an operator's other orgs, and every key, see none.
func (s *OrgService) memberships(ctx context.Context, listed []*Org) (map[uuid.UUID]bool, error) {
	p := session.PrincipalFrom(ctx)
	if !p.IsPerson() {
		return nil, nil
	}
	mine := listed
	if p.Operator {
		var err error
		if mine, err = s.orgs.ListForUser(ctx, p.UserID); err != nil {
			return nil, err
		}
	}
	out := make(map[uuid.UUID]bool, len(mine))
	for _, o := range mine {
		out[o.ID] = true
	}
	return out, nil
}

// UpdateOrg changes an organization's display name and slug.
func (s *OrgService) UpdateOrg(
	ctx context.Context,
	req *connect.Request[orgv1.UpdateOrgRequest],
) (*connect.Response[orgv1.UpdateOrgResponse], error) {
	o, err := s.reachable(ctx, req.Msg.GetId(), authz.ReachWholeOrg)
	if err != nil {
		return nil, err
	}
	o, err = s.orgs.Update(ctx, o.ID, req.Msg.GetName(), req.Msg.GetSlug())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&orgv1.UpdateOrgResponse{Org: OrgProto(o)}), nil
}

// SECURITY: an unknown public id and an org out of reach answer the same not-found.
func (s *OrgService) reachable(ctx context.Context, publicID string, reach func(context.Context, uuid.UUID) bool) (*Org, error) {
	o, err := s.orgs.ByPublicID(ctx, publicID)
	if IsNotFoundError(err) {
		return nil, &NotFoundError{}
	}
	if err != nil {
		return nil, err
	}
	if !reach(ctx, o.ID) {
		return nil, &NotFoundError{}
	}
	return o, nil
}

// SECURITY: a person lists exactly the orgs they belong to (they switch between them); anyone else lists the orgs they reach.
func (s *OrgService) listable(ctx context.Context) ([]*Org, error) {
	p := session.PrincipalFrom(ctx)
	switch {
	case p.Operator:
		return s.orgs.ListAll(ctx)
	case p.IsPerson():
		return s.orgs.ListForUser(ctx, p.UserID)
	}
	all, err := s.orgs.List(ctx)
	if err != nil {
		return nil, err
	}
	reached := all[:0:0]
	for _, o := range all {
		if authz.ReachOrg(ctx, o.ID) {
			reached = append(reached, o)
		}
	}
	return reached, nil
}
