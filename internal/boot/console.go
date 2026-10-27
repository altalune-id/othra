package boot

import (
	"net/http"

	"github.com/altalune-id/othra/internal/web/console"
	"github.com/altalune-id/othra/internal/web/layout"
	"github.com/altalune-id/othra/internal/web/portal"
	"github.com/altalune-id/othra/version"
)

func buildConsoleHandler(basePath string) http.Handler {
	return console.Handler(
		layout.Path(basePath, "/console")+"/",
		layout.Path(basePath, "/api"),
		version.Default(),
	)
}

// NOTE: one embedded portal build: the page the hosted /account/ serves and the files under /portal/; without a build, neither.
func buildPortal(basePath string) (*portal.Page, http.Handler) {
	s, err := portal.Load(layout.Path(basePath, "/portal") + "/")
	if err != nil {
		return nil, nil
	}
	return portal.NewPage(s), s.Files()
}
