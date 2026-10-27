package cli_test

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altalune.id/othra/apperror"
	"altalune.id/othra/config"
	"altalune.id/othra/internal/boot"
	"altalune.id/othra/internal/cli"
	"altalune.id/othra/internal/enduser"
	"altalune.id/othra/internal/user"
)

type passwordApp struct {
	fakeApp
	email, password string
	err             error
	closed          bool
}

func (a *passwordApp) SetPassword(_ context.Context, email, password string) error {
	a.email, a.password = email, password
	return a.err
}

func (a *passwordApp) Close(context.Context) error {
	a.closed = true
	return nil
}

func runSetPassword(t *testing.T, app *passwordApp, stdin string, args ...string) (string, int, error) {
	t.Helper()
	setServeEnv(t)
	var opts int
	bootFn := func(_ context.Context, _ config.Config, _ *slog.Logger, o ...boot.Option) (cli.App, error) {
		opts = len(o)
		return app, nil
	}
	buf := new(bytes.Buffer)
	root := cli.NewRootCmd(bootFn, nil)
	root.SetIn(strings.NewReader(stdin))
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(append([]string{"admin", "set-password"}, args...))
	err := root.ExecuteContext(t.Context())
	return buf.String(), opts, err
}

func TestAdminSetPassword_BootsWithoutWorkers(t *testing.T) {
	app := &passwordApp{}
	_, opts, err := runSetPassword(t, app, "a brand new password\n", "--email", "ada@example.com", "--password-stdin")
	require.NoError(t, err)

	assert.Equal(t, "ada@example.com", app.email)
	assert.Equal(t, "a brand new password", app.password)
	assert.True(t, app.closed)
	assert.Equal(t, 3, opts, "install URL, scheduler and consumer are off")
}

func TestAdminSetPassword_AppError(t *testing.T) {
	app := &passwordApp{err: &user.NotFoundError{}}
	_, _, err := runSetPassword(t, app, "a brand new password\n", "--email", "nobody@example.com", "--password-stdin")
	require.Error(t, err)
	assert.True(t, user.IsNotFoundError(err))
	assert.True(t, app.closed)
}

// SECURITY: set-password changes the database in the local config, so a remote flag must never be silently ignored.
func TestAdminSetPassword_RefusesRemoteFlags(t *testing.T) {
	for _, flag := range []string{"url", "token", "token-file", "profile"} {
		t.Run(flag, func(t *testing.T) {
			app := &passwordApp{}
			_, opts, err := runSetPassword(t, app, "a brand new password\n", "--email", "ada@example.com", "--password-stdin", "--"+flag, "x")

			require.Error(t, err)
			assert.Zero(t, opts, "the server never boots")
			assert.Empty(t, app.email)
			assert.Equal(t, cli.ExitInvalidArg, cli.ExitCodeFor(err))
			assert.Contains(t, err.Error(), "admin set-password runs on the server with database access")
			assert.Contains(t, err.Error(), "--"+flag)
		})
	}
}

func TestAdminSetPassword_RefusedWithPasswordsOff(t *testing.T) {
	t.Setenv("OTHRA_AUTH_PASSWORD_ENABLED", "false")
	app := &passwordApp{}
	_, opts, err := runSetPassword(t, app, "a brand new password\n", "--email", "ada@example.com", "--password-stdin")
	require.Error(t, err)
	assert.Equal(t, cli.ExitInvalidArg, cli.ExitCodeFor(err))
	assert.Contains(t, err.Error(), "sign-in-link")
	ae, ok := apperror.AsAppError(err)
	require.True(t, ok)
	assert.Equal(t, "password sign-in is off on this server (auth.password.enabled); use `othra admin sign-in-link`", ae.Message(),
		"said in words, never as a field error about \"password\"")
	assert.Zero(t, opts, "the server never boots")
	assert.Empty(t, app.email)
}

func TestUserGroupIsGone(t *testing.T) {
	setServeEnv(t)
	bootFn := func(context.Context, config.Config, *slog.Logger, ...boot.Option) (cli.App, error) {
		return fakeApp{}, nil
	}
	root := cli.NewRootCmd(bootFn, nil)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"user", "set-password", "--email", "ada@example.com"})
	assert.Error(t, root.ExecuteContext(t.Context()), "set-password moved to `othra admin`, with no alias")
}

type linkApp struct {
	fakeApp
	email string
	link  enduser.IssuedLink
}

func (a *linkApp) SignInLink(_ context.Context, email string) (enduser.IssuedLink, error) {
	a.email = email
	return a.link, nil
}

func runSignInLink(t *testing.T, app *linkApp, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	setServeEnv(t)
	bootFn := func(context.Context, config.Config, *slog.Logger, ...boot.Option) (cli.App, error) { return app, nil }
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)
	root := cli.NewRootCmd(bootFn, nil)
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs(append([]string{"admin", "sign-in-link"}, args...))
	err = root.ExecuteContext(t.Context())
	return out.String(), errOut.String(), err
}

// SECURITY: the printed link is a bearer secret: stdout carries only the URL, the warning goes to stderr, and nothing is mailed.
func TestAdminSignInLink(t *testing.T) {
	expires := time.Date(2026, 10, 27, 9, 15, 0, 0, time.UTC)
	app := &linkApp{link: enduser.IssuedLink{URL: "https://othra.example.com/p/system/login/verify?t=tok", Email: "ada@example.com", ExpiresAt: expires}}
	out, errOut, err := runSignInLink(t, app, "ada@example.com", "--output", "text")
	require.NoError(t, err)
	assert.Equal(t, "ada@example.com", app.email)
	assert.Equal(t, app.link.URL+"\n", out)
	assert.Contains(t, errOut, "works once, until 2026-10-27T09:15:00Z")

	out, _, err = runSignInLink(t, app, "ada@example.com", "--output", "json")
	require.NoError(t, err)
	assert.JSONEq(t, `{"url":"https://othra.example.com/p/system/login/verify?t=tok","email":"ada@example.com","expires_at":"2026-10-27T09:15:00Z"}`, out)
}

// The link is built from http.baseURL, so without it the operator gets a fix, not a localhost URL.
func TestAdminSignInLink_NeedsBaseURL(t *testing.T) {
	app := &linkApp{}
	setServeEnv(t)
	t.Setenv("OTHRA_HTTP_BASE_URL", "")
	require.NoError(t, os.Unsetenv("OTHRA_HTTP_BASE_URL"))
	t.Setenv("OTHRA_MAIL_DRIVER", "console")
	t.Setenv("OTHRA_ENV", "development")
	bootFn := func(context.Context, config.Config, *slog.Logger, ...boot.Option) (cli.App, error) { return app, nil }
	root := cli.NewRootCmd(bootFn, nil)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"admin", "sign-in-link", "ada@example.com"})
	err := root.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "set http.baseURL to the server's public URL")
	assert.Empty(t, app.email, "nothing is issued")
}
