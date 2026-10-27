package hosted

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/federation"
	"altalune.id/othra/internal/kernel/ratelimit"
	"altalune.id/othra/internal/oauth/hosted/views"
	"altalune.id/othra/internal/upstream"
	"altalune.id/othra/oidcrp"
	"altalune.id/othra/reqid"
	"altalune.id/othra/sealer"
)

const (
	federationCookie = "__Secure-othra_fed"
	federationTTL    = 10 * time.Minute
	federationForm   = "federation"
	modeLink         = "link"
)

// NOTE: N1: one round trip with an upstream, sealed into a cookie between the start and the callback.
type federationState struct {
	Kind        upstream.Kind      `json:"k"`
	Request     oidcrp.AuthRequest `json:"q"`
	ReturnTo    string             `json:"r"`
	Mode        string             `json:"m,omitempty"`
	UserSession string             `json:"u,omitempty"`
	Issued      int64              `json:"t"`
}

// WithFederation serves sign-in with the project's upstream IdPs over f; sl seals the round trip's state cookie.
func (h *Web) WithFederation(f Federation, sl sealer.Sealer) *Web {
	h.fed, h.fedSeal = f, sl
	return h
}

func (h *Web) registerFederation(mux *http.ServeMux, p string) {
	if h.fed == nil {
		return
	}
	g := p + "/federation/" + string(upstream.KindGoogle)
	mux.HandleFunc("POST "+g+"/start", h.page(h.federationStart))
	mux.HandleFunc("GET "+g+"/start", methodNotAllowed)
	mux.HandleFunc("GET "+g+"/continue", h.page(h.federationContinue))
	mux.HandleFunc("GET "+g+"/callback", h.page(h.federationCallback))
}

// NOTE: P10: the start is a POST with a CSRF token, so another site can't send a browser through Google.
func methodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Allow", http.MethodPost)
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}

func fedProject(s *site) federation.Project {
	return federation.Project{OrgID: s.issuer.OrgID, ID: s.issuer.ProjectID, PublicID: s.issuer.PublicID}
}

func (s *site) federationPath() string { return s.path + "/federation/" }

func (h *Web) googleOn(r *http.Request, s *site) bool {
	if h.fed == nil {
		return false
	}
	on, err := h.fed.Enabled(r.Context(), fedProject(s), upstream.KindGoogle)
	return err == nil && on
}

func (h *Web) federationStart(w http.ResponseWriter, r *http.Request, s *site) {
	if !h.guard(w, r, s, federationForm, FormIPRule()) {
		return
	}
	st := federationState{Kind: upstream.KindGoogle, ReturnTo: h.returnTo(s, r.PostFormValue("return_to"))}
	hint := ""
	if r.PostFormValue("intent") == modeLink {
		a, ok := h.active(w, r, s, s.path+string(pageAccount))
		if !ok {
			return
		}
		session := ratelimit.ProjectSubject(s.issuer.ProjectID, ratelimit.UserSessionSubject(a.UserSession))
		if err := h.allow(r, session, LinkSessionRule()); err != nil {
			h.limited(w, r, s, err)
			return
		}
		st.Mode, st.UserSession, st.ReturnTo = modeLink, a.UserSession, s.path+string(pageAccount)
	} else if h.pending(r, s, st.ReturnTo).reauth {
		var ok bool
		if hint, ok = h.linkedEmail(w, r, s, st.Kind); !ok {
			return
		}
	}
	req, err := h.fed.Start(r.Context(), fedProject(s), st.Kind, hint)
	if err != nil {
		h.federationFailed(w, r, s, st, err)
		return
	}
	st.Request, st.Issued = req, h.cookies.now().Unix()
	if err := h.setFederation(w, s, st); err != nil {
		h.fail(w, r, s, "hosted: seal federation state", err)
		return
	}
	next := s.federationPath() + string(st.Kind) + "/continue"
	http.Redirect(w, r, next, http.StatusSeeOther) //nolint:gosec // G710: a fixed page under this issuer's path.
}

// SECURITY: G4: the hop to the upstream leaves from a GET page, so the hosted CSP's form-action stays 'self'.
func (h *Web) federationContinue(w http.ResponseWriter, r *http.Request, s *site) {
	st, ok := h.federation(r, s)
	if !ok {
		h.upstreamFailed(w, r, s, h.returnTo(s, ""), &federation.UpstreamError{Reason: federation.ReasonCookie})
		return
	}
	target, err := h.fed.AuthURL(r.Context(), fedProject(s), st.Kind, upstream.RedirectURI(s.issuerURL, st.Kind), st.Request)
	if err != nil {
		h.clearFederation(w, s)
		h.federationFailed(w, r, s, st, err)
		return
	}
	d := h.layout(r, s)
	d.Refresh = target
	h.kit.Render(w, r, http.StatusOK, views.Continue(d, views.ContinueData{Site: s.views(), Client: "Google", Target: target}))
}

// SECURITY: N1, RFC 9700 §4.7: the state cookie is spent before the code is, so a callback replays nothing.
func (h *Web) federationCallback(w http.ResponseWriter, r *http.Request, s *site) {
	if !h.allowIP(w, r, s, RedeemIPRule()) {
		return
	}
	st, ok := h.federation(r, s)
	h.clearFederation(w, s)
	if !ok {
		h.upstreamFailed(w, r, s, h.returnTo(s, ""), &federation.UpstreamError{Reason: federation.ReasonCookie})
		return
	}
	p := fedProject(s)
	redirectURI := upstream.RedirectURI(s.issuerURL, st.Kind)
	id, err := h.fed.Complete(r.Context(), p, st.Kind, redirectURI, oidcrp.ParseCallback(r.URL.Query()), st.Request)
	if err != nil {
		h.federationFailed(w, r, s, st, err)
		return
	}
	if st.Mode == modeLink {
		h.federationLink(w, r, s, st, id)
		return
	}
	got, err := h.fed.SignIn(r.Context(), p, id, federation.SessionRequest{BrowserSecret: h.cookies.browser(r), UserAgent: userAgent(r)})
	if cerr, ok := errors.AsType[*federation.CollisionError](err); ok {
		h.collision(w, r, s, cerr, st.ReturnTo)
		return
	}
	if uerr, ok := errors.AsType[*federation.UnprovenEmailError](err); ok {
		h.unproven(w, r, s, uerr, st.ReturnTo)
		return
	}
	if err != nil {
		h.upstreamFailed(w, r, s, st.ReturnTo, err)
		return
	}
	h.cookies.setBrowser(w, s.path+"/", got.BrowserSecret, got.ExpiresAt)
	h.csrf.Renew(w, s.path+"/")
	h.next(w, r, s, got.BrowserSecret, federatedAccount(got), st.ReturnTo)
}

// SECURITY: P7: the only login_hint sent upstream is the active account's linked email there; never Othra's email or the app's hint.
func (h *Web) linkedEmail(w http.ResponseWriter, r *http.Request, s *site, kind upstream.Kind) (string, bool) {
	b, err := h.users.Browser(r.Context(), s.project(), h.cookies.browser(r))
	if err != nil {
		h.fail(w, r, s, "hosted: browser", err)
		return "", false
	}
	a, ok := b.Active()
	if !ok {
		return "", true
	}
	ids, err := h.fed.Identities(r.Context(), fedProject(s), a.EndUserID)
	if err != nil {
		h.fail(w, r, s, "hosted: identities", err)
		return "", false
	}
	for _, id := range ids {
		if id.Kind == kind {
			return id.Email, true
		}
	}
	return "", true
}

func federatedAccount(got *federation.Session) Account {
	return Account{
		UserSession: got.UserSession, EndUserID: got.EndUserID, EndUser: got.Subject, Email: got.Email, Name: got.Name,
		AuthTime: got.AuthTime, SelectedAt: got.SelectedAt,
	}
}

// SECURITY: the identity goes to the account that started the link, and only while it is still signed in on this browser.
func (h *Web) federationLink(w http.ResponseWriter, r *http.Request, s *site, st federationState, id federation.Identity) {
	b, err := h.users.Browser(r.Context(), s.project(), h.cookies.browser(r))
	if err != nil {
		h.fail(w, r, s, "hosted: browser", err)
		return
	}
	a, ok := b.Find(st.UserSession)
	if !ok {
		h.toPage(w, r, s, pageLogin, backTo(st.ReturnTo))
		return
	}
	if err := h.fed.Link(r.Context(), fedProject(s), a.EndUserID, id); err != nil {
		h.federationFailed(w, r, s, st, err)
		return
	}
	h.toPage(w, r, s, pageSignIn, url.Values{"linked": {"1"}})
}

// NOTE: MF7: a failed Connect speaks to a signed-in person: "Couldn't connect Google", back to the account, never to sign in.
func (h *Web) federationFailed(w http.ResponseWriter, r *http.Request, s *site, st federationState, err error) {
	if st.Mode != modeLink {
		h.upstreamFailed(w, r, s, st.ReturnTo, err)
		return
	}
	h.renderUpstream(w, r, s, err, func(code apperror.Code) views.ErrorData {
		return views.LinkUpstreamError(r.Context(), s.views(), code, reqid.FromContext(r.Context()))
	})
}

func (h *Web) upstreamFailed(w http.ResponseWriter, r *http.Request, s *site, returnTo string, err error) {
	h.renderUpstream(w, r, s, err, func(code apperror.Code) views.ErrorData {
		return views.UpstreamError(r.Context(), s.views(), code, reqid.FromContext(r.Context()), returnTo)
	})
}

func (h *Web) renderUpstream(w http.ResponseWriter, r *http.Request, s *site, err error, page func(apperror.Code) views.ErrorData) {
	app, ok := apperror.AsAppError(err)
	if !ok {
		h.fail(w, r, s, "hosted: federation", err)
		return
	}
	status, kind := http.StatusBadRequest, apperror.KindOf(app.Code())
	switch {
	case app.Code() == apperror.CodeUnexpected:
		status = http.StatusInternalServerError
	case kind == apperror.KindInternal:
		status = http.StatusBadGateway
	case kind == apperror.KindFailedPrecondition, kind == apperror.KindForbidden:
		status = http.StatusForbidden
	}
	d := page(app.Code())
	d.SupportEmail = s.issuer.SupportEmail
	h.kit.Render(w, r, status, views.Error(h.layout(r, s), d))
}

func (h *Web) setFederation(w http.ResponseWriter, s *site, st federationState) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	sealed, err := h.fedSeal.Seal(raw, federationAAD(s))
	if err != nil {
		return err
	}
	http.SetCookie(w, cookie(federationCookie, s.federationPath(), base64.RawURLEncoding.EncodeToString(sealed), int(federationTTL.Seconds())))
	return nil
}

func (h *Web) federation(r *http.Request, s *site) (federationState, bool) {
	c, err := r.Cookie(federationCookie)
	if err != nil {
		return federationState{}, false
	}
	sealed, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		return federationState{}, false
	}
	raw, err := h.fedSeal.Open(sealed, federationAAD(s))
	if err != nil {
		return federationState{}, false
	}
	var st federationState
	if json.Unmarshal(raw, &st) != nil || h.cookies.now().Sub(time.Unix(st.Issued, 0)) > federationTTL || st.Kind != upstream.KindGoogle {
		return federationState{}, false
	}
	return st, true
}

func (h *Web) clearFederation(w http.ResponseWriter, s *site) {
	http.SetCookie(w, cookie(federationCookie, s.federationPath(), "", -1))
}

// SECURITY: the state is sealed to this project, so a cookie from another issuer never opens here.
func federationAAD(s *site) []byte { return []byte("othra-federation:" + s.issuer.PublicID) }
