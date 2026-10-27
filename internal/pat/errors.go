package pat

import (
	"errors"
	"strconv"

	"github.com/altalune-id/othra/apperror"
)

// NotFoundError signals a token that does not exist or is another end user's.
type NotFoundError struct{}

func (*NotFoundError) Error() string { return "pat: not found" }

// IsNotFoundError reports whether err's tree contains a *NotFoundError.
func IsNotFoundError(err error) bool {
	_, ok := errors.AsType[*NotFoundError](err)
	return ok
}

// ToAppError maps NotFoundError to ACC004.
func (e *NotFoundError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeAccountTokenNotFound, "That token no longer exists").WithCause(e)
}

// InvalidReason is the stable cause of an InvalidError, sent to clients as meta.reason.
type InvalidReason string

// The causes of an InvalidError.
const (
	InvalidName   InvalidReason = "name"
	InvalidServer InvalidReason = "server"
	InvalidScopes InvalidReason = "scopes"
	InvalidExpiry InvalidReason = "expiry"
)

// InvalidError signals a token that can't be created as asked.
type InvalidError struct{ Reason InvalidReason }

func (e *InvalidError) Error() string { return "pat: invalid " + string(e.Reason) }

// IsInvalidError reports whether err's tree contains an *InvalidError.
func IsInvalidError(err error) bool {
	_, ok := errors.AsType[*InvalidError](err)
	return ok
}

// ToAppError maps InvalidError to ACC005 with the reason.
func (e *InvalidError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeAccountTokenInvalid, "This token can't be created").
		WithMeta(map[string]string{"reason": string(e.Reason)}).WithCause(e)
}

// ServerClosedError signals a resource server that doesn't take personal tokens (off, or no introspection secret).
type ServerClosedError struct{ Project string }

func (*ServerClosedError) Error() string { return "pat: the server doesn't accept personal tokens" }

// IsServerClosedError reports whether err's tree contains a *ServerClosedError.
func IsServerClosedError(err error) bool {
	_, ok := errors.AsType[*ServerClosedError](err)
	return ok
}

// ToAppError maps ServerClosedError to ACC006 with the project's name.
func (e *ServerClosedError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeAccountTokenServerClosed, "This server doesn't accept personal tokens").
		WithMeta(map[string]string{"project": e.Project}).WithCause(e)
}

// LimitError signals an end user who already has MaxLive live tokens.
type LimitError struct{ Max int }

func (e *LimitError) Error() string { return "pat: " + strconv.Itoa(e.Max) + " live tokens" }

// IsLimitError reports whether err's tree contains a *LimitError.
func IsLimitError(err error) bool {
	_, ok := errors.AsType[*LimitError](err)
	return ok
}

// ToAppError maps LimitError to ACC007 with the limit.
func (e *LimitError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeAccountTokenLimit, "Too many active tokens").
		WithMeta(map[string]string{"max": strconv.Itoa(e.Max)}).WithCause(e)
}

// ReauthRequiredError signals a sign-in too old to create a token.
type ReauthRequiredError struct{}

func (*ReauthRequiredError) Error() string { return "pat: sign in again to create a token" }

// IsReauthRequiredError reports whether err's tree contains a *ReauthRequiredError.
func IsReauthRequiredError(err error) bool {
	_, ok := errors.AsType[*ReauthRequiredError](err)
	return ok
}

// ToAppError maps ReauthRequiredError to ACC009.
func (e *ReauthRequiredError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeAccountReauthRequired, "Confirm it's you to create a token").WithCause(e)
}

// IsExpectedError reports whether err is a typed personal-token error the service returns as-is.
func IsExpectedError(err error) bool {
	return IsNotFoundError(err) || IsInvalidError(err) || IsServerClosedError(err) || IsLimitError(err) || IsReauthRequiredError(err)
}
