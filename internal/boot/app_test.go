package boot

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/internal/auth"
	"altalune.id/othra/internal/user"
)

type emailAccounts map[string]*user.User

func (a emailAccounts) FindByEmail(_ context.Context, raw string) (*user.User, error) {
	if u, ok := a[raw]; ok {
		return u, nil
	}
	return nil, &user.UnknownEmailError{Email: raw}
}

type seatsByEmail struct {
	org   uuid.UUID
	seats map[string]*auth.Seat
	err   error
}

func (s seatsByEmail) OwnerSeat(_ context.Context, orgID uuid.UUID, email string) (*auth.Seat, error) {
	if s.err != nil || orgID != s.org {
		return nil, s.err
	}
	return s.seats[email], nil
}

// An install with passwords and mail off has only the email-only owner seat; the printed link is how its owner gets in.
func TestLinkTarget(t *testing.T) {
	sys := uuid.New()
	ada := &user.User{Email: "ada@example.com", PublicID: "usr-ada"}
	users := emailAccounts{"ada@example.com": ada}
	seats := seatsByEmail{org: sys, seats: map[string]*auth.Seat{"owner@example.com": {OrgID: sys, Email: "owner@example.com"}}}

	got, err := linkTarget(t.Context(), users, seats, sys, "ada@example.com")
	require.NoError(t, err)
	assert.Equal(t, signInTarget{email: "ada@example.com", user: "usr-ada"}, got)

	got, err = linkTarget(t.Context(), users, seats, sys, "owner@example.com")
	require.NoError(t, err, "the unclaimed owner seat gets a link too")
	assert.Equal(t, signInTarget{email: "owner@example.com"}, got)

	_, err = linkTarget(t.Context(), users, seats, sys, "nobody@example.com")
	assert.True(t, user.IsUnknownEmailError(err), "no account and no seat: the operator sees the unknown email")

	seats.err = assert.AnError
	_, err = linkTarget(t.Context(), users, seats, sys, "nobody@example.com")
	assert.ErrorIs(t, err, assert.AnError)
}
