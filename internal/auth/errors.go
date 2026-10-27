package auth

import (
	"errors"

	"github.com/altalune-id/othra/apperror"
)

// InvalidCredentialsError signals a failed sign-in. SECURITY: one error for every cause, so emails cannot be enumerated.
type InvalidCredentialsError struct{}

func (*InvalidCredentialsError) Error() string { return "auth: invalid credentials" }

// IsInvalidCredentialsError reports whether err wraps an *InvalidCredentialsError.
func IsInvalidCredentialsError(err error) bool {
	_, ok := errors.AsType[*InvalidCredentialsError](err)
	return ok
}

// ToAppError maps InvalidCredentialsError to the canonical Unauthenticated envelope.
func (e *InvalidCredentialsError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeAuthInvalidCredentials, "Invalid email or password").WithCause(e)
}

// ResetReason says why a password reset link can no longer be used.
type ResetReason string

// The reasons a reset is not usable.
const (
	ResetExpired  ResetReason = "expired"
	ResetUsed     ResetReason = "used"
	ResetNotFound ResetReason = "not_found"
)

// ResetNotUsableError reports a reset link that is unknown, used or expired.
type ResetNotUsableError struct{ Reason ResetReason }

func (e *ResetNotUsableError) Error() string { return "auth: reset not usable: " + string(e.Reason) }

// IsResetNotUsableError reports whether err wraps a *ResetNotUsableError.
func IsResetNotUsableError(err error) bool {
	_, ok := errors.AsType[*ResetNotUsableError](err)
	return ok
}

// ResetReasonOf returns why err's reset is not usable, or "" when err is not a *ResetNotUsableError.
func ResetReasonOf(err error) ResetReason {
	if e, ok := errors.AsType[*ResetNotUsableError](err); ok {
		return e.Reason
	}
	return ""
}

// Message is the text shown on the reset page.
func (e *ResetNotUsableError) Message() string {
	switch e.Reason {
	case ResetExpired:
		return "This reset link has expired. Ask for a new one."
	case ResetUsed:
		return "This reset link was already used. Ask for a new one if you still need it."
	}
	return "This reset link is not valid. Check that you copied the whole link from your email."
}

// ToAppError maps ResetNotUsableError to AUT004 with its reason.
func (e *ResetNotUsableError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeAuthResetNotUsable, e.Message()).
		WithMeta(map[string]string{"reason": string(e.Reason)}).
		WithCause(e)
}

// NoConsoleReason is why a /p/system sign-in has no console account, sent to clients as meta.reason.
type NoConsoleReason string

// The reasons a /p/system sign-in has no console account.
const (
	NoConsoleNotInvited NoConsoleReason = "not_invited"
	NoConsoleRemoved    NoConsoleReason = "removed"
	NoConsoleUnverified NoConsoleReason = "email_unverified"
)

// NoConsoleAccountError signals a /p/system sign-in with no console account: never invited, or removed since.
type NoConsoleAccountError struct{ Reason NoConsoleReason }

func (e *NoConsoleAccountError) Error() string {
	return "auth: no console account: " + string(e.Reason)
}

// IsNoConsoleAccountError reports whether err's tree contains a *NoConsoleAccountError.
func IsNoConsoleAccountError(err error) bool {
	_, ok := errors.AsType[*NoConsoleAccountError](err)
	return ok
}

// ToAppError maps NoConsoleAccountError to AUT005 with its reason.
func (e *NoConsoleAccountError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeAuthNoConsoleAccount, "This email has no account on this console").
		WithMeta(map[string]string{"reason": string(e.Reason)}).WithCause(e)
}

// SignInExpiredError signals a console sign-in whose state is missing or expired.
type SignInExpiredError struct{}

func (*SignInExpiredError) Error() string { return "auth: sign-in expired" }

// IsSignInExpiredError reports whether err's tree contains a *SignInExpiredError.
func IsSignInExpiredError(err error) bool {
	_, ok := errors.AsType[*SignInExpiredError](err)
	return ok
}

// ToAppError maps SignInExpiredError to AUT006.
func (e *SignInExpiredError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeAuthSignInExpired, "This sign-in expired").WithCause(e)
}

// IdentityConflictError signals a console account already linked to another /p/system identity.
type IdentityConflictError struct{}

func (*IdentityConflictError) Error() string { return "auth: account linked to another identity" }

// IsIdentityConflictError reports whether err's tree contains an *IdentityConflictError.
func IsIdentityConflictError(err error) bool {
	_, ok := errors.AsType[*IdentityConflictError](err)
	return ok
}

// ToAppError maps IdentityConflictError to AUT007.
func (e *IdentityConflictError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeAuthIdentityConflict, "This account is linked to a different Othra sign-in").WithCause(e)
}
