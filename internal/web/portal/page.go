package portal

import (
	"encoding/json"
	"html"
	"net/http"

	"altalune.id/othra/internal/web/layout"
	"altalune.id/othra/internal/web/middleware"
	"altalune.id/othra/internal/web/spa"
)

// Page serves the portal's index.html with one request's layout.PortalValues in Nuxt's runtime config.
type Page struct {
	spa *spa.SPA
}

// NewPage serves s's index.
func NewPage(s *spa.SPA) *Page {
	return &Page{spa: s}
}

// Serve writes v's index, uncached, with its nonce scripts. SECURITY: every value is a JSON string literal, so none can close its script.
func (p *Page) Serve(w http.ResponseWriter, r *http.Request, v layout.PortalValues) {
	middleware.AllowNonceScripts(w, v.Nonce)
	p.spa.Page(w, r, "no-store", replacements(v)...)
}

func replacements(v layout.PortalValues) []spa.Replacement {
	nonce := html.EscapeString(v.Nonce)
	rs := []spa.Replacement{
		{Old: `baseURL:"/"`, New: `baseURL:` + literal(v.BaseURL)},
		{Old: `<script type="importmap">`, New: `<script type="importmap" nonce="` + nonce + `">`},
		{Old: `<script>window.__NUXT__`, New: `<script nonce="` + nonce + `">window.__NUXT__`},
	}
	for key, value := range map[string]string{
		"apiUrl": v.APIURL, "hostedUrl": v.HostedURL, "handle": v.Handle, "project": v.Project, "logoUrl": v.LogoURL,
		"termsUrl": v.TermsURL, "privacyUrl": v.PrivacyURL,
	} {
		rs = append(rs, spa.Replacement{Old: key + `:""`, New: key + `:` + literal(value)})
	}
	if v.BrandCSS != "" {
		rs = append(rs, spa.Replacement{Old: `</head>`, New: `<style nonce="` + nonce + `">` + v.BrandCSS + `</style></head>`})
	}
	return rs
}

func literal(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
