package hosted

import (
	"context"
	"net/http"
	"net/url"
	"slices"

	"altalune.id/othra/apperror"
	"altalune.id/othra/internal/consent"
	"altalune.id/othra/internal/oauth/hosted/views"
	"altalune.id/othra/internal/oauth/redirecturi"
	"altalune.id/othra/internal/web/i18n"
	"altalune.id/othra/internal/web/layout"
)

const pageConsent page = "/consent"

const descDenied = "the end user denied the request"

// NOTE: a stored consent keeps only the OIDC scopes and the scopes the server still has.
func (r *authRequest) allowedScopes() []string {
	if r.resource == nil {
		return slices.Clone(knownScopes)
	}
	return slices.Concat(knownScopes, r.resource.Catalog)
}

func (r *authRequest) consentKey(a Account) consent.Key {
	return consent.Key{EndUserID: a.EndUserID, ClientID: r.client.RowID, ResourceServerID: r.resourceID()}
}

// NOTE: the marker in q says the request came back from a consent page; without one only the stored consent decides.
func (h *Web) consentPage(ctx context.Context, s *site, req *authRequest, q url.Values, a Account) (consent.Page, *consent.Consent, error) {
	c, err := h.oauth.Consents.Consent(ctx, s.project(), req.consentKey(a))
	if err != nil {
		return consent.PageNone, nil, err
	}
	since, marked := h.oauth.Marker.Since(q, req.client.ID)
	return consent.Decide(c, consent.Request{
		Implicit: !req.client.ConsentRequired, Confidential: req.client.Confidential, PromptConsent: req.prompt[promptConsent],
		Scopes: req.scopes, Marked: marked, Since: since,
	}), c, nil
}

// SECURITY: the consent page returns to /authorize with the request's marker, or a new one, so approving can't loop.
func (h *Web) askConsent(w http.ResponseWriter, r *http.Request, s *site, req *authRequest, q url.Values) {
	if _, ok := h.oauth.Marker.Since(q, req.client.ID); !ok {
		q = h.oauth.Marker.Mark(q, req.client.ID)
	}
	target, ok := h.reentry(s, req, q)
	if !ok {
		h.toClient(w, r, s, req, url.Values{"error": {"invalid_request"}, "error_description": {descTooLong}})
		return
	}
	h.toPage(w, r, s, pageConsent, backTo(target))
}

func (h *Web) authorizeAt(w http.ResponseWriter, r *http.Request, s *site, returnTo string) (*authRequest, bool) {
	u, err := url.Parse(returnTo)
	if err != nil || u.Path != s.path+authorizePath {
		h.toReturnTo(w, r, s, returnTo)
		return nil, false
	}
	req, err := parseAuthorize(r.Context(), h.oauth, s.project(), u.Query())
	if h.authorizeFailed(w, r, s, err) {
		return nil, false
	}
	return req, true
}

func (h *Web) consent(w http.ResponseWriter, r *http.Request, s *site) {
	returnTo := h.returnTo(s, r.URL.Query().Get("return_to"))
	req, ok := h.authorizeAt(w, r, s, returnTo)
	if !ok {
		return
	}
	a, ok := h.active(w, r, s, returnTo)
	if !ok {
		return
	}
	page, c, err := h.consentPage(r.Context(), s, req, nil, a)
	switch {
	case err != nil:
		h.fail(w, r, s, "hosted: consent", err)
	case page == consent.PageNone:
		h.toReturnTo(w, r, s, returnTo)
	default:
		h.renderConsent(w, r, s, req, a, c, page == consent.PageRemembered, returnTo)
	}
}

// NOTE: MCP security: the redirect host is always named, with a warning when it is this device.
func (h *Web) renderConsent(
	w http.ResponseWriter, r *http.Request, s *site, req *authRequest, a Account, c *consent.Consent, remembered bool, returnTo string,
) {
	scopes := make([]views.ConsentScope, 0, len(req.scopes))
	for _, sc := range req.scopes {
		cs := views.ConsentScope{Scope: sc, AllowedBefore: c.Covers([]string{sc})}
		if req.resource != nil {
			cs.Description = req.resource.Descriptions[sc]
		}
		scopes = append(scopes, cs)
	}
	var redirectHost string
	if u, err := url.Parse(req.redirectURI); err == nil {
		redirectHost = u.Hostname()
	}
	d := views.ConsentData{
		Site: s.views(), Client: req.client.Name, ClientHost: req.client.MetadataHost, Email: a.Email, UserSession: a.UserSession,
		ReturnTo: returnTo, ConsentToken: h.token(w, r, s, "consent"), Scopes: scopes, RedirectHost: redirectHost,
		Loopback: redirecturi.IsLoopbackHost(redirectHost), Remembered: remembered,
	}
	if req.resource != nil {
		d.RS, d.RSIdentifier = req.resource.Name, req.resource.Identifier
	}
	h.kit.Render(w, r, http.StatusOK, views.Consent(h.layout(r, s), d))
}

// SECURITY: a CSRF-checked POST from this browser's active account; Deny records nothing and answers access_denied.
func (h *Web) answerConsent(w http.ResponseWriter, r *http.Request, s *site) {
	if !h.guard(w, r, s, "consent", FormIPRule()) {
		return
	}
	returnTo := h.returnTo(s, r.PostFormValue("return_to"))
	if r.PostFormValue("deny") == "1" {
		if !h.declineToClient(w, r, s, returnTo, descDenied) {
			h.refused(w, r, s, apperror.CodeAuthorizeRequestInvalid)
		}
		return
	}
	req, ok := h.authorizeAt(w, r, s, returnTo)
	if !ok {
		return
	}
	b, err := h.users.Browser(r.Context(), s.project(), h.cookies.browser(r))
	if err != nil {
		h.fail(w, r, s, "hosted: browser", err)
		return
	}
	a, ok := b.Active()
	if !ok || a.UserSession != r.PostFormValue("user_session") {
		h.renderPicker(w, r, s, b, returnTo, "", i18n.Error(r.Context(), apperror.CodeSignInSessionGone))
		return
	}
	err = h.oauth.Consents.Grant(r.Context(), s.project(), consent.GrantRequest{
		Key: req.consentKey(a), Scopes: req.scopes, Allowed: req.allowedScopes(),
		IP: layout.ClientIP(r, h.limits.TrustedProxyHops), UserAgent: userAgent(r),
	})
	if err != nil {
		h.fail(w, r, s, "hosted: grant consent", err)
		return
	}
	h.toReturnTo(w, r, s, returnTo)
}
