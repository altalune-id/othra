// Package ui embeds the built MCP App bundle, published at ui://othra/app.
package ui

import (
	"embed"
	"io/fs"
)

// ResourceURI is the ui:// URI the bundle is published at; it must match buf.gen.yaml's ui_prefix plus the tool's ui name.
const ResourceURI = "ui://othra/app"

//go:embed all:assets
//nolint:gochecknoglobals // embedded asset dir.
var assets embed.FS

const fallback = `<!doctype html>
<html lang="en">
  <head><meta charset="utf-8" /><title>othra</title></head>
  <body><p>othra MCP app not built. Run: make ui-mcp</p></body>
</html>
`

// Document returns the self-contained HTML app, or a placeholder when it has not been built.
func Document() string { return document(assets) }

func document(fsys fs.FS) string {
	if b, err := fs.ReadFile(fsys, "assets/app.html"); err == nil {
		return string(b)
	}
	return fallback
}
