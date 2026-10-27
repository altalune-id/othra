package grant

import (
	"errors"
)

// InvalidGrantError signals a code or refresh token that is unknown, expired, used, revoked or not this client's (RFC 6749 §5.2).
type InvalidGrantError struct{}

func (*InvalidGrantError) Error() string { return "grant: invalid grant" }

// IsInvalidGrantError reports whether err's tree contains an *InvalidGrantError.
func IsInvalidGrantError(err error) bool {
	_, ok := errors.AsType[*InvalidGrantError](err)
	return ok
}

// InvalidScopeError signals a refresh that asks for a scope the grant does not hold (RFC 6749 §6).
type InvalidScopeError struct{}

func (*InvalidScopeError) Error() string { return "grant: invalid scope" }

// IsInvalidScopeError reports whether err's tree contains an *InvalidScopeError.
func IsInvalidScopeError(err error) bool {
	_, ok := errors.AsType[*InvalidScopeError](err)
	return ok
}

// InvalidTokenError signals an access token that is unknown, expired or whose grant ended (RFC 6750 §3.1).
type InvalidTokenError struct{}

func (*InvalidTokenError) Error() string { return "grant: invalid token" }

// IsInvalidTokenError reports whether err's tree contains an *InvalidTokenError.
func IsInvalidTokenError(err error) bool {
	_, ok := errors.AsType[*InvalidTokenError](err)
	return ok
}

// InvalidRequestError signals a code_verifier sent for a code issued without a code_challenge (RFC 9700 §2.1.1).
type InvalidRequestError struct{}

func (*InvalidRequestError) Error() string { return "grant: invalid request" }

// IsInvalidRequestError reports whether err's tree contains an *InvalidRequestError.
func IsInvalidRequestError(err error) bool {
	_, ok := errors.AsType[*InvalidRequestError](err)
	return ok
}

// InvalidTargetError signals a resource the grant is not bound to (RFC 8707 §2).
type InvalidTargetError struct{}

func (*InvalidTargetError) Error() string { return "grant: invalid target" }

// IsInvalidTargetError reports whether err's tree contains an *InvalidTargetError.
func IsInvalidTargetError(err error) bool {
	_, ok := errors.AsType[*InvalidTargetError](err)
	return ok
}

// SessionGoneError signals a code asked for a user session that has ended, or a client or end user that is gone.
type SessionGoneError struct{}

func (*SessionGoneError) Error() string { return "grant: user session gone" }

// IsSessionGoneError reports whether err's tree contains a *SessionGoneError.
func IsSessionGoneError(err error) bool {
	_, ok := errors.AsType[*SessionGoneError](err)
	return ok
}

// IsExpectedError reports whether err is a typed grant error the service returns as-is.
func IsExpectedError(err error) bool {
	return IsInvalidGrantError(err) || IsInvalidScopeError(err) || IsInvalidTokenError(err) || IsSessionGoneError(err) ||
		IsInvalidRequestError(err) || IsInvalidTargetError(err)
}
