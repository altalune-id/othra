package invite_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/internal/kernel/events"
	"github.com/altalune-id/othra/internal/org/invite"
	"github.com/altalune-id/othra/internal/org/member"
)

// With console passwords off, install invites the first owner by email: no inviter, no email sent, renewed on a repeat.
func TestService_InviteOwner(t *testing.T) {
	f := newFixture(t)
	other := uuid.New()
	pending, err := f.svc.PendingOwner(f.ctx, other)
	require.NoError(t, err)
	assert.False(t, pending)

	inv, err := f.svc.InviteOwner(f.ctx, other, " Owner@Example.com ")
	require.NoError(t, err)
	assert.Equal(t, "owner@example.com", inv.Email)
	assert.Equal(t, member.RoleOwner, inv.Role)
	assert.Equal(t, uuid.Nil, inv.InvitedBy)
	assert.Equal(t, f.clock.Add(invite.TTL), inv.ExpiresAt)
	assert.Zero(t, f.mail.count(), "the owner signs in through Othra; nothing is emailed")
	got := f.events.all()
	require.NotEmpty(t, got)
	assert.Equal(t, events.InviteCreated, got[len(got)-1].t)

	pending, err = f.svc.PendingOwner(f.ctx, other)
	require.NoError(t, err)
	assert.True(t, pending)

	f.clock = f.clock.Add(invite.TTL)
	pending, err = f.svc.PendingOwner(f.ctx, other)
	require.NoError(t, err)
	assert.False(t, pending, "an expired owner invite no longer counts")

	again, err := f.svc.InviteOwner(f.ctx, other, "owner@example.com")
	require.NoError(t, err)
	assert.Equal(t, inv.PublicID, again.PublicID, "the live invite is renewed, not duplicated")
	assert.Equal(t, f.clock.Add(invite.TTL), again.ExpiresAt)

	_, err = f.svc.InviteOwner(f.ctx, other, "not an email")
	assert.Error(t, err)
	f.store.Err = assert.AnError
	_, err = f.svc.InviteOwner(f.ctx, other, "owner@example.com")
	assert.Error(t, err)
	_, err = f.svc.PendingOwner(f.ctx, other)
	assert.Error(t, err)
}

// SECURITY: only the seat InviteOwner leaves is claimed by email alone; any other invite needs its link.
func TestInvite_EmailOnlyOwner(t *testing.T) {
	for name, tc := range map[string]struct {
		inv  invite.Invite
		want bool
	}{
		"install's owner seat": {invite.Invite{Role: member.RoleOwner}, true},
		"an owner's invite":    {invite.Invite{Role: member.RoleOwner, InvitedBy: uuid.New()}, false},
		"a member invite":      {invite.Invite{Role: member.RoleMember}, false},
		"an admin invite":      {invite.Invite{Role: member.RoleAdmin}, false},
		"a draft":              {invite.Invite{Role: member.RoleOwner, Draft: true}, false},
	} {
		assert.Equal(t, tc.want, tc.inv.EmailOnlyOwner(), name)
	}
	f := newFixture(t)
	inv, err := f.svc.InviteOwner(f.ctx, uuid.New(), "owner@example.com")
	require.NoError(t, err)
	assert.True(t, inv.EmailOnlyOwner())
}

// Usable reads the service's clock, so a caller outside the package never brings its own.
func TestService_Usable(t *testing.T) {
	f := newFixture(t)
	inv, err := f.svc.InviteOwner(f.ctx, uuid.New(), "owner@example.com")
	require.NoError(t, err)
	require.NoError(t, f.svc.Usable(inv))
	f.clock = f.clock.Add(invite.TTL)
	assert.Error(t, f.svc.Usable(inv), "expired by the service's clock")
}
