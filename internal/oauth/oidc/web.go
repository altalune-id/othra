package oidc

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"altalune.id/othra/internal/oauth/oidc/views"
	"altalune.id/othra/internal/project"
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
	// CORS wraps the public documents and the token endpoint; nil leaves them as they are.
	CORS func(http.Handler) http.Handler
}

// Web serves each issuer's discovery documents, JWKS and endpoint stubs.
type Web struct {
	resolver *Resolver
	kit      layout.Kit
	cfg      Config
}

// NewWeb binds the issuer routes to the resolver its documents and pages share.
func NewWeb(resolver *Resolver, kit layout.Kit, cfg Config) *Web {
	return &Web{resolver: resolver, kit: kit, cfg: cfg}
}

// Register mounts the public documents and the token endpoint on mux.
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
	mux.Handle(issuer+tokenPath, public(http.HandlerFunc(h.token)))
}

// Pages is the issuer's browser pages, mounted with the other pre-sign-in pages.
type Pages struct{ h *Web }

// Pages returns the issuer's browser pages.
func (h *Web) Pages() Pages { return Pages{h: h} }

// Register mounts the authorize stub on mux.
func (p Pages) Register(mux *http.ServeMux) {
	mux.HandleFunc(layout.Path(p.h.cfg.BasePath, "/p/{handle}")+authorizePath, p.h.authorize)
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

func (h *Web) token(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST, OPTIONS")
		writeJSON(w, http.StatusMethodNotAllowed, nil)
		return
	}
	e, ok := h.entry(w, r)
	if !ok {
		return
	}
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("WWW-Authenticate", `Basic realm="`+project.IssuerURL(h.cfg.PublicBase, e.issuer.Handle)+`"`)
	writeJSON(w, http.StatusUnauthorized, invalidClient)
}

func (h *Web) authorize(w http.ResponseWriter, r *http.Request) {
	e, ok := h.entry(w, r)
	if !ok {
		return
	}
	name := cmp.Or(e.issuer.DisplayName, e.issuer.Name)
	w.Header().Set("Cache-Control", "no-store")
	h.kit.Render(w, r, http.StatusBadRequest, views.Unregistered(h.kit.Layout("This app isn't registered"), name))
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
