package tenant

import (
	"errors"

	"github.com/altalune-id/othra/apperror"
)

// MissingError signals that the tenant Context is absent from ctx.
type MissingError struct{}

func (*MissingError) Error() string { return "tenant: missing context" }

// ToAppError maps MissingError to the canonical Unauthenticated envelope.
func (*MissingError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeTenantMissing, "Tenant context missing")
}

// UnscopedError signals that ctx carries a tenant Context whose OrgID is the zero uuid.
type UnscopedError struct{}

func (*UnscopedError) Error() string { return "tenant: context names no org" }

// ToAppError maps UnscopedError to the canonical Unauthenticated envelope.
func (*UnscopedError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeTenantMissing, "Tenant context names no org")
}

// IsUnscopedError reports whether err wraps an *UnscopedError.
func IsUnscopedError(err error) bool {
	_, ok := errors.AsType[*UnscopedError](err)
	return ok
}

// IsMissingError reports whether err wraps a *MissingError.
func IsMissingError(err error) bool {
	_, ok := errors.AsType[*MissingError](err)
	return ok
}

// ProjectOutsideOrgError signals a project id that is not in the request's org, or does not exist.
type ProjectOutsideOrgError struct{}

func (*ProjectOutsideOrgError) Error() string { return "tenant: project is not in this organization" }

// IsProjectOutsideOrgError reports whether err wraps a *ProjectOutsideOrgError.
func IsProjectOutsideOrgError(err error) bool {
	_, ok := errors.AsType[*ProjectOutsideOrgError](err)
	return ok
}

// ToAppError maps ProjectOutsideOrgError to the validation envelope.
func (e *ProjectOutsideOrgError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeProjectNotInOrg, "Project is not in this organization").WithCause(e)
}
