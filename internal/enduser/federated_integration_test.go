//go:build integration

package enduser_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/internal/enduser"
)

func TestPostgres_Federated(t *testing.T) {
	f := newPGFixture(t)
	p := f.project(t)
	ctx := t.Context()
	id, err := f.svc.CreateFederated(ctx, p, "ana@gmail.com", "Ana")
	require.NoError(t, err)
	_, err = f.svc.CreateFederated(ctx, p, "ana@gmail.com", "")
	require.ErrorIs(t, err, enduser.ErrEmailTaken)

	got, found, err := f.svc.EndUserIDByEmail(ctx, p, "ana@gmail.com")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, id, got)
	_, found, err = f.svc.EndUserIDByEmail(ctx, p, "nobody@gmail.com")
	require.NoError(t, err)
	assert.False(t, found)

	in, err := f.svc.SignInFederated(ctx, p, enduser.FederatedSignIn{EndUserID: id, Method: "google", UserAgent: "UA"})
	require.NoError(t, err)
	assert.Equal(t, "ana@gmail.com", in.Account.Email)
	_, err = f.svc.SignInFederated(ctx, p, enduser.FederatedSignIn{EndUserID: id, Method: "google", BrowserSecret: in.Secret})
	require.NoError(t, err)

	var method, name string
	var verified bool
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT s.method, u.name, u.email_verified_at IS NOT NULL FROM user_sessions s
		JOIN end_users u ON u.id = s.end_user_id WHERE s.public_id = $1`, in.Account.PublicID).Scan(&method, &name, &verified))
	assert.Equal(t, "google", method)
	assert.Equal(t, "Ana", name)
	assert.True(t, verified)
}
