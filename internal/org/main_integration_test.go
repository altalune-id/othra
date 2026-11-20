//go:build integration

package org_test

import (
	"testing"

	"github.com/altalune-id/othra/internal/testutil/pgtest"
)

func TestMain(m *testing.M) {
	pgtest.Main(m)
}
