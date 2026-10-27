package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type issuerDocs struct{ basePath string }

func (d issuerDocs) Register(mux *http.ServeMux) {
	doc := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=300")
		_, _ = w.Write([]byte(`{}`))
	}
	mux.HandleFunc("GET "+d.basePath+"/p/{handle}/.well-known/openid-configuration", doc)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server"+d.basePath+"/p/{handle}", doc)
}

func setCookie(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "othra_session", Value: "x"})
		next.ServeHTTP(w, r)
	})
}

// SECURITY: public issuer documents carry no cookie and keep their public cache, so a CDN may hold them and no session leaks.
func TestPublicRoutesSkipSessionAndNoStore(t *testing.T) {
	for _, basePath := range []string{"", "/auth"} {
		srv := NewServer(ServerOpts{
			BasePath:     basePath,
			Routes:       []Registrar{preSignIn{}},
			PublicRoutes: []Registrar{issuerDocs{basePath: basePath}},
			Session:      setCookie,
			SessionGate:  tag("session-gate"),
		})
		for _, p := range []string{basePath + "/p/acme/.well-known/openid-configuration", "/.well-known/oauth-authorization-server" + basePath + "/p/acme"} {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, p, nil)
			req.Header.Set("Origin", "https://app.example")
			req.Header.Set("Sec-Fetch-Site", "cross-site")
			srv.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code, p)
			assert.Equal(t, "public, max-age=300", rec.Header().Get("Cache-Control"), p)
			assert.Empty(t, rec.Header().Values("Set-Cookie"), p)
			assert.Empty(t, rec.Header().Values("X-Trace"), p)
			assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"), p)
			assert.Empty(t, rec.Header().Get("Cross-Origin-Resource-Policy"), p)
		}
		rec := get(t, srv, basePath+"/login")
		assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"), "pre-sign-in pages stay no-store beside public routes")
	}
}
