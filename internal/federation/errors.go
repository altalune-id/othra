package federation

import (
	"errors"

	"altalune.id/othra/apperror"
	"altalune.id/othra/oidcrp"
)

// CollisionError signals a new upstream identity whose verified email already belongs to an end user: nothing is linked.
type CollisionError struct {
	Email    string
	Identity Identity
}

func (*CollisionError) Error() string { return "federation: the email belongs to another end user" }

// ToAppError maps CollisionError to SIG020; the hosted pages answer it with a magic link instead.
func (e *CollisionError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeSignInUpstreamEmailTaken, "The email belongs to another account").WithCause(e)
}

// IsCollisionError reports whether err's tree contains a *CollisionError.
func IsCollisionError(err error) bool {
	_, ok := errors.AsType[*CollisionError](err)
	return ok
}

// UnprovenEmailError signals a first sign-in whose email Google isn't authoritative for: nothing is created, the inbox proves it.
type UnprovenEmailError struct {
	Email    string
	Identity Identity
}

func (*UnprovenEmailError) Error() string {
	return "federation: the upstream is not authoritative for the email"
}

// IsUnprovenEmailError reports whether err's tree contains an *UnprovenEmailError.
func IsUnprovenEmailError(err error) bool { return isType[*UnprovenEmailError](err) }

// UnverifiedEmailError signals an upstream that doesn't vouch for the email; every Othra end user keeps a verified email.
type UnverifiedEmailError struct{}

func (*UnverifiedEmailError) Error() string { return "federation: the upstream email is not verified" }

// ToAppError maps UnverifiedEmailError to SIG013.
func (e *UnverifiedEmailError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeSignInUpstreamEmailUnverified, "The upstream email is not verified").WithCause(e)
}

// LinkedElsewhereError signals an upstream identity that belongs to another end user, or a second identity of one upstream.
type LinkedElsewhereError struct{}

func (*LinkedElsewhereError) Error() string {
	return "federation: the identity is linked to another end user"
}

// ToAppError maps LinkedElsewhereError to SIG018.
func (e *LinkedElsewhereError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeSignInUpstreamLinkedElsewhere, "The identity is linked to another account").WithCause(e)
}

// LastMethodError signals an unlink that would leave the end user no way to sign in.
type LastMethodError struct{}

func (*LastMethodError) Error() string { return "federation: the last sign-in method" }

// ToAppError maps LastMethodError to SIG019.
func (e *LastMethodError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeSignInLastMethod, "The last sign-in method can't be removed").WithCause(e)
}

// IdentityNotFoundError signals an identity that is unknown or another end user's.
type IdentityNotFoundError struct{}

func (*IdentityNotFoundError) Error() string { return "federation: identity not found" }

// ToAppError maps IdentityNotFoundError to the generic not-found, the same for an unknown identity and another end user's.
func (e *IdentityNotFoundError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeNotFound, "That sign-in method is not linked to your account").WithCause(e)
}

// UpstreamError is a failed round trip with the upstream; Reason is for logs only.
type UpstreamError struct {
	Reason oidcrp.Reason
	Err    error
}

func (e *UpstreamError) Error() string { return "federation: upstream " + string(e.Reason) }

func (e *UpstreamError) Unwrap() error { return e.Err }

// ToAppError maps a cancelled sign-in to SIG012, an upstream outage to SIG015, a disabled upstream to SIG017 and a bad answer to SIG016.
func (e *UpstreamError) ToAppError() *apperror.AppError {
	code := apperror.CodeSignInUpstreamResponseInvalid
	switch e.Reason {
	case oidcrp.ReasonCancelled:
		code = apperror.CodeSignInUpstreamCancelled
	case oidcrp.ReasonDiscovery, oidcrp.ReasonJWKS, oidcrp.ReasonTokenExchange, oidcrp.ReasonTimeout, oidcrp.ReasonError,
		oidcrp.ReasonMisconfigured:
		code = apperror.CodeSignInUpstreamUnavailable
	case reasonDisabled:
		code = apperror.CodeSignInUpstreamDisabled
	}
	return apperror.New(code, "Signing in with the upstream IdP failed").WithMeta(map[string]string{"reason": string(e.Reason)}).WithCause(e)
}

// Reasons federation adds to oidcrp's: an upstream that is off or gone, and a state cookie that didn't come back.
const (
	reasonDisabled oidcrp.Reason = "disabled"
	// ReasonCookie is the state cookie missing, expired or unreadable at the callback.
	ReasonCookie oidcrp.Reason = "cookie"
)

// IsExpectedError reports whether err is a typed federation error the service returns as-is.
func IsExpectedError(err error) bool {
	for _, ok := range []bool{
		IsCollisionError(err), IsUnprovenEmailError(err), isType[*UnverifiedEmailError](err), isType[*LinkedElsewhereError](err),
		isType[*LastMethodError](err), isType[*IdentityNotFoundError](err), isType[*UpstreamError](err),
	} {
		if ok {
			return true
		}
	}
	return false
}

func isType[T error](err error) bool {
	_, ok := errors.AsType[T](err)
	return ok
}

// ErrEmailTaken reports a Create whose email another end user just took; federation answers it as a collision.
var ErrEmailTaken = errors.New("federation: the email belongs to another end user")

var (
	errSubjectTaken  = errors.New("federation: the (issuer, subject) is linked")
	errUpstreamTaken = errors.New("federation: the end user already has an identity of this upstream")
)
