package hosted

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/oauth/hosted/views"
	"altalune.id/othra/internal/oauth/redirecturi"
)

const (
	maxParamLen  = 512
	maxHintLen   = 8192
	challengeLen = 43
)

const (
	promptNone          = "none"
	promptLogin         = "login"
	promptSelectAccount = "select_account"
	promptConsent       = "consent"
)

// SECURITY: E2: fixed ASCII descriptions only, never the request echoed.
const (
	descResponseType = "response_type must be code"
	descRepeated     = "a parameter is repeated"
	descScope        = "no requested scope is allowed for this client"
	descPKCE         = "code_challenge with code_challenge_method S256 is required"
	descNonce        = "nonce is required when PKCE is not used"
	descPrompt       = "prompt none can't be combined with another value"
	descMaxAge       = "max_age must be a non-negative integer"
	descTooLong      = "the authorization request is too long"
	descHintAccount  = "the signed-in account is not the one id_token_hint names"
	descHintGone     = "the sign-in took too long to confirm the id_token_hint account"
)

var singleParams = []string{ //nolint:gochecknoglobals // the fixed list of one-value authorize parameters.
	"client_id", "redirect_uri", "response_type", "response_mode", "scope", "state", "nonce", "code_challenge",
	"code_challenge_method", "prompt", "max_age", "login_hint", "id_token_hint", "ui_locales", "request", "request_uri", "registration",
}

var knownScopes = []string{"openid", "profile", "email", "offline_access"} //nolint:gochecknoglobals // the scopes this issuer owns.

type authRequest struct {
	client      Client
	redirectURI string
	scopes      []string
	state       string
	nonce       string
	challenge   string
	prompt      map[string]bool
	maxAge      int
	hasMaxAge   bool
	loginHint   string
	idTokenHint string
	hint        *Hint
}

// NOTE: E2: the client hears about this failure by redirect, with state and iss.
type authorizeError struct {
	code        string
	description string
	request     *authRequest
}

func (e *authorizeError) Error() string { return "hosted: authorize error " + e.code }

func parseAuthorize(ctx context.Context, clients Clients, p Project, q url.Values) (*authRequest, error) {
	for _, k := range []string{"client_id", "redirect_uri", "response_mode"} {
		if len(q[k]) > 1 {
			return nil, &RefusedError{Code: apperror.CodeAuthorizeRequestInvalid}
		}
	}
	id := q.Get("client_id")
	if id == "" {
		return nil, &RefusedError{Code: apperror.CodeAuthorizeClientUnknown}
	}
	c, ok, err := clients.Client(ctx, p, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, &RefusedError{Code: apperror.CodeAuthorizeClientUnknown}
	}
	redirect := q.Get("redirect_uri")
	if !registered(c, redirect) {
		return nil, &RefusedError{Code: apperror.CodeAuthorizeRedirectNotRegistered}
	}
	if mode := q.Get("response_mode"); mode != "" && mode != "query" {
		return nil, &RefusedError{Code: apperror.CodeAuthorizeRequestInvalid}
	}
	for _, k := range singleParams {
		if len(q.Get(k)) > maxLen(k) {
			return nil, &RefusedError{Code: apperror.CodeAuthorizeRequestInvalid}
		}
	}
	req := &authRequest{client: c, redirectURI: redirect, state: q.Get("state")}
	err = req.validate(q)
	return req, err
}

func maxLen(param string) int {
	if param == "id_token_hint" || param == "request" {
		return maxHintLen
	}
	return maxParamLen
}

func registered(c Client, redirect string) bool {
	appType := redirecturi.Web
	if c.Native {
		appType = redirecturi.Native
	}
	return slices.ContainsFunc(c.RedirectURIs, func(r string) bool { return redirecturi.Match(appType, r, redirect) })
}

func (r *authRequest) fail(code, description string) error {
	return &authorizeError{code: code, description: description, request: r}
}

func (r *authRequest) validate(q url.Values) error {
	for _, k := range singleParams {
		if len(q[k]) > 1 {
			return r.fail("invalid_request", descRepeated)
		}
	}
	switch {
	case q.Get("request") != "":
		return r.fail("request_not_supported", "request objects are not supported")
	case q.Get("request_uri") != "":
		return r.fail("request_uri_not_supported", "request_uri is not supported")
	case q.Get("registration") != "":
		return r.fail("registration_not_supported", "registration is not supported")
	case q.Get("response_type") == "":
		return r.fail("invalid_request", descResponseType)
	case q.Get("response_type") != "code":
		return r.fail("unsupported_response_type", descResponseType)
	}
	r.scopes = grantedScopes(r.client, strings.Fields(q.Get("scope")))
	if r.scopes == nil {
		return r.fail("invalid_scope", descScope)
	}
	if err := r.pkce(q); err != nil {
		return err
	}
	if err := r.promptAndAge(q); err != nil {
		return err
	}
	r.loginHint, r.idTokenHint = q.Get("login_hint"), q.Get("id_token_hint")
	return nil
}

// NOTE: A5, A19: unknown scopes are ignored, offline_access needs refresh, a known scope the client may not use refuses.
func grantedScopes(c Client, requested []string) []string {
	var out []string
	for _, s := range requested {
		switch {
		case !slices.Contains(knownScopes, s), slices.Contains(out, s):
			continue
		case s == "offline_access" && !c.Refresh:
			continue
		case !slices.Contains(c.AllowedScopes, s):
			return nil
		}
		out = append(out, s)
	}
	return out
}

func (r *authRequest) pkce(q url.Values) error {
	r.nonce = q.Get("nonce")
	challenge, method := q.Get("code_challenge"), q.Get("code_challenge_method")
	switch {
	case challenge == "" && method == "" && !r.client.PKCERequired:
		if r.nonce == "" {
			return r.fail("invalid_request", descNonce)
		}
		return nil
	case method != "S256", !validChallenge(challenge):
		return r.fail("invalid_request", descPKCE)
	}
	r.challenge = challenge
	return nil
}

func validChallenge(s string) bool {
	if len(s) != challengeLen {
		return false
	}
	for i := range len(s) {
		if !base64URLChar(s[i]) {
			return false
		}
	}
	return true
}

// NOTE: RFC 7636 §4.2: an S256 challenge is the unpadded base64url of a SHA-256 digest, 43 characters.
func base64URLChar(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_'
}

func (r *authRequest) promptAndAge(q url.Values) error {
	r.prompt = map[string]bool{}
	for v := range strings.FieldsSeq(q.Get("prompt")) {
		r.prompt[v] = true
	}
	if r.prompt[promptNone] && (r.prompt[promptLogin] || r.prompt[promptSelectAccount] || r.prompt[promptConsent]) {
		return r.fail("invalid_request", descPrompt)
	}
	if raw := q.Get("max_age"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return r.fail("invalid_request", descMaxAge)
		}
		r.maxAge, r.hasMaxAge = n, true
	}
	return nil
}

const authorizePath = "/oauth2/authorize"

func (h *Web) authorize(w http.ResponseWriter, r *http.Request, s *site) {
	if !h.allowIP(w, r, s, AuthorizeIPRule()) {
		return
	}
	q := r.URL.Query()
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			h.refused(w, r, s, apperror.CodeAuthorizeRequestInvalid)
			return
		}
		q = r.PostForm
	}
	req, err := parseAuthorize(r.Context(), h.clients, s.project(), q)
	if !h.authorizeFailed(w, r, s, err) {
		h.decide(w, r, s, req, q)
	}
}

func (h *Web) authorizeFailed(w http.ResponseWriter, r *http.Request, s *site, err error) bool {
	var page *RefusedError
	var redirect *authorizeError
	switch {
	case err == nil:
		return false
	case errors.As(err, &page):
		h.refused(w, r, s, page.Code)
	case errors.As(err, &redirect):
		h.toClient(w, r, s, redirect.request, url.Values{"error": {redirect.code}, "error_description": {redirect.description}})
	default:
		h.fail(w, r, s, "hosted: authorize", err)
	}
	return true
}

// SECURITY: A2, E1, E2: only a redirect URI registered for the client, with state and iss on every answer (RFC 9207).
func (h *Web) toClient(w http.ResponseWriter, r *http.Request, s *site, req *authRequest, params url.Values) {
	u, err := url.Parse(req.redirectURI)
	if err != nil {
		h.fail(w, r, s, "hosted: redirect uri", err)
		return
	}
	q := u.Query()
	maps.Copy(q, params)
	if req.state != "" {
		q.Set("state", req.state)
	}
	q.Set("iss", s.issuerURL)
	u.RawQuery = q.Encode()
	if r.Method == http.MethodPost && r.URL.Path != s.path+authorizePath {
		h.forward(w, r, s, req.client.Name, u.String())
		return
	}
	http.Redirect(w, r, u.String(), http.StatusFound) //nolint:gosec // G710: a redirect URI registered for this client.
}

// NOTE: OIDC Core §3.1.2.1, m5-port §3.1: the decision table; the marker says when Othra last sent the person to sign in or choose.
func (h *Web) decide(w http.ResponseWriter, r *http.Request, s *site, req *authRequest, q url.Values) {
	ctx := r.Context()
	secret := h.cookies.browser(r)
	b, err := h.users.Browser(ctx, s.project(), secret)
	if err != nil {
		h.fail(w, r, s, "hosted: browser", err)
		return
	}
	hint, err := h.hint(ctx, s, req, q)
	if err != nil {
		h.authorizeFailed(w, r, s, err)
		return
	}
	req.hint = hint
	none := req.prompt[promptNone]
	active, signedIn := b.Active()
	if !signedIn {
		h.interact(w, r, s, req, q, none, "login_required", false)
		return
	}
	since, marked := h.marker.Since(q, req.client.ID)
	if h.staleLogin(req, active) && (!marked || !after(active.AuthTime, since)) {
		h.interact(w, r, s, req, q, none, "login_required", true)
		return
	}
	if hint != nil && hint.Subject != active.EndUser && none {
		h.toClient(w, r, s, req, url.Values{"error": {"login_required"}})
		return
	}
	if h.mustChoose(req, b, active, hint) && (!marked || !after(active.SelectedAt, since)) {
		if none && len(b.Accounts) > 1 {
			h.toClient(w, r, s, req, url.Values{"error": {"account_selection_required"}})
			return
		}
		if !none {
			h.interact(w, r, s, req, q, false, "", false)
			return
		}
	}
	if hint != nil && hint.Subject != active.EndUser {
		h.toClient(w, r, s, req, url.Values{"error": {"login_required"}, "error_description": {descHintAccount}})
		return
	}
	h.grant(w, r, s, req, q, secret, active)
}

// NOTE: the marker holds milliseconds, so only a sign-in or choice in a later millisecond satisfies it.
func after(t, marker time.Time) bool { return t.Truncate(time.Millisecond).After(marker) }

func (h *Web) hint(ctx context.Context, s *site, req *authRequest, q url.Values) (*Hint, error) {
	if req.idTokenHint == "" {
		if _, carried := q[HintParam]; !carried {
			return nil, nil
		}
		sub, ok := h.marker.Hinted(q, req.client.ID)
		if !ok {
			return nil, req.fail("login_required", descHintGone)
		}
		return &Hint{Subject: sub}, nil
	}
	hint, ok, err := h.grants.VerifyHint(ctx, s.project(), req.idTokenHint)
	switch {
	case err != nil:
		return nil, err
	case !ok:
		return nil, req.fail("invalid_request", "id_token_hint is not valid")
	}
	return &hint, nil
}

// NOTE: A10: max_age=0 is prompt=login; a longer max_age needs a sign-in no older than it.
func (h *Web) staleLogin(req *authRequest, a Account) bool {
	return req.prompt[promptLogin] || (req.hasMaxAge && (req.maxAge == 0 || time.Since(a.AuthTime) > time.Duration(req.maxAge)*time.Second))
}

// NOTE: m5-port §3.1: the picker when asked, when several accounts share the browser, or when a hint names another account.
func (h *Web) mustChoose(req *authRequest, b *Browser, a Account, hint *Hint) bool {
	switch {
	case req.prompt[promptSelectAccount], len(b.Accounts) > 1:
		return true
	case req.loginHint != "" && !strings.EqualFold(req.loginHint, a.Email):
		return true
	}
	return hint != nil && hint.Subject != a.EndUser
}

func (h *Web) interact(
	w http.ResponseWriter, r *http.Request, s *site, req *authRequest, q url.Values, none bool, noneError string, fresh bool,
) {
	if none {
		h.toClient(w, r, s, req, url.Values{"error": {noneError}})
		return
	}
	target, ok := h.reentry(s, req, h.marker.Mark(q, req.client.ID))
	if !ok {
		h.toClient(w, r, s, req, url.Values{"error": {"invalid_request"}, "error_description": {descTooLong}})
		return
	}
	query := backTo(target)
	if fresh {
		query.Set("add", "1")
	}
	h.toPage(w, r, s, pageLogin, query)
}

var reentryDropped = []string{"id_token_hint", "request", "request_uri", "registration"} //nolint:gochecknoglobals // fixed list.

// SECURITY: F1: the way back to /authorize keeps the request's own parameters and a signed hint subject, never the id_token_hint.
func (h *Web) reentry(s *site, req *authRequest, q url.Values) (string, bool) {
	out := url.Values{}
	for _, k := range singleParams {
		if v := q.Get(k); v != "" && !slices.Contains(reentryDropped, k) {
			out.Set(k, v)
		}
	}
	if v, ok := q[MarkerParam]; ok {
		out[MarkerParam] = v
	}
	if req.hint != nil {
		out = h.marker.WithHint(out, req.client.ID, req.hint.Subject)
	}
	target := s.path + authorizePath + "?" + out.Encode()
	return target, len(target) <= maxReturnToLen
}

func (h *Web) grant(w http.ResponseWriter, r *http.Request, s *site, req *authRequest, q url.Values, secret string, a Account) {
	ctx := r.Context()
	terms, err := h.users.TermsState(ctx, s.project(), secret, a.UserSession)
	switch {
	case codeOf(err) == apperror.CodeSignInSessionGone:
		h.interact(w, r, s, req, q, req.prompt[promptNone], "login_required", false)
		return
	case err != nil:
		h.fail(w, r, s, "hosted: terms state", err)
		return
	case terms.Needed && req.prompt[promptNone]:
		h.toClient(w, r, s, req, url.Values{"error": {"interaction_required"}})
		return
	case terms.Needed:
		target, ok := h.reentry(s, req, q)
		if !ok {
			h.toClient(w, r, s, req, url.Values{"error": {"invalid_request"}, "error_description": {descTooLong}})
			return
		}
		h.toPage(w, r, s, pageTerms, backTo(target))
		return
	case req.client.ConsentRequired:
		// NOTE: D3, D5: no consent page until M6; an implicit-consent client is admin pre-approved, so prompt=consent is satisfied.
		h.toClient(w, r, s, req, url.Values{"error": {"consent_required"}})
		return
	}
	code, err := h.grants.IssueCode(ctx, CodeRequest{
		Project: s.project(), ClientID: req.client.ID, RedirectURI: req.redirectURI, Scopes: req.scopes,
		CodeChallenge: req.challenge, Nonce: req.nonce, UserSession: a.UserSession, EndUser: a.EndUser, AuthTime: a.AuthTime,
	})
	switch {
	case codeOf(err) == apperror.CodeSignInSessionGone:
		h.interact(w, r, s, req, q, req.prompt[promptNone], "login_required", false)
		return
	case err != nil:
		h.fail(w, r, s, "hosted: issue code", err)
		return
	}
	h.toClient(w, r, s, req, url.Values{"code": {code}})
}

type pending struct {
	client string
	hint   string
	reauth bool
}

func (h *Web) pending(r *http.Request, s *site, returnTo string) pending {
	u, err := url.Parse(returnTo)
	if err != nil || h.clients == nil || u.Path != s.path+authorizePath {
		return pending{}
	}
	q := u.Query()
	c, ok, err := h.clients.Client(r.Context(), s.project(), q.Get("client_id"))
	if err != nil || !ok {
		return pending{}
	}
	prompt := strings.Fields(q.Get("prompt"))
	return pending{
		client: c.Name, hint: q.Get("login_hint"),
		reauth: slices.Contains(prompt, promptLogin) || q.Get("max_age") != "",
	}
}

func (h *Web) clientName(r *http.Request, s *site, returnTo string) string {
	return h.pending(r, s, returnTo).client
}

// NOTE: A18, N6: declining the terms records nothing; an authorize request hears access_denied, anything else gets a page.
func (h *Web) declineTerms(w http.ResponseWriter, r *http.Request, s *site, returnTo string) {
	ctx := r.Context()
	if u, err := url.Parse(returnTo); err == nil && h.clients != nil && u.Path == s.path+authorizePath {
		req, err := parseAuthorize(ctx, h.clients, s.project(), u.Query())
		if err == nil {
			h.toClient(w, r, s, req, url.Values{"error": {"access_denied"}, "error_description": {"the end user declined the terms"}})
			return
		}
		if h.authorizeFailed(w, r, s, err) {
			return
		}
	}
	d := views.TermsDeclined(ctx, s.views())
	d.SupportEmail = s.issuer.SupportEmail
	h.kit.Render(w, r, http.StatusOK, views.Error(h.layout(r, s), d))
}
