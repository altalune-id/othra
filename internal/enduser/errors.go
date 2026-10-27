package enduser

import (
	"errors"
	"strconv"

	"github.com/altalune-id/othra/apperror"
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

// ErrEmailTaken reports a new end user whose email another end user of the project already has.
var ErrEmailTaken = errors.New("enduser: the email belongs to another end user")

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
	return IsLinkNotUsableError(err) || IsEmailDisabledError(err) || IsSessionGoneError(err) || IsInvalidEmailError(err) ||
		IsSignedOutError(err) || IsSessionNotFoundError(err) || IsCurrentSessionError(err) || IsEndUserNotFoundError(err) ||
		IsTermsRequiredError(err)
}

// SignedOutError signals an account.v1 call whose browser has no live signed-in account.
type SignedOutError struct{}

func (*SignedOutError) Error() string { return "enduser: signed out" }

// IsSignedOutError reports whether err's tree contains a *SignedOutError.
func IsSignedOutError(err error) bool {
	_, ok := errors.AsType[*SignedOutError](err)
	return ok
}

// ToAppError maps SignedOutError to ACC001.
func (e *SignedOutError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeAccountSignedOut, "Your session ended").WithCause(e)
}

// SessionNotFoundError signals a user session that is not the end user's or already ended.
type SessionNotFoundError struct{}

func (*SessionNotFoundError) Error() string { return "enduser: session not found" }

// IsSessionNotFoundError reports whether err's tree contains a *SessionNotFoundError.
func IsSessionNotFoundError(err error) bool {
	_, ok := errors.AsType[*SessionNotFoundError](err)
	return ok
}

// ToAppError maps SessionNotFoundError to ACC002.
func (e *SessionNotFoundError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeAccountSessionNotFound, "That session is already signed out").WithCause(e)
}

// CurrentSessionError signals an end user ending the session of this very request; Sign out does that.
type CurrentSessionError struct{}

func (*CurrentSessionError) Error() string { return "enduser: the current session" }

// IsCurrentSessionError reports whether err's tree contains a *CurrentSessionError.
func IsCurrentSessionError(err error) bool {
	_, ok := errors.AsType[*CurrentSessionError](err)
	return ok
}

// ToAppError maps CurrentSessionError to ACC008.
func (e *CurrentSessionError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeAccountCurrentSession, "Use Sign out for this device").WithCause(e)
}

// EndUserNotFoundError signals an end user that does not exist in the project.
type EndUserNotFoundError struct{}

func (*EndUserNotFoundError) Error() string { return "enduser: end user not found" }

// IsEndUserNotFoundError reports whether err's tree contains an *EndUserNotFoundError.
func IsEndUserNotFoundError(err error) bool {
	_, ok := errors.AsType[*EndUserNotFoundError](err)
	return ok
}

// ToAppError maps EndUserNotFoundError to EUS001.
func (e *EndUserNotFoundError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeEndUserNotFound, "End user not found").WithCause(e)
}

// TermsRequiredError signals an account.v1 call from an end user who still has to accept the project's current terms.
type TermsRequiredError struct{}

func (*TermsRequiredError) Error() string { return "enduser: terms required" }

// IsTermsRequiredError reports whether err's tree contains a *TermsRequiredError.
func IsTermsRequiredError(err error) bool {
	_, ok := errors.AsType[*TermsRequiredError](err)
	return ok
}

// ToAppError maps TermsRequiredError to ACC010.
func (e *TermsRequiredError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeAccountTermsRequired, "Accept the updated terms to continue").WithCause(e)
}
