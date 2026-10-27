//go:build integration

package boot

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/config"
	accountv1 "altalune.id/othra/gen/go/account/v1"
	"altalune.id/othra/gen/go/account/v1/accountv1connect"
	"altalune.id/othra/internal/client"
	"altalune.id/othra/internal/consent"
	"altalune.id/othra/internal/enduser"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/project"
)

func (f *signInFixture) signInDevice(t *testing.T, base, email string) *http.Client {
	t.Helper()
	asker := f.client()
	login := f.do(asker, http.MethodGet, base+"/login?add=1", nil)
	require.Equal(t, http.StatusOK, login.status, login.body)
	sent := f.submit(asker, login, "/login", url.Values{"email": {email}})
	require.Equal(t, http.StatusSeeOther, sent.status, sent.body)
	mails := claimMail(t, f.svcs)
	require.Len(t, mails, 1)
	link := signInLinkRE.FindString(mails[0].TextBody)
	require.NotEmpty(t, link)
	moved := f.do(asker, http.MethodGet, strings.TrimPrefix(link, f.srv.URL), nil)
	verify := f.do(asker, http.MethodGet, moved.header.Get("Location"), nil)
	require.Equal(t, http.StatusOK, verify.status, verify.body)
	done := f.submit(asker, verify, "/login/verify", nil)
	require.Equal(t, http.StatusSeeOther, done.status, done.body)
	return asker
}

func (f *signInFixture) account(c *http.Client, base string) accountv1connect.AccountServiceClient {
	return accountv1connect.NewAccountServiceClient(c, f.srv.URL+base+"/api")
}

func appCode(t *testing.T, err error) apperror.Code {
	t.Helper()
	var ce *connect.Error
	require.ErrorAs(t, err, &ce)
	for _, d := range ce.Details() {
		if v, derr := d.Value(); derr == nil {
			if det, ok := v.(interface{ GetCode() string }); ok {
				return apperror.Code(det.GetCode())
			}
		}
	}
	return ""
}

func newAccountFixture(t *testing.T) (*signInFixture, *project.Project, string) {
	t.Helper()
	f := newSignInFixture(t, func(c *config.Config) { c.EndUsers.MagicLink.ResendAfter = time.Millisecond })
	p := f.project("acct", "Acct")
	return f, p, "/p/" + p.IssuerHandle
}

// M8 G2, A15, A19: account.v1 serves the signed-in end user on this browser; ending another session ends it and its grants together.
func TestAccount_ProfileAndSessions(t *testing.T) {
	f, p, base := newAccountFixture(t)
	phone := f.signInDevice(t, base, "ana@example.com")
	laptop := f.signInDevice(t, base, "ana@example.com")
	ctx := context.Background()
	acc := f.account(laptop, base)

	prof, err := acc.GetProfile(ctx, connect.NewRequest(&accountv1.GetProfileRequest{}))
	require.NoError(t, err)
	assert.Equal(t, "ana@example.com", prof.Msg.GetProfile().GetEmail())
	assert.NotEmpty(t, prof.Msg.GetSignOutToken())
	assert.Equal(t, "no-store", prof.Header().Get("Cache-Control"))
	named, err := acc.UpdateProfile(ctx, connect.NewRequest(&accountv1.UpdateProfileRequest{Name: "Ana"}))
	require.NoError(t, err)
	assert.Equal(t, "Ana", named.Msg.GetProfile().GetName())
	methods, err := acc.GetSignInMethods(ctx, connect.NewRequest(&accountv1.GetSignInMethodsRequest{}))
	require.NoError(t, err)
	assert.Equal(t, accountv1.SignInMethodKind_SIGN_IN_METHOD_KIND_EMAIL_LINK, methods.Msg.GetMethods()[0].GetKind())

	list, err := acc.ListSessions(ctx, connect.NewRequest(&accountv1.ListSessionsRequest{}))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetSessions(), 2)
	var phoneSession string
	for _, s := range list.Msg.GetSessions() {
		if !s.GetCurrent() {
			phoneSession = s.GetId()
		}
	}
	_, err = acc.EndSession(ctx, connect.NewRequest(&accountv1.EndSessionRequest{SessionId: prof.Msg.GetCurrentSessionId()}))
	assert.Equal(t, apperror.CodeAccountCurrentSession, appCode(t, err), "ACC008: use Sign out")

	family := f.seedFamily(t, p, phoneSession)
	_, err = acc.EndSession(ctx, connect.NewRequest(&accountv1.EndSessionRequest{SessionId: phoneSession}))
	require.NoError(t, err)
	assert.Equal(t, "sign_out", f.familyReason(t, family), "A15: the session's grants end with it, in one unit of work")
	_, err = f.account(phone, base).GetProfile(ctx, connect.NewRequest(&accountv1.GetProfileRequest{}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err), "ACC001: that browser is signed out")
	_, err = acc.EndSession(ctx, connect.NewRequest(&accountv1.EndSessionRequest{SessionId: phoneSession}))
	assert.Equal(t, apperror.CodeAccountSessionNotFound, appCode(t, err))
	other, err := acc.EndOtherSessions(ctx, connect.NewRequest(&accountv1.EndOtherSessionsRequest{}))
	require.NoError(t, err)
	assert.Zero(t, other.Msg.GetEnded())

	_, err = f.account(f.client(), base).GetProfile(ctx, connect.NewRequest(&accountv1.GetProfileRequest{}))
	assert.Equal(t, apperror.CodeAccountSignedOut, appCode(t, err))
}

// R-5, P8: Disconnect deletes the app's consents and revokes its grants together; the app is then gone.
func TestAccount_Disconnect(t *testing.T) {
	f, p, base := newAccountFixture(t)
	acc := f.account(f.signInDevice(t, base, "ana@example.com"), base)
	ctx := context.Background()
	web, endUser, family := f.connectWeb(t, p, acc)

	apps, err := acc.ListConnectedApps(ctx, connect.NewRequest(&accountv1.ListConnectedAppsRequest{}))
	require.NoError(t, err)
	require.Len(t, apps.Msg.GetApps(), 1)
	assert.Equal(t, "Web", apps.Msg.GetApps()[0].GetName())
	assert.True(t, apps.Msg.GetApps()[0].GetConsented())

	_, err = acc.DisconnectApp(ctx, connect.NewRequest(&accountv1.DisconnectAppRequest{ClientId: web.PublicID}))
	require.NoError(t, err)
	assert.Equal(t, "consent_revoked", f.familyReason(t, family))
	assert.Zero(t, f.consents(t, endUser))
	_, err = acc.DisconnectApp(ctx, connect.NewRequest(&accountv1.DisconnectAppRequest{ClientId: web.PublicID}))
	assert.Equal(t, apperror.CodeAccountAppNotConnected, appCode(t, err))
}

var errMidUnit = errors.New("fails mid unit of work")

type failingConsentGrants struct{ consentGrants }

func (g failingConsentGrants) RevokeClient(ctx context.Context, p consent.Project, endUserID, clientID uuid.UUID) (bool, error) {
	if _, err := g.consentGrants.RevokeClient(ctx, p, endUserID, clientID); err != nil {
		return false, err
	}
	return false, errMidUnit
}

// A13, R-5: a failure after the consents are deleted and the grants revoked rolls both back; nothing changes.
func TestAccount_DisconnectRollsBackTogether(t *testing.T) {
	f, p, base := newAccountFixture(t)
	web, endUser, family := f.connectWeb(t, p, f.account(f.signInDevice(t, base, "ana@example.com"), base))
	f.svcs.Consents.WithGrants(failingConsentGrants{consentGrants{grants: f.svcs.Grants}}, db.NewTransactor(f.pool))

	err := f.svcs.Consents.Disconnect(t.Context(), consent.Project{OrgID: p.OrgID, ID: p.ID, PublicID: p.PublicID},
		consent.Connection{EndUserID: endUser, ClientID: web.ID, ClientOAuthID: web.PublicID})
	require.Error(t, err)
	assert.Empty(t, f.familyReason(t, family), "the grant is still live")
	assert.Equal(t, 1, f.consents(t, endUser), "the consent is still there")
}

type failingEndUserGrants struct{ endUserGrants }

func (g failingEndUserGrants) RevokeUserSession(ctx context.Context, p enduser.Project, userSession string, reason enduser.EndReason) error {
	if err := g.endUserGrants.RevokeUserSession(ctx, p, userSession, reason); err != nil {
		return err
	}
	return errMidUnit
}

// A15: ending a session and revoking its grants is one unit of work; a failure after both rolls both back.
func TestAccount_EndSessionRollsBackTogether(t *testing.T) {
	f, p, base := newAccountFixture(t)
	acc := f.account(f.signInDevice(t, base, "ana@example.com"), base)
	prof, err := acc.GetProfile(t.Context(), connect.NewRequest(&accountv1.GetProfileRequest{}))
	require.NoError(t, err)
	here := prof.Msg.GetCurrentSessionId()
	family := f.seedFamily(t, p, here)
	f.svcs.EndUsers.WithGrants(failingEndUserGrants{endUserGrants{grants: f.svcs.Grants}})

	var endUser uuid.UUID
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT id FROM end_users WHERE public_id = $1`, prof.Msg.GetProfile().GetId()).Scan(&endUser))
	err = f.svcs.EndUsers.EndUserSession(t.Context(), enduser.Project{OrgID: p.OrgID, ID: p.ID, PublicID: p.PublicID}, endUser, here)
	require.Error(t, err)
	assert.Empty(t, f.familyReason(t, family), "the grant is still live")
	_, err = acc.GetProfile(t.Context(), connect.NewRequest(&accountv1.GetProfileRequest{}))
	assert.NoError(t, err, "the session is still live")
}

func (f *signInFixture) connectWeb(t *testing.T, p *project.Project, acc accountv1connect.AccountServiceClient) (*client.Client, uuid.UUID, uuid.UUID) {
	t.Helper()
	prof, err := acc.GetProfile(t.Context(), connect.NewRequest(&accountv1.GetProfileRequest{}))
	require.NoError(t, err)
	tctx := tenant.Into(t.Context(), tenant.Context{OrgID: p.OrgID, ProjectID: p.ID})
	web, err := f.svcs.Clients.Create(tctx, p.OrgID, p.ID, client.KindWeb, client.Settings{Name: "Web", RedirectURIs: []string{"https://web.example/cb"}})
	require.NoError(t, err)
	var endUser uuid.UUID
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT id FROM end_users WHERE public_id = $1`, prof.Msg.GetProfile().GetId()).Scan(&endUser))
	_, err = f.svcs.Consents.Grant(t.Context(), consent.Project{OrgID: p.OrgID, ID: p.ID, PublicID: p.PublicID}, consent.GrantRequest{
		Key: consent.Key{EndUserID: endUser, ClientID: web.ID}, Scopes: []string{"openid", "email"},
	})
	require.NoError(t, err)
	return web, endUser, f.seedFamilyFor(t, p, web.ID, endUser, prof.Msg.GetCurrentSessionId())
}

func (f *signInFixture) consents(t *testing.T, endUser uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM scope_consents WHERE end_user_id = $1`, endUser).Scan(&n))
	return n
}

// SECURITY: G2, R4, A19: POST JSON from this origin only, never GET; and no account.v1 on the admin /api.
func TestAccount_HTTPGuards(t *testing.T) {
	f, _, base := newAccountFixture(t)
	laptop := f.signInDevice(t, base, "ana@example.com")
	call := func(c *http.Client, method, path, contentType string, header ...string) int {
		t.Helper()
		req, err := http.NewRequestWithContext(t.Context(), method, f.srv.URL+path, strings.NewReader("{}"))
		require.NoError(t, err)
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		for i := 0; i+1 < len(header); i += 2 {
			req.Header.Set(header[i], header[i+1])
		}
		resp, err := c.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		return resp.StatusCode
	}
	profile := base + "/api/account.v1.AccountService/GetProfile"
	assert.Equal(t, http.StatusOK, call(laptop, http.MethodPost, profile, "application/json"))
	assert.Equal(t, http.StatusMethodNotAllowed, call(laptop, http.MethodGet, profile, ""))
	assert.Equal(t, http.StatusUnsupportedMediaType, call(laptop, http.MethodPost, profile, "text/plain"))
	assert.Equal(t, http.StatusForbidden, call(laptop, http.MethodPost, profile, "application/json", "Sec-Fetch-Site", "cross-site"))
	assert.Equal(t, http.StatusNotFound, call(laptop, http.MethodPost, "/p/nope/api/account.v1.AccountService/GetProfile", "application/json"))
	assert.NotEqual(t, http.StatusOK, call(laptop, http.MethodPost, "/api/account.v1.AccountService/GetProfile", "application/json"),
		"the admin /api has no account.v1")
}

func (f *signInFixture) seedFamily(t *testing.T, p *project.Project, userSession string) uuid.UUID {
	t.Helper()
	var endUser uuid.UUID
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT end_user_id FROM user_sessions WHERE public_id = $1`, userSession).Scan(&endUser))
	tctx := tenant.Into(t.Context(), tenant.Context{OrgID: p.OrgID, ProjectID: p.ID})
	web, err := f.svcs.Clients.Create(tctx, p.OrgID, p.ID, client.KindWeb, client.Settings{
		Name: "Web " + userSession, RedirectURIs: []string{"https://web.example/cb"},
	})
	require.NoError(t, err)
	return f.seedFamilyFor(t, p, web.ID, endUser, userSession)
}

func (f *signInFixture) seedFamilyFor(t *testing.T, p *project.Project, clientID, endUser uuid.UUID, userSession string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := f.db.ExecContext(t.Context(), `INSERT INTO grant_families (id, public_id, org_id, project_id, client_id, end_user_id,
		user_session_id, user_session_public_id, scopes, auth_time, created_at, last_used_at, absolute_expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, (SELECT id FROM user_sessions WHERE public_id = $7), $7, '{openid}', now(), now(), now(),
		now() + interval '1 day')`, id, "f"+id.String()[:12], p.OrgID, p.ID, clientID, endUser, userSession)
	require.NoError(t, err)
	return id
}

func (f *signInFixture) familyReason(t *testing.T, id uuid.UUID) string {
	t.Helper()
	var reason *string
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT revoked_reason FROM grant_families WHERE id = $1`, id).Scan(&reason))
	if reason == nil {
		return ""
	}
	return *reason
}
