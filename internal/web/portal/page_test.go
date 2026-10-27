package portal

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/internal/web/layout"
)

const builtIndex = `<html><head><script type="importmap">{}</script></head><body>` +
	`<script>window.__NUXT__.config={public:{apiUrl:"",handle:"",project:"",logoUrl:"",termsUrl:"",privacyUrl:"",hostedUrl:"",i18n:{baseUrl:""}},app:{baseURL:"/",cdnURL:"/portal/"}}</script></body></html>`

func serve(t *testing.T, v layout.PortalValues) string {
	t.Helper()
	s, err := load(fstest.MapFS{"index.html": {Data: []byte(builtIndex)}}, AssetPath)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	NewPage(s).Serve(rec, httptest.NewRequest(http.MethodGet, "/p/acme/account/sessions", nil), v)
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	return rec.Body.String()
}

func TestPage_FillsTheRuntimeConfigPerRequest(t *testing.T) {
	t.Parallel()
	out := serve(t, layout.PortalValues{
		BaseURL: "/p/acme/account/", APIURL: "/p/acme/api", HostedURL: "/p/acme", Handle: "acme", Project: `Acme "Store" </script>`,
		LogoURL: "https://files.test/l.png", BrandCSS: ":root{--color-primary:#1e3a8a}", Nonce: "n0nce",
	})
	assert.Contains(t, out, `baseURL:"/p/acme/account/"`)
	assert.Contains(t, out, `apiUrl:"/p/acme/api"`)
	assert.Contains(t, out, `hostedUrl:"/p/acme"`)
	assert.Contains(t, out, `project:"Acme \"Store\" \u003c/script\u003e"`, "a project name can never close the script")
	assert.Contains(t, out, `termsUrl:""`)
	assert.Contains(t, out, `i18n:{baseUrl:""}`, "only the portal's own keys change")
	assert.Contains(t, out, `<script type="importmap" nonce="n0nce">`)
	assert.Contains(t, out, `<script nonce="n0nce">window.__NUXT__`)
	assert.Contains(t, out, `<style nonce="n0nce">:root{--color-primary:#1e3a8a}</style></head>`)
}

func TestPage_NoBrandNoStyle(t *testing.T) {
	t.Parallel()
	out := serve(t, layout.PortalValues{BaseURL: "/p/beta/account/", Nonce: "x"})
	assert.Contains(t, out, `baseURL:"/p/beta/account/"`)
	assert.NotContains(t, out, "<style")
}
