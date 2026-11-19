//go:build integration

package pg_test

import (
	"testing"

	"github.com/altalune-id/othra/internal/testutil/pgtest"
)

func TestMain(m *testing.M) {
	pgtest.Main(m)
}
