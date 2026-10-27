package portal

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const indexHTML = `<script type="importmap">{"imports":{"#entry":"/portal/_nuxt/e.js"}}</script>` +
	`<script>window.__NUXT__.config={app:{baseURL:"/",cdnURL:"/portal/"}}</script>`

func index(t *testing.T, assets string) string {
	t.Helper()
	s, err := load(fstest.MapFS{"index.html": {Data: []byte(indexHTML)}}, assets)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	s.Index(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	return rec.Body.String()
}

func TestLoad_RebasesTheAssetsOnlyUnderABasePath(t *testing.T) {
	t.Parallel()
	assert.Equal(t, indexHTML, index(t, "/portal/"))
	under := index(t, "/oa/portal/")
	assert.Contains(t, under, `"#entry":"/oa/portal/_nuxt/e.js"`)
	assert.Contains(t, under, `cdnURL:"/oa/portal/"`)
}

func TestLoad_Embedded(t *testing.T) {
	t.Parallel()
	_, err := Load(AssetPath)
	require.NoError(t, err)
}
