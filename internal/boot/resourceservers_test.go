package boot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/altalune-id/othra/config"
	"github.com/altalune-id/othra/internal/resourceserver"
)

func TestResourceServersConfig(t *testing.T) {
	assert.Equal(t, resourceserver.Config{Interval: time.Hour, StaleAfter: 24 * time.Hour, TrustAvailable: true},
		resourceServersConfig(config.Config{Mode: config.ModeSelfhosted}), "a config literal still gets a schedule")
	assert.Equal(t, resourceserver.Config{Interval: 30 * time.Minute, StaleAfter: 12 * time.Hour},
		resourceServersConfig(config.Config{Mode: config.ModeCloud, ResourceServers: config.ResourceServersConfig{
			Verification: config.ResourceServersVerificationConfig{Interval: 30 * time.Minute, StaleAfter: 12 * time.Hour},
		}}), "cloud never offers trust")
}
