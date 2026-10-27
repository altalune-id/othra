package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPublicCORS(t *testing.T) {
	served := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		served++
		w.WriteHeader(http.StatusOK)
	})
	h := PublicCORS(next)

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/p/acme/oauth2/jwks.json", nil)
		req.Header.Set("Origin", "https://app.example")
		h.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, method)
		assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"), method)
		assert.Equal(t, "ETag", rec.Header().Get("Access-Control-Expose-Headers"), method)
		assert.Empty(t, rec.Header().Get("Access-Control-Allow-Credentials"), method)
	}
	assert.Equal(t, 2, served)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/p/acme/oauth2/jwks.json", nil)
	req.Header.Set("Origin", "https://app.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, 2, served, "a preflight never reaches the handler")
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, HEAD, OPTIONS", rec.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type, If-None-Match", rec.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "86400", rec.Header().Get("Access-Control-Max-Age"))
}

func TestBearerCORS(t *testing.T) {
	served := 0
	h := BearerCORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		served++
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/p/acme/oauth2/token", nil)
	req.Header.Set("Origin", "https://app.example")
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "WWW-Authenticate", rec.Header().Get("Access-Control-Expose-Headers"))
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Credentials"), "never with cookies")

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodOptions, "/p/acme/oauth2/userinfo", nil)
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, 1, served, "a preflight never reaches the handler")
	assert.Equal(t, "GET, POST, OPTIONS", rec.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Authorization, Content-Type", rec.Header().Get("Access-Control-Allow-Headers"))
}
