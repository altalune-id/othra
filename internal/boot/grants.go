package boot

import (
	"cmp"
	"time"

	"altalune.id/othra/config"
	"altalune.id/othra/internal/grant"
	"altalune.id/othra/internal/kernel"
	"altalune.id/othra/internal/kernel/db"
)

const (
	grantCodeTTL         = time.Minute
	grantAccessTokenTTL  = 15 * time.Minute
	grantRefreshIdle     = 30 * 24 * time.Hour
	grantRefreshAbsolute = 90 * 24 * time.Hour
)

func grantConfig(c config.OAuthConfig) grant.Config {
	return grant.Config{
		CodeTTL:         cmp.Or(c.CodeTTL, grantCodeTTL),
		AccessTokenTTL:  cmp.Or(c.AccessTokenTTL, grantAccessTokenTTL),
		RefreshIdle:     cmp.Or(c.Refresh.Idle, grantRefreshIdle),
		RefreshAbsolute: cmp.Or(c.Refresh.Absolute, grantRefreshAbsolute),
	}
}

func buildGrants(k kernel.Kernel, cfg config.Config, uow db.Transactor) *grant.Service {
	store := grant.NewStore(k.Pool, cfg.Postgres.Schema, cfg.Postgres.TablePrefix)
	return grant.NewService(store, uow, grantConfig(cfg.OAuth), k.Reporter.Unexpected).WithLogger(orDiscard(k.Log))
}
