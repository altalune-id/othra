package boot

import (
	"github.com/altalune-id/othra/config"
	"github.com/altalune-id/othra/sealer"
)

// NOTE: a missing or invalid key yields a Disabled sealer, not a boot failure.
func buildSealer(cfg config.SecretsConfig) sealer.Sealer {
	key, err := sealer.ParseKey(cfg.EncryptionKey)
	if err != nil {
		return sealer.Disabled()
	}
	sl, err := sealer.New(key)
	if err != nil {
		return sealer.Disabled()
	}
	return sl
}
