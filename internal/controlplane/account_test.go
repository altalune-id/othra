package controlplane

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	accountv1 "altalune.id/othra/gen/go/account/v1"
	"altalune.id/othra/gen/go/account/v1/accountv1connect"
	"altalune.id/othra/internal/consent"
	"altalune.id/othra/internal/enduser"
	"altalune.id/othra/internal/kernel/ratelimit/memory"
	"altalune.id/othra/internal/kernel/session"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/pat"
	"altalune.id/othra/internal/permission"
	"altalune.id/othra/internal/web/layout"
)

type fakeResolver struct {
	eu       session.EndUser
	err      error
	answered bool
}

func (f *fakeResolver) EndUser(w http.ResponseWriter, _ *http.Request) (session.EndUser, bool, error) {
	if f.answered {
		http.NotFound(w, nil)
	}
	return f.eu, f.answered, f.err
}

type fakeProfile struct {
	accountv1connect.UnimplementedAccountServiceHandler
	panic bool
}

func (f fakeProfile) GetProfile(
	ctx context.Context, _ *connect.Request[accountv1.GetProfileRequest],
) (*connect.Response[accountv1.GetProfileResponse], error) {
	if f.panic {
		panic("boom")
	}
	eu, _ := session.EndUserFrom(ctx)
	if tenant.Peek(ctx).ProjectID != eu.ProjectID {
		return nil, errors.New("no tenant scope")
	}
	return connect.NewResponse(&accountv1.GetProfileResponse{
		Profile: &accountv1.Profile{Id: eu.Subject}, SignOutToken: eu.SignOutToken, CurrentSessionId: eu.UserSession,
	}), nil
}

type fakeApps struct {
	accountv1connect.UnimplementedAccountServiceHandler
	apps []*accountv1.ConnectedApp
	err  error
	gone []string
}

func (f *fakeApps) Apps(context.Context, consent.Project, uuid.UUID) ([]*accountv1.ConnectedApp, error) {
	return f.apps, f.err
}

func (f *fakeApps) Disconnect(_ context.Context, _ consent.Project, _ uuid.UUID, _, oauthID string) error {
	f.gone = append(f.gone, oauthID)
	return f.err
}

type fakeTokens struct {
	accountv1connect.UnimplementedAccountServiceHandler
	tokens  []*accountv1.PersonalToken
	err     error
	revoked []string
}

func (f *fakeTokens) Tokens(context.Context, pat.Project, uuid.UUID) ([]*accountv1.PersonalToken, error) {
	return f.tokens, f.err
}

func (f *fakeTokens) Revoke(_ context.Context, _ pat.Project, _ uuid.UUID, publicID string) error {
	f.revoked = append(f.revoked, publicID)
	return f.err
}

func fakeSlices(profile ProfileSlice) AccountSlices {
	return AccountSlices{
		Profile: profile, Methods: accountv1connect.UnimplementedAccountServiceHandler{}, Apps: &fakeApps{}, Tokens: &fakeTokens{},
	}
}

func testUnexpected(_ context.Context, msg string, err error, _ ...any) *apperror.AppError {
	return apperror.New(apperror.CodeUnexpected, msg).WithCause(err)
}

func newPlane(t *testing.T, res *fakeResolver, profile ProfileSlice) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(Pattern("/app"), NewAccountPlane(fakeSlices(profile), res, nil, layout.Limits{Limiter: memory.New(100)}, testUnexpected).Handler("/app"))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts.URL
}

func signedIn() session.EndUser {
	return session.EndUser{
		OrgID: uuid.New(), ProjectID: uuid.New(), ProjectPublicID: "prj", Subject: "sub-ana", UserSession: "us-1", SignOutToken: "tok",
	}
}

// M8 G2: the plane serves the browser's active account, with the tenant scope and the sign-out form's token.
func TestAccountPlane_ServesTheSignedInEndUser(t *testing.T) {
	base := newPlane(t, &fakeResolver{eu: signedIn()}, fakeProfile{})
	c := accountv1connect.NewAccountServiceClient(http.DefaultClient, base+"/app/p/acme/api")
	resp, err := c.GetProfile(context.Background(), connect.NewRequest(&accountv1.GetProfileRequest{}))
	require.NoError(t, err)
	assert.Equal(t, "sub-ana", resp.Msg.GetProfile().GetId())
	assert.Equal(t, "tok", resp.Msg.GetSignOutToken())
	assert.Equal(t, "us-1", resp.Msg.GetCurrentSessionId())
	assert.Equal(t, "no-store", resp.Header().Get("Cache-Control"))
}

func TestAccountPlane_RefusesWhatIsNotTheEndUser(t *testing.T) {
	for name, tc := range map[string]struct {
		res  *fakeResolver
		code connect.Code
		want apperror.Code
	}{
		"signed out":     {&fakeResolver{err: &enduser.SignedOutError{}}, connect.CodeUnauthenticated, apperror.CodeAccountSignedOut},
		"terms pending":  {&fakeResolver{err: &enduser.TermsRequiredError{}}, connect.CodeFailedPrecondition, apperror.CodeAccountTermsRequired},
		"resolver fails": {&fakeResolver{err: errors.New("db down")}, connect.CodeInternal, apperror.CodeUnexpected},
	} {
		t.Run(name, func(t *testing.T) {
			c := accountv1connect.NewAccountServiceClient(http.DefaultClient, newPlane(t, tc.res, fakeProfile{})+"/app/p/acme/api")
			_, err := c.GetProfile(context.Background(), connect.NewRequest(&accountv1.GetProfileRequest{}))
			assert.Equal(t, tc.code, connect.CodeOf(err), err)
			assert.Equal(t, string(tc.want), detailOf(t, err).GetCode())
		})
	}
}

func TestAccountPlane_HTTPGuards(t *testing.T) {
	base := newPlane(t, &fakeResolver{eu: signedIn()}, fakeProfile{})
	url := base + "/app/p/acme/api/account.v1.AccountService/GetProfile"
	do := func(method, contentType string, header ...string) (int, http.Header) {
		t.Helper()
		req, err := http.NewRequestWithContext(context.Background(), method, url, strings.NewReader("{}"))
		require.NoError(t, err)
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		for i := 0; i+1 < len(header); i += 2 {
			req.Header.Set(header[i], header[i+1])
		}
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		return resp.StatusCode, resp.Header
	}
	status, _ := do(http.MethodPost, "application/json")
	assert.Equal(t, http.StatusOK, status)
	status, header := do(http.MethodGet, "")
	assert.Equal(t, http.StatusMethodNotAllowed, status, "Connect GET is off")
	assert.Equal(t, http.MethodPost, header.Get("Allow"))
	status, _ = do(http.MethodPost, "text/plain")
	assert.Equal(t, http.StatusUnsupportedMediaType, status, "a simple-request body is refused")
	status, _ = do(http.MethodPost, "application/json", "Sec-Fetch-Site", "cross-site")
	assert.Equal(t, http.StatusForbidden, status, "R4")

	unknown := newPlane(t, &fakeResolver{answered: true}, fakeProfile{}) + "/app/p/nope/api/account.v1.AccountService/GetProfile"
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, unknown, strings.NewReader("{}"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "the resolver answered an unknown issuer itself")
}

func TestAccountPlane_RecoversAPanic(t *testing.T) {
	c := accountv1connect.NewAccountServiceClient(http.DefaultClient,
		newPlane(t, &fakeResolver{eu: signedIn()}, fakeProfile{panic: true})+"/app/p/acme/api")
	_, err := c.GetProfile(context.Background(), connect.NewRequest(&accountv1.GetProfileRequest{}))
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

type fakeRequest struct {
	connect.AnyRequest
	spec connect.Spec
}

func (r fakeRequest) Spec() connect.Spec { return r.spec }

// SECURITY: G2: deny by default, and nothing runs without an end user.
func TestEndUserGate(t *testing.T) {
	ok := func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) { return nil, nil }
	now := time.Now()
	gate := endUserGate(func() time.Time { return now })(ok)
	_, err := gate(context.Background(), fakeRequest{spec: connect.Spec{Procedure: "/org.v1.OrgService/CreateOrg"}})
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "an admin procedure is not on this plane")
	known := fakeRequest{spec: connect.Spec{Procedure: accountv1connect.AccountServiceGetProfileProcedure}}
	_, err = gate(context.Background(), known)
	assert.True(t, enduser.IsSignedOutError(err))
	_, err = gate(session.EndUserInto(context.Background(), signedIn()), known)
	assert.NoError(t, err)

	create := fakeRequest{spec: connect.Spec{Procedure: accountv1connect.AccountServiceCreatePersonalTokenProcedure}}
	eu := signedIn()
	eu.AuthTime = now.Add(-permission.RecentSignIn - time.Second)
	_, err = gate(session.EndUserInto(context.Background(), eu), create)
	assert.True(t, pat.IsReauthRequiredError(err), "G5: a stale sign-in can't create a token (ACC009)")
	_, err = gate(session.EndUserInto(context.Background(), eu), known)
	assert.NoError(t, err, "a Self procedure needs no fresh sign-in")
	eu.AuthTime = now.Add(-time.Minute)
	_, err = gate(session.EndUserInto(context.Background(), eu), create)
	assert.NoError(t, err)
}

func TestAccountSlices_Complete(t *testing.T) {
	assert.False(t, AccountSlices{}.Complete())
	assert.True(t, fakeSlices(fakeProfile{}).Complete())
}

// R-3, G11, P13: the 11th token creation in an hour is refused for the user session; reads are never counted.
func TestAccountPlane_RateLimits(t *testing.T) {
	eu := signedIn()
	eu.AuthTime = time.Now()
	c := accountv1connect.NewAccountServiceClient(http.DefaultClient, newPlane(t, &fakeResolver{eu: eu}, fakeProfile{})+"/app/p/acme/api")
	ctx := context.Background()
	for i := range 10 {
		_, err := c.CreatePersonalToken(ctx, connect.NewRequest(&accountv1.CreatePersonalTokenRequest{}))
		assert.NotEqual(t, connect.CodeResourceExhausted, connect.CodeOf(err), "create %d", i+1)
	}
	_, err := c.CreatePersonalToken(ctx, connect.NewRequest(&accountv1.CreatePersonalTokenRequest{}))
	assert.Equal(t, connect.CodeResourceExhausted, connect.CodeOf(err))
	assert.Equal(t, string(apperror.CodeAuthRateLimited), detailOf(t, err).GetCode())
	for range 40 {
		_, err = c.GetProfile(ctx, connect.NewRequest(&accountv1.GetProfileRequest{}))
		require.NoError(t, err, "a read is not a write")
	}
	for range 20 {
		_, err = c.UpdateProfile(ctx, connect.NewRequest(&accountv1.UpdateProfileRequest{}))
	}
	assert.Equal(t, connect.CodeResourceExhausted, connect.CodeOf(err), "the 31st write in a minute")
}
