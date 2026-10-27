//go:build integration

package boot

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/config"
	"altalune.id/othra/internal/project"
	"altalune.id/othra/internal/upstream"
)

// Install seeds /p/system and the console client; every serving boot rewrites only the client's URIs, from the current public URL.
func TestFirstBoot_SeedsAndReconcilesTheConsole(t *testing.T) {
	cfg := devLocalConfig()
	cfg.Auth.Genesis = config.GenesisConfig{Email: "root@example.com", Password: "genesis-password"}
	cfg.Upstreams.Google = config.UpstreamsGoogleConfig{ClientID: "google-client", ClientSecret: "google-secret"}
	svcs, cfg := migratedServices(t, cfg)
	svcs.Caps.EnvGenesis = true
	ctx := t.Context()
	require.NoError(t, firstBoot(ctx, cfg, svcs, testLogger(), false))

	sys, p, err := consoleProject(ctx, svcs)
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.True(t, p.IsSystem())
	assert.Equal(t, sys.ID, p.OrgID)
	c, err := svcs.Clients.System(ctx, sys.ID, p.ID)
	require.NoError(t, err)
	want := consoleURIs(cfg.HTTP)
	assert.Equal(t, "http://localhost:5150/oauth/callback", want.RedirectURI)
	assert.Equal(t, []string{want.RedirectURI}, c.RedirectURIs)
	assert.Equal(t, []string{want.PostLogoutURI}, c.PostLogoutRedirectURIs)

	ups, err := svcs.Upstreams.List(ctx, sys.ID, p.ID)
	require.NoError(t, err)
	require.Len(t, ups, 1, "upstreams.google seeds /p/system")
	assert.Equal(t, upstream.KindGoogle, ups[0].Kind)
	def, err := svcs.Projects.BySlug(ctx, sys.ID, project.DefaultSlug)
	require.NoError(t, err)
	none, err := svcs.Upstreams.List(ctx, sys.ID, def.ID)
	require.NoError(t, err)
	assert.Empty(t, none, "the default project is no longer seeded")

	require.NoError(t, firstBoot(ctx, cfg, svcs, testLogger(), true))
	again, err := svcs.Clients.System(ctx, sys.ID, p.ID)
	require.NoError(t, err)
	assert.Equal(t, c.UpdatedAt, again.UpdatedAt, "a second boot writes nothing")

	moved := cfg
	moved.HTTP.BaseURL = "http://127.0.0.1:5150"
	require.NoError(t, firstBoot(ctx, moved, svcs, testLogger(), false))
	kept, err := svcs.Clients.System(ctx, sys.ID, p.ID)
	require.NoError(t, err)
	assert.Equal(t, c.RedirectURIs, kept.RedirectURIs, "an operator command's boot never rewrites the console client")

	log, buf := bufferLogger()
	require.NoError(t, firstBoot(ctx, moved, svcs, log, true))
	after, err := svcs.Clients.System(ctx, sys.ID, p.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"http://127.0.0.1:5150/oauth/callback"}, after.RedirectURIs)
	assert.Equal(t, []string{"http://127.0.0.1:5150/console/"}, after.PostLogoutRedirectURIs)
	assert.Equal(t, c.Name, after.Name)
	assert.Contains(t, buf.String(), "console client")
	assert.NotContains(t, buf.String(), "upstreams.google differs")

	drift := moved
	drift.Upstreams.Google.ClientID = "another-client"
	log, buf = bufferLogger()
	require.NoError(t, firstBoot(ctx, drift, svcs, log, true))
	assert.Contains(t, buf.String(), "upstreams.google differs from /p/system's Google upstream IdP")
	assert.NotContains(t, buf.String(), "another-client")
	assert.NotContains(t, buf.String(), "google-secret")

	bad := moved
	bad.HTTP.BaseURL = "http://othra.example"
	require.Error(t, firstBoot(ctx, bad, svcs, testLogger(), true), "a public URL the redirect rules refuse stops boot")
}

// With passwords off, env genesis takes only the email: the system org waits for its owner to sign in through /p/system.
func TestFirstBoot_EmailOnlyGenesis(t *testing.T) {
	off := false
	cfg := devLocalConfig()
	cfg.Auth.Password.Enabled = &off
	cfg.Auth.Genesis = config.GenesisConfig{Email: "root@example.com"}
	svcs, cfg := migratedServices(t, cfg)
	svcs.Caps.EnvGenesis = true
	ctx := t.Context()
	log, buf := bufferLogger()
	require.NoError(t, firstBoot(ctx, cfg, svcs, log, false))
	assert.Contains(t, buf.String(), "invited the first owner")

	sys, p, err := consoleProject(ctx, svcs)
	require.NoError(t, err)
	require.NotNil(t, p)
	owners, err := svcs.Members.CountOwners(ctx, sys.ID)
	require.NoError(t, err)
	assert.Zero(t, owners)
	pending, err := svcs.Invites.PendingOwner(ctx, sys.ID)
	require.NoError(t, err)
	assert.True(t, pending)
	required, err := installRequired(ctx, svcs)
	require.NoError(t, err)
	assert.False(t, required.Load())
	_, err = svcs.Users.ByEmail(ctx, "root@example.com")
	assert.Error(t, err, "no console user until the invite is claimed")

	require.NoError(t, firstBoot(ctx, cfg, svcs, testLogger(), false), "a second boot neither re-invites nor fails")
}

func TestConsoleProject_BeforeInstall(t *testing.T) {
	svcs, cfg := migratedServices(t, devLocalConfig())
	sys, p, err := consoleProject(t.Context(), svcs)
	require.NoError(t, err)
	assert.Nil(t, sys)
	assert.Nil(t, p)
	require.NoError(t, reconcileConsole(t.Context(), cfg.HTTP, svcs, testLogger()))
	cfg.Upstreams.Google.ClientID = "x"
	require.NoError(t, logUpstreamDrift(t.Context(), cfg.Upstreams, svcs, testLogger()))
}
