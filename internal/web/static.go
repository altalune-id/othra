package web

import (
	"embed"
	"io/fs"
	"net/http"
	"sync"

	"github.com/altalune-id/othra/internal/web/assets"
)

//go:embed all:static
var staticFS embed.FS

// StaticFS returns the embedded static assets: the compiled app.css, password.js and lang.js, the logo and the favicons.
func StaticFS() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return sub
}

//nolint:gochecknoglobals // the embedded assets, loaded once for the handler and the versions.
var staticFiles = sync.OnceValues(func() (assets.Files, error) { return assets.Load(StaticFS()) })

// StaticVersion returns the content hash of name under /static for a cache-busting ?v=, or "" when it is missing.
func StaticVersion(name string) string {
	files, err := staticFiles()
	if err != nil {
		return ""
	}
	return files.Version(name)
}

// SECURITY: unversioned URLs revalidate by ETag, so a deploy never serves stale CSS or JS from a browser cache.
func staticHandler(prefix string, load func() (assets.Files, error)) http.Handler {
	files, err := load()
	if err != nil {
		return http.NotFoundHandler()
	}
	return http.StripPrefix(prefix, files.Handler("no-cache"))
}
