package auth

import (
	"errors"

	"altalune.id/othra/apperror"
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
