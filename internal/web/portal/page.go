package portal

import (
	"encoding/json"
	"html"
	"net/http"

	"altalune.id/othra/internal/web/spa"
)

// Values are one request's portal settings: the router base and this project's endpoints and brand.
type Values struct {
	BaseURL    string
	APIURL     string
	HostedURL  string
	Handle     string
	Project    string
	LogoURL    string
	TermsURL   string
	PrivacyURL string
	BrandCSS   string
	Nonce      string
}

// Page serves the portal's index.html with one request's Values in Nuxt's runtime config.
type Page struct {
	spa *spa.SPA
}

// NewPage serves s's index.
func NewPage(s *spa.SPA) *Page {
	return &Page{spa: s}
}

// Serve writes the index for v, never cached. SECURITY: every value is a JSON string literal, so none can close its script.
func (p *Page) Serve(w http.ResponseWriter, r *http.Request, v Values) {
	p.spa.Page(w, r, "no-store", replacements(v)...)
}

func replacements(v Values) []spa.Replacement {
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
