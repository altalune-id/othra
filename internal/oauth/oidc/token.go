package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/internal/client"
	"altalune.id/othra/internal/enduser"
	"altalune.id/othra/internal/grant"
	"altalune.id/othra/internal/kernel/ratelimit"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/keys"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/web/layout"
)

const (
	maxFormBytes        = 16 << 10
	grantTypeCode       = "authorization_code"
	grantTypeRefresh    = "refresh_token"
	scopeEmail          = "email"
	scopeProfile        = "profile"
	authBasic           = client.AuthClientSecretBasic
	authPost            = client.AuthClientSecretPost
	authNone            = client.AuthNone
	defaultIDTokenTTL   = 5 * time.Minute
	tokenTypeBearer     = "Bearer"
	formContentType     = "application/x-www-form-urlencoded"
	errInvalidRequest   = "invalid_request"
	errInvalidClient    = "invalid_client"
	errInvalidGrant     = "invalid_grant"
	errInvalidScope     = "invalid_scope"
	errInvalidTarget    = "invalid_target"
	errUnauthorized     = "unauthorized_client"
	errUnsupportedGrant = "unsupported_grant_type"
	errServer           = "server_error"
)

// TokenIPRule limits token requests per client IP in a project: 120 a minute, 60 at once.
func TokenIPRule() ratelimit.Rule {
	return ratelimit.Rule{Name: "oauth-token-ip", Rate: 120, Per: time.Minute, Burst: 60}
}

// TokenClientRule limits token requests per client IP and client_id in a project, against secret guessing: 30 a minute.
func TokenClientRule() ratelimit.Rule {
	return ratelimit.Rule{Name: "oauth-token-client-ip", Rate: 30, Per: time.Minute, Burst: 30}
}

// Clients authenticates a project's clients at the token endpoint.
type Clients interface {
	// Authenticate returns the client publicID able to VerifySecret, or *client.NotFoundError.
	Authenticate(ctx context.Context, orgID, projectID uuid.UUID, publicID string) (*client.Client, error)
}

// Grants exchanges codes, rotates refresh tokens and checks access tokens.
type Grants interface {
	Exchange(ctx context.Context, p grant.Project, req grant.ExchangeRequest) (*grant.Tokens, error)
	Refresh(ctx context.Context, p grant.Project, req grant.RefreshRequest) (*grant.Tokens, error)
	Access(ctx context.Context, p grant.Project, token string) (*grant.Access, error)
	Revoke(ctx context.Context, p grant.Project, c grant.Client, token, hint string) error
}

// EndUsers reads the end user a grant names.
type EndUsers interface {
	EndUser(ctx context.Context, p enduser.Project, id uuid.UUID) (*enduser.EndUser, error)
}

// IDTokens signs ID tokens with the project's active key.
type IDTokens interface {
	MintIDToken(ctx context.Context, projectID uuid.UUID, in *keys.MintIDTokenInput) (*keys.Token, error)
}

// Tokens is what the token and userinfo endpoints answer with; without it no client authenticates and no token is valid.
type Tokens struct {
	Clients    Clients
	Grants     Grants
	EndUsers   EndUsers
	IDTokens   IDTokens
	IDTokenTTL time.Duration
	Limits     layout.Limits
	// CORS wraps the token and userinfo endpoints, which browser apps call with an Authorization header.
	CORS func(http.Handler) http.Handler
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope"`
	IDToken      string `json:"id_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type oauthError struct {
	Error       string `json:"error"`
	Description string `json:"error_description,omitempty"`
}

type tokenRequest struct {
	w      http.ResponseWriter
	r      *http.Request
	e      *issuerEntry
	form   url.Values
	basic  bool
	issuer string
}

func (h *Web) token(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST, OPTIONS")
		writeJSON(w, http.StatusMethodNotAllowed, nil)
		return
	}
	e, ok := h.entry(w, r)
	if !ok {
		return
	}
	w.Header().Set("Pragma", "no-cache")
	issuer := project.IssuerURL(h.cfg.PublicBase, e.issuer.Handle)
	if h.tokens == nil {
		w.Header().Set("WWW-Authenticate", `Basic realm="`+issuer+`"`)
		writeJSON(w, http.StatusUnauthorized, invalidClient)
		return
	}
	ip := ratelimit.IPSubject(layout.ClientIP(r, h.tokens.Limits.TrustedProxyHops))
	if !isInProcess(r.Context()) && !h.allow(w, r, e, TokenIPRule(), ip) {
		return
	}
	form, err := postForm(w, r)
	switch {
	case errors.Is(err, errResourceRepeat):
		h.oauthFail(w, http.StatusBadRequest, errInvalidTarget, err.Error())
		return
	case err != nil:
		h.oauthFail(w, http.StatusBadRequest, errInvalidRequest, err.Error())
		return
	}
	req := &tokenRequest{w: w, r: r, e: e, form: form, issuer: issuer}
	c, ok := h.authenticate(req)
	if !ok {
		return
	}
	grantType := form.Get("grant_type")
	switch {
	case grantType == "":
		h.oauthFail(w, http.StatusBadRequest, errInvalidRequest, "grant_type is required")
		return
	case grantType != grantTypeCode && grantType != grantTypeRefresh:
		h.oauthFail(w, http.StatusBadRequest, errUnsupportedGrant, "")
		return
	case !slices.Contains(c.GrantTypes, grantType):
		h.oauthFail(w, http.StatusBadRequest, errUnauthorized, "")
		return
	}
	p := grant.Project{OrgID: e.issuer.OrgID, ID: e.issuer.ProjectID, IssuerURL: issuer}
	who := grantClient(c)
	var issued *grant.Tokens
	if grantType == grantTypeCode {
		if form.Get("code") == "" {
			h.oauthFail(w, http.StatusBadRequest, errInvalidRequest, "code is required")
			return
		}
		issued, err = h.tokens.Grants.Exchange(r.Context(), p, grant.ExchangeRequest{
			Client: who, Code: form.Get("code"), RedirectURI: form.Get("redirect_uri"), CodeVerifier: form.Get("code_verifier"),
			Resource: form.Get("resource"),
		})
	} else {
		if form.Get("refresh_token") == "" {
			h.oauthFail(w, http.StatusBadRequest, errInvalidRequest, "refresh_token is required")
			return
		}
		var scopes []string
		if form.Has("scope") {
			scopes = strings.Fields(form.Get("scope"))
		}
		issued, err = h.tokens.Grants.Refresh(r.Context(), p, grant.RefreshRequest{
			Client: who, RefreshToken: form.Get("refresh_token"), Scopes: scopes, Resource: form.Get("resource"),
		})
	}
	switch {
	case grant.IsInvalidGrantError(err):
		h.oauthFail(w, http.StatusBadRequest, errInvalidGrant, "")
		return
	case grant.IsInvalidScopeError(err):
		h.oauthFail(w, http.StatusBadRequest, errInvalidScope, "")
		return
	case grant.IsInvalidTargetError(err):
		h.oauthFail(w, http.StatusBadRequest, errInvalidTarget, "")
		return
	case grant.IsInvalidRequestError(err):
		h.oauthFail(w, http.StatusBadRequest, errInvalidRequest, "code_verifier was sent for a code without a code_challenge")
		return
	case err != nil:
		h.oauthFail(w, http.StatusInternalServerError, errServer, "")
		return
	}
	h.respondTokens(req, c, issued)
}

// SECURITY: RFC 6749 §2.3.1 and §3.2.1, one authentication method per request, the one the client registered; 401 for every failure.
func (h *Web) authenticate(req *tokenRequest) (*client.Client, bool) {
	r := req.r
	creds, err := readCredentials(req)
	switch {
	case errors.Is(err, errMixedAuth):
		h.oauthFail(req.w, http.StatusBadRequest, errInvalidRequest, err.Error())
		return nil, false
	case err != nil:
		return nil, h.clientFail(req)
	}
	id, secret, method := creds.id, creds.secret, creds.method
	if id == "" {
		return nil, h.clientFail(req)
	}
	c, err := h.tokens.Clients.Authenticate(r.Context(), req.e.issuer.OrgID, req.e.issuer.ProjectID, id)
	switch {
	case client.IsNotFoundError(err):
		return nil, h.clientFail(req)
	case err != nil:
		h.oauthFail(req.w, http.StatusInternalServerError, errServer, "")
		return nil, false
	}
	// SECURITY: the console's client has no secret, so its codes are redeemed only by the console in this process.
	if c.System && !isInProcess(r.Context()) {
		return nil, h.clientFail(req)
	}
	// SECURITY: keyed by a registered client_id only, so made-up ids can't fill the limiter's keys.
	subject := ratelimit.IPSubject(layout.ClientIP(r, h.tokens.Limits.TrustedProxyHops)) + ":client:" + c.PublicID
	if !isInProcess(r.Context()) && !h.allow(req.w, r, req.e, TokenClientRule(), subject) {
		return nil, false
	}
	switch {
	case c.TokenEndpointAuthMethod != method:
		return nil, h.clientFail(req)
	case method != authNone && !c.VerifySecret(secret, time.Now()):
		return nil, h.clientFail(req)
	}
	return c, true
}

type credentials struct {
	id, secret, method string
}

// SECURITY: RFC 6749 §2.3.1, Basic credentials are form-encoded; Basic with client_secret or another client_id in the body is refused.
func readCredentials(req *tokenRequest) (credentials, error) {
	form := req.form
	id, secret, basic := req.r.BasicAuth()
	if basic {
		req.basic = true
		var idErr, secretErr error
		id, idErr = url.QueryUnescape(id)
		secret, secretErr = url.QueryUnescape(secret)
		if idErr != nil || secretErr != nil {
			return credentials{}, errBadCredentials
		}
	}
	switch {
	case basic && (form.Has("client_secret") || form.Get("client_id") != "" && form.Get("client_id") != id):
		return credentials{}, errMixedAuth
	case basic:
		return credentials{id: id, secret: secret, method: authBasic}, nil
	case form.Has("client_secret"):
		return credentials{id: form.Get("client_id"), secret: form.Get("client_secret"), method: authPost}, nil
	}
	return credentials{id: form.Get("client_id"), method: authNone}, nil
}

func grantClient(c *client.Client) grant.Client {
	gc := grant.Client{ID: c.ID, OAuthID: c.OAuthID(), Refresh: c.RefreshTokens()}
	if c.Managed() {
		if u, err := url.Parse(c.MetadataURL); err == nil {
			gc.CIMDHost = u.Hostname()
		}
	}
	return gc
}

func (h *Web) clientFail(req *tokenRequest) bool {
	if req.basic {
		req.w.Header().Set("WWW-Authenticate", `Basic realm="`+req.issuer+`"`)
	}
	h.oauthFail(req.w, http.StatusUnauthorized, errInvalidClient, "")
	return false
}

func (h *Web) respondTokens(req *tokenRequest, c *client.Client, issued *grant.Tokens) {
	ctx, e, f := req.r.Context(), req.e, issued.Family
	out := tokenResponse{
		AccessToken: issued.AccessToken, TokenType: tokenTypeBearer, Scope: strings.Join(issued.Scopes, " "),
		ExpiresIn: max(int64(time.Until(issued.ExpiresAt).Seconds()), 1), RefreshToken: issued.RefreshToken,
	}
	if slices.Contains(issued.Scopes, grant.ScopeOpenID) {
		u, err := h.tokens.EndUsers.EndUser(ctx, enduser.Project{OrgID: e.issuer.OrgID, ID: e.issuer.ProjectID}, f.EndUserID)
		if err != nil || u == nil {
			h.report(ctx, "oidc: id token end user", errors.Join(err, errEndUserGone))
			h.oauthFail(req.w, http.StatusInternalServerError, errServer, "")
			return
		}
		now := time.Now()
		ttl := h.tokens.IDTokenTTL
		if ttl <= 0 {
			ttl = defaultIDTokenTTL
		}
		scoped := tenant.Into(ctx, tenant.Context{OrgID: e.issuer.OrgID, ProjectID: e.issuer.ProjectID})
		id, err := h.tokens.IDTokens.MintIDToken(scoped, e.issuer.ProjectID, &keys.MintIDTokenInput{
			Issuer: req.issuer, Subject: u.Subject, Audience: []string{c.OAuthID()}, ClientID: c.OAuthID(), Nonce: issued.Nonce,
			AccessToken: issued.AccessToken, SessionID: f.PublicID, AuthTime: f.AuthTime, IssuedAt: now, ExpiresAt: now.Add(ttl),
			Profile: profile(u, issued.Scopes),
		})
		if err != nil {
			h.report(ctx, "oidc: mint id token", err)
			h.oauthFail(req.w, http.StatusInternalServerError, errServer, "")
			return
		}
		out.IDToken = id.Compact
	}
	body, err := json.Marshal(out) //nolint:gosec // G117: the token response carries the tokens (RFC 6749 §5.1).
	if err != nil {
		h.oauthFail(req.w, http.StatusInternalServerError, errServer, "")
		return
	}
	writeJSON(req.w, http.StatusOK, body)
}

func profile(u *enduser.EndUser, scopes []string) *keys.Profile {
	return &keys.Profile{
		Email: u.Email, EmailVerified: u.EmailVerified, Name: u.Name,
		WithEmail: slices.Contains(scopes, scopeEmail), WithName: slices.Contains(scopes, scopeProfile) && u.Name != "",
	}
}

func (h *Web) allow(w http.ResponseWriter, r *http.Request, e *issuerEntry, rule ratelimit.Rule, subject string) bool {
	err := h.tokens.Limits.Allow(r.Context(), ratelimit.Key(rule, ratelimit.ProjectSubject(e.issuer.ProjectID, subject)), rule)
	limited, isLimited := errors.AsType[*layout.RateLimitedError](err)
	switch {
	case err == nil:
		return true
	case isLimited:
		w.Header().Set("Retry-After", strconv.Itoa(max(int(limited.RetryAfter.Seconds()), 1)))
		writeJSON(w, http.StatusTooManyRequests, limitedBody)
		return false
	default:
		h.report(r.Context(), "oidc: rate limit", err)
		return true
	}
}

// SECURITY: RFC 6749 §3.2, a form body of at most 16 KB whose parameters each appear once; the query string is ignored.
func postForm(w http.ResponseWriter, r *http.Request) (url.Values, error) {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != formContentType {
		return nil, errFormType
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		return nil, errFormBody
	}
	for k, v := range r.PostForm {
		switch {
		case len(v) < 2:
		case k == "resource" && !slices.ContainsFunc(v, func(s string) bool { return s != v[0] }):
			r.PostForm[k] = v[:1]
		case k == "resource":
			return nil, errResourceRepeat
		default:
			return nil, errFormRepeat
		}
	}
	return r.PostForm, nil
}

func (h *Web) oauthFail(w http.ResponseWriter, status int, code, description string) {
	body, _ := json.Marshal(oauthError{Error: code, Description: description})
	writeJSON(w, status, body)
}
