package pg

import (
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestHealthMonitor_CacheAgeUsesTheMonotonicClock(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	mock.ExpectPing()

	m := NewHealthMonitor(sqlDB, sqlDB, HealthConfig{Interval: time.Hour, Timeout: time.Second}, nil, nil)
	require.NoError(t, m.Probe(t.Context()))
	require.True(t, strings.Contains(m.snap.Load().probed.String(), "m="),
		"a wall-clock step must not make a cached probe look fresh or stale")
}
