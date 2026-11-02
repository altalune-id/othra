package portal

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/internal/web/layout"
)

func TestBuiltIndexKeepsEveryReplacedPlaceholder(t *testing.T) {
	index, err := fs.ReadFile(files, "public/index.html")
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("internal/web/portal/public is not built; run make ui")
	}
	require.NoError(t, err)
	olds := []string{`"` + AssetPath}
	for _, r := range replacements(layout.PortalValues{BrandCSS: "x"}) {
		olds = append(olds, r.Old)
	}
	for _, old := range olds {
		assert.Contains(t, string(index), old)
	}
}
