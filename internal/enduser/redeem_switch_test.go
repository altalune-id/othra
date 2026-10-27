package enduser

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
)

type fakeSwitch struct {
	on    bool
	err   error
	calls int
}

func (f *fakeSwitch) LockEmailLink(context.Context, uuid.UUID) (bool, error) {
	f.calls++
	return f.on, f.err
}

// a sign-up link issued before the email link went off creates no end user; an existing one still signs in.
func TestRedeem_EmailLinkOffCreatesNobody(t *testing.T) {
	sw := &fakeSwitch{on: true}
	h := newHarnessWith(t, &fakeGrants{}, sw)
	known := h.signIn(t, "ada@example.com", "")
	newcomer := h.send(t, "bob@example.com")
	again := h.send(t, "ada@example.com")
	third := h.send(t, "cy@example.com")
	assert.Equal(t, 1, sw.calls, "only a new end user reads the switch")

	sw.on = false
	_, err := h.svc.Redeem(context.Background(), h.project, RedeemRequest{Token: tokenOf(t, newcomer.URL)})
	assert.Equal(t, apperror.CodeSignInEmailDisabled, codeOf(err))
	assert.Len(t, h.store.users, 1, "no end user without a method")

	r, err := h.svc.Redeem(context.Background(), h.project, RedeemRequest{Token: tokenOf(t, again.URL)})
	require.NoError(t, err, "an existing end user keeps signing in")
	assert.Equal(t, known.Account.EndUserID, r.Account.EndUserID)

	sw.err = errors.New("db down")
	sw.on = true
	_, err = h.svc.Redeem(context.Background(), h.project, RedeemRequest{Token: tokenOf(t, third.URL)})
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

func TestRedeem_WithoutASwitchTheProjectDecides(t *testing.T) {
	h := newHarness(t)
	l := h.send(t, "bob@example.com")
	h.project.EmailLink = false
	_, err := h.svc.Redeem(context.Background(), h.project, RedeemRequest{Token: tokenOf(t, l.URL)})
	assert.Equal(t, apperror.CodeSignInEmailDisabled, codeOf(err))
	assert.Empty(t, h.store.users)
}

// with the email link off, only a redeem that links the pending Google identity it carries may create the end user.
func TestRedeem_EmailLinkOffExceptionIsNarrow(t *testing.T) {
	h := newHarnessWith(t, &fakeGrants{}, &fakeSwitch{on: false})
	h.project.EmailLink = false
	_, err := h.svc.IssueLink(context.Background(), h.project, LinkRequest{Email: "plain@example.com"})
	assert.Equal(t, apperror.CodeSignInEmailDisabled, codeOf(err), "a plain link still needs the email link")

	issue := func(email string) string {
		l, err := h.svc.IssueLink(context.Background(), h.project, LinkRequest{Email: email, Pending: []byte(`{"sign_up":true}`)})
		require.NoError(t, err, "a pending link is issued with the email link off")
		require.True(t, l.Issued)
		return tokenOf(t, l.URL)
	}
	confirm := func(linked bool) func(context.Context, *Link, uuid.UUID, bool) (bool, error) {
		return func(_ context.Context, _ *Link, _ uuid.UUID, created bool) (bool, error) {
			assert.True(t, created)
			return linked, nil
		}
	}

	_, err = h.svc.Redeem(context.Background(), h.project, RedeemRequest{Token: issue("other@example.com")})
	assert.Equal(t, apperror.CodeSignInEmailDisabled, codeOf(err), "another browser: no confirm, no sign-up")
	_, err = h.svc.Redeem(context.Background(), h.project, RedeemRequest{Token: issue("skip@example.com"), Confirm: confirm(false)})
	assert.Equal(t, apperror.CodeSignInEmailDisabled, codeOf(err), "a confirm that links nothing creates nobody")

	r, err := h.svc.Redeem(context.Background(), h.project, RedeemRequest{Token: issue("budi@acme.test"), Confirm: confirm(true)})
	require.NoError(t, err, "the inbox proves the Google sign-up")
	assert.Equal(t, "budi@acme.test", r.Account.Email)
}
