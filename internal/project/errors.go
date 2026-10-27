package project

import (
	"errors"
	"fmt"
	"strconv"

	"altalune.id/othra/apperror"
)

// NotFoundError signals a missing project row.
type NotFoundError struct {
	ID   string
	Slug string
}

func (e *NotFoundError) Error() string {
	switch {
	case e == nil:
		return "project: not found"
	case e.Slug != "":
		return fmt.Sprintf("project: not found: slug=%q", e.Slug)
	case e.ID != "":
		return fmt.Sprintf("project: not found: id=%s", e.ID)
	default:
		return "project: not found"
	}
}

// IsNotFoundError reports whether err wraps a *NotFoundError.
func IsNotFoundError(err error) bool {
	_, ok := errors.AsType[*NotFoundError](err)
	return ok
}

// AlreadyExistsError signals a slug collision within an organization.
type AlreadyExistsError struct {
	Slug string
}

func (e *AlreadyExistsError) Error() string {
	return fmt.Sprintf("project: slug already exists: %q", e.Slug)
}

// IsAlreadyExistsError reports whether err wraps an *AlreadyExistsError.
func IsAlreadyExistsError(err error) bool {
	_, ok := errors.AsType[*AlreadyExistsError](err)
	return ok
}

// SlugReason is the stable cause of an InvalidSlugError, sent to clients as meta.reason.
type SlugReason string

// The causes of an InvalidSlugError.
const (
	SlugLength SlugReason = "length"
	SlugFormat SlugReason = "format"
)

// InvalidSlugError signals a slug that fails the aggregate invariants.
type InvalidSlugError struct {
	Slug   string
	Reason SlugReason
}

func (e *InvalidSlugError) Error() string {
	return "project: invalid slug: " + e.text()
}

func (e *InvalidSlugError) text() string {
	switch e.Reason {
	case SlugLength:
		return fmt.Sprintf("must be %d to %d characters", slugMinLen, slugMaxLen)
	case SlugFormat:
		return "must use lowercase letters, digits and dashes"
	default:
		return fmt.Sprintf("%q is invalid", apperror.Echo(e.Slug))
	}
}

// IsInvalidSlugError reports whether err wraps an *InvalidSlugError.
func IsInvalidSlugError(err error) bool {
	_, ok := errors.AsType[*InvalidSlugError](err)
	return ok
}

// NameReason is the stable cause of an InvalidNameError, sent to clients as meta.reason.
type NameReason string

// NameLength is the one cause of an InvalidNameError.
const NameLength NameReason = "length"

// InvalidNameError signals a name that fails the aggregate invariants.
type InvalidNameError struct {
	Reason NameReason
}

func (e *InvalidNameError) Error() string {
	return fmt.Sprintf("project: invalid name: must be %d to %d characters", nameMinLen, nameMaxLen)
}

// IsInvalidNameError reports whether err wraps an *InvalidNameError.
func IsInvalidNameError(err error) bool {
	_, ok := errors.AsType[*InvalidNameError](err)
	return ok
}

// IssuerHandleTakenError signals an issuer handle another project already uses.
type IssuerHandleTakenError struct {
	Handle string
}

func (e *IssuerHandleTakenError) Error() string {
	return fmt.Sprintf("project: issuer handle already taken: %q", apperror.Echo(e.Handle))
}

// IsIssuerHandleTakenError reports whether err wraps an *IssuerHandleTakenError.
func IsIssuerHandleTakenError(err error) bool {
	_, ok := errors.AsType[*IssuerHandleTakenError](err)
	return ok
}

// IssuerHandleReason is the stable cause of an InvalidIssuerHandleError, sent to clients as meta.reason.
type IssuerHandleReason string

// The causes of an InvalidIssuerHandleError.
const (
	IssuerHandleLength   IssuerHandleReason = "length"
	IssuerHandleFormat   IssuerHandleReason = "format"
	IssuerHandleReserved IssuerHandleReason = "reserved"
)

// InvalidIssuerHandleError signals an issuer handle that fails the handle rules.
type InvalidIssuerHandleError struct {
	Handle string
	Reason IssuerHandleReason
}

func (e *InvalidIssuerHandleError) Error() string {
	return fmt.Sprintf("project: invalid issuer handle %q: %s", apperror.Echo(e.Handle), e.Reason)
}

// IsInvalidIssuerHandleError reports whether err wraps an *InvalidIssuerHandleError.
func IsInvalidIssuerHandleError(err error) bool {
	_, ok := errors.AsType[*InvalidIssuerHandleError](err)
	return ok
}

// IssuerHandleLockedError signals a handle change after the project's first OAuth client.
type IssuerHandleLockedError struct{}

func (*IssuerHandleLockedError) Error() string { return "project: issuer handle is locked" }

// IsIssuerHandleLockedError reports whether err wraps an *IssuerHandleLockedError.
func IsIssuerHandleLockedError(err error) bool {
	_, ok := errors.AsType[*IssuerHandleLockedError](err)
	return ok
}

// LogoReason is the stable cause of an InvalidLogoError, sent to clients as meta.reason.
type LogoReason string

// The causes of an InvalidLogoError.
const (
	LogoType LogoReason = "type"
	LogoSize LogoReason = "size"
)

// InvalidLogoError signals a logo that is not PNG, JPEG or WebP, or is too large.
type InvalidLogoError struct {
	Reason LogoReason
}

func (e *InvalidLogoError) Error() string { return "project: invalid logo: " + string(e.Reason) }

// IsInvalidLogoError reports whether err wraps an *InvalidLogoError.
func IsInvalidLogoError(err error) bool {
	_, ok := errors.AsType[*InvalidLogoError](err)
	return ok
}

// TermsNotSetError signals a terms version bump on a project with no terms link.
type TermsNotSetError struct{}

func (*TermsNotSetError) Error() string { return "project: no terms link to publish" }

// IsTermsNotSetError reports whether err wraps a *TermsNotSetError.
func IsTermsNotSetError(err error) bool {
	_, ok := errors.AsType[*TermsNotSetError](err)
	return ok
}

// SignInLastMethodError signals a change that would leave a project's end users with no sign-in method.
type SignInLastMethodError struct{}

func (*SignInLastMethodError) Error() string { return "project: keep at least one sign-in method on" }

// IsSignInLastMethodError reports whether err wraps a *SignInLastMethodError.
func IsSignInLastMethodError(err error) bool {
	_, ok := errors.AsType[*SignInLastMethodError](err)
	return ok
}

// SystemManagedError signals a change to the console project that only its branding and upstream IdPs allow.
type SystemManagedError struct{}

func (*SystemManagedError) Error() string { return "project: the console project is managed by Othra" }

// IsSystemManagedError reports whether err wraps a *SystemManagedError.
func IsSystemManagedError(err error) bool {
	_, ok := errors.AsType[*SystemManagedError](err)
	return ok
}

// ToAppError maps SystemManagedError to PRJ011.
func (e *SystemManagedError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeProjectSystemManaged, "Othra manages the console project").WithCause(e)
}

// IsExpectedError reports whether err is a typed domain error a service returns as-is.
func IsExpectedError(err error) bool {
	return IsNotFoundError(err) || IsAlreadyExistsError(err) || IsInvalidSlugError(err) || IsInvalidNameError(err) ||
		IsIssuerHandleTakenError(err) || IsInvalidIssuerHandleError(err) || IsIssuerHandleLockedError(err) ||
		IsInvalidLogoError(err) || IsTermsNotSetError(err) || IsSignInLastMethodError(err) || IsSystemManagedError(err)
}

// ToAppError maps NotFoundError to the canonical NotFound envelope.
func (e *NotFoundError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeProjectNotFound, "Project not found").WithCause(e)
}

// ToAppError maps AlreadyExistsError to the canonical AlreadyExists envelope.
func (e *AlreadyExistsError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeProjectAlreadyExists, "Project slug already taken").
		WithMeta(map[string]string{"slug": apperror.Echo(e.Slug)}).WithCause(e)
}

// ToAppError maps InvalidSlugError to its own code with the slug, the reason and the length limits.
func (e *InvalidSlugError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeProjectInvalidSlug, "Project slug "+e.text()).
		WithMeta(map[string]string{
			"slug": apperror.Echo(e.Slug), "reason": string(e.Reason),
			"min": strconv.Itoa(slugMinLen), "max": strconv.Itoa(slugMaxLen),
		}).WithCause(e)
}

// ToAppError maps InvalidNameError to its own code with the reason and the length limits.
func (e *InvalidNameError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeProjectInvalidName, fmt.Sprintf("Project name must be %d to %d characters", nameMinLen, nameMaxLen)).
		WithMeta(map[string]string{"reason": string(e.Reason), "min": strconv.Itoa(nameMinLen), "max": strconv.Itoa(nameMaxLen)}).WithCause(e)
}

// ToAppError maps IssuerHandleTakenError to its own code with the handle.
func (e *IssuerHandleTakenError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeProjectIssuerHandleTaken, "Issuer handle already taken").
		WithMeta(map[string]string{"issuer_handle": apperror.Echo(e.Handle)}).WithCause(e)
}

// ToAppError maps InvalidIssuerHandleError to its own code with the handle, the reason and the length limits.
func (e *InvalidIssuerHandleError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeProjectInvalidIssuerHandle, "Invalid issuer handle").
		WithMeta(map[string]string{
			"issuer_handle": apperror.Echo(e.Handle), "reason": string(e.Reason),
			"min": strconv.Itoa(issuerHandleMinLen), "max": strconv.Itoa(issuerHandleMaxLen),
		}).WithCause(e)
}

// ToAppError maps IssuerHandleLockedError to its own code.
func (e *IssuerHandleLockedError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeProjectIssuerHandleLocked, "Issuer handle is locked").WithCause(e)
}

// ToAppError maps InvalidLogoError to its own code with the reason and the size limit in KB.
func (e *InvalidLogoError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeProjectInvalidLogo, "Invalid logo").
		WithMeta(map[string]string{"reason": string(e.Reason), "max": strconv.Itoa(logoMaxBytes >> 10)}).WithCause(e)
}

// ToAppError maps TermsNotSetError to its own code.
func (e *TermsNotSetError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeProjectTermsNotSet, "No terms link: add one before publishing a new terms version.").WithCause(e)
}

// ToAppError maps SignInLastMethodError to PRJ010.
func (e *SignInLastMethodError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeProjectSignInLastMethod, "Keep at least one sign-in method on").WithCause(e)
}

var errNoOrgSlugs = errors.New("project: no org slugs wired")
