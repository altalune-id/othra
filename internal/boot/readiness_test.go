package boot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	natsconn "altalune.id/othra/nats"
	"altalune.id/othra/pg"
	redisconn "altalune.id/othra/redis"
	s3conn "altalune.id/othra/s3"
)

func TestReadinessChecks(t *testing.T) {
	dbh := &pg.HealthMonitor{}
	assert.Len(t, readinessChecks(dbh, nil, nil, nil), 1, "no redis, nats or s3, no extra check")

	rh := redisconn.NewHealth(nil, time.Second, testLogger())
	nh := natsconn.NewHealth(nil, testLogger())
	sh := s3conn.NewHealth(nil, time.Second, testLogger())
	checks := readinessChecks(dbh, rh, nh, sh)
	require.Len(t, checks, 4)
	assert.Same(t, rh, checks[1])
	assert.Same(t, nh, checks[2])
	assert.Same(t, sh, checks[3])

	checks = readinessChecks(dbh, nil, nh, nil)
	require.Len(t, checks, 2)
	assert.Same(t, nh, checks[1])
}
