package client

import (
	"errors"

	"altalune.id/othra/apperror"
)

// NotFoundError signals an OAuth client that does not exist or is out of the caller's reach.
type NotFoundError struct{}

func (*NotFoundError) Error() string { return "client: not found" }

// IsNotFoundError reports whether err's tree contains a *NotFoundError.
func IsNotFoundError(err error) bool {
	_, ok := errors.AsType[*NotFoundError](err)
	return ok
}

// ToAppError maps NotFoundError to CLT001.
func (e *NotFoundError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeClientNotFound, "OAuth client not found").WithCause(e)
}

// RedirectURIReason is the stable cause of an InvalidRedirectURIError, sent to clients as meta.reason.
type RedirectURIReason string

// The causes of an InvalidRedirectURIError.
const (
	RedirectURIRequired      RedirectURIReason = "required"
	RedirectURIUnparsable    RedirectURIReason = "unparsable"
	RedirectURIFragment      RedirectURIReason = "fragment"
	RedirectURIWildcard      RedirectURIReason = "wildcard"
	RedirectURIUserinfo      RedirectURIReason = "userinfo"
	RedirectURIDotSegments   RedirectURIReason = "dot_segments"
	RedirectURIHTTPSRequired RedirectURIReason = "https_required"
	RedirectURILoopbackOnly  RedirectURIReason = "loopback_only"
	RedirectURIPortIgnored   RedirectURIReason = "port_ignored"
	RedirectURINotCanonical  RedirectURIReason = "not_canonical"
	RedirectURITooLong       RedirectURIReason = "too_long"
	RedirectURITooMany       RedirectURIReason = "too_many"
	RedirectURIDuplicate     RedirectURIReason = "duplicate"
)

// InvalidRedirectURIError signals a redirect or sign-out return URI that breaks the redirect URI rules.
type InvalidRedirectURIError struct {
	Field     string
	URI       string
	Reason    RedirectURIReason
	Canonical string
}

func (e *InvalidRedirectURIError) Error() string {
	return "client: invalid " + e.Field + " " + apperror.Echo(e.URI) + ": " + string(e.Reason)
}

// IsInvalidRedirectURIError reports whether err's tree contains an *InvalidRedirectURIError.
func IsInvalidRedirectURIError(err error) bool {
	_, ok := errors.AsType[*InvalidRedirectURIError](err)
	return ok
}

// ToAppError maps InvalidRedirectURIError to CLT002 with the field, the URI, the reason and the canonical form.
func (e *InvalidRedirectURIError) ToAppError() *apperror.AppError {
	meta := map[string]string{"reason": string(e.Reason)}
	if e.Field != "" {
		meta["field"] = e.Field
	}
	if e.URI != "" {
		meta["uri"] = apperror.Echo(e.URI)
	}
	if e.Canonical != "" {
		meta["canonical"] = apperror.Echo(e.Canonical)
	}
	return apperror.New(apperror.CodeClientInvalidRedirectURI, "Invalid redirect URI").WithMeta(meta).WithCause(e)
}

// SettingsReason is the stable cause of an InvalidSettingsError, sent to clients as meta.reason.
type SettingsReason string

// The causes of an InvalidSettingsError.
const (
	SettingsPostLogoutNative SettingsReason = "post_logout_native"
	SettingsPKCENative       SettingsReason = "pkce_native"
	SettingsAuthMethodNative SettingsReason = "auth_method_native"
)

// InvalidSettingsError signals client settings that don't fit together.
type InvalidSettingsError struct {
	Reason SettingsReason
}

func (e *InvalidSettingsError) Error() string { return "client: invalid settings: " + string(e.Reason) }

// IsInvalidSettingsError reports whether err's tree contains an *InvalidSettingsError.
func IsInvalidSettingsError(err error) bool {
	_, ok := errors.AsType[*InvalidSettingsError](err)
	return ok
}

// ToAppError maps InvalidSettingsError to CLT003 with the reason.
func (e *InvalidSettingsError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeClientInvalidSettings, "These client settings don't fit together").
		WithMeta(map[string]string{"reason": string(e.Reason)}).WithCause(e)
}

// NoSecretError signals a secret action on a CLI client, which has no secret.
type NoSecretError struct{}

func (*NoSecretError) Error() string { return "client: a CLI client has no secret" }

// IsNoSecretError reports whether err's tree contains a *NoSecretError.
func IsNoSecretError(err error) bool {
	_, ok := errors.AsType[*NoSecretError](err)
	return ok
}

// ToAppError maps NoSecretError to CLT004.
func (e *NoSecretError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeClientHasNoSecret, "CLI and desktop clients have no secret").WithCause(e)
}

// SecretPersonOnlyError signals a secret action by a caller that isn't a signed-in person.
type SecretPersonOnlyError struct{}

func (*SecretPersonOnlyError) Error() string {
	return "client: only a signed-in person manages secrets"
}

// IsSecretPersonOnlyError reports whether err's tree contains a *SecretPersonOnlyError.
func IsSecretPersonOnlyError(err error) bool {
	_, ok := errors.AsType[*SecretPersonOnlyError](err)
	return ok
}

// ToAppError maps SecretPersonOnlyError to CLT005.
func (e *SecretPersonOnlyError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeClientSecretPersonOnly, "Only a signed-in person can create or rotate a secret").WithCause(e)
}

// SecretNotFoundError signals a secret id that is not a live secret of the client.
type SecretNotFoundError struct{}

func (*SecretNotFoundError) Error() string { return "client: secret not found" }

// IsSecretNotFoundError reports whether err's tree contains a *SecretNotFoundError.
func IsSecretNotFoundError(err error) bool {
	_, ok := errors.AsType[*SecretNotFoundError](err)
	return ok
}

// ToAppError maps SecretNotFoundError to CLT006.
func (e *SecretNotFoundError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeClientSecretNotFound, "Secret not found").WithCause(e)
}

// SecretExistsError signals a generate on a client that already has a current secret.
type SecretExistsError struct{}

func (*SecretExistsError) Error() string { return "client: already has a current secret" }

// IsSecretExistsError reports whether err's tree contains a *SecretExistsError.
func IsSecretExistsError(err error) bool {
	_, ok := errors.AsType[*SecretExistsError](err)
	return ok
}

// ToAppError maps SecretExistsError to CLT007.
func (e *SecretExistsError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeClientSecretExists, "This client already has a secret").WithCause(e)
}

// MetadataManagedError signals a change to a CIMD client, which only its metadata document defines; it can only be deleted.
type MetadataManagedError struct{}

func (*MetadataManagedError) Error() string { return "client: managed by its metadata document" }

// IsMetadataManagedError reports whether err's tree contains a *MetadataManagedError.
func IsMetadataManagedError(err error) bool {
	_, ok := errors.AsType[*MetadataManagedError](err)
	return ok
}

// ToAppError maps MetadataManagedError to CLT008.
func (e *MetadataManagedError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeClientMetadataManaged, "This app is managed by its metadata document").WithCause(e)
}

// ProjectNotFoundError signals a project that does not exist or is out of the caller's reach.
type ProjectNotFoundError struct{}

func (*ProjectNotFoundError) Error() string { return "client: project not found" }

// IsProjectNotFoundError reports whether err's tree contains a *ProjectNotFoundError.
func IsProjectNotFoundError(err error) bool {
	_, ok := errors.AsType[*ProjectNotFoundError](err)
	return ok
}

// ToAppError maps ProjectNotFoundError to the project not-found code.
func (e *ProjectNotFoundError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeProjectNotFound, "Project not found").WithCause(e)
}

// IsExpectedError reports whether err is a typed client error a service returns as-is.
func IsExpectedError(err error) bool {
	return IsNotFoundError(err) || IsInvalidRedirectURIError(err) || IsInvalidSettingsError(err) || IsNoSecretError(err) ||
		IsSecretPersonOnlyError(err) || IsSecretNotFoundError(err) || IsSecretExistsError(err) || IsProjectNotFoundError(err) ||
		IsMetadataManagedError(err)
}

// MetadataReason is the stable cause of a *MetadataError; it goes only to the log.
type MetadataReason string

// The causes of a *MetadataError.
const (
	MetadataFetch        MetadataReason = "fetch"
	MetadataTooLarge     MetadataReason = "too_large"
	MetadataContentType  MetadataReason = "content_type"
	MetadataNotJSON      MetadataReason = "not_json"
	MetadataClientID     MetadataReason = "client_id"
	MetadataClientName   MetadataReason = "client_name"
	MetadataRedirectURIs MetadataReason = "redirect_uris"
	MetadataAuthMethod   MetadataReason = "auth_method"
	MetadataGrantTypes   MetadataReason = "grant_types"
	MetadataURI          MetadataReason = "uri"
)

// MetadataError signals a client metadata document that can't be used; Err is the fetch failure, for the log.
type MetadataError struct {
	Reason MetadataReason
	Err    error
}

func (e *MetadataError) Error() string { return "client: metadata document: " + string(e.Reason) }

func (e *MetadataError) Unwrap() error { return e.Err }

// ToAppError maps MetadataError to SIG011; the reason goes only to the log.
func (e *MetadataError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeAuthorizeClientMetadataUnusable, "This app's details can't be used").WithCause(e)
}

// AdmissionReason is the stable cause of a *NotAdmittedError; it goes only to the log.
type AdmissionReason string

// The causes of a *NotAdmittedError.
const (
	AdmissionClientID AdmissionReason = "client_id"
	AdmissionResource AdmissionReason = "resource"
	AdmissionOff      AdmissionReason = "off"
	AdmissionHost     AdmissionReason = "host"
)

// NotAdmittedError signals a metadata document client no resource server of the request admits (ADR-0021 D1, D3).
type NotAdmittedError struct {
	Reason AdmissionReason
}

func (e *NotAdmittedError) Error() string { return "client: not admitted: " + string(e.Reason) }

// IsNotAdmittedError reports whether err's tree contains a *NotAdmittedError.
func IsNotAdmittedError(err error) bool {
	_, ok := errors.AsType[*NotAdmittedError](err)
	return ok
}

// ToAppError maps NotAdmittedError to SIG010, one answer for every miss so the page is no oracle.
func (e *NotAdmittedError) ToAppError() *apperror.AppError {
	return apperror.New(apperror.CodeAuthorizeClientNotAdmitted, "This app isn't allowed to sign in").WithCause(e)
}

// IsMetadataError reports whether err's tree contains a *MetadataError.
func IsMetadataError(err error) bool {
	_, ok := AsMetadataError(err)
	return ok
}

// AsMetadataError returns the *MetadataError in err's tree.
func AsMetadataError(err error) (*MetadataError, bool) {
	return errors.AsType[*MetadataError](err)
}
