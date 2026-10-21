package enduser

import (
	"errors"
	"strconv"

	"altalune.id/othra/apperror"
)

// LinkNotUsableError signals a magic link that is unknown, expired or already used. SECURITY: one error for every cause.
type LinkNotUsableError struct{}

func (*LinkNotUsableError) Error() string { return "enduser: sign-in link not usable" }

// IsLinkNotUsableError reports whether err's tree contains a *LinkNotUsableError.
func IsLinkNotUsableError(err error) bool {
	_, ok := errors.AsType[*LinkNotUsableError](err)
	return ok
}

// ToAppError maps LinkNotUsableError to SIG001.
func (e *LinkNotUsableError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeSignInLinkNotUsable, "This sign-in link can't be used anymore").WithCause(e)
}

// EmailDisabledError signals a project that has signing in by email turned off.
type EmailDisabledError struct{}

func (*EmailDisabledError) Error() string { return "enduser: email sign-in is off" }

// IsEmailDisabledError reports whether err's tree contains an *EmailDisabledError.
func IsEmailDisabledError(err error) bool {
	_, ok := errors.AsType[*EmailDisabledError](err)
	return ok
}

// ToAppError maps EmailDisabledError to SIG002.
func (e *EmailDisabledError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeSignInEmailDisabled, "Signing in by email is turned off for this project").WithCause(e)
}

// SessionGoneError signals an account that is no longer signed in on this browser.
type SessionGoneError struct{}

func (*SessionGoneError) Error() string { return "enduser: user session gone" }

// IsSessionGoneError reports whether err's tree contains a *SessionGoneError.
func IsSessionGoneError(err error) bool {
	_, ok := errors.AsType[*SessionGoneError](err)
	return ok
}

// ToAppError maps SessionGoneError to SIG003.
func (e *SessionGoneError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeSignInSessionGone, "That account isn't signed in on this browser anymore").WithCause(e)
}

// ErrLiveLink reports a concurrent request that issued the email's one live link first; the caller answers it as a cooldown.
var ErrLiveLink = errors.New("enduser: the email already has a live link")

// TermsChangedError signals an acceptance of a terms version that is no longer current; the page shows the current terms again.
type TermsChangedError struct{ Seen, Current int }

func (e *TermsChangedError) Error() string {
	return "enduser: terms changed from version " + strconv.Itoa(e.Seen) + " to " + strconv.Itoa(e.Current)
}

// IsTermsChangedError reports whether err's tree contains a *TermsChangedError.
func IsTermsChangedError(err error) bool {
	_, ok := errors.AsType[*TermsChangedError](err)
	return ok
}

// InvalidEmailError signals an address that is not a bare email.
type InvalidEmailError struct{}

func (*InvalidEmailError) Error() string { return "enduser: invalid email" }

// IsInvalidEmailError reports whether err's tree contains an *InvalidEmailError.
func IsInvalidEmailError(err error) bool {
	_, ok := errors.AsType[*InvalidEmailError](err)
	return ok
}

// ToAppError maps InvalidEmailError to the email field's validation error.
func (e *InvalidEmailError) ToAppError() *apperror.AppError {
	return apperror.Invalid("email", apperror.FieldInvalidFormat)
}

// IsExpectedError reports whether err is a typed end-user error a service returns as-is.
func IsExpectedError(err error) bool {
	return IsLinkNotUsableError(err) || IsEmailDisabledError(err) || IsSessionGoneError(err) || IsInvalidEmailError(err)
}
