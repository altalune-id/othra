package enduser

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// N2: a collision link carries the pending identity, skips the cooldown and hands it to Confirm inside the redeem.
func TestPendingLink_ConfirmRunsInTheRedeem(t *testing.T) {
	h := newHarness(t)
	h.send(t, "ada@example.com")
	pending := []byte(`{"Subject":"g-1"}`)
	l, err := h.svc.SendLink(context.Background(), h.project, LinkRequest{Email: "ada@example.com", ReturnTo: "/p/acme/account/", Pending: pending}, compose)
	require.NoError(t, err)
	require.True(t, l.Issued, "a pending link skips the resend cooldown")
	assert.NotEmpty(t, l.PublicID)

	link, err := h.svc.LinkByToken(context.Background(), h.project, tokenOf(t, l.URL))
	require.NoError(t, err)
	assert.Equal(t, pending, link.Pending)
	assert.Equal(t, l.PublicID, link.PublicID)

	var got uuid.UUID
	r, err := h.svc.Redeem(context.Background(), h.project, RedeemRequest{
		Token: tokenOf(t, l.URL),
		Confirm: func(_ context.Context, cl *Link, endUserID uuid.UUID, created bool) (bool, error) {
			assert.Equal(t, pending, cl.Pending)
			assert.True(t, created, "this redeem creates ada")
			got = endUserID
			return true, nil
		},
	})
	require.NoError(t, err)
	assert.Equal(t, r.Account.EndUserID, got)
	assert.Equal(t, MethodMagicLink, r.Account.Method)
}

func TestPendingLink_ConfirmFailureFailsTheRedeem(t *testing.T) {
	h := newHarness(t)
	l, err := h.svc.IssueLink(context.Background(), h.project, LinkRequest{Email: "ada@example.com", Pending: []byte(`{}`)})
	require.NoError(t, err)
	refused := errors.New("refused")
	_, err = h.svc.Redeem(context.Background(), h.project, RedeemRequest{
		Token:   tokenOf(t, l.URL),
		Confirm: func(context.Context, *Link, uuid.UUID, bool) (bool, error) { return false, refused },
	})
	assert.ErrorIs(t, err, refused)
}

func TestPendingLink_PlainLinkNeverConfirms(t *testing.T) {
	h := newHarness(t)
	l := h.send(t, "ada@example.com")
	_, err := h.svc.Redeem(context.Background(), h.project, RedeemRequest{
		Token: tokenOf(t, l.URL),
		Confirm: func(context.Context, *Link, uuid.UUID, bool) (bool, error) {
			t.Fatal("a link without a pending identity confirms nothing")
			return false, nil
		},
	})
	require.NoError(t, err)
}
