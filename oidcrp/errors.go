package oidcrp

import "errors"

// Reason is why a sign-in failed, for logs and the upstream error codes; never shown with upstream text.
type Reason string

// The failures of a upstream round trip.
const (
	ReasonDiscovery     Reason = "discovery"
	ReasonJWKS          Reason = "jwks"
	ReasonTokenExchange Reason = "token_exchange"
	ReasonTimeout       Reason = "timeout"
	ReasonState         Reason = "state"
	ReasonIssuer        Reason = "iss"
	ReasonNonce         Reason = "nonce"
	ReasonIDToken       Reason = "id_token"
	ReasonCancelled     Reason = "cancelled"
	ReasonError         Reason = "error"
	// ReasonMisconfigured is the deployment's fault, not the upstream IdP's: discovery names another issuer.
	ReasonMisconfigured Reason = "misconfigured"
)

// Error is a failed upstream round trip with its Reason; Err is the cause, for logs.
type Error struct {
	Reason Reason
	Err    error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return "oidcrp: " + string(e.Reason)
	}
	return "oidcrp: " + string(e.Reason) + ": " + e.Err.Error()
}

func (e *Error) Unwrap() error { return e.Err }

// ReasonOf returns the Reason in err's tree, or "" when err is not an *Error.
func ReasonOf(err error) Reason {
	if e, ok := errors.AsType[*Error](err); ok {
		return e.Reason
	}
	return ""
}
