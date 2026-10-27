package ui

import (
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
)

func TestDocument(t *testing.T) {
	doc := Document()

	assert.Contains(t, doc, "<!doctype html>")
	assert.Contains(t, doc, "Othra")
	assert.Equal(t, "ui://othra/app", ResourceURI)
}

func TestDocument_FallbackWhenUnbuilt(t *testing.T) {
	assert.Contains(t, document(fstest.MapFS{}), "not built")
}

func TestDocument_ReadsEmbedded(t *testing.T) {
	fsys := fstest.MapFS{"assets/app.html": &fstest.MapFile{Data: []byte("<!doctype html>embedded")}}
	assert.Equal(t, "<!doctype html>embedded", document(fsys))
}

// The MCP view runs in a sandboxed iframe: it must load nothing from another host.
func TestDocument_SelfContained(t *testing.T) {
	doc := Document()
	if strings.Contains(doc, "not built") {
		t.Skip("the MCP app is not built; run make ui-mcp")
	}
	for _, host := range []string{"api.iconify.design", "api.simplesvg.com", "api.unisvg.com"} {
		assert.NotContains(t, doc, host, "icons are inlined, never fetched")
	}
	external := regexp.MustCompile(`(?i)(?:\b(?:src|href)\s*=\s*["']?|url\(\s*["']?|@import\s+["'])(?:https?:)?//`)
	assert.Empty(t, external.FindAllString(doc, -1), "no script, style, image or font from another host")
}
