package federation

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/apperror"
	accountv1 "github.com/altalune-id/othra/gen/go/account/v1"
	"github.com/altalune-id/othra/internal/kernel/session"
	"github.com/altalune-id/othra/internal/upstream"
)

func TestCanRemove(t *testing.T) {
	assert.False(t, canRemove(false, 1), "the only identity, no email link")
	assert.True(t, canRemove(true, 1), "the email link remains")
	assert.True(t, canRemove(false, 2), "another identity remains")
}

// one read lists the methods with the same removable rule Unlink enforces.
func TestSignInMethods(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	ana := uuid.New()

	got, err := h.svc.SignInMethods(ctx, h.p, ana)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, SignInMethod{Linked: true, LastMethod: true}, got[0], "the email link, the only method")
	assert.Equal(t, SignInMethod{Kind: upstream.KindGoogle}, got[1], "Google, enabled and not linked")

	require.NoError(t, h.svc.Link(ctx, h.p, ana, identity(h)))
	got, err = h.svc.SignInMethods(ctx, h.p, ana)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.False(t, got[0].LastMethod)
	assert.True(t, got[1].Linked)
	assert.True(t, got[1].CanRemove)

	h.switches.on = false
	h.upstreams.creds.Upstream.Enabled = false
	got, err = h.svc.SignInMethods(ctx, h.p, ana)
	require.NoError(t, err)
	require.Len(t, got, 1, "a disabled upstream's linked identity is still listed, and no email link")
	assert.True(t, got[0].LastMethod)
	assert.False(t, got[0].CanRemove, "the same rule as Unlink's SIG019")

	h.upstreams.err = errBoom
	_, err = h.svc.SignInMethods(ctx, h.p, ana)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	h.upstreams.err, h.store.err = nil, errBoom
	_, err = h.svc.SignInMethods(ctx, h.p, ana)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	h.store.err, h.switches.err = nil, errBoom
	_, err = h.svc.SignInMethods(ctx, h.p, ana)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

func TestAccountService_Methods(t *testing.T) {
	h := newHarness(t)
	ana := uuid.New()
	require.NoError(t, h.svc.Link(context.Background(), h.p, ana, identity(h)))
	ctx := session.EndUserInto(context.Background(), session.EndUser{
		OrgID: h.p.OrgID, ProjectID: h.p.ID, ProjectPublicID: h.p.PublicID, EndUserID: ana, LinkToken: "link-tok",
	})
	a := NewAccountService(h.svc)

	got, err := a.GetSignInMethods(ctx, connect.NewRequest(&accountv1.GetSignInMethodsRequest{}))
	require.NoError(t, err)
	assert.Equal(t, "link-tok", got.Msg.GetLinkToken())
	ms := got.Msg.GetMethods()
	require.Len(t, ms, 2)
	assert.Equal(t, accountv1.SignInMethodKind_SIGN_IN_METHOD_KIND_EMAIL_LINK, ms[0].GetKind())
	assert.Equal(t, accountv1.SignInMethodKind_SIGN_IN_METHOD_KIND_GOOGLE, ms[1].GetKind())
	assert.Equal(t, "ana@gmail.com", ms[1].GetEmail())
	assert.NotEmpty(t, ms[1].GetIdentityId())

	_, err = a.UnlinkIdentity(ctx, connect.NewRequest(&accountv1.UnlinkIdentityRequest{IdentityId: "nope"}))
	assert.Equal(t, apperror.CodeNotFound, codeOf(err), "an unknown identity is NotFound")
	h.switches.on = false
	_, err = a.UnlinkIdentity(ctx, connect.NewRequest(&accountv1.UnlinkIdentityRequest{IdentityId: "nope"}))
	assert.Equal(t, apperror.CodeNotFound, codeOf(err), "ownership before the last-method rule, so never SIG019")
	_, err = a.UnlinkIdentity(ctx, connect.NewRequest(&accountv1.UnlinkIdentityRequest{IdentityId: ms[1].GetIdentityId()}))
	assert.Equal(t, apperror.CodeSignInLastMethod, codeOf(err))
	h.switches.on = true
	_, err = a.UnlinkIdentity(ctx, connect.NewRequest(&accountv1.UnlinkIdentityRequest{IdentityId: ms[1].GetIdentityId()}))
	require.NoError(t, err)
	assert.Empty(t, h.store.rows)

	lock := MethodsProto([]SignInMethod{{Linked: true, LastMethod: true}})
	assert.Equal(t, accountv1.SignInMethodLock_SIGN_IN_METHOD_LOCK_LAST_METHOD, lock[0].GetLock())

	bare := context.Background()
	_, err = a.GetSignInMethods(bare, connect.NewRequest(&accountv1.GetSignInMethodsRequest{}))
	assert.Error(t, err)
	_, err = a.UnlinkIdentity(bare, connect.NewRequest(&accountv1.UnlinkIdentityRequest{}))
	assert.Error(t, err)
	h.switches.err = errBoom
	_, err = a.GetSignInMethods(ctx, connect.NewRequest(&accountv1.GetSignInMethodsRequest{}))
	assert.Error(t, err)
}

// /login (Enabled) and the portal (SignInMethods) read the offered upstream IdPs through the one upstream read.
func TestOfferedIsTheOneRead(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	on, err := h.svc.Enabled(ctx, h.p, upstream.KindGoogle)
	require.NoError(t, err)
	assert.True(t, on)
	_, err = h.svc.SignInMethods(ctx, h.p, uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 2, h.upstreams.offered)

	h.upstreams.creds.Upstream.Enabled = false
	on, err = h.svc.Enabled(ctx, h.p, upstream.KindGoogle)
	require.NoError(t, err)
	assert.False(t, on)
	got, err := h.svc.SignInMethods(ctx, h.p, uuid.New())
	require.NoError(t, err)
	assert.Len(t, got, 1, "turning Google off removes it from both in the same step")
}
