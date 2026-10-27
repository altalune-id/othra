package hosted

import (
	"net/http"
	"net/url"
	"slices"

	"github.com/altalune-id/othra/apperror"
	"github.com/altalune-id/othra/internal/oauth/hosted/views"
	"github.com/altalune-id/othra/internal/web/i18n"
	"github.com/altalune-id/othra/internal/web/layout"
	"github.com/altalune-id/othra/reqid"
)

const endSessionPath = "/oauth2/end-session"

// CrossSitePOSTs returns the patterns a relying party may POST to from its own origin: authorize and end-session (RPL §2).
func CrossSitePOSTs(basePath string) []string {
	p := layout.Path(basePath, "/p/{handle}")
	return []string{"POST " + p + authorizePath, "POST " + p + endSessionPath}
}

type endSession struct {
	hint       string
	sid        string
	subject    string
	client     Client
	postLogout string
	state      string
}

// NOTE: RP-Initiated Logout 1.0 §2–§4: a hint we signed, an exact post-logout URI, a confirm for another browser's session.
func (h *Web) endSession(w http.ResponseWriter, r *http.Request, s *site) {
	if !h.allowIP(w, r, s, AuthorizeIPRule()) {
		return
	}
	q := r.URL.Query()
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			h.refused(w, r, s, apperror.CodeSignOutHintInvalid)
			return
		}
		q = r.PostForm
	}
	es, code, err := h.parseEndSession(r, s, q)
	switch {
	case err != nil:
		h.fail(w, r, s, "hosted: end session", err)
		return
	case code != "":
		h.refused(w, r, s, code)
		return
	}
	confirmed := r.Method == http.MethodPost && q.Get("confirm") == "1"
	if confirmed && h.csrf.Check(r, s.issuer.PublicID, "endsession") != nil {
		h.formExpired(w, r, s)
		return
	}
	if es.hint == "" {
		h.endWithoutHint(w, r, s, es, confirmed)
		return
	}
	userSession, live, err := h.oauth.Grants.SessionOf(r.Context(), s.project(), es.sid)
	if err != nil {
		h.fail(w, r, s, "hosted: end session", err)
		return
	}
	here, onBrowser := h.onThisBrowser(r, s, userSession, es.subject)
	switch {
	case onBrowser:
		userSession = here
	case live && !confirmed:
		h.kit.Render(w, r, http.StatusOK, views.EndSession(h.layout(r, s), views.EndSessionData{
			Site: s.views(), Client: es.client.Name, Token: h.token(w, r, s, "endsession"), Hint: es.hint,
			PostLogout: es.postLogout, State: es.state,
		}))
		return
	case !live:
		h.afterLogout(w, r, s, es)
		return
	}
	if err := h.oauth.Grants.EndSession(r.Context(), s.project(), es.sid, userSession); err != nil {
		h.fail(w, r, s, "hosted: end session", err)
		return
	}
	h.afterLogout(w, r, s, es)
}

func (h *Web) parseEndSession(r *http.Request, s *site, q url.Values) (*endSession, apperror.Code, error) {
	raw := q.Get("id_token_hint")
	if raw == "" {
		return h.parseWithoutHint(r, s, q)
	}
	if len(raw) > maxHintLen {
		return nil, apperror.CodeSignOutHintInvalid, nil
	}
	hint, ok, err := h.oauth.Grants.VerifyHint(r.Context(), s.project(), raw)
	if err != nil || !ok || len(hint.Audience) == 0 {
		return nil, apperror.CodeSignOutHintInvalid, err
	}
	if id := q.Get("client_id"); id != "" && !slices.Contains(hint.Audience, id) {
		return nil, apperror.CodeSignOutHintInvalid, nil
	}
	c, ok, err := h.oauth.Clients.Client(r.Context(), s.project(), hint.Audience[0])
	if err != nil || !ok {
		return nil, apperror.CodeSignOutHintInvalid, err
	}
	es := &endSession{
		hint: raw, sid: hint.SID, subject: hint.Subject, client: c, postLogout: q.Get("post_logout_redirect_uri"), state: q.Get("state"),
	}
	if es.postLogout != "" && !slices.Contains(c.PostLogoutRedirectURIs, es.postLogout) {
		return nil, apperror.CodeSignOutRedirectNotRegistered, nil
	}
	return es, "", nil
}

// NOTE: RPL §2: with no hint the client_id names the app, and its post-logout URI must be registered exactly as with one.
func (h *Web) parseWithoutHint(r *http.Request, s *site, q url.Values) (*endSession, apperror.Code, error) {
	id := q.Get("client_id")
	if id == "" {
		return nil, apperror.CodeSignOutHintInvalid, nil
	}
	c, ok, err := h.oauth.Clients.Client(r.Context(), s.project(), id)
	if err != nil || !ok {
		return nil, apperror.CodeSignOutHintInvalid, err
	}
	es := &endSession{client: c, postLogout: q.Get("post_logout_redirect_uri"), state: q.Get("state")}
	if es.postLogout != "" && !slices.Contains(c.PostLogoutRedirectURIs, es.postLogout) {
		return nil, apperror.CodeSignOutRedirectNotRegistered, nil
	}
	return es, "", nil
}

// SECURITY: RPL §2: a request with no hint names no session, so the person confirms first; then this browser's account signs out.
func (h *Web) endWithoutHint(w http.ResponseWriter, r *http.Request, s *site, es *endSession, confirmed bool) {
	if !confirmed {
		h.kit.Render(w, r, http.StatusOK, views.EndSession(h.layout(r, s), views.EndSessionData{
			Site: s.views(), Client: es.client.Name, ClientID: es.client.ID, Token: h.token(w, r, s, "endsession"),
			PostLogout: es.postLogout, State: es.state,
		}))
		return
	}
	b, err := h.users.Browser(r.Context(), s.project(), h.cookies.browser(r))
	if err != nil {
		h.fail(w, r, s, "hosted: end session", err)
		return
	}
	if a, ok := b.Active(); ok {
		if err := h.oauth.Grants.EndSession(r.Context(), s.project(), "", a.UserSession); err != nil {
			h.fail(w, r, s, "hosted: end session", err)
			return
		}
	}
	h.afterLogout(w, r, s, es)
}

// NOTE: the hinted user session when this browser holds it, else this browser's account for the hint's sub.
func (h *Web) onThisBrowser(r *http.Request, s *site, userSession, subject string) (string, bool) {
	b, err := h.users.Browser(r.Context(), s.project(), h.cookies.browser(r))
	if err != nil {
		return "", false
	}
	if _, ok := b.Find(userSession); ok && userSession != "" {
		return userSession, true
	}
	for _, a := range b.Accounts {
		if a.EndUser == subject {
			return a.UserSession, true
		}
	}
	return "", false
}

// SECURITY: RPL §3: only an exactly registered post-logout URI, with state; after a POST the continue page makes the last hop a GET.
func (h *Web) afterLogout(w http.ResponseWriter, r *http.Request, s *site, es *endSession) {
	if es.postLogout == "" {
		h.toPage(w, r, s, pageSignedOut, nil)
		return
	}
	u, err := url.Parse(es.postLogout)
	if err != nil {
		h.fail(w, r, s, "hosted: post logout uri", err)
		return
	}
	if es.state != "" {
		q := u.Query()
		q.Set("state", es.state)
		u.RawQuery = q.Encode()
	}
	if r.Method == http.MethodPost {
		h.forward(w, r, s, es.client.Name, u.String())
		return
	}
	http.Redirect(w, r, u.String(), http.StatusFound) //nolint:gosec // G710: a post-logout URI registered for this client.
}

// NOTE: RPL §4: a request refused before its redirect is trusted gets a page with no way back into sign-in.
func (h *Web) refused(w http.ResponseWriter, r *http.Request, s *site, code apperror.Code) {
	ctx := r.Context()
	title := "error.authorize_title"
	if code == apperror.CodeSignOutHintInvalid || code == apperror.CodeSignOutRedirectNotRegistered {
		title = "error.sign_out_title"
	}
	h.kit.Render(w, r, http.StatusBadRequest, views.Error(h.layout(r, s), views.ErrorData{
		Site: s.views(), Title: i18n.T(ctx, title), Message: i18n.Error(ctx, code), Code: string(code),
		Reference: reqid.FromContext(ctx), SupportEmail: s.issuer.SupportEmail, NoSignIn: true,
	}))
}
