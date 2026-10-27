package client

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/apperror"
	clientv1 "github.com/altalune-id/othra/gen/go/client/v1"
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
		{&MetadataManagedError{}, apperror.CodeClientMetadataManaged, IsMetadataManagedError},
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

func TestToProto_SystemRegistration(t *testing.T) {
	c, err := NewSystem(uuid.New(), uuid.New(), "c1", "http://localhost:5150/oauth/callback", "http://localhost:5150/console/", time.Now())
	require.NoError(t, err)
	assert.Equal(t, "Othra console", c.Name, "the hosted pages read \"Sign in to continue to Othra console\"")
	assert.Equal(t, clientv1.ClientRegistration_CLIENT_REGISTRATION_SYSTEM, toProto(c).GetRegistration())
	assert.Empty(t, toProto(c).GetSecrets())
	assert.True(t, IsSystemManagedError(&SystemManagedError{}))
	assert.Equal(t, apperror.CodeClientSystemManaged, (&SystemManagedError{}).ToAppError().Code())
}
