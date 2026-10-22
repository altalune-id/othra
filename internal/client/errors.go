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
		IsSecretPersonOnlyError(err) || IsSecretNotFoundError(err) || IsSecretExistsError(err) || IsProjectNotFoundError(err)
}
