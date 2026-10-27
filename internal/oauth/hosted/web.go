package hosted

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"

	"altalune.id/othra/apperror"
	"altalune.id/othra/emailaddr"
	"altalune.id/othra/internal/kernel/ratelimit"
	"altalune.id/othra/internal/oauth/hosted/views"
	"altalune.id/othra/internal/oauth/oidc"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/upstream"
	"altalune.id/othra/internal/web/i18n"
	"altalune.id/othra/internal/web/layout"
	"altalune.id/othra/internal/web/portal"
	"altalune.id/othra/mailer"
	"altalune.id/othra/reqid"
	"altalune.id/othra/sealer"
)

const (
	maxFormBytes    = 16 << 10
	maxUserAgentLen = 512
)

// Issuers resolves the {handle} of a hosted request to its project.
type Issuers interface {
	Resolve(r *http.Request) (*project.Issuer, error)
}

// Config is what the hosted pages need from the deployment; boot maps http.* and endUsers.* into it.
type Config struct {
	BasePath   string
	PublicBase string
	MailFrom   string
	// CSRFKey and CookieKey are derived from http.stateSecret, one per purpose.
	CSRFKey    []byte
	CookieKey  []byte
	CSRFMaxAge time.Duration
	LinkTTL    time.Duration
	LogoURL    func(key string) string
}

// Web serves the hosted end-user pages under <basePath>/p/{handle}/.
type Web struct {
	users   EndUsers
	issuers Issuers
	kit     layout.Kit
	cfg     Config
	csrf    *CSRF
	cookies cookies
	limits  layout.Limits
	oauth   OAuth
	fed     Federation
	fedSeal sealer.Sealer

	portalPage *portal.Page
}

// OAuth is what /oauth2/authorize, /consent and /oauth2/end-session need.
type OAuth struct {
	Clients    Clients
	Resources  Resources
	Admissions Admissions
	Consents   Consents
	Grants     Grants
	Marker     *Marker
}

// NewWeb builds the hosted pages over users and issuers.
func NewWeb(users EndUsers, issuers Issuers, kit layout.Kit, cfg Config) *Web {
	return &Web{
		users: users, issuers: issuers, kit: kit, cfg: cfg,
		csrf: NewCSRF(cfg.CSRFKey, cfg.CSRFMaxAge), cookies: cookies{key: cfg.CookieKey, now: time.Now},
	}
}

// WithOAuth serves /oauth2/authorize, /consent and /oauth2/end-session over o.
func (h *Web) WithOAuth(o OAuth) *Web {
	h.oauth = o
	return h
}

// WithLimits counts the hosted forms on l; without it nothing is limited.
func (h *Web) WithLimits(l layout.Limits) *Web {
	h.limits = l
	return h
}

// Register mounts the hosted pages and the branded catch-all 404 on mux.
func (h *Web) Register(mux *http.ServeMux) {
	p := layout.Path(h.cfg.BasePath, "/p/{handle}")
	mux.HandleFunc("GET "+p+"/login", h.page(h.login))
	mux.HandleFunc("POST "+p+"/login", h.page(h.issue))
	mux.HandleFunc("GET "+p+"/login/sent", h.page(h.sent))
	mux.HandleFunc("POST "+p+"/login/resend", h.page(h.resend))
	mux.HandleFunc("GET "+p+"/login/verify", h.page(h.verify))
	mux.HandleFunc("POST "+p+"/login/verify", h.page(h.redeem))
	mux.HandleFunc("POST "+p+"/login/select", h.page(h.selectAccount))
	mux.HandleFunc("POST "+p+"/login/signout", h.page(h.signOut))
	mux.HandleFunc("GET "+p+"/terms", h.page(h.terms))
	mux.HandleFunc("POST "+p+"/terms", h.page(h.acceptTerms))
	mux.HandleFunc("GET "+p+"/account/{rest...}", h.page(h.portal))
	if h.fed != nil {
		mux.HandleFunc("GET "+p+string(linkingPage), h.page(h.linking))
	}
	mux.HandleFunc("GET "+p+"/signed-out", h.page(h.signedOut))
	if h.oauth.Grants != nil {
		mux.HandleFunc("GET "+p+authorizePath, h.page(h.authorize))
		mux.HandleFunc("POST "+p+authorizePath, h.page(h.authorize))
		mux.HandleFunc("GET "+p+endSessionPath, h.page(h.endSession))
		mux.HandleFunc("POST "+p+endSessionPath, h.page(h.endSession))
		mux.HandleFunc("GET "+p+string(pageConsent), h.page(h.consent))
		mux.HandleFunc("POST "+p+string(pageConsent), h.page(h.answerConsent))
	}
	h.registerFederation(mux, p)
	mux.HandleFunc(p+"/", h.page(h.notFound))
}

type site struct {
	issuer    *project.Issuer
	issuerURL string
	path      string
	name      string
}

func (s *site) project() Project {
	i := s.issuer
	return Project{
		OrgID: i.OrgID, ID: i.ProjectID, PublicID: i.PublicID, IssuerURL: s.issuerURL, EmailLink: i.SignInEmailLink, TermsVersion: i.TermsVersion,
		TermsURL: i.TermsURL, PrivacyURL: i.PrivacyURL,
	}
}

func (s *site) views() views.Site { return views.Site{Handle: s.issuer.Handle, Project: s.name} }

func (h *Web) page(fn func(w http.ResponseWriter, r *http.Request, s *site)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
		i, err := h.issuers.Resolve(r)
		switch {
		case errors.Is(err, oidc.ErrUnknownIssuer):
			h.unknown(w, r)
			return
		case err != nil:
			if limited, ok := errors.AsType[*oidc.LookupLimitedError](err); ok {
				h.limited(w, r, nil, &layout.RateLimitedError{RetryAfter: time.Duration(limited.RetryAfter) * time.Second})
				return
			}
			h.fail(w, r, nil, "hosted: resolve issuer", err)
			return
		}
		fn(w, r, h.site(i))
	}
}

func (h *Web) site(i *project.Issuer) *site {
	return &site{
		issuer: i, issuerURL: project.IssuerURL(h.cfg.PublicBase, i.Handle), path: h.kit.Path("/p/" + i.Handle),
		name: cmp.Or(i.DisplayName, i.Name),
	}
}

func (h *Web) layout(r *http.Request, s *site) layout.LayoutData {
	d := layout.LayoutData{BasePath: h.cfg.BasePath, Lang: i18n.Locale(r.Context())}
	if s == nil {
		return d
	}
	logo := ""
	if s.issuer.LogoKey != "" && h.cfg.LogoURL != nil {
		logo = h.cfg.LogoURL(s.issuer.LogoKey)
	}
	d.Brand = layout.NewBrand(s.name, logo, s.issuer.PrimaryColor)
	d.Brand.TermsURL, d.Brand.PrivacyURL = s.issuer.TermsURL, s.issuer.PrivacyURL
	return d
}

func (h *Web) token(w http.ResponseWriter, r *http.Request, s *site, form string) string {
	return h.csrf.Token(h.csrf.Cookie(w, r, s.path+"/"), s.issuer.PublicID, form)
}

func (h *Web) returnTo(s *site, raw string) string {
	return ReturnTo(s.issuerURL, raw)
}

// SECURITY: D3: a form is only handled after its CSRF token and its per-IP limit pass.
func (h *Web) guard(w http.ResponseWriter, r *http.Request, s *site, form string, rules ...ratelimit.Rule) bool {
	if err := h.csrf.Check(r, s.issuer.PublicID, form); err != nil {
		h.formExpired(w, r, s)
		return false
	}
	return h.allowIP(w, r, s, rules...)
}

func (h *Web) allowIP(w http.ResponseWriter, r *http.Request, s *site, rules ...ratelimit.Rule) bool {
	ip := ratelimit.IPSubject(layout.ClientIP(r, h.limits.TrustedProxyHops))
	if err := h.allow(r, ratelimit.ProjectSubject(s.issuer.ProjectID, ip), rules...); err != nil {
		h.limited(w, r, s, err)
		return false
	}
	return true
}

func (h *Web) allow(r *http.Request, subject string, rules ...ratelimit.Rule) error {
	for _, rule := range rules {
		if err := h.limits.Allow(r.Context(), ratelimit.Key(rule, subject), rule); err != nil {
			return err
		}
	}
	return nil
}

func (h *Web) login(w http.ResponseWriter, r *http.Request, s *site) {
	returnTo := h.returnTo(s, r.URL.Query().Get("return_to"))
	b, err := h.users.Browser(r.Context(), s.project(), h.cookies.browser(r))
	if err != nil {
		h.fail(w, r, s, "hosted: browser", err)
		return
	}
	notice, _ := h.cookies.flash(noticeCookie, s.path+"/login").Get(r)
	if notice != "" {
		h.cookies.flash(noticeCookie, s.path+"/login").Clear(w)
	}
	reauth := h.portalReauth(r, s, returnTo)
	if len(b.Accounts) > 0 && r.URL.Query().Get("add") != "1" && !reauth {
		h.renderPicker(w, r, s, b, returnTo, notice, "")
		return
	}
	google := h.googleOn(r, s)
	if !s.issuer.SignInEmailLink && !google {
		h.codeError(w, r, s, http.StatusForbidden, apperror.CodeSignInEmailDisabled)
		return
	}
	pend := h.pending(r, s, returnTo)
	if a, ok := b.Active(); ok && reauth {
		pend.reauth, pend.hint = true, a.Email
	}
	h.renderLogin(w, r, s, http.StatusOK, views.LoginData{
		ReturnTo: returnTo, Notice: notice, FromPicker: len(b.Accounts) > 0,
		Client: pend.client, ClientHost: pend.host, Reauth: pend.reauth, Email: pend.hint,
	})
}

// NOTE: M8 G5, P7: the portal asks prompt=login before it creates a token; a new sign-in here refreshes the account's auth_time.
func (h *Web) portalReauth(r *http.Request, s *site, returnTo string) bool {
	u, err := url.Parse(returnTo)
	return err == nil && r.URL.Query().Get("prompt") == promptLogin && strings.HasPrefix(u.Path, s.path+string(pageAccount))
}

func (h *Web) renderLogin(w http.ResponseWriter, r *http.Request, s *site, status int, v views.LoginData) {
	v.Site, v.LoginToken, v.EmailOff = s.views(), h.token(w, r, s, "login"), !s.issuer.SignInEmailLink
	if h.googleOn(r, s) {
		v.Google, v.GoogleToken = true, h.token(w, r, s, federationForm)
	}
	h.kit.Render(w, r, status, views.Login(h.layout(r, s), v))
}

func (h *Web) renderPicker(w http.ResponseWriter, r *http.Request, s *site, b *Browser, returnTo, notice, errMsg string) {
	tiles := make([]views.AccountTile, 0, len(b.Accounts))
	for _, a := range b.Accounts {
		tiles = append(tiles, views.AccountTile{SessionID: a.UserSession, Email: a.Email, Google: a.Method == string(upstream.KindGoogle)})
	}
	if len(tiles) == 0 {
		h.toPage(w, r, s, pageLogin, backTo(returnTo))
		return
	}
	pend := h.pending(r, s, returnTo)
	h.kit.Render(w, r, http.StatusOK, views.Picker(h.layout(r, s), views.PickerData{
		Site: s.views(), Accounts: tiles, ReturnTo: returnTo, Notice: notice, Error: errMsg, Client: pend.client, ClientHost: pend.host,
		SelectToken: h.token(w, r, s, "select"), SignOutToken: h.token(w, r, s, "signout"),
	}))
}

func (h *Web) issue(w http.ResponseWriter, r *http.Request, s *site) {
	if !h.guard(w, r, s, "login", IssueIPRules()...) {
		return
	}
	returnTo := h.returnTo(s, r.PostFormValue("return_to"))
	if !s.issuer.SignInEmailLink {
		h.codeError(w, r, s, http.StatusForbidden, apperror.CodeSignInEmailDisabled)
		return
	}
	raw := strings.TrimSpace(r.PostFormValue("email"))
	email, err := emailaddr.Normalize(raw)
	if err != nil {
		b, err := h.users.Browser(r.Context(), s.project(), h.cookies.browser(r))
		if err != nil {
			h.fail(w, r, s, "hosted: browser", err)
			return
		}
		pend := h.pending(r, s, returnTo)
		h.renderLogin(w, r, s, http.StatusUnprocessableEntity, views.LoginData{
			Email: raw, Error: i18n.T(r.Context(), "login.email_invalid"), ReturnTo: returnTo, FromPicker: len(b.Accounts) > 0,
			Client: pend.client, ClientHost: pend.host, Reauth: pend.reauth,
		})
		return
	}
	if _, ok := h.sendLink(w, r, s, email, returnTo, nil); !ok {
		return
	}
	h.cookies.flash(sentCookie, s.path+"/login").Set(w, email, time.Now().Add(flashTTL))
	h.toPage(w, r, s, pageSent, backTo(returnTo))
}

// SECURITY: S5, S11: the per-email limit runs after the cooldown, so a cooled-down request spends nothing; both answer as sent.
func (h *Web) sendLink(
	w http.ResponseWriter, r *http.Request, s *site, email, returnTo string, pending *pendingLink,
) (Link, bool) {
	var raw []byte
	if pending != nil {
		var err error
		if raw, err = json.Marshal(pending); err != nil {
			h.fail(w, r, s, "hosted: pending identity", err)
			return Link{}, false
		}
	}
	var limiterErr error
	allow := func(context.Context) (bool, error) {
		err := h.allow(r, ratelimit.ProjectSubject(s.issuer.ProjectID, ratelimit.EmailSubject(email)), IssueEmailRules()...)
		switch {
		case layout.IsRateLimitedError(err):
			return false, nil
		case err != nil:
			limiterErr = err
			return false, err
		}
		return true, nil
	}
	l, _, err := h.users.IssueLink(r.Context(), IssueRequest{
		Project: s.project(), Email: email, ReturnTo: returnTo, IP: layout.ClientIP(r, h.limits.TrustedProxyHops),
		Compose: func(l Link) (mailer.Message, error) { return h.compose(r, s, l, pending) },
		Allow:   allow, Pending: raw,
	})
	switch {
	case limiterErr != nil:
		h.limited(w, r, s, limiterErr)
		return Link{}, false
	case err != nil:
		h.fail(w, r, s, "hosted: issue link", err)
		return Link{}, false
	}
	return l, true
}

func (h *Web) compose(r *http.Request, s *site, l Link, pending *pendingLink) (mailer.Message, error) {
	m := views.MailData{
		Project: s.name, Link: l.URL, Minutes: int(h.cfg.LinkTTL / time.Minute), SignUp: l.SignUp,
		HasReplyTo: s.issuer.SupportEmail != "", Primary: s.issuer.PrimaryColor,
	}
	if pending != nil {
		m.GoogleEmail, m.GoogleSignUp = cmp.Or(pending.Email, l.Email), pending.SignUp
	}
	var html bytes.Buffer
	if err := views.MailHTML(m).Render(r.Context(), &html); err != nil {
		return mailer.Message{}, err
	}
	from := h.cfg.MailFrom
	if addr, err := mail.ParseAddress(from); err == nil {
		from = (&mail.Address{Name: s.name, Address: addr.Address}).String()
	}
	return mailer.Message{
		To: l.Email, From: from, ReplyTo: s.issuer.SupportEmail, Subject: views.MailSubject(r.Context(), m),
		TextBody: views.MailText(r.Context(), m), HTMLBody: html.String(),
	}, nil
}

func (h *Web) sent(w http.ResponseWriter, r *http.Request, s *site) {
	returnTo := h.returnTo(s, r.URL.Query().Get("return_to"))
	email, ok := h.cookies.flash(sentCookie, s.path+"/login").Get(r)
	if !ok {
		h.toPage(w, r, s, pageLogin, backTo(returnTo))
		return
	}
	h.kit.Render(w, r, http.StatusOK, views.Sent(h.layout(r, s), views.SentData{
		Site: s.views(), Email: email, Minutes: int(h.cfg.LinkTTL / time.Minute), ReturnTo: returnTo,
		ResendToken: h.token(w, r, s, "resend"), Resent: r.URL.Query().Get("resent") == "1",
	}))
}

func (h *Web) resend(w http.ResponseWriter, r *http.Request, s *site) {
	if !h.guard(w, r, s, "resend", IssueIPRules()...) {
		return
	}
	returnTo := h.returnTo(s, r.PostFormValue("return_to"))
	email, ok := h.cookies.flash(sentCookie, s.path+"/login").Get(r)
	if !ok {
		h.toPage(w, r, s, pageLogin, backTo(returnTo))
		return
	}
	if _, ok := h.sendLink(w, r, s, email, returnTo, nil); !ok {
		return
	}
	h.toPage(w, r, s, pageSent, url.Values{"resent": {"1"}, "return_to": {returnTo}})
}

func (h *Web) verify(w http.ResponseWriter, r *http.Request, s *site) {
	if t := r.URL.Query().Get("t"); t != "" {
		h.cookies.setLink(w, s.path+"/login/verify", t, h.cfg.LinkTTL)
		h.toPage(w, r, s, pageVerify, nil)
		return
	}
	l, err := h.users.LinkByToken(r.Context(), s.project(), h.cookies.link(r))
	if err != nil {
		h.linkFailed(w, r, s, err)
		return
	}
	v := views.VerifyData{Site: s.views(), Email: l.Email, VerifyToken: h.token(w, r, s, "verify")}
	if pend, ok := h.pendingIdentity(r, l); ok && pend.SignUp {
		v.SignUpEmail = cmp.Or(pend.Email, l.Email)
	} else if ok {
		v.LinkEmail = pend.Email
	}
	h.kit.Render(w, r, http.StatusOK, views.Verify(h.layout(r, s), v))
}

func (h *Web) redeem(w http.ResponseWriter, r *http.Request, s *site) {
	if err := h.csrf.Check(r, s.issuer.PublicID, "verify"); err != nil {
		h.repeatContinue(w, r, s)
		return
	}
	if !h.allowIP(w, r, s, RedeemIPRule()) {
		return
	}
	got, err := h.users.Redeem(r.Context(), RedeemRequest{
		Project: s.project(), Token: h.cookies.link(r), BrowserSecret: h.cookies.browser(r),
		IP: layout.ClientIP(r, h.limits.TrustedProxyHops), UserAgent: userAgent(r), Confirm: h.confirmPending(r, s),
	})
	switch codeOf(err) {
	case apperror.CodeSignInUpstreamLinkedElsewhere:
		h.upstreamFailed(w, r, s, h.returnTo(s, ""), err)
		return
	case apperror.CodeSignInEmailDisabled:
		h.cookies.clearLink(w, s.verifyPath())
		h.cookies.clearBinding(w, s.verifyPath())
		h.codeError(w, r, s, http.StatusForbidden, apperror.CodeSignInEmailDisabled)
		return
	}
	if err != nil {
		h.linkFailed(w, r, s, err)
		return
	}
	h.cookies.clearLink(w, s.path+"/login/verify")
	h.cookies.clearBinding(w, s.verifyPath())
	h.cookies.setBrowser(w, s.path+"/", got.BrowserSecret, got.ExpiresAt)
	h.csrf.Renew(w, s.path+"/")
	h.next(w, r, s, got.BrowserSecret, got.Account, h.returnTo(s, got.ReturnTo))
}

// NOTE: H23, a second Continue carries a token from before the sign-in's CSRF renewal; it lands on the account page.
func (h *Web) repeatContinue(w http.ResponseWriter, r *http.Request, s *site) {
	b, err := h.users.Browser(r.Context(), s.project(), h.cookies.browser(r))
	if err != nil {
		h.fail(w, r, s, "hosted: browser", err)
		return
	}
	if _, ok := b.Active(); ok && h.cookies.link(r) == "" {
		h.toPage(w, r, s, pageAccount, nil)
		return
	}
	h.formExpired(w, r, s)
}

func (h *Web) linkFailed(w http.ResponseWriter, r *http.Request, s *site, err error) {
	if codeOf(err) != apperror.CodeSignInLinkNotUsable {
		h.fail(w, r, s, "hosted: magic link", err)
		return
	}
	h.cookies.clearLink(w, s.path+"/login/verify")
	h.kit.Render(w, r, http.StatusBadRequest, views.LinkUnusable(h.layout(r, s), views.LinkUnusableData{
		Site: s.views(), ReturnTo: h.returnTo(s, ""),
	}))
}

func (h *Web) next(w http.ResponseWriter, r *http.Request, s *site, secret string, a Account, returnTo string) {
	state, err := h.users.TermsState(r.Context(), s.project(), secret, a.UserSession)
	switch {
	case codeOf(err) == apperror.CodeSignInSessionGone:
		h.toPage(w, r, s, pageLogin, backTo(returnTo))
	case err != nil:
		h.fail(w, r, s, "hosted: terms state", err)
	case state.Needed:
		h.toPage(w, r, s, pageTerms, backTo(returnTo))
	default:
		h.toReturnTo(w, r, s, returnTo)
	}
}

func (h *Web) selectAccount(w http.ResponseWriter, r *http.Request, s *site) {
	if !h.guard(w, r, s, "select", FormIPRule()) {
		return
	}
	returnTo := h.returnTo(s, r.PostFormValue("return_to"))
	secret := h.cookies.browser(r)
	a, err := h.users.Select(r.Context(), s.project(), secret, r.PostFormValue("user_session"))
	if err != nil {
		h.sessionFailed(w, r, s, secret, returnTo, err)
		return
	}
	h.next(w, r, s, secret, a, returnTo)
}

func (h *Web) sessionFailed(w http.ResponseWriter, r *http.Request, s *site, secret, returnTo string, err error) {
	if codeOf(err) != apperror.CodeSignInSessionGone {
		h.fail(w, r, s, "hosted: session", err)
		return
	}
	b, err := h.users.Browser(r.Context(), s.project(), secret)
	if err != nil {
		h.fail(w, r, s, "hosted: browser", err)
		return
	}
	h.renderPicker(w, r, s, b, returnTo, "", i18n.Error(r.Context(), apperror.CodeSignInSessionGone))
}

func (h *Web) signOut(w http.ResponseWriter, r *http.Request, s *site) {
	if !h.guard(w, r, s, "signout", FormIPRule()) {
		return
	}
	returnTo := h.returnTo(s, r.PostFormValue("return_to"))
	secret, userSession := h.cookies.browser(r), r.PostFormValue("user_session")
	b, err := h.users.Browser(r.Context(), s.project(), secret)
	if err != nil {
		h.fail(w, r, s, "hosted: browser", err)
		return
	}
	a, known := b.Find(userSession)
	if err := h.users.SignOut(r.Context(), s.project(), secret, userSession); err != nil {
		h.fail(w, r, s, "hosted: sign out", err)
		return
	}
	if r.PostFormValue("next") == "signed_out" {
		h.toPage(w, r, s, pageSignedOut, nil)
		return
	}
	if known {
		h.cookies.flash(noticeCookie, s.path+"/login").Set(w, a.Email, time.Now().Add(flashTTL))
	}
	h.toPage(w, r, s, pageLogin, backTo(returnTo))
}

func (h *Web) terms(w http.ResponseWriter, r *http.Request, s *site) {
	returnTo := h.returnTo(s, r.URL.Query().Get("return_to"))
	a, ok := h.active(w, r, s, returnTo)
	if !ok {
		return
	}
	state, err := h.users.TermsState(r.Context(), s.project(), h.cookies.browser(r), a.UserSession)
	if err != nil {
		h.fail(w, r, s, "hosted: terms state", err)
		return
	}
	if !state.Needed {
		h.toReturnTo(w, r, s, returnTo)
		return
	}
	h.renderTerms(w, r, s, http.StatusOK, a, state.Updated, returnTo, "")
}

func (h *Web) renderTerms(w http.ResponseWriter, r *http.Request, s *site, status int, a Account, updated bool, returnTo, errMsg string) {
	h.kit.Render(w, r, status, views.Terms(h.layout(r, s), views.TermsData{
		Site: s.views(), Email: a.Email, UserSession: a.UserSession, ReturnTo: returnTo, Updated: updated,
		TermsURL: s.issuer.TermsURL, PrivacyURL: s.issuer.PrivacyURL, Error: errMsg, TermsToken: h.token(w, r, s, "terms"),
		Version: s.issuer.TermsVersion,
	}))
}

// SECURITY: T4: the account comes from this browser's sessions; the form only names which of them.
func (h *Web) acceptTerms(w http.ResponseWriter, r *http.Request, s *site) {
	if !h.guard(w, r, s, "terms", FormIPRule()) {
		return
	}
	returnTo := h.returnTo(s, r.PostFormValue("return_to"))
	secret, userSession := h.cookies.browser(r), r.PostFormValue("user_session")
	b, err := h.users.Browser(r.Context(), s.project(), secret)
	if err != nil {
		h.fail(w, r, s, "hosted: browser", err)
		return
	}
	a, ok := b.Find(userSession)
	if !ok {
		h.renderPicker(w, r, s, b, returnTo, "", i18n.Error(r.Context(), apperror.CodeSignInSessionGone))
		return
	}
	if r.PostFormValue("decline") == "1" {
		h.declineTerms(w, r, s, returnTo)
		return
	}
	if r.PostFormValue("accept_terms") != "1" {
		state, err := h.users.TermsState(r.Context(), s.project(), h.cookies.browser(r), a.UserSession)
		if err != nil {
			h.fail(w, r, s, "hosted: terms state", err)
			return
		}
		h.renderTerms(w, r, s, http.StatusUnprocessableEntity, a, state.Updated, returnTo, i18n.T(r.Context(), "terms.must_accept"))
		return
	}
	err = h.users.AcceptTerms(r.Context(), TermsAcceptance{
		Project: s.project(), BrowserSecret: secret, UserSession: userSession, Seen: seenVersion(r),
		IP: layout.ClientIP(r, h.limits.TrustedProxyHops), UserAgent: userAgent(r),
	})
	if errors.Is(err, ErrTermsChanged) {
		h.toPage(w, r, s, pageTerms, backTo(returnTo))
		return
	}
	if err != nil {
		h.sessionFailed(w, r, s, secret, returnTo, err)
		return
	}
	h.toReturnTo(w, r, s, returnTo)
}

func (h *Web) active(w http.ResponseWriter, r *http.Request, s *site, returnTo string) (Account, bool) {
	b, err := h.users.Browser(r.Context(), s.project(), h.cookies.browser(r))
	if err != nil {
		h.fail(w, r, s, "hosted: browser", err)
		return Account{}, false
	}
	a, ok := b.Active()
	if !ok {
		h.toPage(w, r, s, pageLogin, backTo(returnTo))
	}
	return a, ok
}

func (h *Web) signedOut(w http.ResponseWriter, r *http.Request, s *site) {
	h.kit.Render(w, r, http.StatusOK, views.SignedOut(h.layout(r, s), views.SignedOutData{Site: s.views()}))
}

func (h *Web) notFound(w http.ResponseWriter, r *http.Request, s *site) {
	ctx := r.Context()
	h.renderError(w, r, s, http.StatusNotFound, i18n.T(ctx, "error.not_found_title"), i18n.T(ctx, "error.not_found_body"), "")
}

func (h *Web) unknown(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h.renderError(w, r, nil, http.StatusNotFound, i18n.T(ctx, "error.not_found_title"), i18n.T(ctx, "error.not_found_body"), "")
}

func (h *Web) formExpired(w http.ResponseWriter, r *http.Request, s *site) {
	ctx := r.Context()
	h.renderError(w, r, s, http.StatusForbidden, i18n.T(ctx, "error.form_expired_title"), i18n.T(ctx, "error.form_expired_body"),
		apperror.CodeSignInFormExpired)
}

// SECURITY: a limited form answers 429 with Retry-After; a broken limiter fails closed with 503.
func (h *Web) limited(w http.ResponseWriter, r *http.Request, s *site, err error) {
	ctx := r.Context()
	rl, ok := errors.AsType[*layout.RateLimitedError](err)
	if !ok {
		layout.LimiterFailed(w, r, h.kit, err)
		h.codeError(w, r, s, http.StatusServiceUnavailable, apperror.CodeUnexpected)
		return
	}
	retry := rl.RetryAfterSeconds()
	w.Header().Set("Retry-After", strconv.Itoa(retry))
	h.renderError(w, r, s, http.StatusTooManyRequests, i18n.T(ctx, "error.rate_limited_title"),
		i18n.T(ctx, "error.rate_limited_body", "retry_after", strconv.Itoa(retry)), apperror.CodeAuthRateLimited)
}

func (h *Web) fail(w http.ResponseWriter, r *http.Request, s *site, msg string, err error) {
	if h.kit.Unexpected != nil {
		_ = h.kit.Unexpected(r.Context(), msg, err)
	}
	h.codeError(w, r, s, http.StatusInternalServerError, apperror.CodeUnexpected)
}

func (h *Web) codeError(w http.ResponseWriter, r *http.Request, s *site, status int, code apperror.Code) {
	var args []string
	if s != nil {
		args = []string{"project", s.views().Project}
	}
	h.renderError(w, r, s, status, "", i18n.Error(r.Context(), code, args...), code)
}

func (h *Web) renderError(w http.ResponseWriter, r *http.Request, s *site, status int, title, message string, code apperror.Code) {
	v := views.ErrorData{Title: title, Message: message, Code: string(code), Reference: reqid.FromContext(r.Context())}
	if s != nil {
		v.Site, v.SupportEmail = s.views(), s.issuer.SupportEmail
		v.NoSignIn = protocolPage(r, s)
	}
	h.kit.Render(w, r, status, views.Error(h.layout(r, s), v))
}

// NOTE: product-design-m6 §6: an error answering /authorize or /end-session leads back to the app, never into sign-in.
func protocolPage(r *http.Request, s *site) bool {
	return r.URL.Path == s.path+authorizePath || r.URL.Path == s.path+endSessionPath
}

type page string

const (
	pageLogin     page = "/login"
	pageSent      page = "/login/sent"
	pageVerify    page = "/login/verify"
	pageTerms     page = "/terms"
	pageAccount   page = "/account/"
	pageSignIn    page = "/account/sign-in"
	pageSignedOut page = "/signed-out"
)

func backTo(returnTo string) url.Values { return url.Values{"return_to": {returnTo}} }

// SECURITY: a fixed page of this issuer; query values are encoded, never a path.
func (h *Web) toPage(w http.ResponseWriter, r *http.Request, s *site, p page, query url.Values) {
	target := s.path + string(p)
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	http.Redirect(w, r, target, http.StatusSeeOther) //nolint:gosec // G710: a fixed page under this issuer's path.
}

// SECURITY: H8, RFC 9700 §4.11: every return_to redirect runs ReturnTo here, whatever the caller checked.
func (h *Web) toReturnTo(w http.ResponseWriter, r *http.Request, s *site, raw string) {
	target := ReturnTo(s.issuerURL, raw)
	if r.Method == http.MethodPost && strings.HasPrefix(target, s.path+authorizePath+"?") {
		h.forward(w, r, s, h.pending(r, s, target).client, target)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther) //nolint:gosec // G710: target is validated by ReturnTo (H8).
}

// SECURITY: G4, S11: after a POST the next hop is a GET, so form-action never blocks the 302; target is ReturnTo's or registered.
func (h *Web) forward(w http.ResponseWriter, r *http.Request, s *site, client, target string) {
	d := h.layout(r, s)
	d.Refresh = target
	h.kit.Render(w, r, http.StatusOK, views.Continue(d, views.ContinueData{Site: s.views(), Client: client, Target: target}))
}

func seenVersion(r *http.Request) int32 {
	v, err := strconv.ParseInt(r.PostFormValue("terms_version"), 10, 32)
	if err != nil {
		return 0
	}
	return int32(v)
}

func codeOf(err error) apperror.Code {
	if app, ok := apperror.AsAppError(err); ok {
		return app.Code()
	}
	return ""
}

func userAgent(r *http.Request) string {
	ua := r.UserAgent()
	if len(ua) > maxUserAgentLen {
		return strings.ToValidUTF8(ua[:maxUserAgentLen], "")
	}
	return ua
}
