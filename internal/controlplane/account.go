package controlplane

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"altalune.id/othra/apperror"
	accountv1 "altalune.id/othra/gen/go/account/v1"
	"altalune.id/othra/gen/go/account/v1/accountv1connect"
	"altalune.id/othra/internal/consent"
	"altalune.id/othra/internal/enduser"
	"altalune.id/othra/internal/kernel/ratelimit"
	"altalune.id/othra/internal/kernel/session"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/pat"
	"altalune.id/othra/internal/permission"
	"altalune.id/othra/internal/web/layout"
)

// EndUserResolver finds who an account.v1 request runs for: the browser's active account in the project of the path.
type EndUserResolver interface {
	EndUser(w http.ResponseWriter, r *http.Request) (eu session.EndUser, answered bool, err error)
}

// ProfileSlice is enduser's part of account.v1: the profile and the sessions.
type ProfileSlice interface {
	GetProfile(context.Context, *connect.Request[accountv1.GetProfileRequest]) (*connect.Response[accountv1.GetProfileResponse], error)
	UpdateProfile(
		context.Context, *connect.Request[accountv1.UpdateProfileRequest],
	) (*connect.Response[accountv1.UpdateProfileResponse], error)
	ListSessions(context.Context, *connect.Request[accountv1.ListSessionsRequest]) (*connect.Response[accountv1.ListSessionsResponse], error)
	EndSession(context.Context, *connect.Request[accountv1.EndSessionRequest]) (*connect.Response[accountv1.EndSessionResponse], error)
	EndOtherSessions(
		context.Context, *connect.Request[accountv1.EndOtherSessionsRequest],
	) (*connect.Response[accountv1.EndOtherSessionsResponse], error)
}

// MethodSlice is federation's part of account.v1: the sign-in methods.
type MethodSlice interface {
	GetSignInMethods(
		context.Context, *connect.Request[accountv1.GetSignInMethodsRequest],
	) (*connect.Response[accountv1.GetSignInMethodsResponse], error)
	UnlinkIdentity(
		context.Context, *connect.Request[accountv1.UnlinkIdentityRequest],
	) (*connect.Response[accountv1.UnlinkIdentityResponse], error)
}

// AppSlice is consent's part of account.v1, and the connected apps the admin view shows and disconnects.
type AppSlice interface {
	ListConnectedApps(
		context.Context, *connect.Request[accountv1.ListConnectedAppsRequest],
	) (*connect.Response[accountv1.ListConnectedAppsResponse], error)
	DisconnectApp(
		context.Context, *connect.Request[accountv1.DisconnectAppRequest],
	) (*connect.Response[accountv1.DisconnectAppResponse], error)
	Apps(ctx context.Context, p consent.Project, endUserID uuid.UUID) ([]*accountv1.ConnectedApp, error)
	Disconnect(ctx context.Context, p consent.Project, endUserID uuid.UUID, endUserPublicID, oauthID string) error
}

// TokenSlice is pat's part of account.v1, and the personal tokens the admin view shows and revokes.
type TokenSlice interface {
	ListTokenServers(
		context.Context, *connect.Request[accountv1.ListTokenServersRequest],
	) (*connect.Response[accountv1.ListTokenServersResponse], error)
	ListPersonalTokens(
		context.Context, *connect.Request[accountv1.ListPersonalTokensRequest],
	) (*connect.Response[accountv1.ListPersonalTokensResponse], error)
	CreatePersonalToken(
		context.Context, *connect.Request[accountv1.CreatePersonalTokenRequest],
	) (*connect.Response[accountv1.CreatePersonalTokenResponse], error)
	RevokePersonalToken(
		context.Context, *connect.Request[accountv1.RevokePersonalTokenRequest],
	) (*connect.Response[accountv1.RevokePersonalTokenResponse], error)
	Tokens(ctx context.Context, p pat.Project, endUserID uuid.UUID) ([]*accountv1.PersonalToken, error)
	Revoke(ctx context.Context, p pat.Project, endUserID uuid.UUID, publicID string) error
}

// AccountSlices are the domain handlers account.v1 is made of; each owns its part of the service.
type AccountSlices struct {
	Profile ProfileSlice
	Methods MethodSlice
	Apps    AppSlice
	Tokens  TokenSlice
}

// Complete reports whether every slice is set, so the plane can serve.
func (s AccountSlices) Complete() bool {
	return s.Profile != nil && s.Methods != nil && s.Apps != nil && s.Tokens != nil
}

type accountService struct {
	ProfileSlice
	MethodSlice
	AppSlice
	TokenSlice
}

var _ accountv1connect.AccountServiceHandler = accountService{}

// AccountPlane serves account.v1 to end users at <basePath>/p/{handle}/api/, beside and apart from the admin /api.
type AccountPlane struct {
	svc        accountService
	resolver   EndUserResolver
	validator  *Validator
	unexpected apperror.UnexpectedFunc
	now        func() time.Time
	limits     layout.Limits
}

// NewAccountPlane builds the account plane over slices, authenticating each call through resolver.
func NewAccountPlane(
	slices AccountSlices, resolver EndUserResolver, validator *Validator, limits layout.Limits, unexpected apperror.UnexpectedFunc,
) *AccountPlane {
	return &AccountPlane{
		svc:      accountService{slices.Profile, slices.Methods, slices.Apps, slices.Tokens},
		resolver: resolver, validator: validator, unexpected: unexpected, now: time.Now, limits: limits,
	}
}

// Pattern is the mux pattern the plane serves under basePath.
func Pattern(basePath string) string { return basePath + "/p/{handle}/api/" }

// Handler serves account.v1 for requests matched by Pattern(basePath).
func (a *AccountPlane) Handler(basePath string) http.Handler {
	path, connectHandler := accountv1connect.NewAccountServiceHandler(a.svc,
		connect.WithRecover(a.recoverPanic),
		connect.WithReadMaxBytes(maxRequestBytes),
		connect.WithInterceptors(RequestID(), Wrap(a.unexpected), endUserGate(a.now), portalLimits(a.limits), a.validator.interceptor()),
	)
	inner := http.NewServeMux()
	inner.Handle(path, connectHandler)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		if media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); media != "application/json" && media != "application/proto" {
			http.Error(w, http.StatusText(http.StatusUnsupportedMediaType), http.StatusUnsupportedMediaType)
			return
		}
		eu, answered, err := a.resolver.EndUser(w, r)
		if answered {
			return
		}
		prefix := basePath + "/p/" + r.PathValue("handle") + "/api"
		http.StripPrefix(prefix, inner).ServeHTTP(w, r.WithContext(resolved(r.Context(), eu, err)))
	})
}

type resolvedKey struct{}

func resolved(ctx context.Context, eu session.EndUser, err error) context.Context {
	if err != nil {
		return context.WithValue(ctx, resolvedKey{}, err)
	}
	ctx = tenant.WithProject(tenant.WithOrg(ctx, eu.OrgID), eu.ProjectID)
	return session.EndUserInto(ctx, eu)
}

// SECURITY: only a signed-in end user reaches a listed procedure, and a SelfRecent one only after a fresh sign-in.
func endUserGate(now func() time.Time) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			marker := permission.EndUserTable()[req.Spec().Procedure]
			if marker == "" {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("account: no procedure %s", req.Spec().Procedure))
			}
			if err, ok := ctx.Value(resolvedKey{}).(error); ok {
				return nil, err
			}
			eu, ok := session.EndUserFrom(ctx)
			switch {
			case !ok:
				return nil, &enduser.SignedOutError{}
			case marker == permission.SelfRecent && now().Sub(eu.AuthTime) > permission.RecentSignIn:
				return nil, &pat.ReauthRequiredError{}
			}
			return next(ctx, req)
		}
	}
}

func (a *AccountPlane) recoverPanic(ctx context.Context, _ connect.Spec, _ http.Header, p any) error {
	return Translate(ctx, fmt.Errorf("panic: %v", p), a.unexpected)
}

// PortalWriteRule limits an end user's portal writes per user session: 30 a minute.
func PortalWriteRule() ratelimit.Rule {
	return ratelimit.Rule{Name: "account-write-session", Rate: 30, Per: time.Minute, Burst: 30}
}

// TokenCreateRule limits personal-token creation per user session: 10 an hour.
func TokenCreateRule() ratelimit.Rule {
	return ratelimit.Rule{Name: "account-token-create-session", Rate: 10, Per: time.Hour, Burst: 10}
}

// SECURITY: every portal write counts against the user session, and creating a token against a tighter rule too.
func portalLimits(limits layout.Limits) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			proc := req.Spec().Procedure
			method := proc[strings.LastIndex(proc, "/")+1:]
			if strings.HasPrefix(method, "Get") || strings.HasPrefix(method, "List") {
				return next(ctx, req)
			}
			eu, _ := session.EndUserFrom(ctx)
			subject := ratelimit.ProjectSubject(eu.ProjectID, ratelimit.UserSessionSubject(eu.UserSession))
			rules := []ratelimit.Rule{PortalWriteRule()}
			if proc == accountv1connect.AccountServiceCreatePersonalTokenProcedure {
				rules = append(rules, TokenCreateRule())
			}
			for _, rule := range rules {
				if err := limits.Allow(ctx, ratelimit.Key(rule, subject), rule); err != nil {
					return nil, err
				}
			}
			return next(ctx, req)
		}
	}
}
