package hosted

import (
	"net/http"

	"github.com/a-h/templ"

	"altalune.id/othra/internal/web/middleware"
	"altalune.id/othra/internal/web/portal"
)

// WithPortal serves the end-user portal app at /account/ from p; without it /account/ is not found.
func (h *Web) WithPortal(p *portal.Page) *Web {
	h.portalPage = p
	return h
}

// NOTE: the portal app owns every route under /account/; this gates it on a live session and the terms, then fills in this project.
func (h *Web) portal(w http.ResponseWriter, r *http.Request, s *site) {
	if h.portalPage == nil {
		h.notFound(w, r, s)
		return
	}
	here := s.path + "/account/" + r.PathValue("rest")
	a, ok := h.active(w, r, s, here)
	if !ok {
		return
	}
	state, err := h.users.TermsState(r.Context(), s.project(), h.cookies.browser(r), a.UserSession)
	if err != nil {
		h.fail(w, r, s, "hosted: terms state", err)
		return
	}
	if state.Needed {
		h.toPage(w, r, s, pageTerms, backTo(here))
		return
	}
	nonce := templ.GetNonce(r.Context())
	middleware.AllowNonceScripts(w, nonce)
	brand := h.layout(r, s).Brand
	h.portalPage.Serve(w, r, portal.Values{
		BaseURL: s.path + "/account/", APIURL: s.path + "/api", HostedURL: s.path,
		Handle: s.issuer.Handle, Project: s.name, LogoURL: brand.LogoURL, TermsURL: brand.TermsURL, PrivacyURL: brand.PrivacyURL,
		BrandCSS: brand.CSS(), Nonce: nonce,
	})
}
