package boot

import (
	"log/slog"

	"github.com/altalune-id/othra/config"
)

// SECURITY: these qa switches change whom Othra trusts for sign-in, so a deployer sees them at every boot.
func warnUpstreamSwitches(c config.UpstreamsConfig, log *slog.Logger) {
	if log == nil {
		return
	}
	if c.Google.IssuerOverride != "" {
		log.Warn("boot: upstreams.google.issuerOverride is set; Google sign-in trusts that issuer, for qa only",
			slog.String("issuer", c.Google.IssuerOverride))
	}
	if c.AllowPrivateHosts {
		log.Warn("boot: upstreams.allowPrivateHosts is on; upstream IdP fetches may reach private addresses, for development only")
	}
}
