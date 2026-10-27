package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/internal/web/i18n"
)

type hostedPages struct{ basePath string }

func (p hostedPages) Register(mux *http.ServeMux) {
	page := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Locale", i18n.Locale(r.Context()))
		w.Header().Set("X-Nonce", templ.GetNonce(r.Context()))
		_, _ = w.Write([]byte("hosted " + r.PathValue("handle")))
	}
	mux.HandleFunc("GET "+p.basePath+"/p/{handle}/login", page)
	mux.HandleFunc("POST "+p.basePath+"/p/{handle}/login", page)
}

func adminSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Trace", "session")
		next.ServeHTTP(w, r)
	})
}

func welcomeGate(basePath string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, err := r.Cookie("othra_sid"); err == nil {
				http.Redirect(w, r, basePath+"/welcome", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func hostedServer(basePath string) http.Handler {
	return NewServer(ServerOpts{
		BasePath:     basePath,
		Routes:       []Registrar{preSignIn{}},
		PublicRoutes: []Registrar{issuerDocs{basePath: basePath}},
		HostedPages:  []Registrar{hostedPages{basePath: basePath}},
		Session:      adminSession,
		SessionGate:  welcomeGate(basePath),
		HostedGate:   tag("hosted-gate"),

		HostedImageBases: []string{"https://files.example.com/"},
	})
}

// SECURITY: an admin cookie on a hosted page never meets the admin session or the welcome gate.
func TestHostedPages_NoAdminSessionOrWelcomeGate(t *testing.T) {
	for _, basePath := range []string{"", "/auth"} {
		srv := hostedServer(basePath)
		req := httptest.NewRequest(http.MethodGet, basePath+"/p/acme/login", nil)
		req.AddCookie(&http.Cookie{Name: "othra_sid", Value: "admin"})
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "no redirect to /welcome")
		assert.Equal(t, "hosted acme", rec.Body.String())
		assert.Equal(t, []string{"hosted-gate"}, rec.Header().Values("X-Trace"), "only the hosted gate runs")

		admin := httptest.NewRequest(http.MethodGet, basePath+"/login", nil)
		admin.AddCookie(&http.Cookie{Name: "othra_sid", Value: "admin"})
		rec = httptest.NewRecorder()
		srv.ServeHTTP(rec, admin)
		assert.Equal(t, http.StatusSeeOther, rec.Code, "admin pages keep their welcome gate")
	}
}

// SECURITY: hosted pages are never cached, send no Referer, refuse framing, and load only this origin, the storage images and their nonce style.
func TestHostedPages_Headers(t *testing.T) {
	rec := get(t, hostedServer("/auth"), "/auth/p/acme/login")
	require.Equal(t, http.StatusOK, rec.Code)
	h := rec.Header()
	assert.Equal(t, "no-store", h.Get("Cache-Control"))
	assert.Equal(t, "no-referrer", h.Get("Referrer-Policy"))
	assert.Equal(t, "DENY", h.Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", h.Get("X-Content-Type-Options"))
	nonce := h.Get("X-Nonce")
	require.NotEmpty(t, nonce)
	assert.Equal(t, "default-src 'self'; img-src 'self' data: https://files.example.com; style-src 'self' 'nonce-"+nonce+
		"'; form-action 'self'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'", h.Get("Content-Security-Policy"))
	assert.Contains(t, h.Values("Vary"), "Accept-Language")
}

func TestHostedPages_Locale(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/p/acme/login", nil)
	req.Header.Set("Accept-Language", "id-ID,id;q=0.9,en;q=0.5")
	rec := httptest.NewRecorder()
	hostedServer("").ServeHTTP(rec, req)
	assert.Equal(t, "id-ID", rec.Header().Get("X-Locale"))
	assert.Equal(t, "en-US", get(t, hostedServer(""), "/p/acme/login").Header().Get("X-Locale"))
}

// SECURITY: a cross-origin form post to a hosted page is refused before its handler.
func TestHostedPages_CrossOriginPostIsRefused(t *testing.T) {
	srv := hostedServer("")
	post := func(site string) int {
		req := httptest.NewRequest(http.MethodPost, "/p/acme/login", strings.NewReader("email=a%40b.c"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Sec-Fetch-Site", site)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		return rec.Code
	}
	assert.Equal(t, http.StatusForbidden, post("cross-site"))
	assert.Equal(t, http.StatusOK, post("same-origin"))
}

func TestHostedPages_BesideTheIssuerDocumentsAndUnknownPaths(t *testing.T) {
	srv := hostedServer("/auth")
	rec := get(t, srv, "/auth/p/acme/.well-known/openid-configuration")
	assert.Equal(t, "public, max-age=300", rec.Header().Get("Cache-Control"), "public documents keep their own group")
	assert.Equal(t, http.StatusNotFound, get(t, srv, "/auth/p/acme/nope").Code)
	assert.Equal(t, "no-store", get(t, srv, "/auth/p/acme/nope").Header().Get("Cache-Control"))
	assert.Equal(t, http.StatusOK, get(t, NewServer(ServerOpts{Routes: []Registrar{preSignIn{}}}), "/login").Code,
		"no hosted pages mounts nothing under /p/")
}
