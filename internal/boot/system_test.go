package boot

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/config"
	"altalune.id/othra/internal/install"
)

func TestConsoleURIs(t *testing.T) {
	assert.Equal(t, install.Console{
		RedirectURI: "https://admin.example/othra/oauth/callback", PostLogoutURI: "https://admin.example/othra/console/",
	}, consoleURIs(config.HTTPConfig{BaseURL: "https://admin.example/", BasePath: "/othra"}))
	assert.Equal(t, "http://localhost:5150/oauth/callback", consoleURIs(config.HTTPConfig{Addr: ":5150"}).RedirectURI)
}

// http.baseURL must pass the web redirect rules, or install and every boot would fail later with an unclear error.
func TestCheckPublicBase(t *testing.T) {
	for _, ok := range []config.HTTPConfig{
		{Addr: ":5150"},
		{BaseURL: "http://localhost:5150"},
		{BaseURL: "http://127.0.0.1:8080", BasePath: "/othra"},
		{BaseURL: "https://othra.example.com"},
		{BaseURL: "https://othra.example.com/"},
	} {
		assert.NoError(t, checkPublicBase(ok), "%+v", ok)
	}
	for _, bad := range []config.HTTPConfig{
		{BaseURL: "http://localhost"},
		{BaseURL: "http://othra:8080"},
		{BaseURL: "http://othra.localhost:5150"},
		{Addr: ":80"},
	} {
		err := checkPublicBase(bad)
		var invalid *config.InvalidSettingError
		require.ErrorAs(t, err, &invalid, "%+v", bad)
		assert.Equal(t, "http.baseURL", invalid.Key)
		assert.Contains(t, invalid.Reason, "https://")
	}
	assert.ErrorContains(t, checkPublicBase(config.HTTPConfig{BaseURL: "http://othra:8080"}), "(https_required)")
	for _, odd := range []config.HTTPConfig{
		{BaseURL: "https://Othra.Example.com"},
		{BaseURL: "https://othra.example.com:443/"},
		{BaseURL: "HTTPS://othra.example.com", BasePath: "/app"},
	} {
		err := checkPublicBase(odd)
		require.Error(t, err, "%+v", odd)
		assert.ErrorContains(t, err, `(not_canonical): use "https://othra.example.com"`, "%+v", odd)
	}
}
