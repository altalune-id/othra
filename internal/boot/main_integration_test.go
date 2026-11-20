//go:build integration

package boot

import (
	"testing"

	"github.com/altalune-id/othra/internal/testutil/pgtest"
	"github.com/altalune-id/othra/internal/testutil/redistest"
)

func TestMain(m *testing.M) {
	pgtest.Main(redistest.Wrap(m))
}
