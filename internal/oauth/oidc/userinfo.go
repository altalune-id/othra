package oidc

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"

	"altalune.id/othra/internal/enduser"
	"altalune.id/othra/internal/grant"
	"altalune.id/othra/internal/kernel/ratelimit"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/web/layout"
)

// UserinfoIPRule limits userinfo requests per client IP in a project: 300 a minute, 100 at once.
func UserinfoIPRule() ratelimit.Rule {
	return ratelimit.Rule{Name: "oauth-userinfo-ip", Rate: 300, Per: time.Minute, Burst: 100}
}

type userinfoResponse struct {
	Subject       string `json:"sub"`
	Email         string `json:"email,omitempty"`
	EmailVerified *bool  `json:"email_verified,omitempty"`
	Name          string `json:"name,omitempty"`
}

// SECURITY: RFC 6750 §2, the bearer comes from the header or a POST form body, never both; an ended grant is invalid_token.
func (h *Web) userinfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		writeJSON(w, http.StatusMethodNotAllowed, nil)
		return
	}
	e, ok := h.entry(w, r)
	if !ok {
		return
	}
	w.Header().Set("Pragma", "no-cache")
	w.Header().Add("Vary", "Authorization")
	realm := `Bearer realm="` + project.IssuerURL(h.cfg.PublicBase, e.issuer.Handle) + `"`
	if h.tokens == nil {
		bearerFail(w, realm, http.StatusUnauthorized, "invalid_token")
		return
	}
	if !h.allow(w, r, e, UserinfoIPRule(), ratelimit.IPSubject(layout.ClientIP(r, h.tokens.Limits.TrustedProxyHops))) {
		return
	}
	token, status, code := bearer(w, r)
	if status != 0 {
		bearerFail(w, realm, status, code)
		return
	}
	p := grant.Project{OrgID: e.issuer.OrgID, ID: e.issuer.ProjectID}
	a, err := h.tokens.Grants.Access(r.Context(), p, token)
	switch {
	case grant.IsInvalidTokenError(err):
		bearerFail(w, realm, http.StatusUnauthorized, "invalid_token")
		return
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, unavailableBody)
		return
	case !slices.Contains(a.Scopes, grant.ScopeOpenID):
		bearerFail(w, realm, http.StatusForbidden, "insufficient_scope")
		return
	}
	u, err := h.tokens.EndUsers.EndUser(r.Context(), enduser.Project{OrgID: p.OrgID, ID: p.ID}, a.Family.EndUserID)
	switch {
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, unavailableBody)
		return
	case u == nil:
		bearerFail(w, realm, http.StatusUnauthorized, "invalid_token")
		return
	}
	out := userinfoResponse{Subject: u.Subject}
	if slices.Contains(a.Scopes, scopeEmail) {
		out.Email, out.EmailVerified = u.Email, &u.EmailVerified
	}
	if slices.Contains(a.Scopes, scopeProfile) {
		out.Name = u.Name
	}
	body, err := json.Marshal(out)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, unavailableBody)
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func bearer(w http.ResponseWriter, r *http.Request) (token string, status int, code string) {
	header := r.Header.Get("Authorization")
	if scheme, value, found := strings.Cut(header, " "); found && strings.EqualFold(scheme, "Bearer") {
		header = strings.TrimSpace(value)
	} else if header != "" {
		return "", http.StatusUnauthorized, ""
	}
	if r.Method == http.MethodPost && r.Header.Get("Content-Type") != "" {
		form, err := postForm(w, r)
		if err != nil {
			return "", http.StatusBadRequest, "invalid_request"
		}
		if form.Has("access_token") {
			if header != "" {
				return "", http.StatusBadRequest, "invalid_request"
			}
			header = form.Get("access_token")
		}
	}
	if header == "" {
		return "", http.StatusUnauthorized, ""
	}
	return header, 0, ""
}

// SECURITY: RFC 6750 §3, a request without a token gets the challenge with no error code.
func bearerFail(w http.ResponseWriter, realm string, status int, code string) {
	challenge := realm
	if code != "" {
		challenge += `, error="` + code + `"`
	}
	w.Header().Set("WWW-Authenticate", challenge)
	body, _ := json.Marshal(oauthError{Error: code})
	if code == "" {
		body = nil
	}
	writeJSON(w, status, body)
}
