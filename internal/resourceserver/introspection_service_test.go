package resourceserver

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/kernel/secret"
)

// RFC 7662 §2.1, M8 P3: a resource server introspects with its own secret: shown once, hashed, rotatable with a grace.
func TestService_IntrospectionSecrets(t *testing.T) {
	h := newHarness(t, false)
	ctx := context.Background()
	r := h.create(t, "https://mcp.example.com/mcp")

	got, first, err := h.svc.GenerateIntrospectionSecret(ctx, h.org, h.project, r.PublicID)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(first, IntrospectionSecretPrefix))
	require.Len(t, got.IntrospectionSecrets, 1)
	assert.Equal(t, first[len(first)-4:], got.IntrospectionSecrets[0].Hint)
	assert.True(t, h.store.secrets[r.ID][0].hash == secret.Versioned(first), "only the hash is kept")

	_, _, err = h.svc.GenerateIntrospectionSecret(ctx, h.org, h.project, r.PublicID)
	assert.Equal(t, apperror.CodeAlreadyExists, codeOf(err), "a second Generate rotates instead")

	rotated, second, err := h.svc.RotateIntrospectionSecret(ctx, h.org, h.project, r.PublicID, secret.GraceHour)
	require.NoError(t, err)
	assert.NotEqual(t, first, second)
	require.Len(t, rotated.IntrospectionSecrets, 2, "the old secret works through its grace")

	caller, ok, err := h.svc.Authenticate(ctx, h.org, h.project, r.PublicID, first)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, r.ID, caller.ID)
	_, ok, err = h.svc.Authenticate(ctx, h.org, h.project, r.PublicID, second)
	require.NoError(t, err)
	assert.True(t, ok)

	h.now = h.now.Add(time.Hour)
	_, ok, err = h.svc.Authenticate(ctx, h.org, h.project, r.PublicID, first)
	require.NoError(t, err)
	assert.False(t, ok, "the grace is over")

	_, ok, err = h.svc.Authenticate(ctx, h.org, h.project, r.PublicID, "othra_rs_wrong")
	require.NoError(t, err)
	assert.False(t, ok)
	_, ok, err = h.svc.Authenticate(ctx, h.org, h.project, "r"+uuid.NewString()[:20], second)
	require.NoError(t, err)
	assert.False(t, ok, "an unknown server is a miss, never an error")

	current, _ := rotated.CurrentIntrospectionSecret()
	revoked, err := h.svc.RevokeIntrospectionSecret(ctx, h.org, h.project, r.PublicID, current.PublicID)
	require.NoError(t, err)
	assert.Empty(t, revoked.IntrospectionSecrets)
	_, err = h.svc.RevokeIntrospectionSecret(ctx, h.org, h.project, r.PublicID, current.PublicID)
	assert.Equal(t, apperror.CodeNotFound, codeOf(err))
	_, ok, err = h.svc.Authenticate(ctx, h.org, h.project, r.PublicID, second)
	require.NoError(t, err)
	assert.False(t, ok, "a revoked secret stops at once")

	_, _, err = h.svc.RotateIntrospectionSecret(ctx, h.org, h.project, "r"+uuid.NewString()[:20], secret.GraceNow)
	assert.True(t, IsNotFoundError(err))
}

// M8 P6, RSV012: personal tokens are switched on only while the server has an introspection secret.
func TestService_PersonalTokenAccess(t *testing.T) {
	h := newHarness(t, false)
	ctx := context.Background()
	r := h.create(t, "https://mcp.example.com/mcp")
	other := h.create(t, "https://other.example.com/mcp")

	_, err := h.svc.SetPersonalTokenAccess(ctx, h.org, h.project, r.PublicID, true)
	assert.Equal(t, apperror.CodeResourceServerAdmissionUnverified, codeOf(err), "R-1: a pending server first")
	assert.Equal(t, "personal_tokens", reasonOf(err), "design 9: personal-token wording, not CIMD's")
	_, _, err = h.svc.Verify(ctx, h.org, h.project, r.PublicID)
	require.NoError(t, err)
	_, err = h.svc.SetPersonalTokenAccess(ctx, h.org, h.project, r.PublicID, true)
	assert.Equal(t, apperror.CodeResourceServerIntrospectionRequired, codeOf(err))

	_, _, err = h.svc.GenerateIntrospectionSecret(ctx, h.org, h.project, r.PublicID)
	require.NoError(t, err)
	on, err := h.svc.SetPersonalTokenAccess(ctx, h.org, h.project, r.PublicID, true)
	require.NoError(t, err)
	assert.True(t, on.AcceptPersonalTokens)
	assert.True(t, on.TakesPersonalTokens(h.now, false))

	servers, err := h.svc.PersonalTokenServers(ctx, h.org, h.project)
	require.NoError(t, err)
	require.Len(t, servers, 1)
	assert.Equal(t, r.ID, servers[0].ID, "only a switched-on server with a secret")
	byID, err := h.svc.ByID(ctx, h.org, h.project, other.ID)
	require.NoError(t, err)
	assert.False(t, byID.TakesPersonalTokens(h.now, false))

	current, _ := on.CurrentIntrospectionSecret()
	_, err = h.svc.RevokeIntrospectionSecret(ctx, h.org, h.project, r.PublicID, current.PublicID)
	require.NoError(t, err)
	servers, err = h.svc.PersonalTokenServers(ctx, h.org, h.project)
	require.NoError(t, err)
	assert.Empty(t, servers, "without a live secret the server takes no tokens, whatever the switch says")

	off, err := h.svc.SetPersonalTokenAccess(ctx, h.org, h.project, r.PublicID, false)
	require.NoError(t, err, "turning off is always allowed")
	assert.False(t, off.AcceptPersonalTokens)

	h.store.failOn = "secrets"
	_, err = h.svc.PersonalTokenServers(ctx, h.org, h.project)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	_, _, err = h.svc.Authenticate(ctx, h.org, h.project, r.PublicID, "x")
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	_, err = h.svc.ByID(ctx, h.org, h.project, r.ID)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

// R-1, profile O20: personal tokens work only on an admissible server; one that fails its re-check stops taking them.
func TestService_PersonalTokensNeedAnAdmissibleServer(t *testing.T) {
	h := newHarness(t, false)
	ctx := context.Background()
	r := h.create(t, "https://mcp.example.com/mcp")
	_, _, err := h.svc.Verify(ctx, h.org, h.project, r.PublicID)
	require.NoError(t, err)
	_, _, err = h.svc.GenerateIntrospectionSecret(ctx, h.org, h.project, r.PublicID)
	require.NoError(t, err)
	on, err := h.svc.SetPersonalTokenAccess(ctx, h.org, h.project, r.PublicID, true)
	require.NoError(t, err)
	require.True(t, on.TakesPersonalTokens(h.now, false))

	h.checker.failure = &Failure{Reason: FailureResourceMismatch, Resource: "https://elsewhere.example.com/mcp"}
	h.now = h.now.Add(25 * time.Hour)
	_, err = h.svc.Recheck(ctx, h.org, h.project)
	require.NoError(t, err)
	failed, err := h.svc.ByID(ctx, h.org, h.project, r.ID)
	require.NoError(t, err)
	require.Equal(t, StateFailed, failed.State)
	assert.True(t, failed.AcceptPersonalTokens, "the switch is untouched")
	assert.False(t, failed.TakesPersonalTokens(h.now, false), "a failed server takes no personal tokens")
	servers, err := h.svc.PersonalTokenServers(ctx, h.org, h.project)
	require.NoError(t, err)
	assert.Empty(t, servers)

	_, err = h.svc.SetPersonalTokenAccess(ctx, h.org, h.project, r.PublicID, true)
	assert.Equal(t, apperror.CodeResourceServerAdmissionUnverified, codeOf(err), "like SetAdmission, on needs an admissible server")
	assert.Equal(t, "personal_tokens", reasonOf(err))
	_, err = h.svc.SetPersonalTokenAccess(ctx, h.org, h.project, r.PublicID, false)
	require.NoError(t, err, "off is always allowed")
}
