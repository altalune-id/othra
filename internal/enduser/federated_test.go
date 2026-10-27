package enduser

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/apperror"
)

func TestCreateFederatedAndFindByEmail(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_, found, err := h.svc.EndUserIDByEmail(ctx, h.project, "ana@gmail.com")
	require.NoError(t, err)
	assert.False(t, found)

	id, err := h.svc.CreateFederated(ctx, h.project, "ana@gmail.com", strings.Repeat("a", nameMax+10))
	require.NoError(t, err)
	got, found, err := h.svc.EndUserIDByEmail(ctx, h.project, "ana@gmail.com")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, id, got)
	assert.Len(t, h.store.users[0].name, nameMax, "the upstream name is clipped")
	assert.NotNil(t, h.store.users[0].verifiedAt, "an upstream-verified email is verified")

	_, err = h.svc.CreateFederated(ctx, h.project, "ana@gmail.com", "")
	assert.ErrorIs(t, err, ErrEmailTaken)

	h.store.failOn = "CreateEndUser"
	_, err = h.svc.CreateFederated(ctx, h.project, "budi@gmail.com", "")
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	h.store.failOn = "EndUserIDByEmail"
	_, _, err = h.svc.EndUserIDByEmail(ctx, h.project, "ana@gmail.com")
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

func TestSignInFederated(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	id, err := h.svc.CreateFederated(ctx, h.project, "ana@gmail.com", "Ana")
	require.NoError(t, err)
	got, err := h.svc.SignInFederated(ctx, h.project, FederatedSignIn{EndUserID: id, Method: "google", UserAgent: "UA"})
	require.NoError(t, err)
	assert.NotEmpty(t, got.Secret)
	assert.Equal(t, id, got.Account.EndUserID)
	assert.Equal(t, "ana@gmail.com", got.Account.Email)
	require.Len(t, h.store.sessions, 1)
	assert.Equal(t, "google", h.store.sessions[0].method)

	again, err := h.svc.SignInFederated(ctx, h.project, FederatedSignIn{EndUserID: id, Method: "google", BrowserSecret: got.Secret})
	require.NoError(t, err)
	assert.NotEqual(t, got.Secret, again.Secret, "the browser secret rotates on every sign-in")
	assert.Len(t, h.store.sessions, 1, "the same browser and end user keep one session")

	_, err = h.svc.SignInFederated(ctx, h.project, FederatedSignIn{EndUserID: uuid.New(), Method: "google"})
	require.Error(t, err, "an end user that isn't there")
}

// FillName gives a name only to an end user who has none, trimmed and clipped.
func TestFillName(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	id, err := h.svc.CreateFederated(ctx, h.project, "budi@acme.test", "")
	require.NoError(t, err)
	require.NoError(t, h.svc.FillName(ctx, h.project, id, "   "), "a blank name is no change")
	assert.Empty(t, h.store.users[0].name)
	require.NoError(t, h.svc.FillName(ctx, h.project, id, "  "+strings.Repeat("b", nameMax+5)))
	assert.Len(t, h.store.users[0].name, nameMax)
	require.NoError(t, h.svc.FillName(ctx, h.project, id, "Other"))
	assert.Len(t, h.store.users[0].name, nameMax, "a name the person has is kept")

	h.store.failOn = "FillName"
	assert.Equal(t, apperror.CodeUnexpected, codeOf(h.svc.FillName(ctx, h.project, id, "X")))
}
