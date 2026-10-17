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
	Err           error
	Project       *project.Project
	Projects      []*project.Project
	SaveFn        func(*project.Project) error
	Saved         *project.Project
	TakenHandles  []string
	LogoContent   *project.Logo
	Issuer        *project.Issuer
	HandleSetErr  error
	LockedAt      *time.Time
	DeletedLogoOf uuid.UUID
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

// Logo implements project.Store; it returns LogoContent.
func (f *ProjectStore) Logo(context.Context, uuid.UUID) (*project.Logo, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	return f.LogoContent, nil
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

// LockIssuerHandle implements project.Store and records the lock time.
func (f *ProjectStore) LockIssuerHandle(_ context.Context, _ uuid.UUID, at time.Time) error {
	f.LockedAt = &at
	return f.Err
}

// SaveLogo implements project.Store; it keeps the logo as LogoContent.
func (f *ProjectStore) SaveLogo(_ context.Context, l *project.Logo) error {
	if f.Err != nil {
		return f.Err
	}
	f.LogoContent = l
	return nil
}

// DeleteLogo implements project.Store and records the project id.
func (f *ProjectStore) DeleteLogo(_ context.Context, projectID uuid.UUID) error {
	f.DeletedLogoOf = projectID
	f.LogoContent = nil
	return f.Err
}
