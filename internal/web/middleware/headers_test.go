package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
)

func TestSecurityHeaders(t *testing.T) {
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	rec := httptest.NewRecorder()
	SecurityHeaders(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))

	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	assert.Equal(t, "frame-ancestors 'none'", rec.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "strict-origin-when-cross-origin", rec.Header().Get("Referrer-Policy"))
}

func TestSecurityHeaders_HandlerCanOverride(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Referrer-Policy", "no-referrer")
	})
	rec := httptest.NewRecorder()
	SecurityHeaders(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/invite/x", nil))

	assert.Equal(t, "no-referrer", rec.Header().Get("Referrer-Policy"))
}

func TestNoStore(t *testing.T) {
	for name, status := range map[string]int{"ok": http.StatusOK, "redirect": http.StatusSeeOther, "gone": http.StatusGone} {
		t.Run(name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) })
			rec := httptest.NewRecorder()
			NoStore(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/invite/x", nil))

			assert.Equal(t, status, rec.Code)
			assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
		})
	}
}

func TestHostedCSP(t *testing.T) {
	var seen []string
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = append(seen, templ.GetNonce(r.Context()))
	})
	h := SecurityHeaders(HostedCSP("https://files.example.com/", "https://files.example.com/other", "http://cdn.test:8080", "", "javascript:alert(1)", "not a url", "https://x.test;script-src *", "https://ok.test/path;script-src *")(next))
	var policies []string
	for range 2 {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/p/acme/login", nil))
		policies = append(policies, rec.Header().Get("Content-Security-Policy"))
	}

	assert.Len(t, seen[0], 22)
	assert.NotEqual(t, seen[0], seen[1], "a fresh nonce per response")
	want := func(nonce string) string {
		return "default-src 'self'; img-src 'self' data: https://files.example.com http://cdn.test:8080 https://ok.test; style-src 'self' 'nonce-" + nonce +
			"'; form-action 'self'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'"
	}
	assert.Equal(t, want(seen[0]), policies[0])
	assert.Equal(t, want(seen[1]), policies[1])
}

func TestHostedCSP_WithoutImageBases(t *testing.T) {
	rec := httptest.NewRecorder()
	HostedCSP()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/p/acme/login", nil))
	assert.Contains(t, rec.Header().Get("Content-Security-Policy"), "default-src 'self'; img-src 'self' data:; style-src 'self' 'nonce-")
}
