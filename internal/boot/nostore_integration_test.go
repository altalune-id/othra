//go:build integration

package boot

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

// SECURITY: a browser that caches a pre-sign-in page replays its verdict (a 410 for a later valid invite) without asking the server.
func TestPreSignInRoutesAreNeverCached(t *testing.T) {
	l := localLockdown(t)
	form := http.Header{"Content-Type": {"application/x-www-form-urlencoded"}, "Sec-Fetch-Site": {"same-origin"}}
	badLogin := url.Values{"email": {"nobody@example.com"}, "password": {"wrong-password-123"}}.Encode()
	for _, tc := range []struct {
		method, path string
		header       http.Header
		body         string
	}{
		{http.MethodGet, "/login", nil, ""},
		{http.MethodPost, "/login", form, badLogin},
		{http.MethodGet, "/forgot-password", nil, ""},
		{http.MethodGet, "/reset-password/not-a-token", nil, ""},
		{http.MethodGet, "/invite/not-a-token", nil, ""},
		{http.MethodGet, "/install", nil, ""},
		{http.MethodGet, "/terms", nil, ""},
		{http.MethodGet, "/no-such-page", nil, ""},
	} {
		rec := l.do(tc.method, tc.path, tc.header, tc.body)
		assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"), "%s %s answered %d", tc.method, tc.path, rec.Code)
	}

	rec := l.do(http.MethodGet, "/static/favicon.ico", nil, "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "public, max-age=3600", rec.Header().Get("Cache-Control"), "static assets keep their cache")
}
