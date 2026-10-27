package enduser

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	accountv1 "altalune.id/othra/gen/go/account/v1"
	"altalune.id/othra/internal/kernel/session"
)

type revoked struct {
	userSession string
	reason      EndReason
}

type fakeGrants struct {
	revoked []revoked
	err     error
}

func (g *fakeGrants) RevokeUserSession(_ context.Context, _ Project, userSession string, reason EndReason) error {
	g.revoked = append(g.revoked, revoked{userSession, reason})
	return g.err
}

func withGrants(t *testing.T) (*harness, *fakeGrants) {
	t.Helper()
	h := newHarness(t)
	g := &fakeGrants{}
	h.svc.WithGrants(g)
	return h, g
}

// G16, P9: every way a user session ends revokes its grants in the same unit of work.
func TestEndingASessionRevokesItsGrants(t *testing.T) {
	h, g := withGrants(t)
	ctx := context.Background()
	here := h.signIn(t, "ada@example.com", "")
	require.NoError(t, h.svc.SignOut(ctx, h.project, here.Secret, here.Account.PublicID))
	again := h.signIn(t, "ada@example.com", "")
	require.NoError(t, h.svc.EndSession(ctx, h.project, again.Account.PublicID))
	assert.Equal(t, []revoked{{here.Account.PublicID, EndSignOut}, {again.Account.PublicID, EndLogout}}, g.revoked)

	g.err = errFakeGrants
	third := h.signIn(t, "ada@example.com", "")
	assert.ErrorIs(t, h.svc.EndSession(ctx, h.project, third.Account.PublicID), errFakeGrants)
}

var errFakeGrants = apperror.New(apperror.CodeUnexpected, "grants down")

func TestEndUserSession_OnlyTheirsAndLive(t *testing.T) {
	h, g := withGrants(t)
	ctx := context.Background()
	phone := h.signIn(t, "ada@example.com", "")
	laptop := h.signIn(t, "ada@example.com", "")
	bob := h.signIn(t, "bob@example.com", "")
	ada := laptop.Account.EndUserID

	require.NoError(t, h.svc.EndUserSession(ctx, h.project, ada, phone.Account.PublicID))
	assert.Equal(t, []revoked{{phone.Account.PublicID, EndSignOut}}, g.revoked)
	assert.True(t, IsSessionNotFoundError(h.svc.EndUserSession(ctx, h.project, ada, phone.Account.PublicID)), "already ended")
	assert.True(t, IsSessionNotFoundError(h.svc.EndUserSession(ctx, h.project, ada, bob.Account.PublicID)), "not theirs")
	assert.True(t, IsSessionNotFoundError(h.svc.EndUserSession(ctx, h.project, ada, "nope")))

	h.store.failOn = "EndOwnedSession"
	assert.Equal(t, apperror.CodeUnexpected, codeOf(h.svc.EndUserSession(ctx, h.project, ada, laptop.Account.PublicID)))
}

func TestEndOtherSessions_KeepsThisOne(t *testing.T) {
	h, g := withGrants(t)
	ctx := context.Background()
	h.signIn(t, "ada@example.com", "")
	h.signIn(t, "ada@example.com", "")
	here := h.signIn(t, "ada@example.com", "")
	n, err := h.svc.EndOtherSessions(ctx, h.project, here.Account.EndUserID, here.Account.PublicID)
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Len(t, g.revoked, 2)
	live, err := h.svc.Sessions(ctx, h.project, here.Account.EndUserID)
	require.NoError(t, err)
	require.Len(t, live, 1)
	assert.Equal(t, here.Account.PublicID, live[0].PublicID)

	h.store.failOn = "LiveSessions"
	_, err = h.svc.EndOtherSessions(ctx, h.project, here.Account.EndUserID, "")
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	_, err = h.svc.Sessions(ctx, h.project, here.Account.EndUserID)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

// A18, A25: a name is clipped to 200 characters, as protovalidate and the column count them, and the change is audited by public id.
func TestProfileAndName(t *testing.T) {
	h := newHarness(t)
	var logs bytes.Buffer
	h.svc.WithLogger(slog.New(slog.NewJSONHandler(&logs, nil)))
	ctx := context.Background()
	r := h.signIn(t, "ada@example.com", "")
	u, err := h.svc.UpdateName(ctx, h.project, r.Account.EndUserID, "  "+strings.Repeat("a", nameMax+3)+" ")
	require.NoError(t, err)
	assert.Len(t, u.Name, nameMax, "trimmed and clipped")
	assert.Contains(t, logs.String(), `"msg":"enduser: profile updated","project":"`+h.project.PublicID+`","end_user":"`+r.Account.Subject+`"`)
	assert.NotContains(t, logs.String(), u.Name, "no PII beyond ids")
	u, err = h.svc.UpdateName(ctx, h.project, r.Account.EndUserID, strings.Repeat("é", nameMax))
	require.NoError(t, err)
	assert.Equal(t, strings.Repeat("é", nameMax), u.Name, "runes, not bytes")
	assert.Equal(t, strings.Repeat("é", nameMax), clip(strings.Repeat("é", nameMax+1), nameMax))
	got, err := h.svc.Profile(ctx, h.project, r.Account.EndUserID)
	require.NoError(t, err)
	assert.Equal(t, u.Name, got.Name)
	byPublic, err := h.svc.ByPublicID(ctx, h.project, r.Account.Subject)
	require.NoError(t, err)
	assert.Equal(t, r.Account.EndUserID, byPublic.ID)

	_, err = h.svc.UpdateName(ctx, h.project, uuid.New(), "x")
	assert.True(t, IsEndUserNotFoundError(err))
	_, err = h.svc.Profile(ctx, h.project, uuid.New())
	assert.True(t, IsEndUserNotFoundError(err))
	_, err = h.svc.ByPublicID(ctx, h.project, "nope")
	assert.True(t, IsEndUserNotFoundError(err))
	h.store.failOn = "UpdateName"
	_, err = h.svc.UpdateName(ctx, h.project, r.Account.EndUserID, "x")
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
	h.store.failOn = "EndUserByPublicID"
	_, err = h.svc.ByPublicID(ctx, h.project, r.Account.Subject)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

func TestListEndUsers_Pages(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	for _, e := range []string{"ada@example.com", "bob@example.com", "anna@example.com"} {
		h.signIn(t, e, "")
	}
	page, next, err := h.svc.ListEndUsers(ctx, h.project, "", "", 2)
	require.NoError(t, err)
	require.Len(t, page, 2)
	assert.Equal(t, "anna@example.com", page[0].Email, "newest first")
	assert.Equal(t, page[1].Subject, next)
	rest, next, err := h.svc.ListEndUsers(ctx, h.project, "", next, 2)
	require.NoError(t, err)
	assert.Len(t, rest, 1)
	assert.Empty(t, next, "a short page has no next")
	as, _, err := h.svc.ListEndUsers(ctx, h.project, " A", "", 0)
	require.NoError(t, err)
	assert.Len(t, as, 2, "the query is a lower-cased email prefix; 0 means the max page")
	h.store.failOn = "ListEndUsers"
	_, _, err = h.svc.ListEndUsers(ctx, h.project, "", "", 2)
	assert.Equal(t, apperror.CodeUnexpected, codeOf(err))
}

func signedInCtx(h *harness, r *Redeemed) context.Context {
	return session.EndUserInto(context.Background(), session.EndUser{
		OrgID: h.project.OrgID, ProjectID: h.project.ID, EndUserID: r.Account.EndUserID, Subject: r.Account.Subject,
		UserSession: r.Account.PublicID, SignOutToken: "tok",
	})
}

// M8 G2: enduser's account.v1 slice works on the signed-in end user only.
func TestAccountService(t *testing.T) {
	h, _ := withGrants(t)
	other := h.signIn(t, "ada@example.com", "")
	here := h.signIn(t, "ada@example.com", "")
	ctx, a := signedInCtx(h, here), NewAccountService(h.svc)

	prof, err := a.GetProfile(ctx, connect.NewRequest(&accountv1.GetProfileRequest{}))
	require.NoError(t, err)
	assert.Equal(t, here.Account.Subject, prof.Msg.GetProfile().GetId())
	assert.Equal(t, "tok", prof.Msg.GetSignOutToken())
	assert.Equal(t, here.Account.PublicID, prof.Msg.GetCurrentSessionId())
	upd, err := a.UpdateProfile(ctx, connect.NewRequest(&accountv1.UpdateProfileRequest{Name: "Ada"}))
	require.NoError(t, err)
	assert.Equal(t, "Ada", upd.Msg.GetProfile().GetName())

	list, err := a.ListSessions(ctx, connect.NewRequest(&accountv1.ListSessionsRequest{}))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetSessions(), 2)
	current := 0
	for _, s := range list.Msg.GetSessions() {
		if s.GetCurrent() {
			current++
		}
		assert.Equal(t, accountv1.SignInMethodKind_SIGN_IN_METHOD_KIND_EMAIL_LINK, s.GetMethod())
	}
	assert.Equal(t, 1, current)

	_, err = a.EndSession(ctx, connect.NewRequest(&accountv1.EndSessionRequest{SessionId: here.Account.PublicID}))
	assert.True(t, IsCurrentSessionError(err), "ACC008: use Sign out")
	_, err = a.EndSession(ctx, connect.NewRequest(&accountv1.EndSessionRequest{SessionId: other.Account.PublicID}))
	require.NoError(t, err)
	ended, err := a.EndOtherSessions(ctx, connect.NewRequest(&accountv1.EndOtherSessionsRequest{}))
	require.NoError(t, err)
	assert.Zero(t, ended.Msg.GetEnded())

	bare := context.Background()
	_, err = a.GetProfile(bare, connect.NewRequest(&accountv1.GetProfileRequest{}))
	assert.Error(t, err)
	_, err = a.UpdateProfile(bare, connect.NewRequest(&accountv1.UpdateProfileRequest{}))
	assert.Error(t, err)
	_, err = a.ListSessions(bare, connect.NewRequest(&accountv1.ListSessionsRequest{}))
	assert.Error(t, err)
	_, err = a.EndSession(bare, connect.NewRequest(&accountv1.EndSessionRequest{}))
	assert.Error(t, err)
	_, err = a.EndOtherSessions(bare, connect.NewRequest(&accountv1.EndOtherSessionsRequest{}))
	assert.Error(t, err)

	h.store.failOn = "LiveSessions"
	_, err = a.ListSessions(ctx, connect.NewRequest(&accountv1.ListSessionsRequest{}))
	assert.Error(t, err)
	_, err = a.EndOtherSessions(ctx, connect.NewRequest(&accountv1.EndOtherSessionsRequest{}))
	assert.Error(t, err)
	h.store.failOn = "UpdateName"
	_, err = a.UpdateProfile(ctx, connect.NewRequest(&accountv1.UpdateProfileRequest{}))
	assert.Error(t, err)
	h.store.failOn = "EndUserByID"
	_, err = a.GetProfile(ctx, connect.NewRequest(&accountv1.GetProfileRequest{}))
	assert.Error(t, err)
	h.store.failOn = "EndOwnedSession"
	_, err = a.EndSession(ctx, connect.NewRequest(&accountv1.EndSessionRequest{SessionId: "x"}))
	assert.Error(t, err)
}

func TestProtos(t *testing.T) {
	assert.Equal(t, accountv1.SignInMethodKind_SIGN_IN_METHOD_KIND_GOOGLE, MethodProto("google"))
	assert.Equal(t, accountv1.SignInMethodKind_SIGN_IN_METHOD_KIND_UNSPECIFIED, MethodProto("other"))
	u := &EndUser{Subject: "eu-1", Email: "a@x.test"}
	assert.Nil(t, EndUserProto(u).GetLastSignInAt())
	at := u.CreatedAt
	u.LastSignInAt = &at
	assert.NotNil(t, EndUserProto(u).GetLastSignInAt())
}
