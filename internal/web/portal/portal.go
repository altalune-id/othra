// Package portal embeds the built end-user portal SPA.
package portal

import (
	"embed"
	"io/fs"

	"github.com/altalune-id/othra/internal/web/spa"
)

//go:embed all:public
//nolint:gochecknoglobals // embedded asset dir.
var files embed.FS

// AssetPath is where the portal's build files are served, under the deployment's base path.
const AssetPath = "/portal/"

// Load reads the embedded build once, pointing its baked asset URLs at assets, the full path the files are served from.
func Load(assets string) (*spa.SPA, error) {
	sub, err := fs.Sub(files, "public")
	if err != nil {
		return nil, err
	}
	return load(sub, assets)
}

func load(fsys fs.FS, assets string) (*spa.SPA, error) {
	if assets == AssetPath {
		return spa.Load(fsys)
	}
	return spa.Load(fsys, spa.Replacement{Old: `"` + AssetPath, New: `"` + assets})
}
