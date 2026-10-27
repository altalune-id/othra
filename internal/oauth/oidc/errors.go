package oidc

import (
	"errors"
)

// ErrUnknownIssuer is an unknown, reserved or malformed issuer handle; every surface answers it with one identical not-found.
var ErrUnknownIssuer = errors.New("oidc: unknown issuer")

// LookupLimitedError refuses a cache-missing issuer lookup over LookupIPRule.
type LookupLimitedError struct{ RetryAfter int }

func (*LookupLimitedError) Error() string { return "oidc: issuer lookup rate limited" }

// SECURITY: every error_description is fixed text, never a value or a parameter name from the request.
var (
	errEndUserGone    = errors.New("oidc: end user gone")
	errFormType       = errors.New("the request body must be application/x-www-form-urlencoded")
	errFormBody       = errors.New("the request body is not a valid form")
	errFormRepeat     = errors.New("a request parameter appears more than once")
	errResourceRepeat = errors.New("a token is issued for one resource")
	errMixedAuth      = errors.New("use one client authentication method")
	errBadCredentials = errors.New("oidc: malformed Basic credentials")
)
