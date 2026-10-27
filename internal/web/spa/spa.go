// Package spa serves an embedded single-page app build: its files with build-aware caching and one index.html rebased at boot.
package spa

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/altalune-id/othra/internal/web/assets"
)

const (
	indexName     = "index.html"
	manifestName  = "_nuxt/builds/latest.json"
	hashedPrefix  = "_nuxt/"
	cacheHashed   = "public, max-age=31536000, immutable"
	cacheShort    = "public, max-age=3600"
	cacheValidate = "no-cache"
)

// Replacement swaps every Old for New in index.html.
type Replacement struct {
	Old, New string
}

// Rebase applies each replacement to html in order.
func Rebase(html string, rs ...Replacement) string {
	for _, r := range rs {
		html = strings.ReplaceAll(html, r.Old, r.New)
	}
	return html
}

// SPA is a loaded build: its files and its index.html, rebased once.
type SPA struct {
	files assets.Files
	html  string
	index *assets.Asset
}

// Load reads every file of fsys once and rebases its index.html with once; a build without index.html serves files only.
func Load(fsys fs.FS, once ...Replacement) (*SPA, error) {
	files, err := assets.Load(fsys)
	if err != nil {
		return nil, err
	}
	s := &SPA{files: files}
	if b, err := fs.ReadFile(fsys, indexName); err == nil {
		s.html = Rebase(string(b), once...)
		s.index = assets.New(indexName, []byte(s.html))
	}
	return s, nil
}

// ServeHTTP serves a build file by path and the index for every other path, so the client router owns its routes.
func (s *SPA) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	switch {
	case name == indexName:
		loc := "./"
		if r.URL.RawQuery != "" {
			loc += "?" + r.URL.RawQuery
		}
		w.Header().Set("Location", loc)
		w.WriteHeader(http.StatusMovedPermanently)
	case s.serveFile(w, r, name):
	default:
		s.Index(w, r)
	}
}

// Files serves only the build's files and answers 404 for anything else.
func (s *SPA) Files() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if !s.serveFile(w, r, name) {
			http.NotFound(w, r)
		}
	})
}

// Index writes index.html with each applied on top of the boot rebase, revalidated on every load.
func (s *SPA) Index(w http.ResponseWriter, r *http.Request, each ...Replacement) {
	s.Page(w, r, cacheValidate, each...)
}

// Page writes index.html with each applied on top of the boot rebase, under cacheControl.
func (s *SPA) Page(w http.ResponseWriter, r *http.Request, cacheControl string, each ...Replacement) {
	if s.index == nil {
		http.NotFound(w, r)
		return
	}
	idx := s.index
	if len(each) > 0 {
		idx = assets.New(indexName, []byte(Rebase(s.html, each...)))
	}
	idx.Serve(w, r, cacheControl)
}

func (s *SPA) serveFile(w http.ResponseWriter, r *http.Request, name string) bool {
	switch {
	case s.files.Sibling(name):
		http.NotFound(w, r)
	case s.files[name] != nil:
		s.files[name].Serve(w, r, cacheControl(name))
	default:
		return false
	}
	return true
}

func cacheControl(name string) string {
	switch {
	case name == manifestName:
		return cacheValidate
	case strings.HasPrefix(name, hashedPrefix):
		return cacheHashed
	default:
		return cacheShort
	}
}
