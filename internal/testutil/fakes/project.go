package fakes

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/internal/project"
)

// ProjectStore is a project.Store test double that returns its configured values or Err.
type ProjectStore struct {
	Err          error
	Project      *project.Project
	Projects     []*project.Project
	SaveFn       func(*project.Project) error
	Saved        *project.Project
	TakenHandles []string
	Issuer       *project.Issuer
	HandleSetErr error
	LockedAt     *time.Time
	LogoSetErr   error
}

var _ project.Store = (*ProjectStore)(nil)

// Save implements project.Store.
func (f *ProjectStore) Save(_ context.Context, p *project.Project) error {
	f.Saved = p
	if f.SaveFn != nil {
		return f.SaveFn(p)
	}
	return f.Err
}

// ByID implements project.Store.
func (f *ProjectStore) ByID(context.Context, uuid.UUID) (*project.Project, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	return f.Project, nil
}

// RefByPublicID implements project.Store; the org slug is "org-" plus the project's slug.
func (f *ProjectStore) RefByPublicID(ctx context.Context, publicID string) (*project.Ref, error) {
	p, err := f.ByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	return &project.Ref{ID: p.ID, OrgID: p.OrgID, Slug: p.Slug, OrgSlug: "org-" + p.Slug}, nil
}

// ByPublicID implements project.Store; it finds Project or one of Projects by public id.
func (f *ProjectStore) ByPublicID(_ context.Context, publicID string) (*project.Project, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	for _, p := range append([]*project.Project{f.Project}, f.Projects...) {
		if p != nil && p.PublicID == publicID {
			return p, nil
		}
	}
	return nil, &project.NotFoundError{ID: publicID}
}

// BySlug implements project.Store.
func (f *ProjectStore) BySlug(context.Context, uuid.UUID, string) (*project.Project, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	return f.Project, nil
}

// List implements project.Store.
func (f *ProjectStore) List(context.Context, uuid.UUID) ([]*project.Project, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	return f.Projects, nil
}

// IssuerHandleExists implements project.Store; a handle in TakenHandles exists.
func (f *ProjectStore) IssuerHandleExists(_ context.Context, handle string) (bool, error) {
	if f.Err != nil {
		return false, f.Err
	}
	return slices.Contains(f.TakenHandles, handle), nil
}

// ResolveIssuer implements project.Store; it returns Issuer when its handle matches.
func (f *ProjectStore) ResolveIssuer(_ context.Context, handle string) (*project.Issuer, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	if f.Issuer == nil || f.Issuer.Handle != handle {
		return nil, &project.NotFoundError{}
	}
	return f.Issuer, nil
}

// SetIssuerHandle implements project.Store; it sets Project's handle or returns HandleSetErr.
func (f *ProjectStore) SetIssuerHandle(_ context.Context, _ uuid.UUID, handle string) error {
	if f.HandleSetErr != nil {
		return f.HandleSetErr
	}
	if f.Project != nil {
		f.Project.IssuerHandle = handle
	}
	return f.Err
}

// LockSignIn implements project.Store and returns Project's email-link switch.
func (f *ProjectStore) LockSignIn(context.Context, uuid.UUID) (bool, error) {
	if f.Err != nil {
		return false, f.Err
	}
	return f.Project.SignInEmailLink, nil
}

// LockIssuerHandle implements project.Store and records the lock time.
func (f *ProjectStore) LockIssuerHandle(_ context.Context, _ uuid.UUID, at time.Time) error {
	f.LockedAt = &at
	return f.Err
}

// SetLogoFile implements project.Store; it moves Project's logo file to fileID and returns the one it replaced.
func (f *ProjectStore) SetLogoFile(_ context.Context, _ uuid.UUID, fileID *uuid.UUID) (*uuid.UUID, error) {
	if f.LogoSetErr != nil {
		return nil, f.LogoSetErr
	}
	if f.Err != nil {
		return nil, f.Err
	}
	if f.Project == nil {
		return nil, &project.NotFoundError{}
	}
	old := f.Project.LogoFileID
	f.Project.LogoFileID = fileID
	return old, nil
}

// OrgSlugs is a project.OrgSlugs that names every org by its own value.
type OrgSlugs string

// Slug returns s.
func (s OrgSlugs) Slug(context.Context, uuid.UUID) (string, error) { return string(s), nil }
