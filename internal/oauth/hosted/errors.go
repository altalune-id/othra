package hosted

import (
	"errors"

	"altalune.id/othra/apperror"
)

// ErrTermsChanged signals an acceptance of terms that changed after the page showed them.
var ErrTermsChanged = errors.New("hosted: terms changed")

// FormReason is the cause of a FormExpiredError, kept for logs; the page never shows it.
type FormReason string

// The causes of a FormExpiredError.
const (
	FormNoCookie FormReason = "no_cookie"
	FormNoToken  FormReason = "no_token"
	FormBadToken FormReason = "bad_token"
	FormExpired  FormReason = "expired"
)

// FormExpiredError signals a hosted form post whose CSRF token is missing, wrong or too old.
type FormExpiredError struct {
	Reason FormReason
}

func (e *FormExpiredError) Error() string { return "hosted: form expired: " + string(e.Reason) }

// IsFormExpiredError reports whether err's tree contains a *FormExpiredError.
func IsFormExpiredError(err error) bool {
	_, ok := errors.AsType[*FormExpiredError](err)
	return ok
}

// ToAppError maps FormExpiredError to SIG004; the reason stays out of the response.
func (e *FormExpiredError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeSignInFormExpired, "This page expired").WithCause(e)
}
