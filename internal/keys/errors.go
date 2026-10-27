package keys

import (
	"errors"

	"github.com/altalune-id/othra/apperror"
)

// NoActiveKeyError signals a project with no key that signs; after a project's creation this never happens.
type NoActiveKeyError struct{ ProjectID string }

func (e *NoActiveKeyError) Error() string {
	return "keys: project " + e.ProjectID + " has no active signing key"
}

// IsNoActiveKeyError reports whether err's tree contains a *NoActiveKeyError.
func IsNoActiveKeyError(err error) bool {
	_, ok := errors.AsType[*NoActiveKeyError](err)
	return ok
}

// ToAppError maps NoActiveKeyError to SGK001.
func (e *NoActiveKeyError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeSigningKeyNoActive, "This project has no active signing key").WithCause(e)
}

// RotationPendingError signals a rotate while a next key already waits to take over.
type RotationPendingError struct{ ProjectID string }

func (e *RotationPendingError) Error() string {
	return "keys: project " + e.ProjectID + " already has a next key waiting"
}

// IsRotationPendingError reports whether err's tree contains a *RotationPendingError.
func IsRotationPendingError(err error) bool {
	_, ok := errors.AsType[*RotationPendingError](err)
	return ok
}

// ToAppError maps RotationPendingError to SGK002.
func (e *RotationPendingError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeSigningKeyRotationPending, "A new key is already waiting to take over").WithCause(e)
}

// ProjectNotFoundError signals a project that does not exist or is out of the caller's reach.
type ProjectNotFoundError struct{}

func (*ProjectNotFoundError) Error() string { return "keys: project not found" }

// IsProjectNotFoundError reports whether err's tree contains a *ProjectNotFoundError.
func IsProjectNotFoundError(err error) bool {
	_, ok := errors.AsType[*ProjectNotFoundError](err)
	return ok
}

// ToAppError maps ProjectNotFoundError to the project not-found code.
func (e *ProjectNotFoundError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeProjectNotFound, "Project not found").WithCause(e)
}

// IsExpectedError reports whether err is a typed keys error a service returns as-is.
func IsExpectedError(err error) bool {
	return IsNoActiveKeyError(err) || IsRotationPendingError(err) || IsProjectNotFoundError(err)
}

// ErrInvalidToken is the cause of every token Verify refuses.
var ErrInvalidToken = errors.New("keys: invalid token")
