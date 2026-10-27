package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type preSignIn struct{}

func (preSignIn) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /login", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("login")) })
	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/console/", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /invite/{token}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusGone) })
	mux.HandleFunc("POST /reset-password", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnprocessableEntity) })
}

// SECURITY: a cached pre-sign-in page replays a token's verdict (a 410 for a later valid invite) without asking the server.
func TestPreSignInPagesAreNeverCached(t *testing.T) {
	srv := NewServer(ServerOpts{Routes: []Registrar{preSignIn{}}})
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/login", http.StatusOK},
		{http.MethodPost, "/login", http.StatusSeeOther},
		{http.MethodGet, "/invite/abc", http.StatusGone},
		{http.MethodPost, "/reset-password", http.StatusUnprocessableEntity},
		{http.MethodGet, "/unknown", http.StatusNotFound},
	} {
		var req *http.Request
		if tc.method == http.MethodPost {
			req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(url.Values{}.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Sec-Fetch-Site", "same-origin")
		} else {
			req = httptest.NewRequest(tc.method, tc.path, nil)
		}
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		assert.Equal(t, tc.want, rec.Code, tc.method+" "+tc.path)
		assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"), tc.method+" "+tc.path)
	}
}

func TestStaticAssetsKeepTheirCacheBesidePreSignInPages(t *testing.T) {
	srv := NewServer(ServerOpts{Routes: []Registrar{preSignIn{}}})
	rec := get(t, srv, "/static/favicon.ico")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "public, max-age=3600", rec.Header().Get("Cache-Control"))
}
