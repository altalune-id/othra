package boot

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"altalune.id/othra/config"
)

func TestWarnUpstreamSwitches(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	warnUpstreamSwitches(config.UpstreamsConfig{}, log)
	assert.Empty(t, buf.String(), "no switch, no warning")

	warnUpstreamSwitches(config.UpstreamsConfig{
		Google: config.UpstreamsGoogleConfig{IssuerOverride: "https://fakeidp.test", ClientSecret: "GOCSPX-secret"}, AllowPrivateHosts: true,
	}, log)
	assert.Contains(t, buf.String(), "issuerOverride")
	assert.Contains(t, buf.String(), "allowPrivateHosts")
	assert.NotContains(t, buf.String(), "GOCSPX", "the secret is never logged")
	warnUpstreamSwitches(config.UpstreamsConfig{AllowPrivateHosts: true}, nil)
}
