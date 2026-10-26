package federation

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// P8 = A, R-1: a first sign-in Google isn't authoritative for creates nothing and asks the inbox instead.
func TestSignIn_UnprovenEmailCreatesNobody(t *testing.T) {
	h := newHarness(t)
	id := identity(h)
	id.Email = "budi@acme.test"
	_, err := h.svc.SignIn(context.Background(), h.p, id, SessionRequest{})
	uerr, ok := errors.AsType[*UnprovenEmailError](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, "budi@acme.test", uerr.Email)
	assert.Equal(t, "sub-1", uerr.Identity.Subject)
	assert.True(t, IsExpectedError(err))
	assert.Zero(t, h.users.created)
	assert.Empty(t, h.store.rows)
	assert.Empty(t, h.users.signedIn)
}

func TestSignIn_AuthoritativeEmailsSignUpStraightAway(t *testing.T) {
	for name, change := range map[string]func(*Identity){
		"gmail":            func(id *Identity) { id.Email = "ana@gmail.com" },
		"googlemail, case": func(id *Identity) { id.Email = "Ana@GoogleMail.COM" },
		"workspace hd":     func(id *Identity) { id.Email, id.HostedDomain = "ana@acme.test", "acme.test" },
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			id := identity(h)
			change(&id)
			_, err := h.svc.SignIn(context.Background(), h.p, id, SessionRequest{})
			require.NoError(t, err)
			assert.Equal(t, 1, h.users.created)
			assert.Len(t, h.store.rows, 1)
		})
	}
}

func TestSignIn_UnprovenRulesLeaveKnownAndTakenAlone(t *testing.T) {
	t.Run("a known identity signs in", func(t *testing.T) {
		h := newHarness(t)
		id := identity(h)
		_, err := h.svc.SignIn(context.Background(), h.p, id, SessionRequest{})
		require.NoError(t, err)
		id.Email = "ana@acme.test"
		_, err = h.svc.SignIn(context.Background(), h.p, id, SessionRequest{})
		assert.NoError(t, err)
	})
	t.Run("a taken email is a collision", func(t *testing.T) {
		h := newHarness(t)
		id := identity(h)
		id.Email = "budi@acme.test"
		h.users.byEmail[id.Email] = h.p.ID
		_, err := h.svc.SignIn(context.Background(), h.p, id, SessionRequest{})
		assert.True(t, IsCollisionError(err), "%v", err)
	})
}
