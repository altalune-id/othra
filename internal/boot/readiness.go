package boot

import (
	"altalune.id/othra/internal/health"
	natsconn "altalune.id/othra/nats"
	"altalune.id/othra/pg"
	redisconn "altalune.id/othra/redis"
)

var _ health.Check = (*pg.HealthMonitor)(nil)

func readinessChecks(dbHealth *pg.HealthMonitor, rh *redisconn.Health, nh *natsconn.Health) []health.Check {
	checks := []health.Check{dbHealth}
	if rh != nil {
		checks = append(checks, rh)
	}
	if nh != nil {
		checks = append(checks, nh)
	}
	return checks
}
