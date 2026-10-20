package oidc

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"altalune.id/othra/internal/kernel/ratelimit"
	"altalune.id/othra/internal/kernel/tenant"
	"altalune.id/othra/internal/oauth/oidc/views"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/web/layout"
)

const cacheTTL = 30 * time.Second

//nolint:gochecknoglobals // fixed response bodies.
var (
	notFoundBody    = []byte(`{"error":"not_found"}`)
	limitedBody     = []byte(`{"error":"too_many_requests"}`)
	unavailableBody = []byte(`{"error":"temporarily_unavailable"}`)
	invalidClient   = []byte(`{"error":"invalid_client"}`)
)

var errUnknownIssuer = errors.New("oidc: unknown issuer")

// LookupIPRule limits issuer lookups that miss the cache per client IP: 120 a minute, 60 at once.
func LookupIPRule() ratelimit.Rule {
	return ratelimit.Rule{Name: "issuer-lookup-ip", Rate: 120, Per: time.Minute, Burst: 60}
}

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
	issuers Issuers
	keys    Publisher
	kit     layout.Kit
	cfg     Config
	limiter ratelimit.Limiter
	hops    int
	cache   *issuerCache
	now     func() time.Time
}

// NewWeb binds the issuer routes to the issuer lookup and the key publisher.
func NewWeb(issuers Issuers, keys Publisher, kit layout.Kit, cfg Config) *Web {
	return &Web{
		issuers: issuers, keys: keys, kit: kit, cfg: cfg, limiter: ratelimit.Disabled(),
		cache: &issuerCache{ttl: cacheTTL, entries: map[string]*issuerEntry{}}, now: time.Now,
	}
}

// WithLimiter limits cache-missing lookups per client IP, read through trustedHops proxies.
func (h *Web) WithLimiter(l ratelimit.Limiter, trustedHops int) *Web {
	h.limiter, h.hops = l, trustedHops
	return h
}

// SetClock replaces the cache's clock, for tests.
func (h *Web) SetClock(now func() time.Time) { h.now = now }

// Forget drops a project's cached documents, after its keys or handle change.
func (h *Web) Forget(projectID uuid.UUID) { h.cache.forget(projectID) }

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

// SECURITY: an unknown, reserved or malformed handle all answer the same 404, and only a well-formed miss reaches the database.
func (h *Web) entry(w http.ResponseWriter, r *http.Request) (*issuerEntry, bool) {
	handle := r.PathValue("handle")
	if project.ValidateIssuerHandle(handle) != nil {
		writeJSON(w, http.StatusNotFound, notFoundBody)
		return nil, false
	}
	now := h.now()
	if e := h.cache.get(handle, now); e != nil {
		return e, true
	}
	if retry, limited := h.limited(r); limited {
		w.Header().Set("Retry-After", strconv.Itoa(retry))
		writeJSON(w, http.StatusTooManyRequests, limitedBody)
		return nil, false
	}
	e, err := h.load(r.Context(), handle)
	switch {
	case errors.Is(err, errUnknownIssuer):
		writeJSON(w, http.StatusNotFound, notFoundBody)
		return nil, false
	case err != nil:
		h.report(r.Context(), "oidc: load issuer", err)
		writeJSON(w, http.StatusServiceUnavailable, unavailableBody)
		return nil, false
	}
	h.cache.put(handle, e, now)
	return e, true
}

func (h *Web) load(ctx context.Context, handle string) (*issuerEntry, error) {
	i, err := h.issuers.ResolveIssuer(ctx, handle)
	if project.IsNotFoundError(err) {
		return nil, errUnknownIssuer
	}
	if err != nil {
		return nil, err
	}
	jwks, err := h.keys.JWKS(tenant.Into(ctx, tenant.Context{OrgID: i.OrgID, ProjectID: i.ProjectID}), i.ProjectID)
	if err != nil {
		return nil, err
	}
	discovery, err := json.Marshal(Discovery(project.IssuerURL(h.cfg.PublicBase, i.Handle)))
	if err != nil {
		return nil, err
	}
	return &issuerEntry{issuer: i, discovery: newDocument(discovery), jwks: newDocument(jwks)}, nil
}

// SECURITY: a broken limiter lets lookups through, so a Redis outage never takes every issuer's keys offline.
func (h *Web) limited(r *http.Request) (int, bool) {
	rule := LookupIPRule()
	res, err := h.limiter.Allow(r.Context(), ratelimit.Key(rule, ratelimit.IPSubject(layout.ClientIP(r, h.hops))), rule)
	if err != nil {
		h.report(r.Context(), "oidc: rate limit", err)
		return 0, false
	}
	if res.Allowed {
		return 0, false
	}
	return res.RetryAfterSeconds(), true
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
