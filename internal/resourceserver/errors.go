package resourceserver

import (
	"errors"
	"strings"
	"time"

	"altalune.id/othra/apperror"
)

// NotFoundError signals a resource server that does not exist or is out of the caller's reach.
type NotFoundError struct{}

func (*NotFoundError) Error() string { return "resourceserver: not found" }

// IsNotFoundError reports whether err's tree contains a *NotFoundError.
func IsNotFoundError(err error) bool {
	_, ok := errors.AsType[*NotFoundError](err)
	return ok
}

// ToAppError maps NotFoundError to RSV001.
func (e *NotFoundError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeResourceServerNotFound, "Resource server not found").WithCause(e)
}

// IdentifierReason is the stable cause of an InvalidIdentifierError, sent to clients as meta.reason.
type IdentifierReason string

// The causes of an InvalidIdentifierError.
const (
	IdentifierUnparsable    IdentifierReason = "unparsable"
	IdentifierHTTPSRequired IdentifierReason = "https_required"
	IdentifierQuery         IdentifierReason = "query"
	IdentifierFragment      IdentifierReason = "fragment"
	IdentifierUserinfo      IdentifierReason = "userinfo"
	IdentifierNotCanonical  IdentifierReason = "not_canonical"
	IdentifierTooLong       IdentifierReason = "too_long"
	IdentifierNeedsTrust    IdentifierReason = "needs_trust"
)

// InvalidIdentifierError signals a resource server identifier that breaks the identifier rules.
type InvalidIdentifierError struct {
	Identifier string
	Reason     IdentifierReason
	Canonical  string
}

func (e *InvalidIdentifierError) Error() string {
	return "resourceserver: invalid identifier " + apperror.Echo(e.Identifier) + ": " + string(e.Reason)
}

// IsInvalidIdentifierError reports whether err's tree contains an *InvalidIdentifierError.
func IsInvalidIdentifierError(err error) bool {
	_, ok := errors.AsType[*InvalidIdentifierError](err)
	return ok
}

// ToAppError maps InvalidIdentifierError to RSV002 with the reason and the canonical form.
func (e *InvalidIdentifierError) ToAppError() *apperror.AppError {
	meta := map[string]string{"reason": string(e.Reason)}
	if e.Canonical != "" {
		meta["canonical"] = apperror.Echo(e.Canonical)
	}
	return apperror.New(apperror.CodeResourceServerInvalidIdentifier, "Invalid identifier").WithMeta(meta).WithCause(e)
}

// IdentifierTakenError signals an identifier another resource server of the project already has.
type IdentifierTakenError struct {
	Identifier string
}

func (e *IdentifierTakenError) Error() string {
	return "resourceserver: identifier already taken: " + apperror.Echo(e.Identifier)
}

// IsIdentifierTakenError reports whether err's tree contains an *IdentifierTakenError.
func IsIdentifierTakenError(err error) bool {
	_, ok := errors.AsType[*IdentifierTakenError](err)
	return ok
}

// ToAppError maps IdentifierTakenError to RSV003.
func (e *IdentifierTakenError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeResourceServerIdentifierTaken, "Identifier already taken").WithCause(e)
}

// ScopeReason is the stable cause of an InvalidScopeError, sent to clients as meta.reason.
type ScopeReason string

// The causes of an InvalidScopeError.
const (
	ScopeFormat    ScopeReason = "format"
	ScopeReserved  ScopeReason = "reserved"
	ScopeDuplicate ScopeReason = "duplicate"
	ScopeTooMany   ScopeReason = "too_many"
)

// InvalidScopeError signals a resource server scope that breaks the scope rules.
type InvalidScopeError struct {
	Scope  string
	Reason ScopeReason
}

func (e *InvalidScopeError) Error() string {
	return "resourceserver: invalid scope " + apperror.Echo(e.Scope) + ": " + string(e.Reason)
}

// IsInvalidScopeError reports whether err's tree contains an *InvalidScopeError.
func IsInvalidScopeError(err error) bool {
	_, ok := errors.AsType[*InvalidScopeError](err)
	return ok
}

// ToAppError maps InvalidScopeError to RSV004 with the scope and the reason.
func (e *InvalidScopeError) ToAppError() *apperror.AppError {
	meta := map[string]string{"reason": string(e.Reason)}
	if e.Scope != "" {
		meta["scope"] = apperror.Echo(e.Scope)
	}
	return apperror.New(apperror.CodeResourceServerInvalidScope, "Invalid scope").WithMeta(meta).WithCause(e)
}

// ScopePinnedError signals the removal of a scope that links still allow.
type ScopePinnedError struct {
	Scope   string
	Clients []string
}

func (e *ScopePinnedError) Error() string {
	return "resourceserver: scope " + apperror.Echo(e.Scope) + " is still allowed by links"
}

// IsScopePinnedError reports whether err's tree contains a *ScopePinnedError.
func IsScopePinnedError(err error) bool {
	_, ok := errors.AsType[*ScopePinnedError](err)
	return ok
}

// ToAppError maps ScopePinnedError to RSV005 with the scope and the client names.
func (e *ScopePinnedError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeResourceServerScopePinned, "Scope is still allowed for linked clients").
		WithMeta(map[string]string{"scope": apperror.Echo(e.Scope), "clients": names(e.Clients)}).WithCause(e)
}

// LinkedError signals a delete of a resource server that clients are still linked to.
type LinkedError struct {
	Clients []string
}

func (*LinkedError) Error() string { return "resourceserver: clients are still linked" }

// IsLinkedError reports whether err's tree contains a *LinkedError.
func IsLinkedError(err error) bool {
	_, ok := errors.AsType[*LinkedError](err)
	return ok
}

// ToAppError maps LinkedError to RSV006 with the client names.
func (e *LinkedError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeResourceServerLinked, "Clients are still linked").
		WithMeta(map[string]string{"clients": names(e.Clients)}).WithCause(e)
}

// TrustSelfHostedOnlyError signals trust without verification outside self-hosted mode.
type TrustSelfHostedOnlyError struct{}

func (*TrustSelfHostedOnlyError) Error() string {
	return "resourceserver: trust without verification is self-hosted only"
}

// IsTrustSelfHostedOnlyError reports whether err's tree contains a *TrustSelfHostedOnlyError.
func IsTrustSelfHostedOnlyError(err error) bool {
	_, ok := errors.AsType[*TrustSelfHostedOnlyError](err)
	return ok
}

// ToAppError maps TrustSelfHostedOnlyError to RSV007.
func (e *TrustSelfHostedOnlyError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeResourceServerTrustSelfHostedOnly, "Trust without verification is only for self-hosted Othra").
		WithCause(e)
}

// VerifyTooSoonError signals a manual check before the rate limit allows the next one.
type VerifyTooSoonError struct {
	RetryAt time.Time
}

func (e *VerifyTooSoonError) Error() string {
	return "resourceserver: checked a moment ago, retry at " + e.RetryAt.UTC().Format(time.RFC3339)
}

// IsVerifyTooSoonError reports whether err's tree contains a *VerifyTooSoonError.
func IsVerifyTooSoonError(err error) bool {
	_, ok := errors.AsType[*VerifyTooSoonError](err)
	return ok
}

// ToAppError maps VerifyTooSoonError to RSV008 with the retry time in RFC 3339.
func (e *VerifyTooSoonError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeResourceServerVerifyTooSoon, "Checked a moment ago").
		WithMeta(map[string]string{"time": e.RetryAt.UTC().Format(time.RFC3339)}).WithCause(e)
}

// LinkScopesReason is the stable cause of an InvalidLinkScopesError, sent to clients as meta.reason.
type LinkScopesReason string

// The causes of an InvalidLinkScopesError.
const (
	LinkScopesEmpty        LinkScopesReason = "empty"
	LinkScopesNotInCatalog LinkScopesReason = "not_in_catalog"
)

// InvalidLinkScopesError signals link scopes that are empty or not the resource server's.
type InvalidLinkScopesError struct {
	Scope  string
	Reason LinkScopesReason
}

func (e *InvalidLinkScopesError) Error() string {
	return "resourceserver: invalid link scopes: " + string(e.Reason)
}

// IsInvalidLinkScopesError reports whether err's tree contains an *InvalidLinkScopesError.
func IsInvalidLinkScopesError(err error) bool {
	_, ok := errors.AsType[*InvalidLinkScopesError](err)
	return ok
}

// ToAppError maps InvalidLinkScopesError to RSV009 with the scope and the reason.
func (e *InvalidLinkScopesError) ToAppError() *apperror.AppError {
	meta := map[string]string{"reason": string(e.Reason)}
	if e.Scope != "" {
		meta["scope"] = apperror.Echo(e.Scope)
	}
	return apperror.New(apperror.CodeResourceServerInvalidLinkScopes, "Invalid allowed scopes").WithMeta(meta).WithCause(e)
}

// ClientNotFoundError signals a linked client that does not exist or is out of the caller's reach.
type ClientNotFoundError struct{}

func (*ClientNotFoundError) Error() string { return "resourceserver: client not found" }

// IsClientNotFoundError reports whether err's tree contains a *ClientNotFoundError.
func IsClientNotFoundError(err error) bool {
	_, ok := errors.AsType[*ClientNotFoundError](err)
	return ok
}

// ToAppError maps ClientNotFoundError to CLT001.
func (e *ClientNotFoundError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeClientNotFound, "OAuth client not found").WithCause(e)
}

// ProjectNotFoundError signals a project that does not exist or is out of the caller's reach.
type ProjectNotFoundError struct{}

func (*ProjectNotFoundError) Error() string { return "resourceserver: project not found" }

// IsProjectNotFoundError reports whether err's tree contains a *ProjectNotFoundError.
func IsProjectNotFoundError(err error) bool {
	_, ok := errors.AsType[*ProjectNotFoundError](err)
	return ok
}

// ToAppError maps ProjectNotFoundError to the project not-found code.
func (e *ProjectNotFoundError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeProjectNotFound, "Project not found").WithCause(e)
}

func names(clients []string) string {
	echoed := make([]string, 0, len(clients))
	for _, c := range clients {
		echoed = append(echoed, apperror.Echo(c))
	}
	return apperror.Echo(strings.Join(echoed, ", "))
}

// IsExpectedError reports whether err is a typed resourceserver error a service returns as-is.
func IsExpectedError(err error) bool {
	return IsNotFoundError(err) || IsInvalidIdentifierError(err) || IsIdentifierTakenError(err) || IsInvalidScopeError(err) ||
		IsScopePinnedError(err) || IsLinkedError(err) || IsTrustSelfHostedOnlyError(err) || IsVerifyTooSoonError(err) ||
		IsInvalidLinkScopesError(err) || IsClientNotFoundError(err) || IsProjectNotFoundError(err)
}
