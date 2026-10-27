package upstream

import (
	"errors"

	"github.com/altalune-id/othra/apperror"
)

// NotFoundError signals an upstream IdP that does not exist or is out of the caller's reach.
type NotFoundError struct{}

func (*NotFoundError) Error() string { return "upstream: not found" }

// IsNotFoundError reports whether err's tree contains a *NotFoundError.
func IsNotFoundError(err error) bool {
	_, ok := errors.AsType[*NotFoundError](err)
	return ok
}

// ToAppError maps NotFoundError to UPS001.
func (e *NotFoundError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeUpstreamNotFound, "Upstream IdP not found").WithCause(e)
}

// ConfigReason is the stable cause of an InvalidConfigError, sent to clients as meta.reason.
type ConfigReason string

// The causes of an InvalidConfigError.
const (
	ConfigClientID     ConfigReason = "client_id"
	ConfigClientSecret ConfigReason = "client_secret"
)

// InvalidConfigError signals upstream settings that can't work: a missing client id or secret.
type InvalidConfigError struct{ Reason ConfigReason }

func (e *InvalidConfigError) Error() string { return "upstream: invalid config: " + string(e.Reason) }

// IsInvalidConfigError reports whether err's tree contains an *InvalidConfigError.
func IsInvalidConfigError(err error) bool {
	_, ok := errors.AsType[*InvalidConfigError](err)
	return ok
}

// ToAppError maps InvalidConfigError to UPS002 with the reason.
func (e *InvalidConfigError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeUpstreamInvalidConfig, "Invalid upstream settings").
		WithMeta(map[string]string{"reason": string(e.Reason)}).WithCause(e)
}

// ExistsError signals a second upstream of a kind the project already has.
type ExistsError struct{}

func (*ExistsError) Error() string { return "upstream: the project already has this kind" }

// IsExistsError reports whether err's tree contains an *ExistsError.
func IsExistsError(err error) bool {
	_, ok := errors.AsType[*ExistsError](err)
	return ok
}

// ToAppError maps ExistsError to UPS003.
func (e *ExistsError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeUpstreamExists, "The project already has this upstream IdP").WithCause(e)
}

// HasIdentitiesError signals a delete of an upstream that end users have linked; it can only be turned off.
type HasIdentitiesError struct{}

func (*HasIdentitiesError) Error() string { return "upstream: end users have linked it" }

// IsHasIdentitiesError reports whether err's tree contains a *HasIdentitiesError.
func IsHasIdentitiesError(err error) bool {
	_, ok := errors.AsType[*HasIdentitiesError](err)
	return ok
}

// ToAppError maps HasIdentitiesError to UPS004.
func (e *HasIdentitiesError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeUpstreamHasIdentities, "End users have linked this upstream IdP").WithCause(e)
}

// IsExpectedError reports whether err is a typed upstream error a service returns as-is.
func IsExpectedError(err error) bool {
	return IsNotFoundError(err) || IsInvalidConfigError(err) || IsExistsError(err) || IsHasIdentitiesError(err) ||
		IsLastMethodError(err) || IsProjectNotFoundError(err)
}

// ProjectNotFoundError signals a project that does not exist or is out of the caller's reach.
type ProjectNotFoundError struct{}

func (*ProjectNotFoundError) Error() string { return "upstream: project not found" }

// IsProjectNotFoundError reports whether err's tree contains a *ProjectNotFoundError.
func IsProjectNotFoundError(err error) bool {
	_, ok := errors.AsType[*ProjectNotFoundError](err)
	return ok
}

// ToAppError maps ProjectNotFoundError to the project not-found code.
func (e *ProjectNotFoundError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeProjectNotFound, "Project not found").WithCause(e)
}

// LastMethodError signals turning off or removing the upstream while the project's email link is off.
type LastMethodError struct{}

func (*LastMethodError) Error() string { return "upstream: it is the project's only sign-in method" }

// IsLastMethodError reports whether err's tree contains a *LastMethodError.
func IsLastMethodError(err error) bool {
	_, ok := errors.AsType[*LastMethodError](err)
	return ok
}

// ToAppError maps LastMethodError to PRJ010.
func (e *LastMethodError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeProjectSignInLastMethod, "Keep at least one sign-in method on").WithCause(e)
}
