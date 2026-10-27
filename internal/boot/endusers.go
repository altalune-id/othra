package boot

import (
	"cmp"
	"time"

	"altalune.id/othra/config"
	"altalune.id/othra/internal/enduser"
	"altalune.id/othra/internal/kernel"
	"altalune.id/othra/internal/kernel/db"
)

const (
	endUserLinkTTL     = 15 * time.Minute
	endUserResendAfter = time.Minute
	endUserIdle        = 30 * 24 * time.Hour
	endUserAbsolute    = 90 * 24 * time.Hour
	endUserMaxAccounts = 20
)

func endUsersConfig(c config.EndUsersConfig) enduser.Config {
	return enduser.Config{
		LinkTTL:     cmp.Or(c.MagicLink.TTL, endUserLinkTTL),
		ResendAfter: cmp.Or(c.MagicLink.ResendAfter, endUserResendAfter),
		Idle:        cmp.Or(c.Session.Idle, endUserIdle),
		Absolute:    cmp.Or(c.Session.Absolute, endUserAbsolute),
		MaxAccounts: cmp.Or(c.Session.MaxAccounts, endUserMaxAccounts),
	}
}

func buildEndUsers(
	k kernel.Kernel, cfg config.Config, uow db.Transactor, mail enduser.MailQueue, grants enduser.Grants, signIn enduser.SignInSwitch,
) *enduser.Service {
	store := enduser.NewStore(k.Pool, cfg.Postgres.Schema, cfg.Postgres.TablePrefix)
	return enduser.NewService(store, uow, mail, grants, signIn, endUsersConfig(cfg.EndUsers), k.Reporter.Unexpected).
		WithLogger(orDiscard(k.Log))
}
