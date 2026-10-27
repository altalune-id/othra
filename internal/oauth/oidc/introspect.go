package oidc

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"altalune.id/othra/internal/enduser"
	"altalune.id/othra/internal/grant"
	"altalune.id/othra/internal/kernel/ratelimit"
	"altalune.id/othra/internal/pat"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/web/layout"
)

const introspectPath = "/oauth2/introspect"

//nolint:gochecknoglobals // a fixed response body.
var inactiveBody = []byte(`{"active":false}`)

// IntrospectIPRule limits introspection requests per client IP in a project, failed authentications included: 600 a minute.
func IntrospectIPRule() ratelimit.Rule {
	return ratelimit.Rule{Name: "oauth-introspect-ip", Rate: 600, Per: time.Minute, Burst: 300}
}

// IntrospectServerRule limits introspection requests per authenticated resource server in a project: 600 a minute.
func IntrospectServerRule() ratelimit.Rule {
	return ratelimit.Rule{Name: "oauth-introspect-server", Rate: 600, Per: time.Minute, Burst: 300}
}

// Introspection is what the introspection endpoint answers with; without it the endpoint is not mounted or advertised.
type Introspection struct {
	Callers        IntrospectionCallers
	PersonalTokens PersonalTokens
	ClientIDs      ClientIDs
}

type introspectionResponse struct {
	Active    bool   `json:"active"`
	Scope     string `json:"scope"`
	ClientID  string `json:"client_id,omitempty"`
	Subject   string `json:"sub"`
	Audience  string `json:"aud"`
	Issuer    string `json:"iss"`
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
	TokenType string `json:"token_type"`
}

// SECURITY: RFC 7662 §2, a resource server authenticates with its own secret and learns only about tokens bound to it.
func (h *Web) introspect(w http.ResponseWriter, r *http.Request) {
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
	if !h.allow(w, r, e, IntrospectIPRule(), ratelimit.IPSubject(layout.ClientIP(r, h.tokens.Limits.TrustedProxyHops))) {
		return
	}
	form, err := postForm(w, r)
	if err != nil {
		h.oauthFail(w, http.StatusBadRequest, errInvalidRequest, err.Error())
		return
	}
	req := &tokenRequest{w: w, r: r, e: e, form: form, issuer: issuer}
	creds, err := readCredentials(req)
	switch {
	case errors.Is(err, errMixedAuth):
		h.oauthFail(w, http.StatusBadRequest, errInvalidRequest, err.Error())
		return
	case err != nil || creds.method == authNone || creds.id == "":
		h.clientFail(req)
		return
	}
	caller, ok, err := h.intro.Callers.Authenticate(r.Context(), e.issuer.OrgID, e.issuer.ProjectID, creds.id, creds.secret)
	switch {
	case err != nil:
		h.unavailable(w)
		return
	case !ok:
		h.clientFail(req)
		return
	}
	if !h.allow(w, r, e, IntrospectServerRule(), "server:"+caller.PublicID) {
		return
	}
	token := form.Get("token")
	if token == "" {
		h.oauthFail(w, http.StatusBadRequest, errInvalidRequest, "token is required")
		return
	}
	var active IntrospectedToken
	if strings.HasPrefix(token, pat.Prefix) {
		active, ok, err = h.intro.PersonalTokens.Introspect(r.Context(), e.issuer.OrgID, e.issuer.ProjectID, caller.ID, token)
	} else {
		active, ok, err = h.introspectAccess(req, caller, token)
	}
	switch {
	case err != nil:
		h.unavailable(w)
	case !ok:
		writeJSON(w, http.StatusOK, inactiveBody)
	default:
		body, err := json.Marshal(introspectionResponse{
			Active: true, Scope: strings.Join(active.Scopes, " "), ClientID: active.ClientID, Subject: active.Subject,
			Audience: active.Audience, Issuer: issuer, ExpiresAt: active.ExpiresAt.Unix(), IssuedAt: active.IssuedAt.Unix(),
			TokenType: tokenTypeBearer,
		})
		if err != nil {
			h.unavailable(w)
			return
		}
		writeJSON(w, http.StatusOK, body)
	}
}

func (h *Web) introspectAccess(req *tokenRequest, caller Introspector, token string) (IntrospectedToken, bool, error) {
	ctx, i := req.r.Context(), req.e.issuer
	a, err := h.tokens.Grants.Access(ctx, grant.Project{OrgID: i.OrgID, ID: i.ProjectID, IssuerURL: req.issuer}, token)
	switch {
	case grant.IsInvalidTokenError(err):
		return IntrospectedToken{}, false, nil
	case err != nil:
		return IntrospectedToken{}, false, err
	case a.ResourceServerID == nil || *a.ResourceServerID != caller.ID:
		return IntrospectedToken{}, false, nil
	}
	u, err := h.tokens.EndUsers.EndUser(ctx, enduser.Project{OrgID: i.OrgID, ID: i.ProjectID}, a.Family.EndUserID)
	if err != nil || u == nil {
		return IntrospectedToken{}, false, err
	}
	clientID, err := h.intro.ClientIDs.OAuthID(ctx, i.OrgID, i.ProjectID, a.Family.ClientID)
	if err != nil || clientID == "" {
		return IntrospectedToken{}, false, err
	}
	return IntrospectedToken{
		Scopes: grant.ResourceScopes(a.Scopes), ClientID: clientID, Subject: u.Subject, Audience: caller.Identifier, IssuedAt: a.IssuedAt,
		ExpiresAt: a.ExpiresAt,
	}, true, nil
}

func (h *Web) unavailable(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "5")
	writeJSON(w, http.StatusServiceUnavailable, unavailableBody)
}
