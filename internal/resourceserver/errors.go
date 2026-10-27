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
	// ScopeDescription is a scope description outside 1 to 120 characters.
	ScopeDescription ScopeReason = "description"
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

// ScopePinnedError signals the removal of a scope that links, or the CIMD admission when CIMD is set, still allow.
type ScopePinnedError struct {
	Scope   string
	Clients []string
	CIMD    bool
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
	meta := map[string]string{"scope": apperror.Echo(e.Scope), "clients": names(e.Clients)}
	if e.CIMD {
		meta["reason"] = "cimd_admission"
	}
	return apperror.New(apperror.CodeResourceServerScopePinned, "Scope is still allowed for linked clients").WithMeta(meta).WithCause(e)
}

// HostReason is the stable cause of an InvalidClientHostError, sent to clients as meta.reason.
type HostReason string

// The causes of an InvalidClientHostError.
const (
	HostFormat    HostReason = "format"
	HostIPLiteral HostReason = "ip_literal"
	HostDuplicate HostReason = "duplicate"
	HostTooMany   HostReason = "too_many"
)

// InvalidClientHostError signals a CIMD allowed host that is not a lower-case DNS name, repeats, or one too many.
type InvalidClientHostError struct {
	Host   string
	Reason HostReason
}

func (e *InvalidClientHostError) Error() string {
	return "resourceserver: invalid CIMD host " + apperror.Echo(e.Host) + ": " + string(e.Reason)
}

// IsInvalidClientHostError reports whether err's tree contains an *InvalidClientHostError.
func IsInvalidClientHostError(err error) bool {
	_, ok := errors.AsType[*InvalidClientHostError](err)
	return ok
}

// ToAppError maps InvalidClientHostError to RSV010 with the host and the reason.
func (e *InvalidClientHostError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeResourceServerInvalidClientHost, "Invalid host").
		WithMeta(map[string]string{"host": apperror.Echo(e.Host), "reason": string(e.Reason)}).WithCause(e)
}

// UnverifiedReason is what an AdmissionUnverifiedError refused to turn on, sent to clients as meta.reason.
type UnverifiedReason string

// The causes of an AdmissionUnverifiedError.
const (
	UnverifiedCIMD           UnverifiedReason = "cimd"
	UnverifiedPersonalTokens UnverifiedReason = "personal_tokens"
)

// AdmissionUnverifiedError signals CIMD admission or personal tokens turned on for a server that is neither verified nor trusted.
type AdmissionUnverifiedError struct{ Reason UnverifiedReason }

func (e *AdmissionUnverifiedError) Error() string {
	return "resourceserver: " + string(e.Reason) + " needs a verified server"
}

// IsAdmissionUnverifiedError reports whether err's tree contains an *AdmissionUnverifiedError.
func IsAdmissionUnverifiedError(err error) bool {
	_, ok := errors.AsType[*AdmissionUnverifiedError](err)
	return ok
}

// ToAppError maps AdmissionUnverifiedError to RSV011 with what was refused.
func (e *AdmissionUnverifiedError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeResourceServerAdmissionUnverified, "Verify the resource server first").
		WithMeta(map[string]string{"reason": string(e.Reason)}).WithCause(e)
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
		IsInvalidLinkScopesError(err) || IsClientNotFoundError(err) || IsProjectNotFoundError(err) || IsInvalidClientHostError(err) ||
		IsAdmissionUnverifiedError(err) || IsClientManagedError(err) || IsClientSystemManagedError(err) || IsSystemProjectError(err) ||
		IsIntrospectionRequiredError(err) ||
		IsIntrospectionSecretExistsError(err) || IsIntrospectionSecretNotFoundError(err)
}

// SystemProjectError signals a new resource server in the console project, which takes none.
type SystemProjectError struct{}

func (*SystemProjectError) Error() string {
	return "resourceserver: the console project takes no resource server"
}

// IsSystemProjectError reports whether err's tree contains a *SystemProjectError.
func IsSystemProjectError(err error) bool {
	_, ok := errors.AsType[*SystemProjectError](err)
	return ok
}

// ToAppError maps SystemProjectError to PRJ011.
func (e *SystemProjectError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeProjectSystemManaged, "Othra manages the console project").WithCause(e)
}

// ClientSystemManagedError signals a link change for the console client, which Othra manages.
type ClientSystemManagedError struct{}

func (*ClientSystemManagedError) Error() string {
	return "resourceserver: the console client takes no links"
}

// IsClientSystemManagedError reports whether err's tree contains a *ClientSystemManagedError.
func IsClientSystemManagedError(err error) bool {
	_, ok := errors.AsType[*ClientSystemManagedError](err)
	return ok
}

// ToAppError maps ClientSystemManagedError to CLT009.
func (e *ClientSystemManagedError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeClientSystemManaged, "Othra manages the console app").WithCause(e)
}

// ClientManagedError signals a link change for a metadata document client, which only its admission sets.
type ClientManagedError struct{}

func (*ClientManagedError) Error() string {
	return "resourceserver: client managed by its metadata document"
}

// IsClientManagedError reports whether err's tree contains a *ClientManagedError.
func IsClientManagedError(err error) bool {
	_, ok := errors.AsType[*ClientManagedError](err)
	return ok
}

// ToAppError maps ClientManagedError to CLT008.
func (e *ClientManagedError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeClientMetadataManaged, "This app is managed by its metadata document").WithCause(e)
}

// IntrospectionRequiredError signals personal tokens switched on for a server without an introspection secret (RFC 7662 §2.1).
type IntrospectionRequiredError struct{}

func (*IntrospectionRequiredError) Error() string {
	return "resourceserver: personal tokens need an introspection secret"
}

// IsIntrospectionRequiredError reports whether err's tree contains an *IntrospectionRequiredError.
func IsIntrospectionRequiredError(err error) bool {
	_, ok := errors.AsType[*IntrospectionRequiredError](err)
	return ok
}

// ToAppError maps IntrospectionRequiredError to RSV012.
func (e *IntrospectionRequiredError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeResourceServerIntrospectionRequired, "Create an introspection credential first").WithCause(e)
}

// IntrospectionSecretExistsError signals a Generate on a server that has a current introspection secret; rotate it instead.
type IntrospectionSecretExistsError struct{}

func (*IntrospectionSecretExistsError) Error() string {
	return "resourceserver: the server has an introspection secret"
}

// IsIntrospectionSecretExistsError reports whether err's tree contains an *IntrospectionSecretExistsError.
func IsIntrospectionSecretExistsError(err error) bool {
	_, ok := errors.AsType[*IntrospectionSecretExistsError](err)
	return ok
}

// ToAppError maps IntrospectionSecretExistsError to GEN006.
func (e *IntrospectionSecretExistsError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeAlreadyExists, "The server already has an introspection secret; rotate it").WithCause(e)
}

// IntrospectionSecretNotFoundError signals an introspection secret that is not one of the server's live ones.
type IntrospectionSecretNotFoundError struct{}

func (*IntrospectionSecretNotFoundError) Error() string {
	return "resourceserver: introspection secret not found"
}

// IsIntrospectionSecretNotFoundError reports whether err's tree contains an *IntrospectionSecretNotFoundError.
func IsIntrospectionSecretNotFoundError(err error) bool {
	_, ok := errors.AsType[*IntrospectionSecretNotFoundError](err)
	return ok
}

// ToAppError maps IntrospectionSecretNotFoundError to GEN005.
func (e *IntrospectionSecretNotFoundError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeNotFound, "Introspection secret not found").WithCause(e)
}
