package project

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	projectv1 "altalune.id/othra/gen/go/project/v1"
	projectv1connect "altalune.id/othra/gen/go/project/v1/projectv1connect"
	"altalune.id/othra/internal/kernel/authz"
	"altalune.id/othra/internal/kernel/session"
)

// ProjectService implements project.v1.ProjectService; every id it takes and returns is a public id.
type ProjectService struct {
	projects *Service
	orgs     OrgRefs
}

// NewControlPlane binds the handler to the project service and the org public id lookup.
func NewControlPlane(projects *Service, orgs OrgRefs) *ProjectService {
	return &ProjectService{projects: projects, orgs: orgs}
}

var _ projectv1connect.ProjectServiceHandler = (*ProjectService)(nil)

// CreateProject creates a project within an organization, minting a slug when one is not supplied.
func (s *ProjectService) CreateProject(
	ctx context.Context,
	req *connect.Request[projectv1.CreateProjectRequest],
) (*connect.Response[projectv1.CreateProjectResponse], error) {
	orgID, err := s.org(ctx, req.Msg.GetOrgId(), authz.ReachWholeOrg)
	if err != nil {
		return nil, err
	}
	p, err := s.projects.Create(ctx, CreateRequest{
		OrgID:        orgID,
		Slug:         req.Msg.GetSlug(),
		Name:         req.Msg.GetName(),
		IssuerHandle: req.Msg.GetIssuerHandle(),
	})
	if err != nil {
		return nil, err
	}
	pb := projectToProto(p, req.Msg.GetOrgId(), s.projects.IssuerURL(p.IssuerHandle), s.projects.LogoURL(p.LogoKey))
	return connect.NewResponse(&projectv1.CreateProjectResponse{Project: pb}), nil
}

// GetProject loads a project by id.
func (s *ProjectService) GetProject(
	ctx context.Context,
	req *connect.Request[projectv1.GetProjectRequest],
) (*connect.Response[projectv1.GetProjectResponse], error) {
	p, err := s.reachable(ctx, req.Msg.GetId(), authz.ReachProject)
	if err != nil {
		return nil, err
	}
	out, err := s.toProto(ctx, p)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&projectv1.GetProjectResponse{Project: out}), nil
}

// ListProjects lists the organization's projects the caller reaches, ordered by creation time.
func (s *ProjectService) ListProjects(
	ctx context.Context,
	req *connect.Request[projectv1.ListProjectsRequest],
) (*connect.Response[projectv1.ListProjectsResponse], error) {
	orgID, byMembership, err := s.listableOrg(ctx, req.Msg.GetOrgId())
	if err != nil {
		return nil, err
	}
	projects, err := s.projects.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]*projectv1.Project, 0, len(projects))
	for _, p := range projects {
		if byMembership || authz.ReachProject(ctx, p.OrgID, p.ID) {
			out = append(out, projectToProto(p, req.Msg.GetOrgId(), s.projects.IssuerURL(p.IssuerHandle), s.projects.LogoURL(p.LogoKey)))
		}
	}
	return connect.NewResponse(&projectv1.ListProjectsResponse{Projects: out}), nil
}

// UpdateProject renames a project and/or changes its slug.
func (s *ProjectService) UpdateProject(
	ctx context.Context,
	req *connect.Request[projectv1.UpdateProjectRequest],
) (*connect.Response[projectv1.UpdateProjectResponse], error) {
	p, err := s.reachable(ctx, req.Msg.GetId(), authz.ReachWholeProject)
	if err != nil {
		return nil, err
	}
	p, err = s.projects.Update(ctx, p.ID, req.Msg.GetName(), req.Msg.GetSlug())
	if err != nil {
		return nil, err
	}
	out, err := s.toProto(ctx, p)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&projectv1.UpdateProjectResponse{Project: out}), nil
}

// UpdateProjectIssuer changes a project's issuer handle while it has no OAuth clients.
func (s *ProjectService) UpdateProjectIssuer(
	ctx context.Context,
	req *connect.Request[projectv1.UpdateProjectIssuerRequest],
) (*connect.Response[projectv1.UpdateProjectIssuerResponse], error) {
	out, err := s.update(ctx, req.Msg.GetId(), func(p *Project) (*Project, error) {
		return s.projects.UpdateIssuerHandle(ctx, p.ID, req.Msg.GetIssuerHandle())
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&projectv1.UpdateProjectIssuerResponse{Project: out}), nil
}

// UpdateProjectBranding sets a project's display name, primary colour and support email.
func (s *ProjectService) UpdateProjectBranding(
	ctx context.Context,
	req *connect.Request[projectv1.UpdateProjectBrandingRequest],
) (*connect.Response[projectv1.UpdateProjectBrandingResponse], error) {
	m := req.Msg
	out, err := s.update(ctx, m.GetId(), func(p *Project) (*Project, error) {
		return s.projects.UpdateBranding(ctx, p.ID, m.GetDisplayName(), m.GetPrimaryColor(), m.GetSupportEmail())
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&projectv1.UpdateProjectBrandingResponse{Project: out}), nil
}

// UpdateProjectTerms sets a project's terms and privacy links.
func (s *ProjectService) UpdateProjectTerms(
	ctx context.Context,
	req *connect.Request[projectv1.UpdateProjectTermsRequest],
) (*connect.Response[projectv1.UpdateProjectTermsResponse], error) {
	out, err := s.update(ctx, req.Msg.GetId(), func(p *Project) (*Project, error) {
		return s.projects.UpdateTerms(ctx, p.ID, req.Msg.GetTermsUrl(), req.Msg.GetPrivacyUrl())
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&projectv1.UpdateProjectTermsResponse{Project: out}), nil
}

// BumpTermsVersion publishes a new terms version, so every end user accepts again.
func (s *ProjectService) BumpTermsVersion(
	ctx context.Context,
	req *connect.Request[projectv1.BumpTermsVersionRequest],
) (*connect.Response[projectv1.BumpTermsVersionResponse], error) {
	out, err := s.update(ctx, req.Msg.GetId(), func(p *Project) (*Project, error) {
		return s.projects.BumpTermsVersion(ctx, p.ID)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&projectv1.BumpTermsVersionResponse{
		Version:     out.GetTermsVersion(),
		PublishedAt: out.GetTermsPublishedAt(),
	}), nil
}

// CreateProjectLogoUpload signs a direct upload of a new logo for a project.
func (s *ProjectService) CreateProjectLogoUpload(
	ctx context.Context,
	req *connect.Request[projectv1.CreateProjectLogoUploadRequest],
) (*connect.Response[projectv1.CreateProjectLogoUploadResponse], error) {
	p, err := s.reachable(ctx, req.Msg.GetId(), authz.ReachWholeProject)
	if err != nil {
		return nil, err
	}
	f, up, err := s.projects.CreateLogoUpload(ctx, p.ID, req.Msg.GetContentType(), int64(req.Msg.GetSizeBytes()))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&projectv1.CreateProjectLogoUploadResponse{
		UploadId:  f.PublicID,
		Url:       up.URL,
		Method:    up.Method,
		Headers:   up.Headers,
		ExpiresAt: timestamppb.New(up.ExpiresAt),
	}), nil
}

// CompleteProjectLogoUpload makes a finished upload the project's logo.
func (s *ProjectService) CompleteProjectLogoUpload(
	ctx context.Context,
	req *connect.Request[projectv1.CompleteProjectLogoUploadRequest],
) (*connect.Response[projectv1.CompleteProjectLogoUploadResponse], error) {
	out, err := s.update(ctx, req.Msg.GetId(), func(p *Project) (*Project, error) {
		return s.projects.CompleteLogoUpload(ctx, p.ID, req.Msg.GetUploadId())
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&projectv1.CompleteProjectLogoUploadResponse{Project: out}), nil
}

// DeleteProjectLogo removes a project's logo.
func (s *ProjectService) DeleteProjectLogo(
	ctx context.Context,
	req *connect.Request[projectv1.DeleteProjectLogoRequest],
) (*connect.Response[projectv1.DeleteProjectLogoResponse], error) {
	out, err := s.update(ctx, req.Msg.GetId(), func(p *Project) (*Project, error) {
		return s.projects.DeleteLogo(ctx, p.ID)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&projectv1.DeleteProjectLogoResponse{Project: out}), nil
}

// CheckIssuerHandle reports whether a handle is free in an org's new project, or in the project named by project_id.
func (s *ProjectService) CheckIssuerHandle(
	ctx context.Context,
	req *connect.Request[projectv1.CheckIssuerHandleRequest],
) (*connect.Response[projectv1.CheckIssuerHandleResponse], error) {
	orgID, err := s.org(ctx, req.Msg.GetOrgId(), authz.ReachWholeOrg)
	if err != nil {
		return nil, err
	}
	var current *Project
	if id := req.Msg.GetProjectId(); id != "" {
		if current, err = s.reachable(ctx, id, authz.ReachWholeProject); err != nil {
			return nil, err
		}
		if current.OrgID != orgID {
			return nil, &NotFoundError{}
		}
	}
	reason, err := s.projects.CheckIssuerHandle(ctx, req.Msg.GetIssuerHandle(), current)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&projectv1.CheckIssuerHandleResponse{Available: reason == HandleOK, Reason: reason}), nil
}

func (s *ProjectService) update(ctx context.Context, publicID string, apply func(*Project) (*Project, error)) (*projectv1.Project, error) {
	p, err := s.reachable(ctx, publicID, authz.ReachWholeProject)
	if err != nil {
		return nil, err
	}
	if p, err = apply(p); err != nil {
		return nil, err
	}
	return s.toProto(ctx, p)
}

// SECURITY: an unknown org and an org out of reach answer the same not-found.
func (s *ProjectService) org(ctx context.Context, publicID string, reach func(context.Context, uuid.UUID) bool) (uuid.UUID, error) {
	id, ok, err := s.orgs.OrgID(ctx, publicID)
	if err != nil {
		return uuid.Nil, err
	}
	if !ok || !reach(ctx, id) {
		return uuid.Nil, &NotFoundError{}
	}
	return id, nil
}

// SECURITY: beyond the reached org, a person (never a key) lists the projects of another org they are a member of, checked per call.
func (s *ProjectService) listableOrg(ctx context.Context, publicID string) (uuid.UUID, bool, error) {
	id, ok, err := s.orgs.OrgID(ctx, publicID)
	if err != nil {
		return uuid.Nil, false, err
	}
	if !ok {
		return uuid.Nil, false, &NotFoundError{}
	}
	if authz.ReachOrg(ctx, id) {
		return id, false, nil
	}
	p := session.PrincipalFrom(ctx)
	if !p.IsPerson() {
		return uuid.Nil, false, &NotFoundError{}
	}
	member, err := s.projects.IsMember(ctx, id, p.UserID)
	if err != nil {
		return uuid.Nil, false, err
	}
	if !member {
		return uuid.Nil, false, &NotFoundError{}
	}
	return id, true, nil
}

// SECURITY: an unknown project and a project out of reach answer the same not-found.
func (s *ProjectService) reachable(
	ctx context.Context,
	publicID string,
	reach func(ctx context.Context, orgID, projectID uuid.UUID) bool,
) (*Project, error) {
	p, err := s.projects.ByPublicID(ctx, publicID)
	if IsNotFoundError(err) {
		return nil, &NotFoundError{}
	}
	if err != nil {
		return nil, err
	}
	if !reach(ctx, p.OrgID, p.ID) {
		return nil, &NotFoundError{}
	}
	return p, nil
}

func (s *ProjectService) toProto(ctx context.Context, p *Project) (*projectv1.Project, error) {
	orgPublicID, err := s.orgs.OrgPublicID(ctx, p.OrgID)
	if err != nil {
		return nil, err
	}
	return projectToProto(p, orgPublicID, s.projects.IssuerURL(p.IssuerHandle), s.projects.LogoURL(p.LogoKey)), nil
}
