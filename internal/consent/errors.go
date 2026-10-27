package consent

import (
	"errors"

	"github.com/altalune-id/othra/apperror"
)

// AppNotConnectedError signals a Disconnect of a client the end user has no consent or live grant with.
type AppNotConnectedError struct{}

func (*AppNotConnectedError) Error() string { return "consent: app not connected" }

// IsAppNotConnectedError reports whether err's tree contains an *AppNotConnectedError.
func IsAppNotConnectedError(err error) bool {
	_, ok := errors.AsType[*AppNotConnectedError](err)
	return ok
}

// ToAppError maps AppNotConnectedError to ACC003.
func (e *AppNotConnectedError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeAccountAppNotConnected, "That app is no longer connected").WithCause(e)
}
