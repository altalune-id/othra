//go:build integration

package redis_test

import (
	"testing"

	"github.com/altalune-id/othra/internal/testutil/redistest"
)

func TestMain(m *testing.M) {
	redistest.Main(m)
}
