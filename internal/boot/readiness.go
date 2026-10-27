package boot

import (
	"github.com/altalune-id/othra/internal/health"
	natsconn "github.com/altalune-id/othra/nats"
	"github.com/altalune-id/othra/pg"
	redisconn "github.com/altalune-id/othra/redis"
	s3conn "github.com/altalune-id/othra/s3"
)

var _ health.Check = (*pg.HealthMonitor)(nil)

func readinessChecks(dbHealth *pg.HealthMonitor, rh *redisconn.Health, nh *natsconn.Health, sh *s3conn.Health) []health.Check {
	checks := []health.Check{dbHealth}
	if rh != nil {
		checks = append(checks, rh)
	}
	if nh != nil {
		checks = append(checks, nh)
	}
	if sh != nil {
		checks = append(checks, sh)
	}
	return checks
}
