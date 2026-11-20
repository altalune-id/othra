//go:build integration

package redis

import (
	"testing"

	"github.com/altalune-id/othra/internal/testutil/redistest"
)

func TestMain(m *testing.M) {
	redistest.Main(m)
}
