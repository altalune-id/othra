package pat

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	accountv1 "altalune.id/othra/gen/go/account/v1"
	"altalune.id/othra/internal/kernel/session"
)

func (h *harness) account() (*AccountService, context.Context) {
	a := NewAccountService(h.svc)
	a.now = func() time.Time { return h.now }
	return a, session.EndUserInto(context.Background(), session.EndUser{
		OrgID: h.p.OrgID, ProjectID: h.p.ID, ProjectPublicID: h.p.PublicID, ProjectName: h.p.Name, EndUserID: h.user,
	})
}

// M8 P5, P6: pat's account.v1 slice lists servers, creates with the secret once, lists and revokes the signed-in end user's tokens.
func TestAccountService_Tokens(t *testing.T) {
	h := newHarness(t)
	a, ctx := h.account()

	servers, err := a.ListTokenServers(ctx, connect.NewRequest(&accountv1.ListTokenServersRequest{}))
	require.NoError(t, err)
	require.Len(t, servers.Msg.GetServers(), 1)
	assert.Equal(t, "rs-open", servers.Msg.GetServers()[0].GetId())

	for expiry, days := range map[accountv1.TokenExpiry]int{
		accountv1.TokenExpiry_TOKEN_EXPIRY_7_DAYS: 7, accountv1.TokenExpiry_TOKEN_EXPIRY_30_DAYS: 30, accountv1.TokenExpiry_TOKEN_EXPIRY_90_DAYS: 90,
	} {
		made, err := a.CreatePersonalToken(ctx, connect.NewRequest(&accountv1.CreatePersonalTokenRequest{
			Name: "cli", ServerId: "rs-open", Scopes: []string{"read"}, Expiry: expiry,
		}))
		require.NoError(t, err)
		assert.True(t, Scheme.Valid(made.Msg.GetSecret()))
		assert.Equal(t, h.now.Add(time.Duration(days)*24*time.Hour), made.Msg.GetToken().GetExpiresAt().AsTime())
		assert.Equal(t, accountv1.PersonalTokenState_PERSONAL_TOKEN_STATE_ACTIVE, made.Msg.GetToken().GetState())
	}
	_, err = a.CreatePersonalToken(ctx, connect.NewRequest(&accountv1.CreatePersonalTokenRequest{
		Name: "cli", ServerId: "rs-open", Scopes: []string{"read"},
	}))
	assert.Equal(t, apperror.CodeAccountTokenInvalid, codeOf(err), "an unset expiry is refused")
	_, err = a.CreatePersonalToken(ctx, connect.NewRequest(&accountv1.CreatePersonalTokenRequest{
		Name: "cli", ServerId: "rs-closed", Scopes: []string{"read"}, Expiry: accountv1.TokenExpiry_TOKEN_EXPIRY_7_DAYS,
	}))
	app, ok := apperror.AsAppError(err)
	require.True(t, ok)
	assert.Equal(t, "Acme Store", app.Meta()["project"], "ACC006 names the project from the principal")

	list, err := a.ListPersonalTokens(ctx, connect.NewRequest(&accountv1.ListPersonalTokensRequest{}))
	require.NoError(t, err)
	tokens := list.Msg.GetTokens()
	require.Len(t, tokens, 3)
	_, err = a.RevokePersonalToken(ctx, connect.NewRequest(&accountv1.RevokePersonalTokenRequest{TokenId: tokens[0].GetId()}))
	require.NoError(t, err)
	_, err = a.RevokePersonalToken(ctx, connect.NewRequest(&accountv1.RevokePersonalTokenRequest{TokenId: tokens[0].GetId()}))
	assert.Equal(t, apperror.CodeAccountTokenNotFound, codeOf(err))
	require.NoError(t, a.Revoke(ctx, h.p, h.user, tokens[1].GetId()), "the admin view's revoke")

	h.now = h.now.Add(8 * 24 * time.Hour)
	got, err := a.Tokens(ctx, h.p, h.user)
	require.NoError(t, err)
	states := map[accountv1.PersonalTokenState]int{}
	for _, tk := range got {
		states[tk.GetState()]++
		if tk.GetState() == accountv1.PersonalTokenState_PERSONAL_TOKEN_STATE_REVOKED {
			assert.NotNil(t, tk.GetRevokedAt())
		}
	}
	assert.Equal(t, 2, states[accountv1.PersonalTokenState_PERSONAL_TOKEN_STATE_REVOKED])
	assert.Equal(t, 1, states[accountv1.PersonalTokenState_PERSONAL_TOKEN_STATE_ACTIVE]+states[accountv1.PersonalTokenState_PERSONAL_TOKEN_STATE_EXPIRED])
}

func TestAccountService_Refusals(t *testing.T) {
	h := newHarness(t)
	a, ctx := h.account()
	bare := context.Background()
	_, err := a.ListTokenServers(bare, connect.NewRequest(&accountv1.ListTokenServersRequest{}))
	assert.Error(t, err)
	_, err = a.ListPersonalTokens(bare, connect.NewRequest(&accountv1.ListPersonalTokensRequest{}))
	assert.Error(t, err)
	_, err = a.CreatePersonalToken(bare, connect.NewRequest(&accountv1.CreatePersonalTokenRequest{}))
	assert.Error(t, err)
	_, err = a.RevokePersonalToken(bare, connect.NewRequest(&accountv1.RevokePersonalTokenRequest{}))
	assert.Error(t, err)

	h.servers.err = errBoom
	_, err = a.ListTokenServers(ctx, connect.NewRequest(&accountv1.ListTokenServersRequest{}))
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	h.servers.err, h.store.err = nil, errBoom
	_, err = a.ListPersonalTokens(ctx, connect.NewRequest(&accountv1.ListPersonalTokensRequest{}))
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))

	assert.Equal(t, accountv1.PersonalTokenState_PERSONAL_TOKEN_STATE_UNSPECIFIED, stateProto("other"))
}
