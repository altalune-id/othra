package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/auth/views"
	"altalune.id/othra/internal/kernel/session"
	"altalune.id/othra/internal/user"
	"altalune.id/othra/internal/web/layout"
	"altalune.id/othra/oidcrp"
)

const (
	signInCookieName = "othra_signin"
	signInTTL        = 10 * time.Minute
)

// ConsoleRP is the console's relying party at /p/system; boot wires it over oidcrp with the in-process transport.
type ConsoleRP interface {
	AuthCodeURL(ctx context.Context, req oidcrp.AuthRequest) (string, error)
	Exchange(ctx context.Context, cb oidcrp.Callback, req oidcrp.AuthRequest) (*oidcrp.IDToken, error)
	EndSessionURL(ctx context.Context, idTokenHint, postLogoutRedirectURI, state string) (string, error)
	EndSystemSession(ctx context.Context, idToken string) error
}

// ConsoleSignIn is the console's "Continue with Othra": the relying party, where sign-out lands, and whether passwords are on.
type ConsoleSignIn struct {
	RP          ConsoleRP
	SignedOut   string
	PasswordOff bool
	OpenSignup  bool
}

type signInState struct {
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
	ReturnTo string `json:"r,omitempty"`
	Invite   string `json:"i,omitempty"`
	IssuedAt int64  `json:"t"`
}

// WithConsoleSignIn adds GET /login/oidc and GET /oauth/callback, and makes sign-out end the /p/system session too.
func (h *Web) WithConsoleSignIn(c ConsoleSignIn) *Web {
	h.console = &c
	h.signInCookie = session.SignedCookie{
		Name: signInCookieName, Path: h.kit.Path("/oauth/callback"), Secret: h.sessions.Secret, Secure: h.sessions.Cookie.Secure,
	}
	return h
}

func (h *Web) othraURL(returnTo, loginHint string) string {
	if h.console == nil {
		return ""
	}
	q := url.Values{}
	if returnTo != "" {
		q.Set("return_to", returnTo)
	}
	if loginHint != "" {
		q.Set("login_hint", loginHint)
	}
	if len(q) == 0 {
		return h.kit.Path("/login/oidc")
	}
	return h.kit.Path("/login/oidc") + "?" + q.Encode()
}

// SECURITY: RFC 9700 §2.1.1, a fresh state, nonce and S256 verifier per attempt, kept in a signed HttpOnly cookie on the callback path.
func (h *Web) startOIDC(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if err := h.limits.LimitIP(r, LoginIPRule()); err != nil {
		h.loginLimited(w, r, err, false, views.LoginData{})
		return
	}
	q := r.URL.Query()
	prompt := q.Get("prompt")
	if prompt != "select_account" && prompt != "login" {
		prompt = ""
	}
	hint := ""
	if email, err := user.NormalizeEmail(q.Get("login_hint")); err == nil {
		hint = email
	}
	req, err := oidcrp.NewAuthRequest(prompt, hint)
	if err != nil {
		h.kit.Fail(w, r, "web: console sign-in", err)
		return
	}
	target, err := h.console.RP.AuthCodeURL(r.Context(), req)
	if err != nil {
		h.kit.Fail(w, r, "web: console sign-in", err)
		return
	}
	raw, err := json.Marshal(signInState{
		State: req.State, Nonce: req.Nonce, Verifier: req.CodeVerifier,
		ReturnTo: layout.SanitizeReturnTo(h.kit.BasePath, q.Get("return_to")), Invite: q.Get("invite"), IssuedAt: h.signInNow().Unix(),
	})
	if err != nil {
		h.kit.Fail(w, r, "web: console sign-in", err)
		return
	}
	h.signInCookie.Set(w, base64.RawURLEncoding.EncodeToString(raw), h.signInNow().Add(signInTTL))
	http.Redirect(w, r, target, http.StatusFound) //nolint:gosec // G710: the authorize URL of /p/system's own discovery.
}

// SECURITY: the in-process code exchange skips /token's limits, so the callback takes the login IP limit itself.
func (h *Web) callback(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if err := h.limits.LimitIP(r, LoginIPRule()); err != nil {
		h.loginLimited(w, r, err, false, views.LoginData{})
		return
	}
	st, ok := h.readSignIn(r)
	h.signInCookie.Clear(w)
	if !ok {
		h.signInFailed(w, r, &SignInExpiredError{}, "", "")
		return
	}
	req := oidcrp.AuthRequest{State: st.State, Nonce: st.Nonce, CodeVerifier: st.Verifier}
	tok, err := h.console.RP.Exchange(r.Context(), oidcrp.ParseCallback(r.URL.Query()), req)
	var rpErr *oidcrp.Error
	switch {
	case errors.As(err, &rpErr) && rpErr.Reason == oidcrp.ReasonCancelled:
		http.Redirect(w, r, h.kit.Path("/login"), http.StatusSeeOther)
		return
	case errors.As(err, &rpErr) && (expiredReason(rpErr.Reason) || refusedCode(rpErr)):
		h.signInFailed(w, r, &SignInExpiredError{}, "", st.ReturnTo)
		return
	case err != nil:
		h.kit.Fail(w, r, "web: console sign-in exchange", err)
		return
	}
	p, err := h.signIn.LoginOIDC(r.Context(), OIDCSignIn{
		Subject: tok.Subject, Email: tok.Email, EmailVerified: tok.EmailVerified, Name: tok.Name, IDToken: tok.Raw, Invite: st.Invite,
	})
	if err != nil {
		h.signInFailed(w, r, err, tok.Email, st.ReturnTo)
		return
	}
	to, err := h.afterSignIn(r.Context(), st, tok.Email)
	if err != nil {
		h.kit.Fail(w, r, "web: console sign-in invite", err)
		return
	}
	if _, _, err := h.sessions.Start(r.Context(), w, p); err != nil {
		h.kit.Fail(w, r, "web: start session", err)
		return
	}
	http.Redirect(w, r, to, http.StatusSeeOther) //nolint:gosec // G710: ResolveReturnTo sanitizes.
}

// NOTE: a sign-in from an invite for another email lands on the invite, whose Mismatch state offers to switch account.
func (h *Web) afterSignIn(ctx context.Context, st signInState, email string) (string, error) {
	if st.Invite != "" {
		other, err := h.signIn.InviteMismatch(ctx, st.Invite, email)
		if err != nil || other {
			return h.kit.Path("/invite"), err
		}
	}
	return layout.ResolveReturnTo(h.kit.BasePath, st.ReturnTo), nil
}

// NOTE: a forged or replayed callback, a mix-up or a token that fails validation are all "start again", never upstream text.
func expiredReason(r oidcrp.Reason) bool {
	switch r {
	case oidcrp.ReasonState, oidcrp.ReasonIssuer, oidcrp.ReasonNonce, oidcrp.ReasonIDToken, oidcrp.ReasonError:
		return true
	}
	return false
}

// NOTE: RFC 6749 §5.2: /token refusing the code (replayed, expired, unknown) is "start again"; a transport or 5xx failure is not.
func refusedCode(e *oidcrp.Error) bool {
	return e.Reason == oidcrp.ReasonTokenExchange && (e.OAuthError == "invalid_grant" || e.OAuthError == "invalid_request")
}

func (h *Web) readSignIn(r *http.Request) (signInState, bool) {
	raw, ok := h.signInCookie.Get(r)
	if !ok {
		return signInState{}, false
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return signInState{}, false
	}
	var st signInState
	if json.Unmarshal(b, &st) != nil || st.State == "" || st.Nonce == "" || st.Verifier == "" {
		return signInState{}, false
	}
	age := h.signInNow().Sub(time.Unix(st.IssuedAt, 0))
	if age < 0 || age > signInTTL {
		return signInState{}, false
	}
	return st, true
}

// NOTE: the signed state carries its own issue time, since a cookie's Max-Age binds only an honest browser.
func (h *Web) signInNow() time.Time {
	if h.signInCookie.Now != nil {
		return h.signInCookie.Now()
	}
	return time.Now()
}

func (h *Web) signInFailed(w http.ResponseWriter, r *http.Request, err error, email, returnTo string) {
	ae, ok := apperror.AsAppError(err)
	if !ok {
		h.kit.Fail(w, r, "web: console sign-in", err)
		return
	}
	v := views.SignInErrorData{
		Code: string(ae.Code()), Reason: ae.Meta()["reason"], Email: email, BackURL: h.kit.Path("/login"),
		PasswordOff: h.console != nil && h.console.PasswordOff,
	}
	status := http.StatusBadRequest
	if IsNoConsoleAccountError(err) {
		status = http.StatusForbidden
	}
	if IsNoConsoleAccountError(err) || IsIdentityConflictError(err) {
		q := url.Values{"prompt": {"select_account"}}
		if returnTo != "" {
			q.Set("return_to", returnTo)
		}
		v.AnotherURL = h.kit.Path("/login/oidc") + "?" + q.Encode()
	}
	h.kit.Render(w, r, status, views.SignInError(h.kit.Layout("Sign in"), v))
}

func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
}
