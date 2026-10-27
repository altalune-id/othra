package install

import (
	"errors"

	"altalune.id/othra/apperror"
)

// AlreadyInstalledError signals an install attempt after the system org already has an owner.
type AlreadyInstalledError struct{}

func (*AlreadyInstalledError) Error() string { return "install: already installed" }

// IsAlreadyInstalledError reports whether err wraps an *AlreadyInstalledError.
func IsAlreadyInstalledError(err error) bool {
	_, ok := errors.AsType[*AlreadyInstalledError](err)
	return ok
}

// ToAppError maps AlreadyInstalledError to the canonical AlreadyExists envelope.
func (e *AlreadyInstalledError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeInstallAlreadyDone, "This instance is already installed").WithCause(e)
}

var errSystemOrgMoved = errors.New("install: the system org was created by another process")
