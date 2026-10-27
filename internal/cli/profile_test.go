package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altalune-id/othra/config"
	"github.com/altalune-id/othra/internal/apiclient"
)

func profileRoot(t *testing.T, cfg *config.Config, args ...string) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "x", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().String("url", "", "")
	root.PersistentFlags().String("token", "", "")
	root.PersistentFlags().String("token-file", "", "")
	root.PersistentFlags().String("profile", "", "")
	leaf := &cobra.Command{Use: "y", RunE: func(*cobra.Command, []string) error { return nil }}
	root.AddCommand(leaf)
	root.SetArgs(args)
	ctx := context.Background()
	if cfg != nil {
		ctx = context.WithValue(ctx, ctxKeyRuntime, &runtime{cfg: cfg})
	}
	require.NoError(t, root.ExecuteContext(ctx))
	return leaf
}

func clearCLIEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"OTHRA_URL", "OTHRA_TOKEN", "OTHRA_PROFILE"} {
		t.Setenv(k, "")
	}
}

func TestProfileName(t *testing.T) {
	clearCLIEnv(t)
	assert.Equal(t, "default", profileName(profileRoot(t, nil, "y")))
	t.Setenv("OTHRA_PROFILE", "env")
	assert.Equal(t, "env", profileName(profileRoot(t, nil, "y")))
	assert.Equal(t, "flag", profileName(profileRoot(t, nil, "y", "--profile", "flag")))
}

func TestResolveURL_Order(t *testing.T) {
	cfg := &config.Config{HTTP: config.HTTPConfig{BaseURL: "http://cfg"}}
	p := &profile{URL: "http://profile"}
	tests := []struct {
		name string
		env  string
		args []string
		cfg  *config.Config
		p    *profile
		want string
	}{
		{name: "flag wins", env: "http://env", args: []string{"--url", "http://flag"}, cfg: cfg, p: p, want: "http://flag"},
		{name: "env beats profile", env: "http://env", cfg: cfg, p: p, want: "http://env"},
		{name: "profile beats config", cfg: cfg, p: p, want: "http://profile"},
		{name: "config without profile", cfg: cfg, want: "http://cfg"},
		{name: "default", want: DefaultURL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearCLIEnv(t)
			t.Setenv("OTHRA_URL", tt.env)
			cmd := profileRoot(t, tt.cfg, append([]string{"y"}, tt.args...)...)
			assert.Equal(t, tt.want, resolveURL(cmd, tt.p))
		})
	}
}

func TestResolveCredential(t *testing.T) {
	now := time.Now()
	live := &profile{URL: "http://a", Session: "sess", ExpiresAt: now.Add(time.Hour)}
	tokenFile := filepath.Join(t.TempDir(), "tok")
	require.NoError(t, os.WriteFile(tokenFile, []byte("othra_file\n"), 0o600))

	tests := []struct {
		name string
		env  string
		args []string
		url  string
		p    *profile
		want apiclient.Credential
	}{
		{name: "token flag beats everything", env: "othra_env", args: []string{"--token", "othra_flag", "--token-file", tokenFile}, url: "http://b", p: live, want: apiclient.Credential{Token: "othra_flag"}},
		{name: "token file beats env", env: "othra_env", args: []string{"--token-file", tokenFile}, url: "http://a", p: live, want: apiclient.Credential{Token: "othra_file"}},
		{name: "env token beats profile", env: "othra_env", url: "http://a", p: live, want: apiclient.Credential{Token: "othra_env"}},
		{name: "profile session on its own url", url: "http://a", p: live, want: apiclient.Credential{Session: "sess"}},
		{name: "trailing slash is the same url", url: "http://a/", p: live, want: apiclient.Credential{Session: "sess"}},
		{name: "no profile is anonymous", url: "http://a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearCLIEnv(t)
			t.Setenv("OTHRA_TOKEN", tt.env)
			cmd := profileRoot(t, nil, append([]string{"y"}, tt.args...)...)
			got, err := resolveCredential(cmd, tt.url, tt.p, now)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveCredential_OtherURLIsRefused(t *testing.T) {
	clearCLIEnv(t)
	p := &profile{URL: "http://a", Session: "sess", ExpiresAt: time.Now().Add(time.Hour)}
	got, err := resolveCredential(profileRoot(t, nil, "y", "--profile", "dev"), "http://b", p, time.Now())
	require.True(t, IsProfileHostMismatchError(err), "%v", err)
	assert.Empty(t, got)
	assert.Contains(t, err.Error(), "othra auth login --url http://b --profile dev")
}

func TestResolveCredential_ExpiredIsRefused(t *testing.T) {
	clearCLIEnv(t)
	p := &profile{URL: "http://a", Session: "sess", ExpiresAt: time.Now().Add(-time.Minute)}
	got, err := resolveCredential(profileRoot(t, nil, "y"), "http://a", p, time.Now())
	require.True(t, IsSessionExpiredError(err), "%v", err)
	assert.Empty(t, got)
	assert.Contains(t, err.Error(), "othra auth login")
}
