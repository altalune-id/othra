package oidc

import (
	"net/http"
	"time"

	"github.com/altalune-id/othra/internal/grant"
	"github.com/altalune-id/othra/internal/kernel/ratelimit"
	"github.com/altalune-id/othra/internal/project"
	"github.com/altalune-id/othra/internal/web/layout"
)

// RevokeIPRule limits revocation requests per client IP in a project: 60 a minute, 30 at once.
func RevokeIPRule() ratelimit.Rule {
	return ratelimit.Rule{Name: "oauth-revoke-ip", Rate: 60, Per: time.Minute, Burst: 30}
}

// SECURITY: RFC 7009 §2, the /token client authentication; an unknown or foreign token still answers 200 (§2.2).
func (h *Web) revoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
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
	if !h.allow(w, r, e, RevokeIPRule(), ratelimit.IPSubject(layout.ClientIP(r, h.tokens.Limits.TrustedProxyHops))) {
		return
	}
	form, err := postForm(w, r)
	if err != nil {
		h.oauthFail(w, http.StatusBadRequest, errInvalidRequest, err.Error())
		return
	}
	c, ok := h.authenticate(&tokenRequest{w: w, r: r, e: e, form: form, issuer: issuer})
	if !ok {
		return
	}
	token := form.Get("token")
	if token == "" {
		h.oauthFail(w, http.StatusBadRequest, errInvalidRequest, "token is required")
		return
	}
	p := grant.Project{OrgID: e.issuer.OrgID, ID: e.issuer.ProjectID, IssuerURL: issuer}
	if err := h.tokens.Grants.Revoke(r.Context(), p, grantClient(c), token, form.Get("token_type_hint")); err != nil {
		h.unavailable(w)
		return
	}
	writeJSON(w, http.StatusOK, nil)
}
