package boot

import (
	"altalune.id/othra/config"
	"altalune.id/othra/internal/kernel"
	"altalune.id/othra/internal/kernel/db"
	"altalune.id/othra/internal/keys"
	"altalune.id/othra/sealer"
)

const defaultRSABits = 2048

func keysConfig(c config.KeysConfig) keys.Config {
	bits := c.RSABits
	if bits == 0 {
		bits = defaultRSABits
	}
	return keys.Config{RSABits: bits, PrePublish: c.Rotation.PrePublish, Overlap: c.Rotation.Overlap}
}

func buildKeys(k kernel.Kernel, cfg config.Config, uow db.Transactor, sl sealer.Sealer) *keys.Service {
	store := keys.NewStore(k.Pool, cfg.Postgres.Schema, cfg.Postgres.TablePrefix)
	return keys.NewService(store, uow, sl, keysConfig(cfg.Keys), k.Reporter.Unexpected).WithLogger(orDiscard(k.Log))
}
