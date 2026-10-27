package spa

import (
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const indexHTML = `<script type="importmap">{"imports":{"#entry":"/portal/_nuxt/e.js"}}</script>` +
	`<script>window.__NUXT__.config={app:{baseURL:"/",cdnURL:"/portal/"}}</script>`

func build() fstest.MapFS {
	return fstest.MapFS{
		"index.html":               {Data: []byte(indexHTML)},
		"_nuxt/e.js":               {Data: []byte("1")},
		"_nuxt/builds/latest.json": {Data: []byte("{}")},
	}
}

func get(h http.Handler, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestLoad_RebasesOnceAndServesEachRequestItsOwnIndex(t *testing.T) {
	t.Parallel()
	s, err := Load(build(), Replacement{Old: `"/portal/`, New: `"/oa/portal/`})
	require.NoError(t, err)

	page := func(handle string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		s.Index(rec, httptest.NewRequest(http.MethodGet, "/p/"+handle+"/account/sessions", nil),
			Replacement{Old: `baseURL:"/"`, New: `baseURL:"/p/` + handle + `/account/"`})
		return rec
	}
	acme, beta := page("acme"), page("beta")
	assert.Contains(t, acme.Body.String(), `baseURL:"/p/acme/account/"`)
	assert.Contains(t, beta.Body.String(), `baseURL:"/p/beta/account/"`)
	for _, rec := range []*httptest.ResponseRecorder{acme, beta} {
		assert.Contains(t, rec.Body.String(), `"#entry":"/oa/portal/_nuxt/e.js"`)
		assert.Contains(t, rec.Body.String(), `cdnURL:"/oa/portal/"`)
		assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
		assert.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
	}
	assert.NotEqual(t, acme.Header().Get("ETag"), beta.Header().Get("ETag"))

	plain := httptest.NewRecorder()
	s.Index(plain, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Contains(t, plain.Body.String(), `baseURL:"/"`)
}

func TestFiles_ServesTheBuildOnly(t *testing.T) {
	t.Parallel()
	s, err := Load(build())
	require.NoError(t, err)
	files := s.Files()
	asset := get(files, "/_nuxt/e.js")
	assert.Equal(t, http.StatusOK, asset.Code)
	assert.Equal(t, "public, max-age=31536000, immutable", asset.Header().Get("Cache-Control"))
	assert.Equal(t, "no-cache", get(files, "/_nuxt/builds/latest.json").Header().Get("Cache-Control"))
	assert.Equal(t, http.StatusNotFound, get(files, "/sessions").Code, "no index fallback for files")
	assert.Equal(t, http.StatusNotFound, get(files, "/_nuxt/missing.js").Code)
}

func TestIndex_WithoutABuiltIndex(t *testing.T) {
	t.Parallel()
	s, err := Load(fstest.MapFS{"_nuxt/e.js": {Data: []byte("1")}})
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, get(s, "/sessions").Code)
	assert.Equal(t, http.StatusOK, get(s, "/_nuxt/e.js").Code)
}

func TestLoad_BrokenBuild(t *testing.T) {
	t.Parallel()
	_, err := Load(brokenFS{})
	require.Error(t, err)
}

type brokenFS struct{}

func (brokenFS) Open(string) (fs.File, error) { return nil, errors.New("broken") }

func TestServeHTTP_FilesFirstThenIndex(t *testing.T) {
	t.Parallel()
	b := build()
	b["_nuxt/e.js.br"] = &fstest.MapFile{Data: []byte("BR")}
	b["200.html"] = &fstest.MapFile{Data: []byte("<html></html>")}
	s, err := Load(b)
	require.NoError(t, err)

	assert.Equal(t, "./?a=1", get(s, "/index.html?a=1").Header().Get("Location"))
	assert.Equal(t, http.StatusMovedPermanently, get(s, "/index.html").Code)
	assert.Equal(t, http.StatusNotFound, get(s, "/_nuxt/e.js.br").Code, "a compressed sibling is never served by name")
	assert.Equal(t, "public, max-age=3600", get(s, "/200.html").Header().Get("Cache-Control"))
	route := get(s, "/o/acme")
	assert.Equal(t, http.StatusOK, route.Code)
	assert.Contains(t, route.Body.String(), `baseURL:"/"`)
}
