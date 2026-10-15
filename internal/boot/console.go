package boot

import (
	"net/http"

	"altalune.id/othra/internal/web/console"
	"altalune.id/othra/internal/web/layout"
	"altalune.id/othra/version"
)

func buildConsoleHandler(basePath string) http.Handler {
	return console.Handler(
		layout.Path(basePath, "/console")+"/",
		layout.Path(basePath, "/api"),
		version.Default(),
	)
}
