package oidc

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"altalune.id/othra/internal/web/layout"
)

//nolint:gochecknoglobals // fixed response bodies.
var (
	notFoundBody    = []byte(`{"error":"not_found"}`)
	limitedBody     = []byte(`{"error":"too_many_requests"}`)
	unavailableBody = []byte(`{"error":"temporarily_unavailable"}`)
	invalidClient   = []byte(`{"error":"invalid_client"}`)
)

// Config places the issuer routes.
type Config struct {
	// PublicBase is the scheme, host and base path every issuer URL starts with.
	PublicBase string
	BasePath   string
	// CORS wraps the public documents, and the token and userinfo endpoints when Tokens.CORS is nil; nil leaves them as they are.
	CORS func(http.Handler) http.Handler
}

// Web serves each issuer's discovery documents, JWKS, token and userinfo endpoints.
type Web struct {
	resolver *Resolver
	kit      layout.Kit
	cfg      Config
	tokens   *Tokens
}

// NewWeb binds the issuer routes to the resolver its documents and pages share.
func NewWeb(resolver *Resolver, kit layout.Kit, cfg Config) *Web {
	return &Web{resolver: resolver, kit: kit, cfg: cfg}
}

// CrossSitePOSTs returns the cookie-less endpoints a browser app may POST to from its own origin: token and userinfo.
func CrossSitePOSTs(basePath string) []string {
	p := layout.Path(basePath, "/p/{handle}")
	return []string{"POST " + p + tokenPath, "POST " + p + userinfoPath}
}

// WithTokens answers the token and userinfo endpoints with t.
func (h *Web) WithTokens(t Tokens) *Web {
	h.tokens = &t
	return h
}

// Register mounts the public documents and the token and userinfo endpoints on mux.
func (h *Web) Register(mux *http.ServeMux) {
	issuer := layout.Path(h.cfg.BasePath, "/p/{handle}")
	public := h.cfg.CORS
	if public == nil {
		public = func(next http.Handler) http.Handler { return next }
	}
	mux.Handle(issuer+jwksPath, public(h.document(func(e *issuerEntry) (document, string) { return e.jwks, "application/jwk-set+json" })))
	discovery := public(h.document(func(e *issuerEntry) (document, string) { return e.discovery, "application/json" }))
	mux.Handle(issuer+"/.well-known/openid-configuration", discovery)
	mux.Handle("/.well-known/oauth-authorization-server"+strings.TrimRight(h.cfg.BasePath, "/")+"/p/{handle}", discovery)
	api := public
	if h.tokens != nil && h.tokens.CORS != nil {
		api = h.tokens.CORS
	}
	mux.Handle(issuer+tokenPath, api(http.HandlerFunc(h.token)))
	mux.Handle(issuer+userinfoPath, api(http.HandlerFunc(h.userinfo)))
	mux.Handle(issuer+revokePath, http.HandlerFunc(h.revoke))
}

func (h *Web) document(pick func(*issuerEntry) (document, string)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD, OPTIONS")
			writeJSON(w, http.StatusMethodNotAllowed, nil)
			return
		}
		e, ok := h.entry(w, r)
		if !ok {
			return
		}
		doc, contentType := pick(e)
		doc.serve(w, r, contentType)
	})
}

func (h *Web) entry(w http.ResponseWriter, r *http.Request) (*issuerEntry, bool) {
	e, err := h.resolver.entry(r)
	limited, isLimited := errors.AsType[*LookupLimitedError](err)
	switch {
	case err == nil:
		return e, true
	case errors.Is(err, ErrUnknownIssuer):
		writeJSON(w, http.StatusNotFound, notFoundBody)
	case isLimited:
		w.Header().Set("Retry-After", strconv.Itoa(limited.RetryAfter))
		writeJSON(w, http.StatusTooManyRequests, limitedBody)
	default:
		h.report(r.Context(), "oidc: load issuer", err)
		writeJSON(w, http.StatusServiceUnavailable, unavailableBody)
	}
	return nil, false
}

func (h *Web) report(ctx context.Context, msg string, err error) {
	if h.kit.Unexpected != nil {
		_ = h.kit.Unexpected(ctx, msg, err)
	}
}

func writeJSON(w http.ResponseWriter, status int, body []byte) {
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	if body != nil {
		h.Set("Content-Type", "application/json")
	}
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
