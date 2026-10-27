// Package console embeds the built console SPA.
package console

import (
	"embed"
	"io/fs"
	"net/http"
	"strconv"

	"altalune.id/othra/internal/web/spa"
)

//go:embed all:public
//nolint:gochecknoglobals // embedded asset dir.
var files embed.FS

// Handler serves the embedded SPA at consoleBase, with Nuxt's baked asset URLs, baseURL, apiUrl and appVersion rebased once.
func Handler(consoleBase, apiBase, appVersion string) http.Handler {
	sub, err := fs.Sub(files, "public")
	if err != nil {
		return http.NotFoundHandler()
	}
	return newSPA(sub, consoleBase, apiBase, appVersion)
}

func newSPA(fsys fs.FS, base, apiBase, appVersion string) http.Handler {
	s, err := spa.Load(fsys, rebase(base, apiBase, appVersion)...)
	if err != nil {
		return http.NotFoundHandler()
	}
	return s
}

func rebase(base, apiBase, appVersion string) []spa.Replacement {
	if base == "" || base == "/" {
		return nil
	}
	rs := []spa.Replacement{{Old: `"/_nuxt/`, New: `"` + base + `_nuxt/`}, {Old: `baseURL:"/"`, New: `baseURL:"` + base + `"`}}
	if apiBase != "" {
		rs = append(rs, spa.Replacement{Old: `apiUrl:""`, New: `apiUrl:"` + apiBase + `"`})
	}
	if appVersion != "" {
		rs = append(rs, spa.Replacement{Old: `appVersion:""`, New: `appVersion:` + strconv.Quote(appVersion)})
	}
	return rs
}
