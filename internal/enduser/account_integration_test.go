//go:build integration

package enduser_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/internal/enduser"
)

func (f *pgFixture) signIn(t *testing.T, p enduser.Project, email, agent string) *enduser.Redeemed {
	t.Helper()
	token, _ := f.link(t, p, email)
	r, err := f.svc.Redeem(t.Context(), p, enduser.RedeemRequest{Token: token, UserAgent: agent})
	require.NoError(t, err)
	f.now = f.now.Add(time.Minute)
	return r
}

// M8 P9: an end user's live sessions on every browser; ending one checks it is theirs and live.
func TestPostgres_SessionsAndEndingThem(t *testing.T) {
	f := newPGFixture(t)
	ctx := t.Context()
	p := f.project(t)
	phone := f.signIn(t, p, "ada@example.com", "phone-agent")
	laptop := f.signIn(t, p, "ada@example.com", "laptop-agent")
	bob := f.signIn(t, p, "bob@example.com", "bob-agent")
	ada := laptop.Account.EndUserID

	live, err := f.svc.Sessions(ctx, p, ada)
	require.NoError(t, err)
	require.Len(t, live, 2)
	assert.Equal(t, laptop.Account.PublicID, live[0].PublicID, "the most recently seen first")
	assert.Equal(t, "laptop-agent", live[0].UserAgent)
	assert.Equal(t, enduser.MethodMagicLink, live[0].Method)

	assert.True(t, enduser.IsSessionNotFoundError(f.svc.EndUserSession(ctx, p, ada, bob.Account.PublicID)), "not theirs")
	require.NoError(t, f.svc.EndUserSession(ctx, p, ada, phone.Account.PublicID))
	assert.True(t, enduser.IsSessionNotFoundError(f.svc.EndUserSession(ctx, p, ada, phone.Account.PublicID)), "already ended")
	n, err := f.svc.EndOtherSessions(ctx, p, ada, laptop.Account.PublicID)
	require.NoError(t, err)
	assert.Zero(t, n)
	live, err = f.svc.Sessions(ctx, p, ada)
	require.NoError(t, err)
	assert.Len(t, live, 1)
	other := f.project(t)
	none, err := f.svc.Sessions(ctx, other, ada)
	require.NoError(t, err)
	assert.Empty(t, none, "RLS: another project sees nothing")
}

func TestPostgres_ProfileAndList(t *testing.T) {
	f := newPGFixture(t)
	ctx := t.Context()
	p := f.project(t)
	for _, e := range []string{"ada@example.com", "a_b@example.com", "bob@example.com"} {
		f.signIn(t, p, e, "agent")
	}
	page, next, err := f.svc.ListEndUsers(ctx, p, "", "", 2)
	require.NoError(t, err)
	require.Len(t, page, 2)
	assert.Equal(t, "bob@example.com", page[0].Email, "newest first")
	assert.NotNil(t, page[0].LastSignInAt)
	rest, next2, err := f.svc.ListEndUsers(ctx, p, "", next, 2)
	require.NoError(t, err)
	require.Len(t, rest, 1)
	assert.Equal(t, "ada@example.com", rest[0].Email)
	assert.Empty(t, next2)
	under, _, err := f.svc.ListEndUsers(ctx, p, "a_", "", 10)
	require.NoError(t, err)
	require.Len(t, under, 1, "an underscore is matched literally, not as a LIKE wildcard")
	assert.Equal(t, "a_b@example.com", under[0].Email)
	gone, _, err := f.svc.ListEndUsers(ctx, p, "", "nope", 10)
	require.NoError(t, err)
	assert.Empty(t, gone, "an unknown cursor lists nothing")

	ada, err := f.svc.ByPublicID(ctx, p, rest[0].Subject)
	require.NoError(t, err)
	named, err := f.svc.UpdateName(ctx, p, ada.ID, " Ada Lovelace ")
	require.NoError(t, err)
	assert.Equal(t, "Ada Lovelace", named.Name)
	_, err = f.svc.UpdateName(ctx, p, uuid.New(), "x")
	assert.True(t, enduser.IsEndUserNotFoundError(err))
	_, err = f.svc.ByPublicID(ctx, f.project(t), ada.Subject)
	assert.True(t, enduser.IsEndUserNotFoundError(err), "RLS: another project's lookup")
}
