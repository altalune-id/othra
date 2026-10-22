package client

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"altalune.id/othra/apperror"
)

func TestErrors_CarryTheirCodes(t *testing.T) {
	for _, tt := range []struct {
		err interface {
			error
			ToAppError() *apperror.AppError
		}
		code apperror.Code
		is   func(error) bool
	}{
		{&NotFoundError{}, apperror.CodeClientNotFound, IsNotFoundError},
		{&InvalidRedirectURIError{Field: "redirect_uris", URI: "http://x", Reason: RedirectURINotCanonical, Canonical: "http://x/"}, apperror.CodeClientInvalidRedirectURI, IsInvalidRedirectURIError},
		{&InvalidSettingsError{Reason: SettingsPostLogoutNative}, apperror.CodeClientInvalidSettings, IsInvalidSettingsError},
		{&InvalidSettingsError{Reason: SettingsPKCENative}, apperror.CodeClientInvalidSettings, IsInvalidSettingsError},
		{&InvalidSettingsError{Reason: SettingsAuthMethodNative}, apperror.CodeClientInvalidSettings, IsInvalidSettingsError},
		{&NoSecretError{}, apperror.CodeClientHasNoSecret, IsNoSecretError},
		{&SecretPersonOnlyError{}, apperror.CodeClientSecretPersonOnly, IsSecretPersonOnlyError},
		{&SecretNotFoundError{}, apperror.CodeClientSecretNotFound, IsSecretNotFoundError},
		{&SecretExistsError{}, apperror.CodeClientSecretExists, IsSecretExistsError},
		{&ProjectNotFoundError{}, apperror.CodeProjectNotFound, IsProjectNotFoundError},
	} {
		assert.Equal(t, tt.code, tt.err.ToAppError().Code(), "%T", tt.err)
		assert.True(t, tt.is(tt.err), "%T", tt.err)
		assert.NotEmpty(t, tt.err.Error())
	}
	assert.False(t, IsNotFoundError(&SecretNotFoundError{}))
}

func TestInvalidRedirectURIError_SendsOnlyWhatItHas(t *testing.T) {
	meta := (&InvalidRedirectURIError{Field: "redirect_uris", Reason: RedirectURIRequired}).ToAppError().Meta()
	assert.Equal(t, map[string]string{"field": "redirect_uris", "reason": "required"}, meta)
}
