package resourceserver

import (
	"strings"
	"testing"
	"time"

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
		{&NotFoundError{}, apperror.CodeResourceServerNotFound, IsNotFoundError},
		{&InvalidIdentifierError{Identifier: "https://A.example", Reason: IdentifierNotCanonical, Canonical: "https://a.example"}, apperror.CodeResourceServerInvalidIdentifier, IsInvalidIdentifierError},
		{&IdentifierTakenError{Identifier: "https://a.example"}, apperror.CodeResourceServerIdentifierTaken, IsIdentifierTakenError},
		{&InvalidScopeError{Scope: "openid", Reason: ScopeReserved}, apperror.CodeResourceServerInvalidScope, IsInvalidScopeError},
		{&ScopePinnedError{Scope: "read", Clients: []string{"Web", "CLI"}}, apperror.CodeResourceServerScopePinned, IsScopePinnedError},
		{&LinkedError{Clients: []string{"Web"}}, apperror.CodeResourceServerLinked, IsLinkedError},
		{&TrustSelfHostedOnlyError{}, apperror.CodeResourceServerTrustSelfHostedOnly, IsTrustSelfHostedOnlyError},
		{&InvalidClientHostError{Host: "x", Reason: HostFormat}, apperror.CodeResourceServerInvalidClientHost, IsInvalidClientHostError},
		{&AdmissionUnverifiedError{Reason: UnverifiedCIMD}, apperror.CodeResourceServerAdmissionUnverified, IsAdmissionUnverifiedError},
		{&VerifyTooSoonError{RetryAt: time.Unix(0, 0)}, apperror.CodeResourceServerVerifyTooSoon, IsVerifyTooSoonError},
		{&InvalidLinkScopesError{Scope: "x", Reason: LinkScopesNotInCatalog}, apperror.CodeResourceServerInvalidLinkScopes, IsInvalidLinkScopesError},
		{&ClientNotFoundError{}, apperror.CodeClientNotFound, IsClientNotFoundError},
		{&ProjectNotFoundError{}, apperror.CodeProjectNotFound, IsProjectNotFoundError},
	} {
		assert.Equal(t, tt.code, tt.err.ToAppError().Code(), "%T", tt.err)
		assert.True(t, tt.is(tt.err), "%T", tt.err)
		assert.NotEmpty(t, tt.err.Error())
	}
	assert.False(t, IsNotFoundError(&ClientNotFoundError{}))
}

func TestErrors_Meta(t *testing.T) {
	assert.Equal(t, "Web, CLI", (&LinkedError{Clients: []string{"Web", "CLI"}}).ToAppError().Meta()["clients"])
	assert.Equal(t, "1970-01-01T00:00:00Z", (&VerifyTooSoonError{RetryAt: time.Unix(0, 0)}).ToAppError().Meta()["time"])
	assert.Equal(t, map[string]string{"reason": "empty"}, (&InvalidLinkScopesError{Reason: LinkScopesEmpty}).ToAppError().Meta())
	assert.Equal(t, map[string]string{"reason": "too_many"}, (&InvalidScopeError{Reason: ScopeTooMany}).ToAppError().Meta())
	assert.Equal(t, map[string]string{"reason": "https_required"}, (&InvalidIdentifierError{Reason: IdentifierHTTPSRequired}).ToAppError().Meta())
	long := strings.Repeat("n", 10*apperror.EchoLimit)
	assert.LessOrEqual(t, len((&LinkedError{Clients: []string{long, long}}).ToAppError().Meta()["clients"]), apperror.EchoLimit)
}
